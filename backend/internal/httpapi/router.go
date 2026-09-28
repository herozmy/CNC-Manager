// Package httpapi 是 HTTP 接口层：负责路由、参数校验、错误映射和响应编码。
//
// 这里不写任何业务规则，业务规则全在 repo / domain 里；
// 这样将来要做命令行工具、后台任务或换 Web 框架，都不用重写业务逻辑。
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"cnccool/internal/config"
	"cnccool/internal/ncstore"
	"cnccool/internal/repo"
	"cnccool/internal/update"
)

// Server 持有 HTTP 层需要的全部依赖。
type Server struct {
	repo    *repo.Repo
	files   *ncstore.Store
	cfg     *config.Config
	log     *slog.Logger
	version string

	// layout 描述当前程序装成什么形态，决定能不能自更新。
	layout update.Layout
	// update 负责查仓库上有没有新版本。
	update *update.Client
	// restart 是「离线包装好了，准备重启」时要执行的动作，由 main 注入。
	restart func()
}

// NewServer 构造 HTTP 服务。
func NewServer(r *repo.Repo, files *ncstore.Store, cfg *config.Config, log *slog.Logger, version string) *Server {
	return &Server{
		repo:    r,
		files:   files,
		cfg:     cfg,
		log:     log,
		version: version,
		layout:  update.DetectLayout(),
		update:  update.NewClient(cfg.UpdateAPI, cfg.UpdateRepo, version),
	}
}

// SetRestartHook 注册「离线包装好了，准备重启」时要执行的动作。
//
// 用 setter 而不是构造参数：只有 main 需要关心这件事，
// 测试里造 Server 时不必凭空编一个出来。
func (s *Server) SetRestartHook(fn func()) { s.restart = fn }

// Router 组装全部路由。
//
// 路由用扁平写法而不是嵌套 Route：嵌套写法在 chi 里会创建子路由挂载点，
// 尾部斜杠的处理容易出现"有的接口带斜杠能通、有的不通"的迷惑现象。
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(s.recoverer)
	r.Use(s.cors)
	r.Use(s.accessLog)

	r.Route("/api", func(r chi.Router) {
		r.Get("/meta", s.handleMeta)
		r.Get("/tree", s.handleTree)

		// 图纸
		r.Get("/drawings", s.handleListDrawings)
		r.Post("/drawings", s.handleCreateDrawing)
		r.Get("/drawings/{id}", s.handleGetDrawing)
		r.Put("/drawings/{id}", s.handleUpdateDrawing)
		r.Delete("/drawings/{id}", s.handleDeleteDrawing)
		// 界面主视图：选中一个图纸，只发这一个请求就拿到整页数据
		r.Get("/drawings/{id}/detail", s.handleDrawingDetail)
		r.Get("/drawings/{id}/operations", s.handleListOperations)
		r.Post("/drawings/{id}/operations", s.handleCreateOperation)

		// 工序
		r.Put("/operations/{id}", s.handleUpdateOperation)
		r.Delete("/operations/{id}", s.handleDeleteOperation)
		r.Get("/operations/{id}/programs", s.handleListPrograms)
		r.Post("/operations/{id}/programs", s.handleCreateProgram)

		// 程序
		r.Get("/programs/{id}", s.handleGetProgram)
		r.Put("/programs/{id}", s.handleUpdateProgram)
		r.Delete("/programs/{id}", s.handleDeleteProgram)
		r.Get("/programs/{id}/tools", s.handleListProgramTools)
		r.Put("/programs/{id}/tools", s.handleReplaceProgramTools)
		r.Get("/programs/{id}/versions", s.handleListVersions)
		r.Post("/programs/{id}/versions", s.handleUploadVersion)
		// 在软件里编辑程序后「另存为新版本」
		r.Post("/programs/{id}/versions/content", s.handleSaveVersionContentAsNew)
		r.Put("/programs/{id}/current-version", s.handleSetCurrentVersion)
		r.Get("/programs/{id}/logs", s.handleListLogs)

		// 版本
		r.Get("/versions/{id}/download", s.handleDownloadVersion)
		r.Get("/versions/{id}/diff", s.handleDiffVersions)
		// 查看程序 / 直接覆盖这一版的内容
		r.Get("/versions/{id}/content", s.handleGetVersionContent)
		r.Put("/versions/{id}/content", s.handleOverwriteVersionContent)

		// 从 NC 文本里识别程序号与刀具调用
		r.Post("/nc/parse", s.handleParseText)

		// 版本更新：查仓库上有没有新版本，以及安装离线包
		r.Get("/update/check", s.handleUpdateCheck)
		r.Post("/update/install", s.handleUpdateInstall)

		// 刀具字典
		r.Get("/tools", s.handleListTools)
		r.Post("/tools", s.handleCreateTool)
		r.Put("/tools/{id}", s.handleUpdateTool)
		r.Delete("/tools/{id}", s.handleDeleteTool)

		// 机台字典
		r.Get("/machines", s.handleListMachines)
		r.Post("/machines", s.handleCreateMachine)
		r.Put("/machines/{id}", s.handleUpdateMachine)
		r.Delete("/machines/{id}", s.handleDeleteMachine)
	})

	// 可选：把前端也挂上来（仅在设置了 CNC_WEB_DIR 时生效）。
	// /api 是显式路由，优先级高于这里的通配，两者不会互相抢。
	s.mountWeb(r)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "接口不存在："+r.Method+" "+r.URL.Path)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "该方法不被支持："+r.Method+" "+r.URL.Path)
	})

	return trimTrailingSlash(r)
}

// handleMeta 返回服务元信息。
func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version":    s.version,
		"serverTime": nowString(),
		"dataDir":    s.cfg.DataDir,
		// 只有「exe 旁边就是 web\index.html」这种免安装布局才谈得上离线安装；
		// 开发模式下 exe 在 backend\ 而前端在别处，界面据此把安装入口藏起来。
		"canInstall": s.layout.CanSelfUpdate,
	})
}
