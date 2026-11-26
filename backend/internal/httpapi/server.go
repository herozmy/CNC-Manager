// Package httpapi 提供路由、参数校验与 HTTP 响应。
package httpapi

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"cnc-manager/internal/backup"
	"cnc-manager/internal/config"
	"cnc-manager/internal/domain"
	"cnc-manager/internal/ncstore"
	"cnc-manager/internal/repo"
)

// Options 是构造 HTTP 服务所需的依赖。
type Options struct {
	DataDir         string
	Version         string
	Repo            *repo.Repo
	Files           *ncstore.Store
	Log             *slog.Logger
	SessionLifetime time.Duration
	Static          http.Handler // 内嵌前端；nil 时非 API 路径返回 404
}

// Server 持有依赖与登录限速状态。
type Server struct {
	http.Handler
	backup          *backup.Manager
	maintenance     sync.RWMutex
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
func New(o Options) *Server {
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if o.SessionLifetime <= 0 {
		o.SessionLifetime = 7 * 24 * time.Hour
	}
	s := &Server{version: o.Version, repo: o.Repo, files: o.Files, log: o.Log,
		sessionLifetime: o.SessionLifetime, loginAttempts: map[string]loginAttempt{}}
	if o.DataDir != "" {
		s.backup = &backup.Manager{Repo: o.Repo, Files: o.Files, Layout: config.NewLayout(o.DataDir), Version: o.Version}
	}

	r := chi.NewRouter()
	r.Use(s.recoverer, s.accessLog)
	r.Route("/api", func(api chi.Router) {
		api.Use(s.sameOrigin, s.guardMaintenance)
		api.NotFound(func(w http.ResponseWriter, _ *http.Request) { writeError(w, http.StatusNotFound, "接口不存在") })
		api.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		})

		api.Get("/meta", s.handleMeta)
		api.Get("/auth/status", s.handleAuthStatus)
		api.Post("/auth/setup", s.handleAuthSetup)
		api.Post("/auth/login", s.handleLogin)

		api.Group(func(p chi.Router) {
			p.Use(s.requireAuth, s.auditFailedRequests)
			p.Get("/auth/me", s.handleMe)
			p.Put("/auth/me", s.handleUpdateMe)
			p.Put("/auth/password", s.handleChangePassword)
			p.Post("/auth/logout", s.handleLogout)
			p.Route("/admin", func(a chi.Router) {
				a.Use(s.requireAdmin)
				a.Get("/update", s.handleSelfUpdate)
				a.Post("/update", s.handleSelfUpdate)
				a.Get("/users", s.handleListUsers)
				a.Post("/users", s.handleCreateUser)
				a.Put("/users/{id}", s.handleUpdateUser)
				a.Post("/users/{id}/reset-password", s.handleResetUserPassword)
				a.Get("/audit-logs", s.handleAuditLogs)
				a.Put("/drawings/{id}/owner", s.handleAssignDrawingOwner)
			})
			p.Route("/backup", func(b chi.Router) {
				b.Use(s.requireBackupAdmin)
				b.Get("/export", s.handleBackupExport)
				b.Get("/schedule", s.handleGetSchedule)
				b.Put("/schedule", s.handleSetSchedule)
				b.Post("/create", s.handleBackupCreate)
				b.Post("/inspect", s.handleBackupInspect)
				b.Post("/import", s.handleBackupImport)
				b.Get("/recoveries", s.handleBackupRecoveries)
				b.Get("/recoveries/{name}", s.handleBackupRecoveryDownload)
				b.Delete("/recoveries/{name}", s.handleBackupRecoveryDelete)
			})

			p.Get("/machines", s.handleListMachines)
			p.Get("/machines/{id}", s.handleGetMachine)
			p.Get("/machines/{id}/drawings", s.handleMachineDrawings)
			p.With(s.requirePermission(domain.PermissionCreate)).Post("/machines", s.handleCreateMachine)
			p.With(s.requirePermission(domain.PermissionEdit)).Put("/machines/{id}", s.handleUpdateMachine)
			p.With(s.requirePermission(domain.PermissionEdit)).Put("/machines/{id}/enabled", s.handleSetMachineEnabled)
			p.With(s.requirePermission(domain.PermissionDelete)).Delete("/machines/{id}", s.handleDeleteMachine)

			p.Get("/drawings", s.handleListDrawings)
			p.With(s.requirePermission(domain.PermissionCreate)).Post("/drawings", s.handleCreateDrawing)
			p.Get("/drawings/{id}/detail", s.handleDrawingDetail)
			p.Get("/drawings/{id}/machines", s.handleDrawingMachines)
			p.With(s.requirePermission(domain.PermissionCreate)).Post("/drawings/{id}/machines", s.handleCreateDrawingMachine)
			p.With(s.requirePermission(domain.PermissionEdit)).Put("/drawings/{id}", s.handleUpdateDrawing)
			p.With(s.requirePermission(domain.PermissionDelete)).Delete("/drawings/{id}", s.handleDeleteDrawing)
			p.With(s.requirePermission(domain.PermissionCreate)).Post("/drawings/{id}/operations", s.handleCreateOperation)
			p.With(s.requirePermission(domain.PermissionEdit)).Put("/drawings/{id}/operations/order", s.handleReorderOperations)

			p.Get("/operations/{id}/photos", s.listOperationPhotos)
			p.Get("/operations/{id}/photos/{digest}/annotations", s.getAnnotations)
			p.With(s.requirePermission(domain.PermissionEdit)).Put("/operations/{id}/photos/{digest}/annotations", s.saveAnnotations)
			p.Get("/operations/{id}/photos/{digest}", s.getOperationPhoto)
			p.With(s.requirePermission(domain.PermissionEdit)).Post("/operations/{id}/photos", s.addOperationPhoto)
			p.With(s.requirePermission(domain.PermissionEdit)).Delete("/operations/{id}/photos/{digest}", s.deleteOperationPhoto)
			p.With(s.requirePermission(domain.PermissionEdit)).Put("/operations/{id}", s.handleUpdateOperation)
			p.With(s.requirePermission(domain.PermissionDelete)).Delete("/operations/{id}", s.handleDeleteOperation)
			p.With(s.requirePermission(domain.PermissionCreate)).Post("/operations/{id}/programs", s.handleCreateProgram)
			p.With(s.requirePermission(domain.PermissionEdit)).Put("/operations/{id}/programs/order", s.handleReorderPrograms)

			p.Get("/programs/{id}", s.handleGetProgram)
			p.With(s.requirePermission(domain.PermissionEdit)).Put("/programs/{id}", s.handleUpdateProgram)
			p.With(s.requirePermission(domain.PermissionDelete)).Delete("/programs/{id}", s.handleDeleteProgram)
			p.Get("/programs/{id}/versions", s.handleListVersions)
			p.With(s.requirePermission(domain.PermissionEdit)).Post("/programs/{id}/versions", s.handleUploadVersion)
			p.With(s.requirePermission(domain.PermissionEdit)).Post("/programs/{id}/versions/content", s.handleSaveContentAsNew)
			p.With(s.requirePermission(domain.PermissionEdit)).Put("/programs/{id}/current-version", s.handleSetCurrentVersion)

			p.With(s.requirePermission(domain.PermissionDownload)).Get("/versions/{id}/download", s.handleDownloadVersion)
			p.Get("/versions/{id}/content", s.handleGetVersionContent)
			p.With(s.requirePermission(domain.PermissionEdit)).Put("/versions/{id}/content", s.handleOverwriteContent)
			p.Get("/versions/{id}/diff", s.handleDiffVersions)
			p.Get("/versions/{id}/tools", s.handleGetVersionTools)
			p.With(s.requirePermission(domain.PermissionEdit)).Put("/versions/{id}/tools", s.handleReplaceVersionTools)
			p.Get("/versions/{id}/tools/recognize", s.handleRecognizeVersionTools)

			p.With(s.requirePermission(domain.PermissionRead)).Post("/nc/parse", s.handleParseText)
		})
	})
	if o.Static != nil {
		r.NotFound(o.Static.ServeHTTP)
	}
	s.Handler = trimTrailingSlash(r)
	return s
}

func (s *Server) handleMeta(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": s.version})
}
