package httpapi

import (
	"cnc-manager/internal/domain"
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
