// CNC-Manager 服务入口。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cnc-manager/internal/config"
	"cnc-manager/internal/httpapi"
	"cnc-manager/internal/ncstore"
	"cnc-manager/internal/repo"
	"cnc-manager/internal/store"
	"cnc-manager/internal/web"
)

// version 通过 -ldflags "-X main.version=..." 注入，来源为根目录 VERSION。
var version = "dev"

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("服务退出", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	db, err := store.Open(cfg.Layout.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := store.Migrate(ctx, db); err != nil {
		return err
	}
	key, err := ncstore.LoadOrCreateKey(cfg.Layout.KeyPath)
	if err != nil {
		return err
	}
	files, err := ncstore.New(cfg.Layout.NCDir, cfg.MaxUploadBytes, key)
	if err != nil {
		return err
	}
	r := repo.New(db)

	go cleanSessions(ctx, r, log)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.New(httpapi.Options{Version: version, Repo: r, Files: files, Log: log, SessionLifetime: cfg.SessionLifetime, Static: web.Handler()}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("CNC-Manager 已启动", "version", version, "addr", cfg.Addr, "data", cfg.DataDir)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		log.Info("收到退出信号，正在关闭")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

// cleanSessions 每小时清理一次过期会话。
func cleanSessions(ctx context.Context, r *repo.Repo, log *slog.Logger) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		if n, err := r.DeleteExpiredSessions(ctx); err != nil {
			log.Warn("清理过期会话失败", "err", err)
		} else if n > 0 {
			log.Info("已清理过期会话", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
