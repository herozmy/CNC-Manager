package httpapi

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"cnccool/internal/config"
	"cnccool/internal/repo"
	"cnccool/internal/store"
)

func newAuthServer(t *testing.T) (http.Handler, *repo.Repo) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("迁移测试数据库失败: %v", err)
	}
	cfg := &config.Config{CORSOrigins: []string{"http://127.0.0.1:5173"}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repository := repo.New(db)
	return NewServer(repository, nil, cfg, logger, "test").Router(), repository
}

func jsonRequest(t *testing.T, handler http.Handler, method, target, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func TestAuthSetupLoginAndSession(t *testing.T) {
	handler, _ := newAuthServer(t)

	status := jsonRequest(t, handler, http.MethodGet, "/api/auth/status", "")
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"setupRequired":true`) {
		t.Fatalf("初始状态不正确: code=%d body=%s", status.Code, status.Body.String())
	}

	unauthorized := jsonRequest(t, handler, http.MethodGet, "/api/drawings", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("未登录业务请求状态码 = %d，期望 401", unauthorized.Code)
	}

	setup := jsonRequest(t, handler, http.MethodPost, "/api/auth/setup",
		`{"username":"admin","displayName":"管理员","password":"secure123"}`)
	if setup.Code != http.StatusOK {
		t.Fatalf("创建管理员失败: code=%d body=%s", setup.Code, setup.Body.String())
	}
	cookies := setup.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("会话 Cookie 属性不正确: %#v", cookies)
	}

	me := jsonRequest(t, handler, http.MethodGet, "/api/auth/me", "", cookies[0])
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"username":"admin"`) {
		t.Fatalf("读取当前用户失败: code=%d body=%s", me.Code, me.Body.String())
	}

	badLogin := jsonRequest(t, handler, http.MethodPost, "/api/auth/login",
		`{"username":"admin","password":"wrong-password"}`)
	if badLogin.Code != http.StatusUnauthorized {
		t.Fatalf("错误密码状态码 = %d，期望 401", badLogin.Code)
	}

	logout := jsonRequest(t, handler, http.MethodPost, "/api/auth/logout", "{}", cookies[0])
	if logout.Code != http.StatusNoContent {
		t.Fatalf("退出状态码 = %d，期望 204", logout.Code)
	}
	expired := jsonRequest(t, handler, http.MethodGet, "/api/auth/me", "", cookies[0])
	if expired.Code != http.StatusUnauthorized {
		t.Fatalf("退出后的会话状态码 = %d，期望 401", expired.Code)
	}
}

func TestAuthSetupCanOnlyRunOnce(t *testing.T) {
	handler, _ := newAuthServer(t)
	body := `{"username":"admin","password":"secure123"}`
	if first := jsonRequest(t, handler, http.MethodPost, "/api/auth/setup", body); first.Code != http.StatusOK {
		t.Fatalf("首次初始化状态码 = %d", first.Code)
	}
	if second := jsonRequest(t, handler, http.MethodPost, "/api/auth/setup", body); second.Code != http.StatusConflict {
		t.Fatalf("重复初始化状态码 = %d，期望 409", second.Code)
	}
}

func TestAuthSetupValidatesPassword(t *testing.T) {
	handler, _ := newAuthServer(t)
	response := jsonRequest(t, handler, http.MethodPost, "/api/auth/setup",
		`{"username":"admin","password":"short"}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("短密码状态码 = %d，期望 400，响应：%s", response.Code, response.Body.String())
	}
	status := jsonRequest(t, handler, http.MethodGet, "/api/auth/status", "")
	if !strings.Contains(status.Body.String(), `"setupRequired":true`) {
		t.Fatalf("初始化失败后不应创建用户：%s", status.Body.String())
	}
}

func TestAuthRejectsCrossOriginWrites(t *testing.T) {
	handler, _ := newAuthServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup",
		bytes.NewBufferString(`{"username":"admin","password":"secure123"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://attacker.example")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("跨站写请求状态码 = %d，期望 403", recorder.Code)
	}
}

func TestAuthRateLimitsRepeatedFailures(t *testing.T) {
	handler, _ := newAuthServer(t)
	setup := jsonRequest(t, handler, http.MethodPost, "/api/auth/setup",
		`{"username":"admin","password":"secure123"}`)
	if setup.Code != http.StatusOK {
		t.Fatalf("创建管理员失败: %s", setup.Body.String())
	}
	for attempt := 1; attempt <= 5; attempt++ {
		response := jsonRequest(t, handler, http.MethodPost, "/api/auth/login",
			`{"username":"admin","password":"wrong-password"}`)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("第 %d 次错误登录状态码 = %d，期望 401", attempt, response.Code)
		}
	}
	blocked := jsonRequest(t, handler, http.MethodPost, "/api/auth/login",
		`{"username":"admin","password":"secure123"}`)
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("限流状态码 = %d，期望 429", blocked.Code)
	}
	if blocked.Header().Get("Retry-After") == "" {
		t.Fatal("限流响应缺少 Retry-After")
	}
}

func TestAuthenticatedWriteRecordsUsername(t *testing.T) {
	handler, repository := newAuthServer(t)
	setup := jsonRequest(t, handler, http.MethodPost, "/api/auth/setup",
		`{"username":"operator01","displayName":"操作员","password":"secure123"}`)
	if setup.Code != http.StatusOK {
		t.Fatalf("创建管理员失败: %s", setup.Body.String())
	}
	cookies := setup.Result().Cookies()
	create := jsonRequest(t, handler, http.MethodPost, "/api/drawings",
		`{"drawingNo":"TEST-001","name":"测试图纸"}`, cookies[0])
	if create.Code != http.StatusOK {
		t.Fatalf("创建图纸失败: code=%d body=%s", create.Code, create.Body.String())
	}
	var actor string
	if err := repository.DB().Get(&actor,
		`SELECT actor FROM audit_log WHERE entity_type = 'drawing' ORDER BY id DESC LIMIT 1`); err != nil {
		t.Fatalf("读取操作日志失败: %v", err)
	}
	if actor != "operator01" {
		t.Fatalf("操作日志 actor = %q，期望 operator01", actor)
	}
}
