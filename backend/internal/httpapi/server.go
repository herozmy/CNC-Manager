// Package httpapi 提供路由、参数校验与 HTTP 响应。
package httpapi

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"cnc-manager/internal/ncstore"
	"cnc-manager/internal/repo"
)

// Options 是构造 HTTP 服务所需的依赖。
type Options struct {
	Version         string
	Repo            *repo.Repo
	Files           *ncstore.Store
	Log             *slog.Logger
	SessionLifetime time.Duration
	Static          http.Handler // 内嵌前端；nil 时非 API 路径返回 404
}

// Server 持有依赖与登录限速状态。
type Server struct {
	version         string
	repo            *repo.Repo
	files           *ncstore.Store
	log             *slog.Logger
	sessionLifetime time.Duration

	loginMu       sync.Mutex
	loginAttempts map[string]loginAttempt
}

type loginAttempt struct {
	failures     int
	blockedUntil time.Time
}

// New 构造根处理器。
func New(o Options) http.Handler {
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if o.SessionLifetime <= 0 {
		o.SessionLifetime = 7 * 24 * time.Hour
	}
	s := &Server{version: o.Version, repo: o.Repo, files: o.Files, log: o.Log,
		sessionLifetime: o.SessionLifetime, loginAttempts: map[string]loginAttempt{}}

	r := chi.NewRouter()
	r.Use(s.recoverer, s.accessLog)
	r.Route("/api", func(api chi.Router) {
		api.Use(s.sameOrigin)
		api.NotFound(func(w http.ResponseWriter, _ *http.Request) { writeError(w, http.StatusNotFound, "接口不存在") })
		api.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		})

		api.Get("/meta", s.handleMeta)
		api.Get("/auth/status", s.handleAuthStatus)
		api.Post("/auth/setup", s.handleAuthSetup)
		api.Post("/auth/login", s.handleLogin)

		api.Group(func(p chi.Router) {
			p.Use(s.requireAuth)
			p.Get("/auth/me", s.handleMe)
			p.Post("/auth/logout", s.handleLogout)

			p.Get("/drawings", s.handleListDrawings)
			p.Post("/drawings", s.handleCreateDrawing)
			p.Get("/drawings/{id}/detail", s.handleDrawingDetail)
			p.Put("/drawings/{id}", s.handleUpdateDrawing)
			p.Delete("/drawings/{id}", s.handleDeleteDrawing)
			p.Post("/drawings/{id}/operations", s.handleCreateOperation)
			p.Put("/drawings/{id}/operations/order", s.handleReorderOperations)

			p.Put("/operations/{id}", s.handleUpdateOperation)
			p.Delete("/operations/{id}", s.handleDeleteOperation)
			p.Post("/operations/{id}/programs", s.handleCreateProgram)
			p.Put("/operations/{id}/programs/order", s.handleReorderPrograms)

			p.Get("/programs/{id}", s.handleGetProgram)
			p.Put("/programs/{id}", s.handleUpdateProgram)
			p.Delete("/programs/{id}", s.handleDeleteProgram)
			p.Get("/programs/{id}/versions", s.handleListVersions)
			p.Post("/programs/{id}/versions", s.handleUploadVersion)
			p.Post("/programs/{id}/versions/content", s.handleSaveContentAsNew)
			p.Put("/programs/{id}/current-version", s.handleSetCurrentVersion)

			p.Get("/versions/{id}/download", s.handleDownloadVersion)
			p.Get("/versions/{id}/content", s.handleGetVersionContent)
			p.Put("/versions/{id}/content", s.handleOverwriteContent)
			p.Get("/versions/{id}/diff", s.handleDiffVersions)

			p.Post("/nc/parse", s.handleParseText)
		})
	})
	if o.Static != nil {
		r.NotFound(o.Static.ServeHTTP)
	}
	return trimTrailingSlash(r)
}

func (s *Server) handleMeta(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": s.version})
}
