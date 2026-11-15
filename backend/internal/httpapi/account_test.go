package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"cnc-manager/internal/domain"
)

func TestUpdateOwnDisplayName(t *testing.T) {
	h, _ := newTestServer(t)
	cookie := setupAdmin(t, h)
	for _, body := range []string{`{}`, `null`, `{"displayName":null}`, `{"displayName":"  "}`, `{"displayName":"` + strings.Repeat("名", 65) + `"}`} {
		if rec := do(t, h, "PUT", "/api/auth/me", body, cookie); rec.Code != 400 {
			t.Fatalf("invalid name: %d %s", rec.Code, rec.Body.String())
		}
	}
	name := strings.Repeat("名", 64)
	body, _ := json.Marshal(map[string]any{"displayName": " " + name + " ", "id": 999, "username": "intruder", "role": "viewer"})
	rec := do(t, h, "PUT", "/api/auth/me", string(body), cookie)
	var got domain.User
	decode(t, rec, &got)
	if rec.Code != 200 || got.DisplayName != name || got.Username != "admin" || got.Role != "admin" || got.ID != 1 {
		t.Fatalf("unexpected profile: %d %+v", rec.Code, got)
	}
	if strings.Contains(rec.Body.String(), "password") {
		t.Fatal("profile response leaks password fields")
	}
	decode(t, do(t, h, "GET", "/api/auth/me", "", cookie), &got)
	if got.DisplayName != name {
		t.Fatal("name was not persisted")
	}
}

func TestChangePasswordAndRevokeSessions(t *testing.T) {
	h, _ := newTestServer(t)
	first := setupAdmin(t, h)
	second := sessionCookie(t, do(t, h, "POST", "/api/auth/login", `{"username":"admin","password":"password123"}`))
	for _, tc := range []struct{ body, message string }{
		{`{}`, "原密码"},
		{`{"currentPassword":"password123","newPassword":"short"}`, "8~128"},
		{`{"currentPassword":"password123","newPassword":"password123"}`, "不能与原密码相同"},
		{`{"currentPassword":"incorrect","newPassword":"newpassword456"}`, "原密码不正确"},
	} {
		rec := do(t, h, "PUT", "/api/auth/password", tc.body, first)
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), tc.message) {
			t.Fatalf("bad password validation: %d %s", rec.Code, rec.Body.String())
		}
		if len(rec.Result().Cookies()) != 0 || do(t, h, "GET", "/api/auth/me", "", first).Code != 200 {
			t.Fatal("failed change should preserve the session")
		}
	}
	rec := do(t, h, "PUT", "/api/auth/password", `{"currentPassword":"password123","newPassword":"newpassword456","userId":999}`, first)
	if rec.Code != 204 || sessionCookie(t, rec).MaxAge != -1 {
		t.Fatalf("password update failed: %d %s", rec.Code, rec.Body.String())
	}
	for _, cookie := range []*http.Cookie{first, second} {
		if do(t, h, "GET", "/api/auth/me", "", cookie).Code != 401 {
			t.Fatal("all old sessions must expire")
		}
	}
	if do(t, h, "POST", "/api/auth/login", `{"username":"admin","password":"password123"}`).Code != 401 {
		t.Fatal("old password must fail")
	}
	if do(t, h, "POST", "/api/auth/login", `{"username":"admin","password":"newpassword456"}`).Code != 200 {
		t.Fatal("new password must work")
	}
}

func TestAccountEndpointsRequireSessionAndLimitPasswordAttempts(t *testing.T) {
	h, _ := newTestServer(t)
	for _, path := range []string{"/api/auth/me", "/api/auth/password"} {
		if do(t, h, "PUT", path, `{}`).Code != 401 {
			t.Fatal("account writes require a valid login")
		}
	}
	cookie := setupAdmin(t, h)
	for range 5 {
		if do(t, h, "PUT", "/api/auth/password", `{"currentPassword":"wrong-password","newPassword":"newpassword456"}`, cookie).Code != 400 {
			t.Fatal("incorrect current password should be a form error")
		}
	}
	rec := do(t, h, "PUT", "/api/auth/password", `{"currentPassword":"password123","newPassword":"newpassword456"}`, cookie)
	if rec.Code != 429 || rec.Header().Get("Retry-After") != "60" {
		t.Fatal("repeated incorrect passwords must be throttled")
	}
	if do(t, h, "POST", "/api/auth/login", `{"username":"admin","password":"password123"}`).Code != 200 {
		t.Fatal("change-password rate limit must not block normal login")
	}
}

func TestWrongCurrentPasswordAuditKeepsAuthenticatedActor(t *testing.T) {
	h, _ := newTestServer(t)
	cookie := setupAdmin(t, h)
	rec := do(t, h, "PUT", "/api/auth/password", `{"currentPassword":"wrong-password","newPassword":"newpassword456"}`, cookie)
	if rec.Code != 400 {
		t.Fatalf("expected form error, got %d: %s", rec.Code, rec.Body.String())
	}
	var logs domain.Page[domain.AuditEntry]
	decode(t, do(t, h, "GET", "/api/admin/audit-logs?result=failed", "", cookie), &logs)
	if logs.Total != 1 || len(logs.Items) != 1 {
		t.Fatalf("expected one authenticated request failure, got %+v", logs)
	}
	event := logs.Items[0]
	if event.Action != "request" || event.Actor != "admin" || event.ActorID == nil || *event.ActorID != 1 || !strings.Contains(event.Detail, "/auth/password") {
		t.Fatalf("incorrect password-change audit: %+v", event)
	}
	decode(t, do(t, h, "GET", "/api/admin/audit-logs?action=login&result=failed", "", cookie), &logs)
	if logs.Total != 0 {
		t.Fatalf("password change must not produce a login failure: %+v", logs)
	}
	if rec := do(t, h, "POST", "/api/auth/login", `{"username":"admin","password":"wrong-password"}`); rec.Code != 401 {
		t.Fatalf("wrong login password returned %d", rec.Code)
	}
	decode(t, do(t, h, "GET", "/api/admin/audit-logs?action=login&result=failed", "", cookie), &logs)
	if logs.Total != 1 {
		t.Fatalf("actual failed login must remain audited: %+v", logs)
	}
}
