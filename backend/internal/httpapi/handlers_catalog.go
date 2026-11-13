package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"cnc-manager/internal/domain"
)

func queryInt(r *http.Request, name string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get(name)))
	if err != nil {
		return def
	}
	return n
}

// ---- 图纸 ----

func (s *Server) handleListDrawings(w http.ResponseWriter, r *http.Request) {
	page, err := s.repo.ListDrawings(r.Context(), r.URL.Query().Get("q"), queryInt(r, "page", 1), queryInt(r, "size", 50))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleCreateDrawing(w http.ResponseWriter, r *http.Request) {
	var in domain.DrawingInput
	if !decodeJSON(w, r, &in) {
		return
	}
	d, err := s.repo.CreateDrawing(r.Context(), in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, d)
}

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

func (s *Server) handleUpdateDrawing(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.DrawingInput
	if !decodeJSON(w, r, &in) {
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

// ---- 工序 ----

func (s *Server) handleCreateOperation(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.OperationInput
	if !decodeJSON(w, r, &in) {
		return
	}
	o, err := s.repo.CreateOperation(r.Context(), id, in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, o)
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
	o, err := s.repo.UpdateOperation(r.Context(), id, in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, o)
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

func (s *Server) handleReorderOperations(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.ReorderInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := s.repo.ReorderOperations(r.Context(), id, in.IDs); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- 程序 ----

func (s *Server) handleCreateProgram(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.ProgramInput
	if !decodeJSON(w, r, &in) {
		return
	}
	p, err := s.repo.CreateProgram(r.Context(), id, in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleGetProgram(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	p, err := s.repo.GetProgram(r.Context(), id)
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

func (s *Server) handleReorderPrograms(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.ReorderInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := s.repo.ReorderPrograms(r.Context(), id, in.IDs); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
