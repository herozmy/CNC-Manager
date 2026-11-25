// CNC-Manager 服务入口。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"cnc-manager/internal/config"
	"cnc-manager/internal/datalock"
	"cnc-manager/internal/httpapi"
	"cnc-manager/internal/ncstore"
	"cnc-manager/internal/recovery"
	"cnc-manager/internal/repo"
	"cnc-manager/internal/store"
	"cnc-manager/internal/web"
)

// version 通过 -ldflags "-X main.version=..." 注入，来源为根目录 VERSION。
var version = "dev"

func main() {
	if len(os.Args) > 1 {
		attachCLIConsole()
	}
	// Keep the portable database next to the executable regardless of the launch directory.
	if runtime.GOOS == "windows" && os.Getenv("CNC_DATA_DIR") == "" {
		exe, err := os.Executable()
		if err == nil {
			err = os.Setenv("CNC_DATA_DIR", filepath.Join(filepath.Dir(exe), "data"))
		}
		if err != nil {
			reportStartupError(err)
			os.Exit(1)
		}
	}
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "reset-password":
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
			defer stop()
			if err := recovery.Run(ctx, os.Args[2:], os.Stdin, os.Stdout); err != nil {
				fmt.Fprintln(os.Stderr, "操作未完成：", err)
				os.Exit(1)
			}
			return
		case "--version", "version":
			fmt.Println(version)
			return
		case "--help", "-h", "help":
			fmt.Println("用法：cnc-manager [reset-password | --version | --help]\n无参数启动服务；reset-password --help 查看密码重置用法。")
			return
		default:
			fmt.Fprintln(os.Stderr, "未知命令，请运行 cnc-manager --help")
			os.Exit(2)
		}
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		reportStartupError(err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return err
	}
	lock, err := datalock.Acquire(filepath.Join(cfg.DataDir, ".cnc-manager.lock"))
	if err != nil {
		return err
	}
	defer lock.Close()
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

	handler := httpapi.New(httpapi.Options{DataDir: cfg.DataDir, Version: version, Repo: r, Files: files, Log: log, SessionLifetime: cfg.SessionLifetime, Static: web.Handler()})
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	closeDesktop, err := startDesktop(listener.Addr().(*net.TCPAddr), stop)
	if err != nil {
		return err
	}
	defer closeDesktop()
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); handler.RunBackupSchedule(ctx) }()
	defer func() { stop(); <-workerDone }()
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(listener) }()
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
