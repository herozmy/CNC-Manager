package httpapi

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"cnc-manager/internal/domain"
)

func TestDrawingTypeSearch(t *testing.T) {
	h, _ := newTestServer(t)
	ck := setupAdmin(t, h)
	for i, productType := range []string{"壳体类", "壳体类", "轴类", ""} {
		rec := do(t, h, "POST", "/api/drawings", fmt.Sprintf(`{"drawingNo":"TYPE-%d","name":%q}`, i, productType), ck)
		if rec.Code != http.StatusCreated {
			t.Fatalf("创建分类图纸失败: %d %s", rec.Code, rec.Body.String())
		}
	}
	for _, keyword := range []string{"壳体", "壳体类"} {
		var page domain.Page[domain.Drawing]
		rec := do(t, h, "GET", "/api/drawings?q="+url.QueryEscape(keyword), "", ck)
		decode(t, rec, &page)
		if rec.Code != http.StatusOK || page.Total != 2 || len(page.Items) != 2 {
			t.Fatalf("应找到两张同类型图纸: %+v", page)
		}
		for _, d := range page.Items {
			if d.Name != "壳体类" {
				t.Fatalf("搜索结果应返回类型: %+v", d)
			}
		}
	}
	var empty domain.Page[domain.Drawing]
	decode(t, do(t, h, "GET", "/api/drawings?q="+url.QueryEscape("不存在类型"), "", ck), &empty)
	if empty.Total != 0 || len(empty.Items) != 0 {
		t.Fatalf("不匹配的类型应返回空列表: %+v", empty)
	}
	rec := do(t, h, "POST", "/api/drawings", fmt.Sprintf(`{"drawingNo":"TOO-LONG","name":%q}`, strings.Repeat("型", 34)), ck)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "类型 不能超过") {
		t.Fatalf("类型超长应使用新字段名称提示: %d %s", rec.Code, rec.Body.String())
	}
}

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

func TestEditDrawingPreservesChildrenAndUpdatesSearch(t *testing.T) {
	h, _ := newTestServer(t)
	ck := setupAdmin(t, h)
	var original, other domain.Drawing
	decode(t, do(t, h, "POST", "/api/drawings", `{"drawingNo":"EDIT-OLD","name":"原类型"}`, ck), &original)
	decode(t, do(t, h, "POST", "/api/drawings", `{"drawingNo":"EXISTS"}`, ck), &other)
	base := fmt.Sprintf("/api/drawings/%d", original.ID)
	var op domain.Operation
	decode(t, do(t, h, "POST", base+"/operations", `{"opNo":10}`, ck), &op)
	var program domain.Program
	decode(t, do(t, h, "POST", fmt.Sprintf("/api/operations/%d/programs", op.ID), `{"programNo":"O1234"}`, ck), &program)

	if rec := do(t, h, "PUT", base, `{"drawingNo":"EXISTS","name":"不应保存"}`, ck); rec.Code != http.StatusConflict {
		t.Fatalf("重复图纸号应拒绝: %d %s", rec.Code, rec.Body.String())
	}
	var unchanged domain.DrawingDetail
	decode(t, do(t, h, "GET", base+"/detail", "", ck), &unchanged)
	if unchanged.DrawingNo != "EDIT-OLD" || unchanged.Name != "原类型" {
		t.Fatalf("修改失败不应改变原数据: %+v", unchanged)
	}

	rec := do(t, h, "PUT", base, `{"drawingNo":"EDIT-NEW","name":"轴类","material":"45#","remark":"新备注"}`, ck)
	var updated domain.Drawing
	decode(t, rec, &updated)
	if rec.Code != http.StatusOK || updated.ID != original.ID || updated.DrawingNo != "EDIT-NEW" || updated.Name != "轴类" || updated.Material != "45#" || updated.Remark != "新备注" {
		t.Fatalf("图纸信息未保存: %d %+v", rec.Code, updated)
	}
	var detail domain.DrawingDetail
	decode(t, do(t, h, "GET", base+"/detail", "", ck), &detail)
	if len(detail.Operations) != 1 || detail.Operations[0].ID != op.ID || len(detail.Operations[0].Programs) != 1 || detail.Operations[0].Programs[0].ID != program.ID {
		t.Fatalf("修改图纸不应改变其工序程序: %+v", detail)
	}
	for keyword, total := range map[string]int{"EDIT-NEW": 1, "轴类": 1, "EDIT-OLD": 0, "原类型": 0} {
		var page domain.Page[domain.Drawing]
		decode(t, do(t, h, "GET", "/api/drawings?q="+url.QueryEscape(keyword), "", ck), &page)
		if page.Total != total || len(page.Items) != total || (total == 1 && page.Items[0].ID != original.ID) {
			t.Fatalf("修改后搜索 %q 异常: %+v", keyword, page)
		}
	}
}

func TestOperationCoordinates(t *testing.T) {
	h, _ := newTestServer(t)
	ck := setupAdmin(t, h)
	var d domain.Drawing
	decode(t, do(t, h, "POST", "/api/drawings", `{"drawingNo":"XYZ"}`, ck), &d)
	base := fmt.Sprintf("/api/drawings/%d", d.ID)
	rec := do(t, h, "POST", base+"/operations", `{"opNo":10,"xCoordinate":-12.3456,"yCoordinate":0.1234,"zHeight":-25.5005}`, ck)
	var op domain.Operation
	decode(t, rec, &op)
	if rec.Code != 201 || op.XCoordinate != -12.346 || op.YCoordinate != 0.123 || op.ZHeight != -25.501 {
		t.Fatalf("三轴应支持负数并保留 0.001 mm 精度: %d %+v", rec.Code, op)
	}
	path := fmt.Sprintf("/api/operations/%d", op.ID)
	var detail domain.DrawingDetail
	decode(t, do(t, h, "GET", base+"/detail", "", ck), &detail)
	if len(detail.Operations) != 1 || detail.Operations[0].XCoordinate != op.XCoordinate || detail.Operations[0].YCoordinate != op.YCoordinate || detail.Operations[0].ZHeight != op.ZHeight {
		t.Fatalf("详情应返回已保存坐标: %+v", detail)
	}
	for _, field := range []string{"xCoordinate", "yCoordinate", "zHeight"} {
		for _, bad := range []string{"100000.001", "-100000.001", `"abc"`} {
			rec := do(t, h, "PUT", path, fmt.Sprintf(`{"opNo":10,%q:%s}`, field, bad), ck)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("非法 %s=%s 应拒绝: %d", field, bad, rec.Code)
			}
		}
	}
	decode(t, do(t, h, "GET", base+"/detail", "", ck), &detail)
	if detail.Operations[0].XCoordinate != op.XCoordinate || detail.Operations[0].YCoordinate != op.YCoordinate || detail.Operations[0].ZHeight != op.ZHeight {
		t.Fatal("非法坐标不应修改已保存值")
	}
	// 旧客户端只传 zHeight 时，新增 X/Y 不应被清空。
	decode(t, do(t, h, "PUT", path, `{"opNo":10,"zHeight":12.5}`, ck), &op)
	if op.XCoordinate != -12.346 || op.YCoordinate != 0.123 || op.ZHeight != 12.5 {
		t.Fatalf("旧请求应保留 X/Y: %+v", op)
	}
	decode(t, do(t, h, "PUT", path, `{"opNo":10,"xCoordinate":0,"yCoordinate":0,"zHeight":0}`, ck), &op)
	if op.XCoordinate != 0 || op.YCoordinate != 0 || op.ZHeight != 0 {
		t.Fatalf("显式 0 应能清空坐标: %+v", op)
	}
}

func TestOperationCenteringMethod(t *testing.T) {
	h, _ := newTestServer(t)
	ck := setupAdmin(t, h)
	var d domain.Drawing
	decode(t, do(t, h, "POST", "/api/drawings", `{"drawingNo":"CENTERING"}`, ck), &d)
	base := fmt.Sprintf("/api/drawings/%d", d.ID)
	var op domain.Operation
	rec := do(t, h, "POST", base+"/operations", `{"opNo":10,"centeringMethod":"  四面分中  ","xCoordinate":-12.345,"yCoordinate":6.5,"zHeight":25.5}`, ck)
	decode(t, rec, &op)
	if rec.Code != http.StatusCreated || op.CenteringMethod != "四面分中" {
		t.Fatalf("创建分中方式失败: %+v %s", op, rec.Body.String())
	}
	path := fmt.Sprintf("/api/operations/%d", op.ID)
	for _, tc := range []struct{ body, want string }{
		{`{"opNo":10,"zHeight":25.5,"centeringMethod":" 孔分中 "}`, "孔分中"},
		{`{"opNo":10,"zHeight":25.5}`, "孔分中"},
		{`{"opNo":10,"zHeight":25.5,"centeringMethod":null}`, "孔分中"},
		{`{"opNo":10,"zHeight":25.5,"centeringMethod":"  "}`, ""},
		{fmt.Sprintf(`{"opNo":10,"zHeight":25.5,"centeringMethod":%q}`, strings.Repeat("中", 200)), strings.Repeat("中", 200)},
	} {
		rec = do(t, h, "PUT", path, tc.body, ck)
		decode(t, rec, &op)
		if rec.Code != http.StatusOK || op.CenteringMethod != tc.want || op.XCoordinate != -12.345 || op.YCoordinate != 6.5 || op.ZHeight != 25.5 {
			t.Fatalf("修改或兼容保留失败: %+v %s", op, rec.Body.String())
		}
		var detail domain.DrawingDetail
		decode(t, do(t, h, "GET", base+"/detail", "", ck), &detail)
		if len(detail.Operations) != 1 || detail.Operations[0].CenteringMethod != tc.want {
			t.Fatalf("详情未返回分中方式: %+v", detail)
		}
	}
	rec = do(t, h, "PUT", path, fmt.Sprintf(`{"opNo":10,"centeringMethod":%q}`, strings.Repeat("中", 201)), ck)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "分中方式") {
		t.Fatalf("过长分中方式应拒绝: %d %s", rec.Code, rec.Body.String())
	}
	var detail domain.DrawingDetail
	decode(t, do(t, h, "GET", base+"/detail", "", ck), &detail)
	if detail.Operations[0].CenteringMethod != strings.Repeat("中", 200) || detail.Operations[0].ZHeight != 25.5 {
		t.Fatal("失败请求不应更改原值")
	}
	decode(t, do(t, h, "POST", base+"/operations", `{"opNo":20}`, ck), &op)
	if op.CenteringMethod != "" {
		t.Fatalf("旧客户端新增工序分中方式应为空: %+v", op)
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
