package httpapi

import (
	"cnc-manager/internal/domain"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPersonalOwnershipAcrossAllBusinessEndpoints(t *testing.T) {
	h, _ := newTestServer(t)
	admin := setupAdmin(t, h)
	request := func(method, path string, body any, cookie *http.Cookie, want int) *httptest.ResponseRecorder {
		t.Helper()
		raw := ""
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			raw = string(b)
		}
		cookies := []*http.Cookie{}
		if cookie != nil {
			cookies = append(cookies, cookie)
		}
		rec := do(t, h, method, path, raw, cookies...)
		if rec.Code != want {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, rec.Code, want, rec.Body.String())
		}
		return rec
	}
	member := func(name string) (domain.User, *http.Cookie) {
		var out domain.User
		decode(t, request("POST", "/api/admin/users", domain.CreateUserInput{Username: name, Password: "password123", Role: "editor"}, admin, 201), &out)
		return out, sessionCookie(t, request("POST", "/api/auth/login", map[string]string{"username": name, "password": "password123"}, nil, 200))
	}
	ua, ca := member("turner")
	_, ca2 := member("turner2")
	ub, cb := member("miller")
	drawing := func(cookie *http.Cookie) domain.Drawing {
		var d domain.Drawing
		decode(t, request("POST", "/api/drawings", map[string]any{"drawingNo": "SAME-001"}, cookie, 201), &d)
		return d
	}
	da, db := drawing(ca), drawing(cb)
	if da.OwnerUserID != ua.ID || db.OwnerUserID != ub.ID {
		t.Fatal("ownership not derived from member")
	}
	request("POST", "/api/drawings", map[string]any{"drawingNo": "forged", "ownerUserId": ub.ID}, ca, 403)
	request("POST", "/api/drawings", map[string]any{"drawingNo": "SAME-001"}, ca, 409)
	var list domain.Page[domain.Drawing]
	decode(t, request("GET", "/api/drawings?q=SAME", nil, ca2, 200), &list)
	if list.Total != 0 || len(list.Items) != 0 {
		t.Fatalf("personal isolation failed: %+v", list)
	}
	decode(t, request("GET", "/api/drawings", nil, admin, 200), &list)
	if list.Total != 2 {
		t.Fatal(list)
	}
	decode(t, request("GET", fmt.Sprintf("/api/drawings?ownerUserId=%d", ub.ID), nil, admin, 200), &list)
	if list.Total != 1 || list.Items[0].ID != db.ID {
		t.Fatal(list)
	}
	request("GET", fmt.Sprintf("/api/drawings?ownerUserId=%d", ub.ID), nil, ca, 403)
	var op domain.Operation
	decode(t, request("POST", fmt.Sprintf("/api/drawings/%d/operations", db.ID), map[string]any{"opNo": 10}, cb, 201), &op)
	var p domain.Program
	decode(t, request("POST", fmt.Sprintf("/api/operations/%d/programs", op.ID), map[string]any{"programNo": "PRIVATE-B"}, cb, 201), &p)
	var v domain.Version
	decode(t, request("POST", fmt.Sprintf("/api/programs/%d/versions/content", p.ID), map[string]any{"content": "O2000\nT0202\nM30\n", "encoding": "utf-8", "fileName": "private.nc"}, cb, 201), &v)
	request("GET", fmt.Sprintf("/api/versions/%d/download", v.ID), nil, cb, 200)
	paths := []struct {
		method, path string
		body         any
	}{
		{"GET", fmt.Sprintf("/api/drawings/%d/detail", db.ID), nil},
		{"PUT", fmt.Sprintf("/api/drawings/%d", db.ID), map[string]any{"drawingNo": "stolen"}},
		{"DELETE", fmt.Sprintf("/api/drawings/%d", db.ID), nil},
		{"POST", fmt.Sprintf("/api/drawings/%d/operations", db.ID), map[string]any{"opNo": 20}},
		{"PUT", fmt.Sprintf("/api/drawings/%d/operations/order", db.ID), map[string]any{"ids": []int64{op.ID}}},
		{"PUT", fmt.Sprintf("/api/operations/%d", op.ID), map[string]any{"opNo": 20}},
		{"DELETE", fmt.Sprintf("/api/operations/%d", op.ID), nil},
		{"POST", fmt.Sprintf("/api/operations/%d/programs", op.ID), map[string]any{"programNo": "forged"}},
		{"PUT", fmt.Sprintf("/api/operations/%d/programs/order", op.ID), map[string]any{"ids": []int64{p.ID}}},
		{"GET", fmt.Sprintf("/api/programs/%d", p.ID), nil},
		{"PUT", fmt.Sprintf("/api/programs/%d", p.ID), map[string]any{"programNo": "forged", "expectedRevision": p.Revision}},
		{"DELETE", fmt.Sprintf("/api/programs/%d", p.ID), nil},
		{"GET", fmt.Sprintf("/api/programs/%d/versions", p.ID), nil},
		{"POST", fmt.Sprintf("/api/programs/%d/versions", p.ID), nil},
		{"POST", fmt.Sprintf("/api/programs/%d/versions/content", p.ID), map[string]any{"content": "forged"}},
		{"PUT", fmt.Sprintf("/api/programs/%d/current-version", p.ID), map[string]any{"versionId": v.ID}},
		{"GET", fmt.Sprintf("/api/versions/%d/content", v.ID), nil},
		{"GET", fmt.Sprintf("/api/versions/%d/download", v.ID), nil},
		{"PUT", fmt.Sprintf("/api/versions/%d/content", v.ID), map[string]any{"content": "forged"}},
		{"GET", fmt.Sprintf("/api/versions/%d/diff?with=%d", v.ID, v.ID), nil},
		{"GET", fmt.Sprintf("/api/versions/%d/tools", v.ID), nil},
		{"GET", fmt.Sprintf("/api/versions/%d/tools/recognize", v.ID), nil},
		{"PUT", fmt.Sprintf("/api/versions/%d/tools", v.ID), map[string]any{"items": []any{}, "expectedRevision": "forged"}},
	}
	for _, x := range paths {
		rec := request(x.method, x.path, x.body, ca, 404)
		if strings.Contains(rec.Body.String(), "private") || strings.Contains(rec.Body.String(), "PRIVATE-B") {
			t.Fatal("rejection leaked target")
		}
	}
	request("GET", fmt.Sprintf("/api/drawings/%d/detail", db.ID), nil, cb, 200)
	decode(t, request("GET", "/api/drawings?q=PRIVATE-B", nil, ca, 200), &list)
	if list.Total != 0 {
		t.Fatal("search leaked B")
	}
	for _, path := range []string{"/api/admin/users", "/api/admin/audit-logs", "/api/backup/recoveries"} {
		request("GET", path, nil, ca, 403)
	}
	var oa domain.Operation
	decode(t, request("POST", fmt.Sprintf("/api/drawings/%d/operations", da.ID), map[string]any{"opNo": 10}, ca, 201), &oa)
	var pa domain.Program
	decode(t, request("POST", fmt.Sprintf("/api/operations/%d/programs", oa.ID), map[string]any{"programNo": "A"}, ca, 201), &pa)
	var va domain.Version
	decode(t, request("POST", fmt.Sprintf("/api/programs/%d/versions/content", pa.ID), map[string]any{"content": "A\n", "encoding": "utf-8"}, ca, 201), &va)
	request("GET", fmt.Sprintf("/api/versions/%d/diff?with=%d", va.ID, v.ID), nil, ca, 404)
	request("PUT", fmt.Sprintf("/api/programs/%d/current-version", pa.ID), map[string]any{"versionId": v.ID}, ca, 404)
	request("PUT", fmt.Sprintf("/api/drawings/%d/operations/order", da.ID), map[string]any{"ids": []int64{op.ID}}, ca, 400)
	// Identity changes revoke sessions without transferring ownership.
	request("PUT", fmt.Sprintf("/api/drawings/%d", da.ID), map[string]any{"drawingNo": "OWN-EDIT"}, ca, 200)
	request("GET", fmt.Sprintf("/api/versions/%d/download", va.ID), nil, ca, 200)
	request("PUT", fmt.Sprintf("/api/admin/users/%d", ua.ID), domain.UpdateUserInput{DisplayName: ua.DisplayName, Role: "admin", Enabled: true}, admin, 200)
	request("GET", "/api/drawings", nil, ca, 401)
	ca = sessionCookie(t, request("POST", "/api/auth/login", map[string]string{"username": ua.Username, "password": "password123"}, nil, 200))
	request("GET", fmt.Sprintf("/api/drawings/%d/detail", db.ID), nil, ca, 200)
	request("PUT", fmt.Sprintf("/api/admin/users/%d", ua.ID), domain.UpdateUserInput{DisplayName: ua.DisplayName, Role: "editor", Enabled: true}, admin, 200)
	request("GET", "/api/drawings", nil, ca, 401)
	ca = sessionCookie(t, request("POST", "/api/auth/login", map[string]string{"username": ua.Username, "password": "password123"}, nil, 200))
	request("GET", fmt.Sprintf("/api/drawings/%d/detail", da.ID), nil, ca, 200)
	request("GET", fmt.Sprintf("/api/drawings/%d/detail", db.ID), nil, ca, 404)
	request("POST", fmt.Sprintf("/api/admin/users/%d/reset-password", ua.ID), map[string]string{"password": "replacement123"}, admin, 204)
	request("GET", "/api/drawings", nil, ca, 401)
	request("POST", "/api/auth/login", map[string]string{"username": ua.Username, "password": "password123"}, nil, 401)
	var logs domain.Page[domain.AuditEntry]
	decode(t, request("GET", fmt.Sprintf("/api/admin/audit-logs?userId=%d&result=failed&size=200", ua.ID), nil, admin, 200), &logs)
	if logs.Total < len(paths) {
		t.Fatalf("missing rejection logs: %d", logs.Total)
	}
	for _, e := range logs.Items {
		if e.EntityName != "" || strings.Contains(e.Detail, "password123") {
			t.Fatal("sensitive failed log", e)
		}
	}
	decode(t, request("GET", fmt.Sprintf("/api/admin/audit-logs?userId=%d&action=download", ub.ID), nil, admin, 200), &logs)
	if logs.Total != 1 || logs.Items[0].OwnerUserID == nil || *logs.Items[0].OwnerUserID != ub.ID {
		t.Fatal("download ownership missing", logs)
	}
	request("PUT", fmt.Sprintf("/api/admin/users/%d", ub.ID), domain.UpdateUserInput{DisplayName: ub.DisplayName, Role: "editor", Enabled: false}, admin, 200)
	request("GET", fmt.Sprintf("/api/drawings/%d/detail", db.ID), nil, admin, 200)
	request("GET", "/api/drawings", nil, cb, 401)
	request("POST", "/api/admin/roles", map[string]any{"name": "removed"}, admin, 404)
}
