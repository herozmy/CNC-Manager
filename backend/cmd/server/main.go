// Command server 是 CNC 加工程序管理系统的后端服务。
//
// 它只提供 JSON 接口，不托管前端静态文件——
// 前端是独立的 Vue 工程，开发时跑 Vite（热更新），
// 部署时由 nginx 单独提供静态文件。这样前端改版完全不用重启这个服务。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cnccool/internal/config"
	"cnccool/internal/httpapi"
	"cnccool/internal/ncstore"
	"cnccool/internal/repo"
	"cnccool/internal/store"
)

// version 是产品版本号。
//
// 单一真源是仓库根目录的 VERSION 文件，构建时由 scripts\run-backend.ps1
// 通过 -ldflags "-X main.version=..." 注入。
// 这里的默认值只是兜底，保证直接 go build 出来的二进制也有个像样的版本号。
//
// 服务启动时会打印这个版本号，GET /api/meta 也会返回，
// 界面上显示出来——现场排查问题时第一件事就是确认装的是哪一版。
var version = "v0.01"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "启动失败:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	ctx := context.Background()

	// 1) 打开数据库（不存在会自动创建），并应用内嵌的迁移脚本
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if err := store.Migrate(ctx, db); err != nil {
		return err
	}

	// 2) 初始化 NC 文件库
	files, err := ncstore.New(cfg.NCDir, cfg.MaxUploadBytes())
	if err != nil {
		return err
	}

	r := repo.New(db)
	if err := r.Ping(ctx); err != nil {
		return fmt.Errorf("数据库连接不可用: %w", err)
	}

	// 3) 启动 HTTP 服务
	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: httpapi.NewServer(r, files, cfg, logger, version).Router(),
		// 上传几十 MB 的 NC 程序可能比较慢，读写超时给足；
		// 只把读请求头的时间卡紧，防止慢速攻击占住连接。
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}

	logger.Info("CNC 加工程序管理服务已启动",
		"地址", "http://"+cfg.Addr,
		"版本", version,
		"数据目录", cfg.DataDir,
		"数据库", cfg.DBPath,
		"NC文件库", cfg.NCDir)

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// 4) 等待退出信号，优雅关闭（保证正在上传的版本不会被截断）
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-stop:
		logger.Info("收到退出信号，正在关闭服务", "信号", sig.String())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func newLogger(level string) *slog.Logger {
	var lv slog.Level
	switch level {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lv}))
}
