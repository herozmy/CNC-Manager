package httpapi

import (
	"cnc-manager/internal/domain"
	"cnc-manager/internal/ncparse"
	"net/http"
)

func (s *Server) handleGetVersionTools(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	table, err := s.repo.GetVersionTools(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, table)
}

func (s *Server) handleReplaceVersionTools(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.ToolTableInput
	if !decodeJSONLimit(w, r, &in, 8<<20) {
		return
	}
	table, err := s.repo.ReplaceVersionTools(r.Context(), id, in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, table)
}

func (s *Server) handleRecognizeVersionTools(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	f, err := s.repo.GetVersionFile(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	_, text, err := s.readVersionText(f)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ncparse.Parse(text))
}
