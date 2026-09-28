package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cnccool/internal/config"
)

// newWebServer 造一个只挂了前端静态文件的最小服务。
// repo / files 传 nil 是故意的：静态托管这条路径不该碰业务依赖。
func newWebServer(t *testing.T, webDir string) http.Handler {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{
		Addr:        "127.0.0.1:0",
		MaxUploadMB: 64,
		CORSOrigins: []string{"http://127.0.0.1:5173"},
		WebDir:      webDir,
	}
	return NewServer(nil, nil, cfg, log, "test").Router()
}

// writeFile 在测试目录里写一个文件，父目录自动创建。
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}
}

// doGet 发一次 GET 并返回响应，调用方负责 Close。
func doGet(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestWebServesIndexAtRoot 验证根路径返回 index.html，且不允许被缓存。
func TestWebServesIndexAtRoot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "<!doctype html><title>CNC-Manager</title>")

	h := newWebServer(t, dir)
	rec := doGet(t, h, "/")

	if rec.Code != http.StatusOK {
		t.Fatalf("根路径状态码 = %d，期望 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "CNC-Manager") {
		t.Errorf("根路径没有返回 index.html，实际内容：%q", rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("index.html 的 Cache-Control = %q，期望 no-store", cc)
	}
}

// TestWebServesAssetsWithLongCache 验证构建产物走长缓存，避免每次刷新都重新下载。
func TestWebServesAssetsWithLongCache(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "<!doctype html>")
	writeFile(t, filepath.Join(dir, "assets", "index-abc123.js"), "console.log(1)")

	h := newWebServer(t, dir)
	rec := doGet(t, h, "/assets/index-abc123.js")

	if rec.Code != http.StatusOK {
		t.Fatalf("静态资源状态码 = %d，期望 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "console.log") {
		t.Errorf("静态资源内容不对：%q", rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("assets 的 Cache-Control = %q，期望带 immutable", cc)
	}
}

// TestWebFallsBackToIndexForSpaRoutes 验证前端路由地址刷新页面时回落到 index.html。
func TestWebFallsBackToIndexForSpaRoutes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "<!doctype html><title>CNC-Manager</title>")

	h := newWebServer(t, dir)
	rec := doGet(t, h, "/drawings/1/operations")

	if rec.Code != http.StatusOK {
		t.Fatalf("前端路由地址状态码 = %d，期望 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "CNC-Manager") {
		t.Errorf("没有回落到 index.html，实际内容：%q", rec.Body.String())
	}
}

// TestWebHeadingIsAllowed 验证 HEAD 请求也能走通。
//
// chi 的 Get 只注册 GET，漏了 HEAD 的话浏览器预检静态资源会莫名拿到 405。
func TestWebHeadingIsAllowed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "<!doctype html>")

	h := newWebServer(t, dir)
	req := httptest.NewRequest(http.MethodHead, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("HEAD 状态码 = %d，期望 200", rec.Code)
	}
}

// TestWebDoesNotEscapeWebDir 是本文件里最重要的一条：静态托管绝不能被用来翻目录。
func TestWebDoesNotEscapeWebDir(t *testing.T) {
	root := t.TempDir()
	webDir := filepath.Join(root, "web")
	writeFile(t, filepath.Join(webDir, "index.html"), "<!doctype html><title>CNC-Manager</title>")
	// 放在 webDir 外面，正常情况下永远不该被读到。
	writeFile(t, filepath.Join(root, "secret.txt"), "TOP-SECRET")

	h := newWebServer(t, webDir)

	for _, p := range []string{
		"/../secret.txt",
		"/..%2fsecret.txt",
		"/assets/../../secret.txt",
		"/%2e%2e/secret.txt",
	} {
		// 手动构造 URL，绕开 httptest 对路径的自动规整，
		// 这样才真的把带 .. 的原始路径喂给了路由。
		req := &http.Request{
			Method: http.MethodGet,
			URL:    &url.URL{Path: p},
			Header: http.Header{},
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if strings.Contains(rec.Body.String(), "TOP-SECRET") {
			t.Errorf("路径 %q 读到了 webDir 之外的文件，存在目录穿越漏洞", p)
		}
	}
}

// TestWebMissingAssetIsNotFound 验证缺失的静态文件返回 404 而不是 index.html。
//
// 回落到 HTML 会让浏览器把 HTML 当 JS 解析，报出来的错误跟真正的原因完全对不上，
// 排查起来非常费劲，所以带扩展名的地址必须老老实实报 404。
func TestWebMissingAssetIsNotFound(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "<!doctype html><title>CNC-Manager</title>")

	h := newWebServer(t, dir)

	for _, p := range []string{"/assets/gone-000000.js", "/favicon.ico"} {
		rec := doGet(t, h, p)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s 状态码 = %d，期望 404（不能回落成 index.html）", p, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "<!doctype html>") {
			t.Errorf("GET %s 返回了 index.html，缺失的静态资源必须报 404", p)
		}
	}
}

// TestApiWinsOverWebFallback 验证接口不会被前端回落吞掉。
func TestApiWinsOverWebFallback(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "<!doctype html><title>CNC-Manager</title>")

	h := newWebServer(t, dir)
	rec := doGet(t, h, "/api/meta")

	if rec.Code != http.StatusOK {
		t.Fatalf("/api/meta 状态码 = %d，期望 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "<!doctype html>") {
		t.Errorf("/api/meta 被前端回落吞掉了，实际内容：%q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"version"`) {
		t.Errorf("/api/meta 没有返回版本信息：%q", rec.Body.String())
	}
}

// TestNoWebDirKeepsApiOnlyBehavior 验证不配 CNC_WEB_DIR 时行为完全不变。
// 这是开发模式的基线：前端仍然只由 Vite 提供。
func TestNoWebDirKeepsApiOnlyBehavior(t *testing.T) {
	h := newWebServer(t, "")

	rec := doGet(t, h, "/api/meta")
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/meta 状态码 = %d，期望 200", rec.Code)
	}

	rec = doGet(t, h, "/")
	if rec.Code != http.StatusNotFound {
		t.Errorf("未配置前端目录时根路径状态码 = %d，期望 404", rec.Code)
	}
}

// TestWebReportsMissingIndex 验证前端目录配错时给出的提示能看懂。
func TestWebReportsMissingIndex(t *testing.T) {
	dir := t.TempDir() // 目录存在但是空的

	h := newWebServer(t, dir)
	rec := doGet(t, h, "/")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d，期望 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "index.html") {
		t.Errorf("错误信息没有点明缺少 index.html：%q", rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// 前端版本比对
// ---------------------------------------------------------------------------

// viteIndex 拼一份和 Vite 构建产物同形的 index.html。
func viteIndex(entry string) string {
	return `<!doctype html><html><head>` +
		`<script type="module" crossorigin src="` + entry + `"></script>` +
		`<link rel="stylesheet" crossorigin href="/assets/index-aaa111.css">` +
		`</head><body><div id="app"></div></body></html>`
}

// TestParseEntryScript 验证能从各种写法的 index.html 里挑出入口脚本。
func TestParseEntryScript(t *testing.T) {
	cases := []struct {
		name string
		html string
		want string
	}{
		{"Vite 默认产出", viteIndex("/assets/index-CpuQPsoE.js"), "/assets/index-CpuQPsoE.js"},
		{"属性顺序反过来", `<script src="/assets/a.js" type="module"></script>`, "/assets/a.js"},
		{"属性用单引号", `<script type='module' src='/assets/b.js'></script>`, "/assets/b.js"},
		{"模块脚本排在后面", `<script src="/x.js"></script><script type="module" src="/assets/c.js"></script>`, "/assets/c.js"},
		{"只有普通脚本", `<script src="/legacy.js"></script>`, ""},
		{"没有 src", `<script type="module">console.log(1)</script>`, ""},
		{"空文档", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseEntryScript([]byte(c.html)); got != c.want {
				t.Errorf("parseEntryScript = %q，期望 %q", got, c.want)
			}
		})
	}
}

// TestWebEntryReadsIndex 验证能报出当前 index.html 引用的入口脚本。
func TestWebEntryReadsIndex(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), viteIndex("/assets/index-CpuQPsoE.js"))

	h := newWebServer(t, dir)
	rec := doGet(t, h, "/api/meta")

	if !strings.Contains(rec.Body.String(), "/assets/index-CpuQPsoE.js") {
		t.Errorf("/api/meta 没有带出 webEntry：%q", rec.Body.String())
	}
}

// TestWebEntryFollowsReplacement 是本组里最关键的一条。
//
// 免安装版的升级方式就是把 web\ 目录整个覆盖掉，而服务进程不一定重启。
// 如果缓存不失效，页面永远等不到「该刷新了」的提示，这个功能就等于没有。
func TestWebEntryFollowsReplacement(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.html")
	writeFile(t, indexPath, viteIndex("/assets/index-OLD111.js"))

	s := NewServer(nil, nil, &config.Config{WebDir: dir, MaxUploadMB: 64},
		slog.New(slog.NewTextHandler(io.Discard, nil)), "test")

	if got := s.webEntry(); got != "/assets/index-OLD111.js" {
		t.Fatalf("首次解析 = %q，期望 /assets/index-OLD111.js", got)
	}

	// 连续两次调用走的是缓存，结果必须一样
	if got := s.webEntry(); got != "/assets/index-OLD111.js" {
		t.Fatalf("缓存命中时 = %q，期望 /assets/index-OLD111.js", got)
	}

	// 覆盖成新版本。显式把修改时间往后推，避免文件系统时间精度不够导致漏判。
	writeFile(t, indexPath, viteIndex("/assets/index-NEW222.js"))
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(indexPath, future, future); err != nil {
		t.Fatalf("调整修改时间失败: %v", err)
	}

	if got := s.webEntry(); got != "/assets/index-NEW222.js" {
		t.Errorf("覆盖后 = %q，期望 /assets/index-NEW222.js（缓存没有失效）", got)
	}
}

// TestWebEntryEmptyWithoutWebDir 验证开发模式下报空串，前端据此跳过比对。
//
// 宁可不提示，也不能拿一个假指纹去误报「有新版本」。
func TestWebEntryEmptyWithoutWebDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), viteIndex("/assets/index-CpuQPsoE.js"))

	// 注意这里故意不配 WebDir，模拟开发模式
	s := NewServer(nil, nil, &config.Config{MaxUploadMB: 64},
		slog.New(slog.NewTextHandler(io.Discard, nil)), "test")

	if got := s.webEntry(); got != "" {
		t.Errorf("未配置前端目录时 webEntry = %q，期望空串", got)
	}
}

// TestWebEntryEmptyWhenIndexUnreadable 验证 index.html 缺失时不会 panic、也不会瞎报。
func TestWebEntryEmptyWhenIndexUnreadable(t *testing.T) {
	s := NewServer(nil, nil, &config.Config{WebDir: t.TempDir(), MaxUploadMB: 64},
		slog.New(slog.NewTextHandler(io.Discard, nil)), "test")

	if got := s.webEntry(); got != "" {
		t.Errorf("index.html 不存在时 webEntry = %q，期望空串", got)
	}
}
