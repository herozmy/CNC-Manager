package httpapi

import (
	"bytes"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"cnc-manager/internal/diff"
	"cnc-manager/internal/domain"
	"cnc-manager/internal/ncparse"
	"cnc-manager/internal/ncstore"
)

// multipartOverhead 是 multipart 封装本身的余量，上传上限 = 文件上限 + 余量。
const multipartOverhead = 1 << 20

// time0 让 ServeContent 不输出 Last-Modified（版本内容由 ID 决定，不依赖文件时间）。
var time0 time.Time

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	items, err := s.repo.ListVersions(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// handleUploadVersion 接收 multipart 表单：file（必填）、changeNote（可选）。
func (s *Server) handleUploadVersion(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.files.MaxBytes()+multipartOverhead)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			s.fail(w, fmt.Errorf("%w: 文件超过上限", ncstore.ErrTooLarge))
			return
		}
		writeError(w, http.StatusBadRequest, "上传格式错误，需要 multipart 表单")
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "缺少 file 字段")
		return
	}
	defer file.Close()
	if _, err := s.repo.GetProgram(r.Context(), id); err != nil {
		s.fail(w, err)
		return
	}
	saved, err := s.files.Save(file)
	if err != nil {
		s.fail(w, err)
		return
	}
	v, err := s.repo.AddVersion(r.Context(), id, blobOf(saved), ncstore.SanitizeName(header.Filename), r.FormValue("changeNote"))
	if err != nil {
		s.fail(w, err)
		return
	}
	res := domain.UploadVersionResult{Version: *v}
	if raw, err := s.files.ReadAll(saved.RelPath); err == nil {
		if text, err := ncstore.DecodeToUTF8(raw, saved.Encoding); err == nil {
			parsed := ncparse.Parse(text)
			res.Parse = &parsed
		}
	}
	writeJSON(w, http.StatusCreated, res)
}

func blobOf(s *ncstore.Saved) domain.BlobInput {
	return domain.BlobInput{SHA256: s.SHA256, SizeBytes: s.SizeBytes, Encoding: s.Encoding, RelPath: s.RelPath}
}

func (s *Server) handleSetCurrentVersion(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.SetCurrentInput
	if !decodeJSON(w, r, &in) {
		return
	}
	p, err := s.repo.SetCurrentVersion(r.Context(), id, in.VersionID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// readVersionText 读出版本文件并解码为 UTF-8 文本。
func (s *Server) readVersionText(f *domain.VersionFile) (raw []byte, text string, err error) {
	raw, err = s.files.ReadAll(f.RelPath)
	if err != nil {
		return nil, "", err
	}
	text, err = ncstore.DecodeToUTF8(raw, f.Encoding)
	return raw, text, err
}

// handleDownloadVersion 下载版本文件。默认原样返回；?encoding=gbk|utf-8 可转换编码，
// 方便在手机上直接取适合机床读取的格式。
func (s *Server) handleDownloadVersion(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	f, err := s.repo.GetVersionFile(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	raw, text, err := s.readVersionText(f)
	if err != nil {
		s.fail(w, err)
		return
	}
	if want := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("encoding"))); want != "" {
		if want != ncstore.EncodingUTF8 && want != ncstore.EncodingGBK {
			writeError(w, http.StatusBadRequest, "encoding 只支持 utf-8 或 gbk")
			return
		}
		if want != f.Encoding {
			if raw, err = ncstore.EncodeFromUTF8(text, want); err != nil {
				s.fail(w, err)
				return
			}
		}
	}
	name := f.FileName
	if name == "" {
		name = fmt.Sprintf("program_v%d.nc", f.VersionNo)
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, name, time0, bytes.NewReader(raw))
}

func (s *Server) handleGetVersionContent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	f, err := s.repo.GetVersionFile(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	raw, text, err := s.readVersionText(f)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.VersionContent{
		VersionID: f.VersionID, ProgramID: f.ProgramID, VersionNo: f.VersionNo, FileName: f.FileName,
		Encoding: f.Encoding, Content: text, LineCount: len(splitLines(text)), SizeBytes: len(raw), IsCurrent: f.IsCurrent,
	})
}

// saveText 把编辑后的文本写入文件库，返回 blob 与提示。
func (s *Server) saveText(in *domain.ContentInput, fallbackEnc string) (*ncstore.Saved, string, error) {
	if !utf8.ValidString(in.Content) {
		return nil, "", fmt.Errorf("%w: 内容不是合法的 UTF-8 文本", domain.ErrInvalid)
	}
	enc := ncstore.NormalizeEncoding(in.Encoding)
	if strings.TrimSpace(in.Encoding) == "" {
		enc = ncstore.NormalizeEncoding(fallbackEnc)
	}
	saved, err := s.files.SaveText(in.Content, enc)
	if err != nil {
		return nil, "", err
	}
	warning := ""
	if len(saved.Unrepresentable) > 0 {
		warning = fmt.Sprintf("以下字符无法用 %s 编码，已被替换：%s", strings.ToUpper(enc), strings.Join(saved.Unrepresentable, " "))
	}
	return saved, warning, nil
}

// handleSaveContentAsNew 把编辑后的文本保存为新版本。
func (s *Server) handleSaveContentAsNew(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.ContentInput
	if !decodeJSON(w, r, &in) {
		return
	}
	p, err := s.repo.GetProgram(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	fallbackEnc, name := ncstore.EncodingUTF8, strings.TrimSpace(in.FileName)
	if p.CurrentVersionID != nil {
		if f, err := s.repo.GetVersionFile(r.Context(), *p.CurrentVersionID); err == nil {
			fallbackEnc = f.Encoding
			if name == "" {
				name = f.FileName
			}
		}
	}
	if name == "" {
		name = p.ProgramNo + ".nc"
	}
	saved, warning, err := s.saveText(&in, fallbackEnc)
	if err != nil {
		s.fail(w, err)
		return
	}
	v, err := s.repo.AddVersion(r.Context(), id, blobOf(saved), ncstore.SanitizeName(name), in.ChangeNote)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, domain.VersionSaveResult{Version: *v, Warning: warning})
}

// handleOverwriteContent 用编辑后的文本覆盖指定版本。
func (s *Server) handleOverwriteContent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.ContentInput
	if !decodeJSON(w, r, &in) {
		return
	}
	f, err := s.repo.GetVersionFile(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	saved, warning, err := s.saveText(&in, f.Encoding)
	if err != nil {
		s.fail(w, err)
		return
	}
	v, err := s.repo.OverwriteVersion(r.Context(), id, blobOf(saved))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.VersionSaveResult{Version: *v, Warning: warning})
}

// handleDiffVersions 比较两个版本：GET /versions/{id}/diff?with={otherId}，id 为左侧（旧），with 为右侧（新）。
func (s *Server) handleDiffVersions(w http.ResponseWriter, r *http.Request) {
	leftID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	rightID := int64(queryInt(r, "with", 0))
	if rightID <= 0 {
		writeError(w, http.StatusBadRequest, "缺少或非法的 with 参数")
		return
	}
	lf, err := s.repo.GetVersionFile(r.Context(), leftID)
	if err != nil {
		s.fail(w, err)
		return
	}
	rf, err := s.repo.GetVersionFile(r.Context(), rightID)
	if err != nil {
		s.fail(w, err)
		return
	}
	if lf.ProgramID != rf.ProgramID {
		writeError(w, http.StatusBadRequest, "只能比较同一个程序的两个版本")
		return
	}
	_, lt, err := s.readVersionText(lf)
	if err != nil {
		s.fail(w, err)
		return
	}
	_, rt, err := s.readVersionText(rf)
	if err != nil {
		s.fail(w, err)
		return
	}
	left, right := splitLines(lt), splitLines(rt)
	res := diff.Compare(left, right)
	writeJSON(w, http.StatusOK, domain.DiffResult{
		LeftVersionNo: lf.VersionNo, RightVersionNo: rf.VersionNo,
		Identical: diff.Identical(left, right), Truncated: res.Truncated, Lines: res.Lines,
	})
}

func (s *Server) handleParseText(w http.ResponseWriter, r *http.Request) {
	var in domain.ParseTextInput
	if !decodeJSON(w, r, &in) {
		return
	}
	writeJSON(w, http.StatusOK, ncparse.Parse(in.Content))
}

// splitLines 按行拆分，统一换行符，去掉末尾换行产生的空行。
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
