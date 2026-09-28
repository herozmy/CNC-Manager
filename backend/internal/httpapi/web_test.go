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
