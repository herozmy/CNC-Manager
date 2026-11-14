package httpapi

import (
	"cnc-manager/internal/domain"
	"encoding/json"
	"fmt"
	"testing"
)

func TestVersionToolAPI(t *testing.T) {
	f := newFixture(t, 1<<20)
	a := f.uploadOK(t, "a.nc", []byte("O1\nT01 M06\nG43 H01\nM30\n"))
	b := f.uploadOK(t, "b.nc", []byte("O2\nT02 M06\nG43 H02\nM30\n"))
	path := fmt.Sprintf("/api/versions/%d/tools", a.ID)
	if rec := do(t, f.h, "GET", path, ""); rec.Code != 401 {
		t.Fatalf("未授权 %d", rec.Code)
	}
	var initial domain.ToolTable
	decode(t, f.get(t, path), &initial)
	if len(initial.Items) != 0 || len(initial.Revision) != 64 {
		t.Fatalf("%+v", initial)
	}
	var parsed domain.ParseResult
	decode(t, f.get(t, path+"/recognize"), &parsed)
	if len(parsed.Tools) != 1 || parsed.Tools[0].ToolNo != "T01" || parsed.Tools[0].OffsetNo != "H01" {
		t.Fatalf("%+v", parsed)
	}
	var stillEmpty domain.ToolTable
	decode(t, f.get(t, path), &stillEmpty)
	if len(stillEmpty.Items) != 0 {
		t.Fatal("识别不能自动写入")
	}
	in := domain.ToolTableInput{ExpectedRevision: initial.Revision, Items: []domain.ProgramToolInput{{ToolNo: "T01", CompAmount: -.025, ToolDia: 12.345, CustomParams: []domain.ToolCustomParam{{Name: "品牌", Value: "测试"}}}}}
	raw, _ := json.Marshal(in)
	rec := do(t, f.h, "PUT", path, string(raw), f.ck)
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var saved domain.ToolTable
	decode(t, rec, &saved)
	if saved.Items[0].VersionID != a.ID || saved.Items[0].CompAmount != -.025 || saved.Items[0].ToolDia != 12.345 {
		t.Fatalf("%+v", saved)
	}
	if rec := do(t, f.h, "PUT", path, string(raw), f.ck); rec.Code != 409 {
		t.Fatalf("冲突 %d", rec.Code)
	}
	var other domain.ToolTable
	decode(t, f.get(t, fmt.Sprintf("/api/versions/%d/tools", b.ID)), &other)
	if len(other.Items) != 0 {
		t.Fatal("影响其他版本")
	}
	for _, body := range []string{`{}`, `{"items":null}`, `{"items":[],"expectedRevision":"bad"}`, fmt.Sprintf(`{"items":[{"toolNo":"T01","feedMode":3}],"expectedRevision":%q}`, saved.Revision)} {
		if rec := do(t, f.h, "PUT", path, body, f.ck); rec.Code != 400 {
			t.Fatalf("未拒绝 %s: %d", body, rec.Code)
		}
	}
	for _, path := range []string{"/api/versions/9999/tools", "/api/versions/9999/tools/recognize"} {
		if rec := f.get(t, path); rec.Code != 404 {
			t.Fatalf("%s %d", path, rec.Code)
		}
	}
}
