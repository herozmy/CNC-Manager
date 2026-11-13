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
