package httpapi

import (
	"cnc-manager/internal/domain"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := domain.UserFromContext(r.Context())
		if !u.IsAdmin() {
			s.fail(w, domain.ErrForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) requirePermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			entity := ""
			pattern := chi.RouteContext(r.Context()).RoutePattern()
			switch {
			case containsPath(pattern, "/drawings/"):
				entity = "drawing"
			case containsPath(pattern, "/operations/"):
				entity = "operation"
			case containsPath(pattern, "/programs/"):
				entity = "program"
			case containsPath(pattern, "/versions/"):
				entity = "version"
			}
			var id int64
			if entity != "" {
				var ok bool
				id, ok = pathID(w, r, "id")
				if !ok {
					return
				}
			}
			if err := s.repo.CheckAccess(r.Context(), permission, entity, id); err != nil {
				s.fail(w, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
func containsPath(s, part string) bool { return strings.Contains(s, part) }
func (s *Server) auditFailedRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		if sw.status >= 400 {
			u, _ := domain.UserFromContext(r.Context())
			e := domain.AuditEntry{Action: "request", EntityType: "http", Actor: u.Username, ActorID: &u.ID, ActorRoleName: u.Role, Result: "failed", Detail: r.Method + " " + chi.RouteContext(r.Context()).RoutePattern() + " HTTP " + strconv.Itoa(sw.status)}
			if err := s.repo.RecordEvent(r.Context(), e); err != nil {
				s.log.Error("失败访问日志写入失败", "err", err)
			}
		}
	})
}
func positiveQueryID(w http.ResponseWriter, r *http.Request, key string) (*int64, bool) {
	id, err := strconv.ParseInt(r.URL.Query().Get(key), 10, 64)
	if err != nil || id < 1 {
		writeError(w, 400, "无效的 "+key)
		return nil, false
	}
	return &id, true
}
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	v, err := s.repo.ListUsers(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, v)
}
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var in domain.CreateUserInput
	if !decodeJSON(w, r, &in) {
		return
	}
	v, err := s.repo.CreateUser(r.Context(), in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 201, v)
}
func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.UpdateUserInput
	if !decodeJSON(w, r, &in) {
		return
	}
	v, err := s.repo.UpdateUser(r.Context(), id, in)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, v)
}
func (s *Server) handleResetUserPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in domain.ResetPasswordInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if err := s.repo.ResetUserPassword(r.Context(), id, in.Password); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(204)
}
func (s *Server) handleAssignDrawingOwner(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var in struct {
		OwnerUserID int64 `json:"ownerUserId"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	v, err := s.repo.AssignDrawingOwner(r.Context(), id, in.OwnerUserID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, v)
}
func (s *Server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := domain.AuditFilter{Action: q.Get("action"), Result: q.Get("result"), EntityType: q.Get("entityType"), Page: queryInt(r, "page", 1), Size: queryInt(r, "size", 50)}
	for _, x := range []struct {
		key    string
		target **int64
	}{{"userId", &f.UserID}} {
		if q.Has(x.key) {
			v, ok := positiveQueryID(w, r, x.key)
			if !ok {
				return
			}
			*x.target = v
		}
	}
	for _, x := range []struct {
		key    string
		target *string
	}{{"from", &f.From}, {"to", &f.To}} {
		if q.Get(x.key) != "" {
			v, err := time.Parse(time.RFC3339, q.Get(x.key))
			if err != nil {
				writeError(w, 400, "时间格式无效")
				return
			}
			*x.target = v.UTC().Format(time.RFC3339Nano)
		}
	}
	if f.From != "" && f.To != "" && f.From > f.To {
		writeError(w, 400, "起始时间不能晚于结束时间")
		return
	}
	v, err := s.repo.ListAuditLogs(r.Context(), f)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, v)
}
