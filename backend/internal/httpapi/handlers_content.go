package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"cnccool/internal/domain"
	"cnccool/internal/ncparse"
	"cnccool/internal/ncstore"
)

// maxContentBytes 限制在界面里编辑的程序文本大小。
//
// 超过这个体积就不适合塞进 JSON 来回传了，提示用户改用「上传 NC」。
// 正常数控程序都在几百 KB 以内，4MB 足够宽松。
const maxContentBytes = 4 << 20

// detectEncodingLimit 上传后顺手检测编码的体积上限。
// 超过就不检测了，留到第一次读取时再检测并回填。
const detectEncodingLimit = 8 << 20

// ---------------------------------------------------------------------------
// 查看程序
// ---------------------------------------------------------------------------

// handleGetVersionContent 读取某个版本的程序文本，供界面查看与编辑。
//
// 返回的 content 已经按源文件的编码解码成 UTF-8；
// 同时把 encoding 一起返回，保存时前端要原样带回来，
// 否则会把一个 GBK 程序存成 UTF-8，机床可能直接不认。
func (s *Server) handleGetVersionContent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	v, err := s.repo.GetVersion(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	relPath, storedEncoding, fileName, err := s.repo.GetVersionFileInfo(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}

	raw, err := s.files.ReadAll(relPath)
	if err != nil {
		s.fail(w, err)
		return
	}

	// 用完整内容重新检测：上传是流式写入，那时判断不可靠。
	// 检测结果和库里记的不一致就回填，下次直接准。
	detected := ncstore.DetectEncoding(raw)
	if detected != ncstore.NormalizeEncoding(storedEncoding) {
		if err := s.repo.SetVersionFileEncoding(r.Context(), id, detected); err != nil {
			s.log.Warn("回填文件编码失败", "versionId", id, "err", err)
		}
	}

	text, err := ncstore.DecodeToUTF8(raw, detected)
	if err != nil {
		s.fail(w, err)
		return
	}

	writeJSON(w, http.StatusOK, domain.VersionContent{
		VersionID: v.ID,
		ProgramID: v.ProgramID,
		VersionNo: v.VersionNo,
		FileName:  fileName,
		Encoding:  detected,
		Content:   text,
		LineCount: countLines(text),
		SizeBytes: v.FileSize,
		IsCurrent: v.IsCurrent,
	})
}

// ---------------------------------------------------------------------------
// 保存程序
// ---------------------------------------------------------------------------

// handleSaveVersionContentAsNew 把编辑好的文本存成一个**新版本**。
// 原来的版本原封不动保留，可以回溯、对比、回滚。
func (s *Server) handleSaveVersionContentAsNew(w http.ResponseWriter, r *http.Request) {
	programID, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	var in domain.ContentInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := validateContent(&in); err != nil {
		s.fail(w, err)
		return
	}

	saved, replaced, err := s.saveEditedText(in, s.versionFileNameFor(r.Context(), programID))
	if err != nil {
		s.fail(w, err)
		return
	}

	v, err := s.repo.AddVersion(r.Context(), programID, saved, in.ChangeNote)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.VersionSaveResult{
		Version: *v,
		Warning: encodingWarning(saved.Encoding, replaced),
	})
}

// handleOverwriteVersionContent 用编辑好的文本**覆盖当前版本**的文件内容，版本号不变。
// 适合改个笔误、补一行注释这种不值得单独占一个版本号的修改。
func (s *Server) handleOverwriteVersionContent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	var in domain.ContentInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := validateContent(&in); err != nil {
		s.fail(w, err)
		return
	}

	_, storedEncoding, fileName, err := s.repo.GetVersionFileInfo(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}

	// 前端没带编码就沿用这一版原有的编码，绝不默认成 UTF-8——
	// 那会把一个 GBK 程序悄悄换掉编码。
	if strings.TrimSpace(in.Encoding) == "" {
		in.Encoding = storedEncoding
	}

	saved, replaced, err := s.saveEditedText(in, fileName)
	if err != nil {
		s.fail(w, err)
		return
	}

	v, err := s.repo.OverwriteVersionContent(r.Context(), id, saved, in.ChangeNote)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.VersionSaveResult{
		Version: *v,
		Warning: encodingWarning(saved.Encoding, replaced),
	})
}

// saveEditedText 把 UTF-8 文本按目标编码写进文件库，
// 同时返回目标编码存不下、已被替换的字符列表。
func (s *Server) saveEditedText(in domain.ContentInput, fileName string) (domain.NCFileInput, []string, error) {
	saved, err := s.files.SaveText(in.Content, in.Encoding, fileName)
	if err != nil {
		return domain.NCFileInput{}, nil, err
	}
	return domain.NCFileInput{
		SHA256:       saved.SHA256,
		SizeBytes:    saved.SizeBytes,
		OriginalName: saved.OriginalName,
		RelPath:      saved.RelPath,
		Encoding:     saved.Encoding,
	}, saved.Unrepresentable, nil
}

// encodingWarning 把「有字符存不下」翻译成给现场看的中文提示。
//
// 这类问题肉眼看不出区别（Ø 和 φ 在屏幕上很像），所以提示必须具体到字符，
// 并给出可操作的下一步，否则用户根本不知道哪里出了问题。
func encodingWarning(enc string, chars []string) string {
	if len(chars) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(chars))
	for _, c := range chars {
		quoted = append(quoted, "「"+c+"」")
	}
	return fmt.Sprintf(
		"目标编码 %s 无法表示这些字符：%s，它们已被替换成占位符。"+
			"请改成 GBK 支持的写法（例如用 φ 或 Φ 代替 Ø），或把这一版另存为 UTF-8。",
		strings.ToUpper(enc), strings.Join(quoted, "、"))
}

// versionFileNameFor 取程序当前版本的文件名，用来给"另存为新版本"命名。
// 取不到就退回「程序号.nc」，保证文件名始终有意义。
func (s *Server) versionFileNameFor(ctx context.Context, programID int64) string {
	p, err := s.repo.GetProgram(ctx, programID)
	if err != nil {
		return "program.nc"
	}
	if p.CurrentVersionID != nil {
		if _, _, name, err := s.repo.GetVersionFileInfo(ctx, *p.CurrentVersionID); err == nil && name != "" {
			return name
		}
	}
	return p.ProgramNo + ".nc"
}

func validateContent(in *domain.ContentInput) error {
	if len(in.Content) > maxContentBytes {
		return fmt.Errorf("%w：程序文本超过 %d MB，请改用「上传 NC」的方式更新",
			domain.ErrInvalid, maxContentBytes>>20)
	}
	if strings.TrimSpace(in.Content) == "" {
		return fmt.Errorf("%w：程序内容不能为空", domain.ErrInvalid)
	}
	return nil
}

// ---------------------------------------------------------------------------
// 程序内容识别
// ---------------------------------------------------------------------------

// handleParseText 解析一段 NC 文本，返回识别到的程序号与刀具调用。
//
// 用途：前端可以拿它做「选一个 NC 文件，自动填上程序号」，
// 省掉手敲程序号，也避免手敲时打错。
func (s *Server) handleParseText(w http.ResponseWriter, r *http.Request) {
	var in domain.ParseTextInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.Content) == "" {
		s.fail(w, fmt.Errorf("%w：内容不能为空", domain.ErrInvalid))
		return
	}
	if len(in.Content) > maxContentBytes {
		s.fail(w, fmt.Errorf("%w：内容超过 %d MB，无法解析", domain.ErrInvalid, maxContentBytes>>20))
		return
	}
	res := ncparse.Parse(in.Content)
	writeJSON(w, http.StatusOK, res)
}

// parseStoredProgram 读取文件库里的程序并解析。
//
// 解析失败不影响上传结果——上传本身已经成功了。识别信息只是附加提醒，
// 绝不能因为它让用户以为上传失败。
func (s *Server) parseStoredProgram(relPath string) *domain.ParseResult {
	raw, err := s.files.ReadAll(relPath)
	if err != nil {
		s.log.Warn("读取程序内容失败，跳过自动识别", "relPath", relPath, "err", err)
		return nil
	}
	text, err := ncstore.DecodeToUTF8(raw, ncstore.DetectEncoding(raw))
	if err != nil {
		s.log.Warn("解码程序内容失败，跳过自动识别", "relPath", relPath, "err", err)
		return nil
	}
	res := ncparse.Parse(text)
	return &res
}

// countLines 统计行数，最后一行没有换行符也算一行。
func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}
