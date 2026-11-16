package httpapi

import (
	"cnc-manager/internal/domain"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMachineManagementIsolationAndValidation(t *testing.T) {
	h, _ := newTestServer(t)
	admin := setupAdmin(t, h)
	request := func(method, path string, body any, cookie *http.Cookie, want int) *httptest.ResponseRecorder {
		t.Helper()
		raw := ""
		if body != nil {
			b, _ := json.Marshal(body)
			raw = string(b)
		}
		var cookies []*http.Cookie
		if cookie != nil {
			cookies = append(cookies, cookie)
		}
		rec := do(t, h, method, path, raw, cookies...)
		if rec.Code != want {
			t.Fatalf("%s %s = %d want %d: %s", method, path, rec.Code, want, rec.Body.String())
		}
		return rec
	}
	member := func(name string) (domain.User, *http.Cookie) {
		var u domain.User
		decode(t, request("POST", "/api/admin/users", domain.CreateUserInput{Username: name, Password: "password123", Role: "editor"}, admin, 201), &u)
		return u, sessionCookie(t, request("POST", "/api/auth/login", map[string]string{"username": name, "password": "password123"}, nil, 200))
	}
	ua, a := member("owner_a")
	ub, b := member("owner_b")
	request("GET", "/api/machines", nil, nil, 401)
	request("POST", "/api/machines", domain.MachineInput{Code: "M"}, nil, 401)
	var ma, mb domain.Machine
	decode(t, request("POST", "/api/machines", domain.MachineInput{Code: " M ", Name: " 铣床 ", Controller: "FANUC", OwnerUserID: &ua.ID}, admin, 201), &ma)
	decode(t, request("POST", "/api/machines", domain.MachineInput{Code: "M"}, b, 201), &mb)
	if ma.Code != "M" || ma.Name != "铣床" || ma.OwnerUserID != ua.ID || !ma.Enabled || mb.OwnerUserID != ub.ID {
		t.Fatal(ma, mb)
	}
	request("POST", "/api/machines", domain.MachineInput{Code: "M"}, a, 409)
	request("POST", "/api/machines", domain.MachineInput{Code: "forbidden", OwnerUserID: &ub.ID}, a, 403)
	request("POST", "/api/machines", domain.MachineInput{Code: " "}, a, 400)
	zero := int64(0)
	request("POST", "/api/machines", domain.MachineInput{Code: "owner", OwnerUserID: &zero}, admin, 400)
	path := fmt.Sprintf("/api/machines/%d", ma.ID)
	for _, tc := range []struct {
		method, suffix string
		body           any
	}{{"DELETE", "", nil}, {"GET", "", nil}, {"GET", "/drawings", nil}, {"PUT", "", domain.MachineInput{Code: "steal"}}, {"PUT", "/enabled", map[string]bool{"enabled": false}}} {
		request(tc.method, path+tc.suffix, tc.body, b, 404)
	}
	request("PUT", path, domain.MachineInput{Code: "M", OwnerUserID: &ub.ID}, admin, 403)
	request("PUT", path+"/enabled", map[string]any{}, a, 400)
	request("PUT", path+"/enabled", map[string]any{"enabled": nil}, a, 400)
	request("GET", "/api/machines?enabled=invalid", nil, a, 400)
	request("GET", "/api/machines?ownerUserId=-1", nil, admin, 400)
	request("GET", fmt.Sprintf("/api/machines?ownerUserId=%d", ub.ID), nil, a, 403)
	var list domain.Page[domain.Machine]
	decode(t, request("GET", "/api/machines", nil, a, 200), &list)
	if list.Total != 1 || list.Items[0].ID != ma.ID {
		t.Fatal(list)
	}
	decode(t, request("GET", "/api/machines?q=FANUC", nil, admin, 200), &list)
	if list.Total != 1 || list.Items[0].ID != ma.ID {
		t.Fatal(list)
	}
	decode(t, request("PUT", path+"/enabled", map[string]bool{"enabled": false}, a, 200), &ma)
	if ma.Enabled {
		t.Fatal(ma)
	}
	decode(t, request("GET", "/api/machines?enabled=true", nil, a, 200), &list)
	if list.Total != 0 {
		t.Fatal(list)
	}
	decode(t, request("GET", fmt.Sprintf("/api/machines?enabled=false&ownerUserId=%d", ua.ID), nil, admin, 200), &list)
	if list.Total != 1 || list.Items[0].Enabled {
		t.Fatal(list)
	}
	decode(t, request("PUT", path, domain.MachineInput{Code: "edited", Controller: "SIEMENS", Remark: "保留"}, a, 200), &ma)
	if ma.Enabled || ma.Code != "edited" || ma.Controller != "SIEMENS" || ma.Remark != "保留" {
		t.Fatal(ma)
	}
	request("PUT", path+"/enabled", map[string]bool{"enabled": true}, admin, 200)
	var logs domain.Page[domain.AuditEntry]
	decode(t, request("GET", "/api/admin/audit-logs?entityType=machine&size=200", nil, admin, 200), &logs)
	found := map[string]bool{}
	for _, e := range logs.Items {
		if e.EntityID == ma.ID && e.Result == "success" {
			if e.OwnerUserID == nil || *e.OwnerUserID != ua.ID {
				t.Fatal(e)
			}
			found[e.Action] = true
		}
	}
	for _, action := range []string{"create", "update", "disable", "enable"} {
		if !found[action] {
			t.Fatalf("missing %s audit", action)
		}
	}
	request("PUT", fmt.Sprintf("/api/admin/users/%d", ub.ID), domain.UpdateUserInput{DisplayName: ub.DisplayName, Role: "editor", Enabled: false}, admin, 200)
	request("POST", "/api/machines", domain.MachineInput{Code: "disabled owner", OwnerUserID: &ub.ID}, admin, 400)
	request("DELETE", path, nil, nil, 401)
	request("DELETE", path, nil, a, 204)
	request("GET", path, nil, a, 404)
	request("DELETE", path, nil, a, 404)

}
