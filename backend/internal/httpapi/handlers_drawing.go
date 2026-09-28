package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"cnccool/internal/domain"
)

// nowString 隔离 domain.Now 的调用，方便将来统一改时间来源。
func nowString() string { return domain.Now() }

// ---------------------------------------------------------------------------
// 入参校验
//
// 校验放在 HTTP 层而不是数据库层：这样错误信息是给人看的完整中文句子，
// 而不是 SQLite 抛出来的 "constraint failed"。
// ---------------------------------------------------------------------------

func validateDrawing(in *domain.DrawingInput) error {
	in.DrawingNo = strings.TrimSpace(in.DrawingNo)
	if in.DrawingNo == "" {
		return fmt.Errorf("%w：图纸号不能为空", domain.ErrInvalid)
	}
	if len([]rune(in.DrawingNo)) > 64 {
		return fmt.Errorf("%w：图纸号最多 64 个字符", domain.ErrInvalid)
	}
	return nil
}

func validateOperation(in *domain.OperationInput) error {
	if in.OpNo <= 0 || in.OpNo > 9999 {
		return fmt.Errorf("%w：工序号必须是 1 ~ 9999 之间的整数（建议用 10、20、30 留出插入余量）", domain.ErrInvalid)
	}
	if len([]rune(in.OpName)) > 64 {
		return fmt.Errorf("%w：工序名称最多 64 个字符", domain.ErrInvalid)
	}
	if in.ZHeight < 0 {
		return fmt.Errorf("%w：Z 轴垫高不能为负数", domain.ErrInvalid)
	}
	// 上限拦一下：这个字段最常见的事故是有人把 25 写成 25000（单位理解成微米），
	// 垫高 25 米是不可能的，早报错比让错误数据流到机床前好。
	if in.ZHeight > 10000 {
		return fmt.Errorf("%w：Z 轴垫高超出合理范围（0 ~ 10000 mm），请检查单位是否填错", domain.ErrInvalid)
	}
	return nil
}

func validateProgram(in *domain.ProgramInput) error {
	in.ProgramNo = strings.TrimSpace(in.ProgramNo)
	if in.ProgramNo == "" {
		return fmt.Errorf("%w：程序号不能为空", domain.ErrInvalid)
	}
	if len([]rune(in.ProgramNo)) > 64 {
		return fmt.Errorf("%w：程序号最多 64 个字符", domain.ErrInvalid)
	}
	return nil
}

func validateTool(in *domain.ToolInput) error {
	in.ToolNo = strings.TrimSpace(in.ToolNo)
	if in.ToolNo == "" {
		return fmt.Errorf("%w：刀具号不能为空", domain.ErrInvalid)
	}
	return nil
}

func validateMachine(in *domain.MachineInput) error {
	in.Code = strings.TrimSpace(in.Code)
	if in.Code == "" {
		return fmt.Errorf("%w：机台编号不能为空", domain.ErrInvalid)
	}
	return nil
}

// ---------------------------------------------------------------------------
// 图纸
// ---------------------------------------------------------------------------

func (s *Server) handleListDrawings(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	size := queryInt(r, "size", 20)

	items, total, err := s.repo.ListDrawings(r.Context(), r.URL.Query().Get("q"), page, size)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, domain.Page[domain.Drawing]{
		Items: items, Total: total, Page: page, Size: size,
	})
}

func (s *Server) handleGetDrawing(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	d, err := s.repo.GetDrawing(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// handleDrawingDetail 返回一张图纸的完整内容（图纸 → 工序 → 程序 → 刀具补偿表）。
// 界面主视图只调这一个接口。
func (s *Server) handleDrawingDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	d, err := s.repo.DrawingDetail(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleCreateDrawing(w http.ResponseWriter, r *http.Request) {
	var in domain.DrawingInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := validateDrawing(&in); err != nil {
		s.fail(w, err)
		return
	}
	d, err := s.repo.CreateDrawing(r.Context(), in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleUpdateDrawing(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.DrawingInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := validateDrawing(&in); err != nil {
		s.fail(w, err)
		return
	}
	d, err := s.repo.UpdateDrawing(r.Context(), id, in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleDeleteDrawing(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.repo.DeleteDrawing(r.Context(), id); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// 工序
// ---------------------------------------------------------------------------

func (s *Server) handleListOperations(w http.ResponseWriter, r *http.Request) {
	drawingID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	// 先确认图纸存在。否则"父级不存在"会返回一个看起来正常的空数组，
	// 把前端拿着已删除 id 请求的 bug 掩盖掉。
	if _, err := s.repo.GetDrawing(r.Context(), drawingID); err != nil {
		s.fail(w, err)
		return
	}
	items, err := s.repo.ListOperations(r.Context(), drawingID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateOperation(w http.ResponseWriter, r *http.Request) {
	drawingID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.OperationInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := validateOperation(&in); err != nil {
		s.fail(w, err)
		return
	}
	// 先确认图纸存在，否则外键报错的信息不如这里直白
	if _, err := s.repo.GetDrawing(r.Context(), drawingID); err != nil {
		s.fail(w, err)
		return
	}
	op, err := s.repo.CreateOperation(r.Context(), drawingID, in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, op)
}

func (s *Server) handleUpdateOperation(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.OperationInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := validateOperation(&in); err != nil {
		s.fail(w, err)
		return
	}
	op, err := s.repo.UpdateOperation(r.Context(), id, in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, op)
}

func (s *Server) handleDeleteOperation(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.repo.DeleteOperation(r.Context(), id); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
