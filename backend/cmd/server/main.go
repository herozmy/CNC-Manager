// Command server 是 CNC 加工程序管理系统的后端服务。
//
// 它默认只提供 JSON 接口；配置了 CNC_WEB_DIR 时会顺带托管前端静态文件，
// 这样免安装版一个 exe 就能交付。开发时前端由 Vite 提供（热更新），
// 也可以交给 nginx 单独提供。
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"

	"cnccool/internal/applog"
	"cnccool/internal/config"
	"cnccool/internal/httpapi"
	"cnccool/internal/ncstore"
	"cnccool/internal/repo"
	"cnccool/internal/store"
	"cnccool/internal/tray"
	"cnccool/internal/update"
)

// version 是产品版本号。
//
// 单一真源是仓库根目录的 VERSION 文件，构建时由 scripts\run-backend.ps1
// 通过 -ldflags "-X main.version=..." 注入。
// 这里的默认值只是兜底，保证直接 go build 出来的二进制也有个像样的版本号。
//
// 服务启动时会打印这个版本号，GET /api/meta 也会返回，
// 界面上显示出来——现场排查问题时第一件事就是确认装的是哪一版。
var version = "v1.0"

// exitRestarting 是「正在为安装离线包而重启」的退出码。
// 替换脚本会等待进程退出，换完文件后直接重新启动 exe。
const exitRestarting = 99

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

	// 日志同时写控制台和文件。
	//
	// 文件这一份是给事后查的：车间里那个黑窗口一关，屏幕上说过什么就全没了，
	// 「服务忽然连不上」就永远是悬案。写不了文件不拦启动——那是辅助功能。
	var logFile *applog.RotatingFile
	logFile, err = applog.Open(cfg.LogFile, applog.DefaultMaxBytes)
	if err != nil {
		logFile = nil
	}

	logger := newLogger(cfg.LogLevel, logFile)
	slog.SetDefault(logger)
	if logFile == nil {
		logger.Warn("日志文件写不了，只输出到控制台", "path", cfg.LogFile, "err", err)
	}

	// 崩了也要留下痕迹：panic 的现场只有控制台的话，用户一关窗口就没了。
	defer recoverTo(logger)

	ctx := context.Background()

	// 0) 清掉上一次离线安装留下的暂存内容
	update.CleanLeftovers()

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
	//
	// srv 先声明再赋值：下面的重启回调里要用到它，而回调又必须在
	// srv 构造之前注册进去（构造时要拿 Handler）。
	var srv *http.Server

	apiServer := httpapi.NewServer(r, files, cfg, logger, version)

	// 离线包装好之后，替换脚本会等这个进程退出才动手（Windows 上运行中的
	// exe 是锁着的），所以这里要主动、尽快地退出去。
	apiServer.SetRestartHook(func() {
		go func() {
			// 留一点时间把响应发完，否则前端只会看到一个连接被断开
			time.Sleep(1200 * time.Millisecond)

			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutdownCtx)

			// 退出码 99 是给 start.cmd 看的：这是升级重启，不是出错，
			// 别停在 pause 上让用户按键，替换脚本会另起一个新窗口。
			os.Exit(exitRestarting)
		}()
	})

	srv = &http.Server{
		Addr:    cfg.Addr,
		Handler: apiServer.Router(),
		// 上传几十 MB 的 NC 程序可能比较慢，读写超时给足；
		// 只把读请求头的时间卡紧，防止慢速攻击占住连接。
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}

	// 3b) 先把端口绑上，再交给 http.Server。
	//
	// 不让 ListenAndServe 内部去绑，是为了能在失败时给一句人话：
	// 端口被占用是最常见的启动失败，而 Go 原样抛出来的
	// "bind: Only one usage of each socket address ... is normally permitted"
	// 现场没人看得懂，只会以为程序坏了。
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return listenFailed(cfg.Addr, err)
	}

	logger.Info("CNC 加工程序管理服务已启动",
		"地址", "http://"+cfg.Addr,
		"版本", version,
		"数据目录", cfg.DataDir,
		"数据库", cfg.DBPath,
		"NC文件库", cfg.NCDir,
		"日志文件", cfg.LogFile)

	appURL := "http://" + browserAddr(cfg.Addr)
	trayExit := make(chan struct{})
	var stopTray func()
	if cfg.WebDir != "" {
		stopTray, err = tray.Start(tray.Actions{
			OpenApp:  func() { openBrowser(appURL, 0) },
			OpenLogs: func() { openFolder(filepath.Dir(cfg.LogFile)) },
			Exit:     func() { close(trayExit) },
		})
		if err != nil {
			logger.Warn("系统托盘启动失败", "err", err)
		} else {
			defer stopTray()
		}
	}
	if cfg.WebDir != "" && os.Getenv("CNC_NO_BROWSER") == "" {
		openBrowser(appURL, 1200*time.Millisecond)
	}

	if logFile != nil {
		// 退出前把文件关掉，保证最后几行落盘
		defer func() { _ = logFile.Close() }()
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
	case <-trayExit:
		logger.Info("从系统托盘退出，正在关闭服务")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	logger.Info("服务已停止")
	return nil
}

func browserAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

func openBrowser(url string, delay time.Duration) {
	go func() {
		time.Sleep(delay)
		command := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
		if err := command.Start(); err != nil {
			slog.Warn("无法自动打开浏览器，请手工访问", "url", url, "err", err)
		}
	}()
}

func openFolder(path string) {
	if err := exec.Command("explorer.exe", path).Start(); err != nil {
		slog.Warn("无法打开日志目录", "path", path, "err", err)
	}
}

// listenFailed 把「端口绑不上」翻译成一句能照着做的话。
//
// 这个失败现场太常见了：上一次的服务还开着、或者别的程序占了端口。
// 原始错误是英文的系统调用信息，用户看到只会以为程序坏了，
// 然后来问「为什么打不开」。
func listenFailed(addr string, err error) error {
	_, port, splitErr := net.SplitHostPort(addr)
	if splitErr != nil {
		return fmt.Errorf("监听 %s 失败: %w", addr, err)
	}

	// 建议的端口从当前这个推出来，不写死。
	// 写死的话，用户本来就是用 start.cmd 8090 起的，提示再让他
	// "例如 start.cmd 8090"，等于让他再撞一次同一堵墙。
	alt := "8081"
	if n, convErr := strconv.Atoi(port); convErr == nil && n > 0 && n < 65535 {
		alt = strconv.Itoa(n + 1)
	}

	return fmt.Errorf(
		"端口 %s 已被占用。\n"+
			"  最常见的原因是上一次的服务还开着（黑窗口没关），\n"+
			"  或者被别的程序（另一个副本、杀毒软件等）占了。\n"+
			"\n"+
			"  处理办法一：关掉占用端口的程序，再重新启动。\n"+
			"  处理办法二：换一个端口启动，例如\n"+
			"      start.cmd %s\n"+
			"  然后浏览器访问 http://127.0.0.1:%s\n"+
			"\n"+
			"  系统原始错误：%v", port, alt, alt, err)
}

// recoverTo 把 panic 记进日志，然后原样抛出去。
//
// 记是为了留下现场（日志文件里能看到 panic 内容与完整堆栈）；
// 原样抛是为了不改变程序的行为——吞掉 panic 假装没事，比崩掉更糟糕。
func recoverTo(logger *slog.Logger) {
	rec := recover()
	if rec == nil {
		return
	}
	logger.Error("服务发生未捕获的 panic，即将退出",
		"panic", fmt.Sprint(rec), "堆栈", string(debug.Stack()))
	panic(rec)
}

// newLogger 构造日志器：同时写控制台和文件（file 为 nil 时只写控制台）。
//
// 控制台那份是给人当场看的，文件那份是给事后查的。两份内容完全一样。
func newLogger(level string, file io.Writer) *slog.Logger {
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

	out := io.Writer(os.Stdout)
	if file != nil {
		out = io.MultiWriter(os.Stdout, file)
	}
	return slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{Level: lv}))
}
