package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cnccool/internal/diff"
	"cnccool/internal/domain"
	"cnccool/internal/ncstore"
)

// maxDiffBytes 是版本对比时单个文件最多读取的字节数。
// 超过就截断——防止有人上传一个几百 MB 的日志文件把内存吃光。
const maxDiffBytes = 8 << 20

// ---------------------------------------------------------------------------
// 程序
// ---------------------------------------------------------------------------

func (s *Server) handleListPrograms(w http.ResponseWriter, r *http.Request) {
	operationID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	// 父级不存在时给明确的 404，而不是空数组（理由见 handleListOperations）
	if _, err := s.repo.GetOperation(r.Context(), operationID); err != nil {
		s.fail(w, err)
		return
	}
	items, err := s.repo.ListPrograms(r.Context(), operationID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleGetProgram(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	detail, err := s.programDetail(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// programDetail 组装程序详情：基本信息 + 所属图纸/工序 + 刀具表 + 版本列表。
//
// 做成一次性返回，是为了让前端点一个树节点只发一个请求界面就完整，
// 而不是先拿基本信息、再拿刀具、再拿版本（三个来回，界面会一段一段地跳）。
func (s *Server) programDetail(ctx context.Context, id int64) (*domain.ProgramDetail, error) {
	p, err := s.repo.GetProgram(ctx, id)
	if err != nil {
		return nil, err
	}
	op, err := s.repo.GetOperation(ctx, p.OperationID)
	if err != nil {
		return nil, err
	}
	d, err := s.repo.GetDrawing(ctx, op.DrawingID)
	if err != nil {
		return nil, err
	}
	tools, err := s.repo.ListProgramTools(ctx, id)
	if err != nil {
		return nil, err
	}
	versions, err := s.repo.ListVersions(ctx, id)
	if err != nil {
		return nil, err
	}
	return &domain.ProgramDetail{
		Program: *p, Drawing: *d, Operation: *op, Tools: tools, Versions: versions,
	}, nil
}

func (s *Server) handleCreateProgram(w http.ResponseWriter, r *http.Request) {
	operationID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.ProgramInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := validateProgram(&in); err != nil {
		s.fail(w, err)
		return
	}
	// 先确认工序存在，这样报错信息比外键约束直白得多
	if _, err := s.repo.GetOperation(r.Context(), operationID); err != nil {
		s.fail(w, err)
		return
	}
	p, err := s.repo.CreateProgram(r.Context(), operationID, in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleUpdateProgram(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.ProgramInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := validateProgram(&in); err != nil {
		s.fail(w, err)
		return
	}
	p, err := s.repo.UpdateProgram(r.Context(), id, in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleDeleteProgram(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.repo.DeleteProgram(r.Context(), id); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListLogs(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if _, err := s.repo.GetProgram(r.Context(), id); err != nil {
		s.fail(w, err)
		return
	}
	items, err := s.repo.ListLogs(r.Context(), id, queryInt(r, "limit", 100))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// ---------------------------------------------------------------------------
// 刀具刀补表
// ---------------------------------------------------------------------------

func (s *Server) handleListProgramTools(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if _, err := s.repo.GetProgram(r.Context(), id); err != nil {
		s.fail(w, err)
		return
	}
	items, err := s.repo.ListProgramTools(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleReplaceProgramTools(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Items []domain.ProgramToolInput `json:"items"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Items == nil {
		body.Items = []domain.ProgramToolInput{}
	}
	if err := validateProgramTools(body.Items); err != nil {
		s.fail(w, err)
		return
	}
	items, err := s.repo.ReplaceProgramTools(r.Context(), id, body.Items)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// validateProgramTools 校验刀具表。
//
// 这里挡住的是现场最容易犯的几类错：行号重复、填了负数、
// 把 G96 的线速度当成转速填进去（模式值超范围）。
func validateProgramTools(items []domain.ProgramToolInput) error {
	seen := make(map[int]bool, len(items))
	for i, it := range items {
		seq := it.Seq
		if seq <= 0 {
			seq = i + 1
		}
		if seen[seq] {
			return fmt.Errorf("%w：刀具表第 %d 行的行号重复", domain.ErrInvalid, seq)
		}
		seen[seq] = true

		if it.ToolDia < 0 || it.CornerRadius < 0 || it.Feed < 0 || it.CutDepth < 0 {
			return fmt.Errorf("%w：第 %d 行的直径 / 刀尖圆弧 / 进给 / 切深不能为负数", domain.ErrInvalid, seq)
		}
		if it.CompAmount < 0 {
			return fmt.Errorf("%w：第 %d 行的刀具补偿量不能为负数", domain.ErrInvalid, seq)
		}
		if it.CompAmount > 10000 {
			return fmt.Errorf("%w：第 %d 行的刀具补偿量超出合理范围（0 ~ 10000 mm）", domain.ErrInvalid, seq)
		}
		if it.ToolDia > 10000 || it.CutDepth > 10000 || it.Feed > 100000 {
			return fmt.Errorf("%w：第 %d 行的数值超出合理范围，请检查是否填错了单位", domain.ErrInvalid, seq)
		}
		if it.SpeedMode != 0 && it.SpeedMode != 1 {
			return fmt.Errorf("%w：第 %d 行的速度模式只能是 0（G97 恒转速 r/min）或 1（G96 恒线速 m/min）", domain.ErrInvalid, seq)
		}
		if it.FeedMode != 0 && it.FeedMode != 1 {
			return fmt.Errorf("%w：第 %d 行的进给模式只能是 0（G94 每分钟 mm/min）或 1（G95 每转 mm/r）", domain.ErrInvalid, seq)
		}
		if it.Coolant < 0 || it.Coolant > 3 {
			return fmt.Errorf("%w：第 %d 行的冷却方式取值必须在 0~3 之间", domain.ErrInvalid, seq)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// 版本：上传、下载、对比、切换
// ---------------------------------------------------------------------------

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if _, err := s.repo.GetProgram(r.Context(), id); err != nil {
		s.fail(w, err)
		return
	}
	items, err := s.repo.ListVersions(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleUploadVersion(w http.ResponseWriter, r *http.Request) {
	programID, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	// 限制整个请求体，避免有人传一个超大文件把磁盘塞满
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes()+1<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "解析上传内容失败（可能文件超过大小上限）："+err.Error())
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "没有收到 NC 文件，表单字段名应为 file")
		return
	}
	defer func() { _ = file.Close() }()

	saved, err := s.files.Save(file, header.Filename)
	if err != nil {
		if errors.Is(err, ncstore.ErrTooLarge) || errors.Is(err, ncstore.ErrEmpty) {
			s.fail(w, fmt.Errorf("%w：%s", domain.ErrInvalid, err.Error()))
			return
		}
		s.fail(w, err)
		return
	}

	// 顺手检测一次文件编码。数控程序都很小，多读一次换来编码记录准确，
	// 值得——否则一个 GBK 程序在界面里可能就是乱码。
	// 超大文件跳过，留到第一次读取时再检测并回填。
	encoding := ""
	if saved.SizeBytes <= detectEncodingLimit {
		if raw, rerr := s.files.ReadAll(saved.RelPath); rerr == nil {
			encoding = ncstore.DetectEncoding(raw)
		}
	}

	v, err := s.repo.AddVersion(r.Context(), programID, domain.NCFileInput{
		SHA256:       saved.SHA256,
		SizeBytes:    saved.SizeBytes,
		OriginalName: saved.OriginalName,
		RelPath:      saved.RelPath,
		Encoding:     encoding,
	}, r.FormValue("changeNote"))
	if err != nil {
		// 注意：这里物理文件已经落盘但数据库没记上，会成为"孤立文件"。
		// 这是刻意取舍——宁可留下一个占空间的孤儿文件，也不能因为删文件而误删
		// 其它程序正在引用的同一份内容（内容是按 sha256 共享的）。
		// 物理回收交给后期的"孤立文件清理"后台任务。
		s.log.Error("版本入库失败，可能留下孤立文件", "programId", programID, "sha256", saved.SHA256, "err", err)
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleSetCurrentVersion(w http.ResponseWriter, r *http.Request) {
	programID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		VersionID int64 `json:"versionId"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.VersionID <= 0 {
		writeError(w, http.StatusBadRequest, "缺少 versionId 参数")
		return
	}
	p, err := s.repo.SetCurrentVersion(r.Context(), programID, body.VersionID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleDownloadVersion(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := s.repo.GetVersion(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	rel, err := s.repo.GetVersionRelPath(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}

	f, err := s.files.Open(rel)
	if err != nil {
		// 物理文件丢了是严重问题（比如有人手工删了 NC 库），必须留下明确日志
		s.log.Error("版本对应的物理文件打不开", "versionId", id, "relPath", rel, "err", err)
		writeError(w, http.StatusInternalServerError,
			"程序文件在服务器上找不到了，请检查 NC 文件库目录是否被移动或清理")
		return
	}
	defer func() { _ = f.Close() }()

	// 默认下载；带 ?inline=1 时内联显示纯文本，供前端做版本内容预览
	kind, ctype := "attachment", "application/octet-stream"
	if r.URL.Query().Get("inline") != "" {
		kind, ctype = "inline", "text/plain; charset=utf-8"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", contentDisposition(kind, v.FileName))
	http.ServeContent(w, r, v.FileName, time.Time{}, f)
}

func (s *Server) handleDiffVersions(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	against := queryInt64(r, "against")
	if against <= 0 {
		writeError(w, http.StatusBadRequest, "缺少对比版本参数 against")
		return
	}

	// 约定：against 是旧版，路径里的 id 是当前要看的新版
	left, err := s.repo.GetVersion(r.Context(), against)
	if err != nil {
		s.fail(w, err)
		return
	}
	right, err := s.repo.GetVersion(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	if left.ProgramID != right.ProgramID {
		writeError(w, http.StatusBadRequest, "这两个版本不属于同一个程序，无法对比")
		return
	}
	if left.ID == right.ID {
		writeError(w, http.StatusBadRequest, "不能拿同一个版本和自己对比")
		return
	}

	leftLines, err := s.readVersionLines(r.Context(), left.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	rightLines, err := s.readVersionLines(r.Context(), right.ID)
	if err != nil {
		s.fail(w, err)
		return
	}

	lines := diff.Compare(leftLines, rightLines)
	identical := true
	for _, l := range lines {
		if l.Type != "same" {
			identical = false
			break
		}
	}
	writeJSON(w, http.StatusOK, domain.DiffResult{
		LeftVersionNo:  left.VersionNo,
		RightVersionNo: right.VersionNo,
		Identical:      identical,
		Lines:          lines,
	})
}

// readVersionLines 读取一个版本的程序文本并按行切分。
//
// 关键：必须先按文件自己的编码解码成 UTF-8，再比较。
// 如果直接按原始字节比，两个内容完全相同的版本会因为一个存的是 GBK、
// 另一个存的是 UTF-8 而被判成"有差异"；而且对比界面上显示的中文会是乱码，
// 因为原始 GBK 字节塞进 JSON 时会被转义成 U+FFFD。
func (s *Server) readVersionLines(ctx context.Context, versionID int64) ([]string, error) {
	rel, err := s.repo.GetVersionRelPath(ctx, versionID)
	if err != nil {
		return nil, err
	}
	raw, err := s.files.ReadAll(rel)
	if err != nil {
		return nil, fmt.Errorf("打开版本 %d 的程序文件失败: %w", versionID, err)
	}
	if len(raw) > maxDiffBytes {
		return nil, fmt.Errorf("%w：第 %d 版程序超过 %d MB，不适合做逐行对比",
			domain.ErrInvalid, versionID, maxDiffBytes>>20)
	}

	text, err := ncstore.DecodeToUTF8(raw, ncstore.DetectEncoding(raw))
	if err != nil {
		return nil, fmt.Errorf("解码第 %d 版程序文件失败: %w", versionID, err)
	}

	// 统一换行符：有人用记事本存过程序，混入的 \r 会让每一行都显示成"已修改"
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	// 文件末尾的换行会切出一个空行，去掉它，避免凭空多一行差异
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines, nil
}

// contentDisposition 同时给出 ASCII 回退名和 RFC 5987 的 UTF-8 文件名。
//
// 只给 UTF-8 一段的话，车间里常见的旧浏览器会把文件名显示成乱码；
// 只给 ASCII 的话，中文文件名会变成下划线。两段都给才稳妥。
func contentDisposition(kind, name string) string {
	ascii := make([]rune, 0, len(name))
	for _, c := range name {
		if c < 128 && c != '"' && c != '\\' {
			ascii = append(ascii, c)
		} else {
			ascii = append(ascii, '_')
		}
	}
	if len(ascii) == 0 {
		ascii = []rune("program.nc")
	}
	return fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`,
		kind, string(ascii), url.PathEscape(name))
}
