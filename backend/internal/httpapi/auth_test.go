package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cnc-manager/internal/domain"
)

func TestMetaAndUnknownAPI(t *testing.T) {
	h, _ := newTestServer(t)
	var meta map[string]string
	rec := do(t, h, http.MethodGet, "/api/meta/", "")
	decode(t, rec, &meta)
	if rec.Code != 200 || meta["version"] != "test" {
		t.Fatalf("meta 异常: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, h, http.MethodGet, "/api/nope", "")
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), "接口不存在") {
		t.Fatalf("未知接口应返回 JSON 404: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodGet, "/drawings/1", ""); rec.Code != 404 {
		t.Fatalf("无静态处理器时非 API 路径应 404，得到 %d", rec.Code)
	}
}

func TestSetupLoginSessionLogout(t *testing.T) {
	h, _ := newTestServer(t)

	var status map[string]bool
	decode(t, do(t, h, http.MethodGet, "/api/auth/status", ""), &status)
	if !status["setupRequired"] {
		t.Fatal("空库应需要初始化")
	}
	if rec := do(t, h, http.MethodGet, "/api/auth/me", ""); rec.Code != 401 {
		t.Fatalf("未登录应 401，得到 %d", rec.Code)
	}

	cookie := setupAdmin(t, h)
	if !cookie.HttpOnly || cookie.Path != "/api" || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge != 3600 {
		t.Fatalf("Cookie 属性异常: %+v", cookie)
	}
	decode(t, do(t, h, http.MethodGet, "/api/auth/status", ""), &status)
	if status["setupRequired"] {
		t.Fatal("初始化后不应再需要初始化")
	}

	var me domain.User
	rec := do(t, h, http.MethodGet, "/api/auth/me", "", cookie)
	decode(t, rec, &me)
	if rec.Code != 200 || me.Username != "admin" || me.Role != "admin" {
		t.Fatalf("me 异常: %d %+v", rec.Code, me)
	}
	if strings.Contains(rec.Body.String(), "password") {
		t.Fatal("响应不应含密码相关字段")
	}

	rec = do(t, h, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"password123"}`)
	if rec.Code != 200 {
		t.Fatalf("登录失败: %d %s", rec.Code, rec.Body.String())
	}
	login := sessionCookie(t, rec)

	if rec := do(t, h, http.MethodPost, "/api/auth/logout", "", login); rec.Code != 204 {
		t.Fatalf("注销应 204，得到 %d", rec.Code)
	}
	if rec := do(t, h, http.MethodGet, "/api/auth/me", "", login); rec.Code != 401 {
		t.Fatal("注销后会话应失效")
	}
	if rec := do(t, h, http.MethodGet, "/api/auth/me", "", cookie); rec.Code != 200 {
		t.Fatal("注销一个会话不应影响另一个会话")
	}
}

func TestSetupOnlyOnceAndValidates(t *testing.T) {
	h, _ := newTestServer(t)
	if rec := do(t, h, http.MethodPost, "/api/auth/setup", `{"username":"a","password":"short"}`); rec.Code != 400 {
		t.Fatalf("弱密码应 400，得到 %d", rec.Code)
	}
	if rec := do(t, h, http.MethodPost, "/api/auth/setup", `{bad json`); rec.Code != 400 {
		t.Fatalf("坏 JSON 应 400，得到 %d", rec.Code)
	}
	setupAdmin(t, h)
	if rec := do(t, h, http.MethodPost, "/api/auth/setup", `{"username":"b","password":"password123"}`); rec.Code != 409 {
		t.Fatalf("二次初始化应 409，得到 %d", rec.Code)
	}
}

func TestLoginFailuresAndRateLimit(t *testing.T) {
	h, _ := newTestServer(t)
	setupAdmin(t, h)
	for i := 0; i < 5; i++ {
		if rec := do(t, h, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"wrong-pass"}`); rec.Code != 401 {
			t.Fatalf("第 %d 次错误密码应 401，得到 %d", i+1, rec.Code)
		}
	}
	rec := do(t, h, http.MethodPost, "/api/auth/login", `{"username":"admin","password":"password123"}`)
	if rec.Code != 429 || rec.Header().Get("Retry-After") != "60" {
		t.Fatalf("5 次失败后应 429，得到 %d", rec.Code)
	}
	// 其他用户名不受影响
	if rec := do(t, h, http.MethodPost, "/api/auth/login", `{"username":"other","password":"x"}`); rec.Code != 401 {
		t.Fatalf("其他用户名应正常处理，得到 %d", rec.Code)
	}
}

func TestCSRFProtection(t *testing.T) {
	h, _ := newTestServer(t)
	body := `{"username":"admin","password":"password123"}`
	send := func(origin, referer string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(body))
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if referer != "" {
			req.Header.Set("Referer", referer)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if c := send("http://evil.com", ""); c != 403 {
		t.Fatalf("跨站 Origin 应 403，得到 %d", c)
	}
	if c := send("", ""); c != 403 {
		t.Fatalf("缺少 Origin 与 Referer 应 403，得到 %d", c)
	}
	if c := send("", "http://evil.com/page"); c != 403 {
		t.Fatalf("跨站 Referer 应 403，得到 %d", c)
	}
	if c := send("", "http://example.com/login"); c != 200 {
		t.Fatalf("同源 Referer 应放行，得到 %d", c)
	}
	// GET 不受限制
	if rec := do(t, h, http.MethodGet, "/api/auth/status", ""); rec.Code != 200 {
		t.Fatal("GET 不应被来源检查拦截")
	}
}

func TestInvalidSessionCookieIsCleared(t *testing.T) {
	h, _ := newTestServer(t)
	rec := do(t, h, http.MethodGet, "/api/auth/me", "", &http.Cookie{Name: authCookieName, Value: "bogus"})
	if rec.Code != 401 {
		t.Fatalf("无效会话应 401，得到 %d", rec.Code)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == authCookieName && c.MaxAge == -1 {
			return
		}
	}
	t.Fatal("应清除无效的会话 Cookie")
}

func TestStaticFallback(t *testing.T) {
	static := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("page")) })
	srv := New(Options{Version: "x", Static: static})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/drawings/1", nil))
	if rec.Body.String() != "page" {
		t.Fatalf("非 API 路径应交给前端，得到 %q", rec.Body.String())
	}
}
