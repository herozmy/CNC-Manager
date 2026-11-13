package httpapi

import (
	"fmt"
	"net/http"
	"testing"

	"cnc-manager/internal/domain"
)

func TestCatalogRequiresLogin(t *testing.T) {
	h, _ := newTestServer(t)
	cases := []struct{ method, path string }{
		{"GET", "/api/drawings"}, {"POST", "/api/drawings"}, {"GET", "/api/drawings/1/detail"},
		{"PUT", "/api/drawings/1"}, {"DELETE", "/api/drawings/1"},
		{"POST", "/api/drawings/1/operations"}, {"PUT", "/api/drawings/1/operations/order"},
		{"PUT", "/api/operations/1"}, {"DELETE", "/api/operations/1"},
		{"POST", "/api/operations/1/programs"}, {"PUT", "/api/operations/1/programs/order"},
		{"GET", "/api/programs/1"}, {"PUT", "/api/programs/1"}, {"DELETE", "/api/programs/1"},
	}
	for _, c := range cases {
		if rec := do(t, h, c.method, c.path, "{}"); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s 未登录应 401，得到 %d", c.method, c.path, rec.Code)
		}
	}
}

func TestDrawingEndpoints(t *testing.T) {
	h, _ := newTestServer(t)
	ck := setupAdmin(t, h)

	rec := do(t, h, "POST", "/api/drawings", `{"drawingNo":"D-1","name":"壳体"}`, ck)
	var d domain.Drawing
	decode(t, rec, &d)
	if rec.Code != 201 || d.ID == 0 || d.DrawingNo != "D-1" {
		t.Fatalf("新建失败: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, "POST", "/api/drawings", `{"drawingNo":"D-1"}`, ck); rec.Code != 409 {
		t.Fatalf("重复图纸号应 409，得到 %d", rec.Code)
	}
	if rec := do(t, h, "POST", "/api/drawings", `{"drawingNo":""}`, ck); rec.Code != 400 {
		t.Fatalf("空图纸号应 400，得到 %d", rec.Code)
	}

	var page domain.Page[domain.Drawing]
	decode(t, do(t, h, "GET", "/api/drawings?q=%E5%A3%B3&page=1&size=10", "", ck), &page)
	if page.Total != 1 || page.Items[0].ID != d.ID {
		t.Fatalf("搜索异常: %+v", page)
	}
	decode(t, do(t, h, "GET", "/api/drawings?page=abc&size=-5", "", ck), &page)
	if page.Page != 1 || page.Total != 1 {
		t.Fatalf("非法分页参数应回退默认: %+v", page)
	}

	rec = do(t, h, "PUT", fmt.Sprintf("/api/drawings/%d", d.ID), `{"drawingNo":"D-1","name":"新名"}`, ck)
	decode(t, rec, &d)
	if rec.Code != 200 || d.Name != "新名" {
		t.Fatalf("修改失败: %d", rec.Code)
	}
	if rec := do(t, h, "PUT", "/api/drawings/999", `{"drawingNo":"X"}`, ck); rec.Code != 404 {
		t.Fatalf("不存在应 404，得到 %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/api/drawings/abc/detail", "", ck); rec.Code != 400 {
		t.Fatalf("非法 ID 应 400，得到 %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/api/drawings/0/detail", "", ck); rec.Code != 400 {
		t.Fatalf("ID 0 应 400，得到 %d", rec.Code)
	}
	if rec := do(t, h, "DELETE", fmt.Sprintf("/api/drawings/%d", d.ID), "", ck); rec.Code != 204 {
		t.Fatalf("删除应 204，得到 %d", rec.Code)
	}
	if rec := do(t, h, "GET", fmt.Sprintf("/api/drawings/%d/detail", d.ID), "", ck); rec.Code != 404 {
		t.Fatalf("删除后详情应 404，得到 %d", rec.Code)
	}
}

func TestOperationAndProgramFlow(t *testing.T) {
	h, _ := newTestServer(t)
	ck := setupAdmin(t, h)
	var d domain.Drawing
	decode(t, do(t, h, "POST", "/api/drawings", `{"drawingNo":"D-1"}`, ck), &d)
	base := fmt.Sprintf("/api/drawings/%d", d.ID)

	var o1, o2 domain.Operation
	decode(t, do(t, h, "POST", base+"/operations", `{"opNo":10,"opName":"粗车","zHeight":25.5,"fixture":"三爪"}`, ck), &o1)
	rec := do(t, h, "POST", base+"/operations", `{"opNo":20,"opName":"精车"}`, ck)
	decode(t, rec, &o2)
	if rec.Code != 201 || o1.ZHeight != 25.5 || o2.SortOrder != 2 {
		t.Fatalf("工序创建异常: %+v %+v", o1, o2)
	}
	if rec := do(t, h, "POST", base+"/operations", `{"opNo":10}`, ck); rec.Code != 409 {
		t.Fatalf("重复工序号应 409，得到 %d", rec.Code)
	}
	if rec := do(t, h, "POST", base+"/operations", `{"opNo":0}`, ck); rec.Code != 400 {
		t.Fatalf("工序号 0 应 400，得到 %d", rec.Code)
	}
	if rec := do(t, h, "POST", "/api/drawings/999/operations", `{"opNo":10}`, ck); rec.Code != 404 {
		t.Fatalf("图纸不存在应 404，得到 %d", rec.Code)
	}

	if rec := do(t, h, "PUT", base+"/operations/order", fmt.Sprintf(`{"ids":[%d,%d]}`, o2.ID, o1.ID), ck); rec.Code != 204 {
		t.Fatalf("排序应 204，得到 %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, "PUT", base+"/operations/order", `{"ids":[1]}`, ck); rec.Code != 400 {
		t.Fatalf("不完整排序应 400，得到 %d", rec.Code)
	}

	opPath := fmt.Sprintf("/api/operations/%d", o1.ID)
	var prog domain.Program
	rec = do(t, h, "POST", opPath+"/programs", `{"programNo":"O1000","controller":"FANUC"}`, ck)
	decode(t, rec, &prog)
	if rec.Code != 201 || prog.ProgramNo != "O1000" || prog.CurrentVersionID != nil {
		t.Fatalf("程序创建异常: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, "POST", opPath+"/programs", `{"programNo":""}`, ck); rec.Code != 400 {
		t.Fatalf("空程序号应 400，得到 %d", rec.Code)
	}
	if rec := do(t, h, "PUT", opPath+"/programs/order", fmt.Sprintf(`{"ids":[%d]}`, prog.ID), ck); rec.Code != 204 {
		t.Fatalf("程序排序应 204，得到 %d", rec.Code)
	}

	var detail domain.DrawingDetail
	decode(t, do(t, h, "GET", base+"/detail", "", ck), &detail)
	if len(detail.Operations) != 2 || detail.Operations[0].ID != o2.ID || len(detail.Operations[1].Programs) != 1 {
		t.Fatalf("详情异常: %+v", detail)
	}
	if detail.ProgramCount != 1 || detail.Operations[0].Programs == nil {
		t.Fatal("详情计数或空数组异常")
	}

	progPath := fmt.Sprintf("/api/programs/%d", prog.ID)
	decode(t, do(t, h, "PUT", progPath, `{"programNo":"O1000A"}`, ck), &prog)
	if prog.ProgramNo != "O1000A" {
		t.Fatal("程序修改失败")
	}
	if rec := do(t, h, "GET", progPath, "", ck); rec.Code != 200 {
		t.Fatalf("取程序应 200，得到 %d", rec.Code)
	}
	if rec := do(t, h, "DELETE", progPath, "", ck); rec.Code != 204 {
		t.Fatalf("删除程序应 204，得到 %d", rec.Code)
	}
	if rec := do(t, h, "GET", progPath, "", ck); rec.Code != 404 {
		t.Fatalf("删除后应 404，得到 %d", rec.Code)
	}

	if rec := do(t, h, "PUT", opPath, `{"opNo":15,"opName":"改"}`, ck); rec.Code != 200 {
		t.Fatalf("工序修改应 200，得到 %d", rec.Code)
	}
	if rec := do(t, h, "DELETE", opPath, "", ck); rec.Code != 204 {
		t.Fatalf("删除工序应 204，得到 %d", rec.Code)
	}
	if rec := do(t, h, "DELETE", opPath, "", ck); rec.Code != 404 {
		t.Fatalf("重复删除应 404，得到 %d", rec.Code)
	}
}

func TestMalformedBodyAndOversizedBody(t *testing.T) {
	h, _ := newTestServer(t)
	ck := setupAdmin(t, h)
	if rec := do(t, h, "POST", "/api/drawings", `not json`, ck); rec.Code != 400 {
		t.Fatalf("坏 JSON 应 400，得到 %d", rec.Code)
	}
	big := `{"drawingNo":"X","remark":"` + string(make([]byte, 2<<20)) + `"}`
	if rec := do(t, h, "POST", "/api/drawings", big, ck); rec.Code != 400 {
		t.Fatalf("超大请求体应被拒绝，得到 %d", rec.Code)
	}
}
