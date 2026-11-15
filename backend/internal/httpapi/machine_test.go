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

func TestDrawingMachineAssociation(t *testing.T) {
	h, _ := newTestServer(t)
	admin := setupAdmin(t, h)
	request := func(method, path string, body any, cookie *http.Cookie, want int) *httptest.ResponseRecorder {
		t.Helper()
		raw := ""
		if body != nil {
			b, _ := json.Marshal(body)
			raw = string(b)
		}
		cookies := []*http.Cookie{}
		if cookie != nil {
			cookies = append(cookies, cookie)
		}
		rec := do(t, h, method, path, raw, cookies...)
		if rec.Code != want {
			t.Fatalf("%s %s got %d want %d: %s", method, path, rec.Code, want, rec.Body.String())
		}
		return rec
	}
	member := func(name string) *http.Cookie {
		request("POST", "/api/admin/users", domain.CreateUserInput{Username: name, Password: "password123", Role: "editor"}, admin, 201)
		return sessionCookie(t, request("POST", "/api/auth/login", map[string]string{"username": name, "password": "password123"}, nil, 200))
	}
	a, b := member("machine_a"), member("machine_b")
	drawing := func(cookie *http.Cookie, no string) domain.Drawing {
		var d domain.Drawing
		decode(t, request("POST", "/api/drawings", map[string]string{"drawingNo": no}, cookie, 201), &d)
		return d
	}
	da, db := drawing(a, "A"), drawing(b, "B")
	pa, pb := fmt.Sprintf("/api/drawings/%d/machines", da.ID), fmt.Sprintf("/api/drawings/%d/machines", db.ID)
	request("GET", pa, nil, nil, 401)
	request("POST", pa, domain.MachineInput{Code: "x"}, nil, 401)
	request("GET", pa, nil, b, 404)
	request("POST", pa, domain.MachineInput{Code: "x"}, b, 404)
	request("GET", "/api/drawings/99999/machines", nil, admin, 404)
	if rec := request("GET", pa, nil, a, 200); strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatal(rec.Body.String())
	}
	var ma, mb, other domain.MachineOption
	decode(t, request("POST", pa, domain.MachineInput{Code: " CNC-01 ", Name: " 车床 "}, admin, 201), &ma)
	decode(t, request("POST", pb, domain.MachineInput{Code: "CNC-01", Name: "B私有"}, b, 201), &mb)
	decode(t, request("POST", pa, domain.MachineInput{Code: "CNC-02"}, a, 201), &other)
	if ma.Code != "CNC-01" || ma.Name != "车床" {
		t.Fatal(ma)
	}
	request("POST", pa, domain.MachineInput{Code: "CNC-01"}, a, 409)
	for _, in := range []domain.MachineInput{{Code: " "}, {Code: strings.Repeat("x", 65)}, {Code: "x", Name: strings.Repeat("中", 101)}} {
		request("POST", pa, in, a, 400)
	}
	da2 := drawing(a, "A2")
	for _, cookie := range []*http.Cookie{a, admin} {
		var list []domain.MachineOption
		decode(t, request("GET", fmt.Sprintf("/api/drawings/%d/machines", da2.ID), nil, cookie, 200), &list)
		if len(list) != 2 || list[0].ID != ma.ID || list[1].ID != other.ID {
			t.Fatal(list)
		}
	}
	var op domain.Operation
	input := domain.OperationInput{OpNo: 10, MachineID: &ma.ID, ZHeight: 25.5}
	decode(t, request("POST", fmt.Sprintf("/api/drawings/%d/operations", da.ID), input, a, 201), &op)
	if op.MachineCode != ma.Code || op.MachineName != ma.Name {
		t.Fatal(op)
	}
	var program domain.Program
	decode(t, request("POST", fmt.Sprintf("/api/operations/%d/programs", op.ID), domain.ProgramInput{ProgramNo: "O100", Controller: "FANUC"}, a, 201), &program)
	path := fmt.Sprintf("/api/operations/%d", op.ID)
	input.MachineID = &mb.ID
	request("PUT", path, input, a, 400)
	request("PUT", path, input, admin, 400)
	input.MachineID = &other.ID
	decode(t, request("PUT", path, input, a, 200), &op)
	if op.MachineID == nil || *op.MachineID != other.ID || op.ZHeight != 25.5 {
		t.Fatal(op)
	}
	input.MachineID = nil
	decode(t, request("PUT", path, input, a, 200), &op)
	if op.MachineID != nil || op.MachineCode != "" || op.MachineName != "" {
		t.Fatal(op)
	}
	var after domain.Program
	decode(t, request("GET", fmt.Sprintf("/api/programs/%d", program.ID), nil, a, 200), &after)
	if after != program {
		t.Fatal("machine association changed program", program, after)
	}
	var logs domain.Page[domain.AuditEntry]
	decode(t, request("GET", "/api/admin/audit-logs?entityType=machine", nil, admin, 200), &logs)
	found := false
	for _, e := range logs.Items {
		if e.EntityID == ma.ID && e.Result == "success" {
			found = e.OwnerUserID != nil && *e.OwnerUserID == da.OwnerUserID && e.EntityName == ma.Code
		}
	}
	if !found {
		t.Fatal("missing machine owner/name audit snapshot", logs)
	}
	legacy := drawing(admin, "unassigned")
	request("POST", fmt.Sprintf("/api/drawings/%d/machines", legacy.ID), domain.MachineInput{Code: "legacy"}, admin, 400)
}
