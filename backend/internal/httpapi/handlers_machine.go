package httpapi

import (
	"cnc-manager/internal/domain"
	"cnc-manager/internal/repo"
	"net/http"
)

func (s *Server) handleDrawingMachines(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	items, err := s.repo.DrawingMachines(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateDrawingMachine(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.MachineInput
	if !decodeJSON(w, r, &in) {
		return
	}
	machine, err := s.repo.CreateDrawingMachine(r.Context(), id, in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, machine)
}

func (s *Server) handleListMachines(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Has("ownerUserId") {
		id, ok := ownerQueryID(w, r)
		if !ok {
			return
		}
		r = r.WithContext(repo.WithOwnerFilter(r.Context(), id))
	}
	f := domain.MachineFilter{Keyword: r.URL.Query().Get("q"), Page: queryInt(r, "page", 1), Size: queryInt(r, "size", 20)}
	if r.URL.Query().Has("enabled") {
		value := r.URL.Query().Get("enabled")
		if value != "true" && value != "false" {
			s.fail(w, domain.ErrInvalid)
			return
		}
		enabled := value == "true"
		f.Enabled = &enabled
	}
	items, err := s.repo.ListMachines(r.Context(), f)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
func (s *Server) handleGetMachine(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	m, err := s.repo.GetMachine(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}
func (s *Server) handleCreateMachine(w http.ResponseWriter, r *http.Request) {
	var in domain.MachineInput
	if !decodeJSON(w, r, &in) {
		return
	}
	m, err := s.repo.CreateMachine(r.Context(), in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
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
	m, err := s.repo.UpdateMachine(r.Context(), id, in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}
func (s *Server) handleSetMachineEnabled(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		Enabled *bool `json:"enabled"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.Enabled == nil {
		s.fail(w, domain.ErrInvalid)
		return
	}
	m, err := s.repo.SetMachineEnabled(r.Context(), id, *in.Enabled)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}
func (s *Server) handleMachineDrawings(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	items, err := s.repo.MachineDrawings(r.Context(), id, r.URL.Query().Get("q"), queryInt(r, "page", 1), queryInt(r, "size", 20))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
