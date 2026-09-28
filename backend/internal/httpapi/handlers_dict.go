package httpapi

import (
	"net/http"

	"cnccool/internal/domain"
)

// handleTree 返回「图纸 → 工序 → 程序」三级树，供前端左侧导航一次性加载。
func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	items, err := s.repo.Tree(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// ---------------------------------------------------------------------------
// 刀具字典
// ---------------------------------------------------------------------------

func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	items, err := s.repo.ListTools(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateTool(w http.ResponseWriter, r *http.Request) {
	var in domain.ToolInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := validateTool(&in); err != nil {
		s.fail(w, err)
		return
	}
	t, err := s.repo.CreateTool(r.Context(), in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleUpdateTool(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.ToolInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := validateTool(&in); err != nil {
		s.fail(w, err)
		return
	}
	t, err := s.repo.UpdateTool(r.Context(), id, in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleDeleteTool(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.repo.DeleteTool(r.Context(), id); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// 机台字典
// ---------------------------------------------------------------------------

func (s *Server) handleListMachines(w http.ResponseWriter, r *http.Request) {
	items, err := s.repo.ListMachines(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateMachine(w http.ResponseWriter, r *http.Request) {
	var in domain.MachineInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := validateMachine(&in); err != nil {
		s.fail(w, err)
		return
	}
	m, err := s.repo.CreateMachine(r.Context(), in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleUpdateMachine(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.MachineInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := validateMachine(&in); err != nil {
		s.fail(w, err)
		return
	}
	m, err := s.repo.UpdateMachine(r.Context(), id, in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleDeleteMachine(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.repo.DeleteMachine(r.Context(), id); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
