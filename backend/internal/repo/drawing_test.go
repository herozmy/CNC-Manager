package repo

import (
	"context"
	"errors"
	"testing"

	"cnc-manager/internal/domain"
)

func seedDrawing(t *testing.T, r *Repo, no string) *domain.Drawing {
	t.Helper()
	d, err := r.CreateDrawing(context.Background(), domain.DrawingInput{DrawingNo: no, Name: "零件" + no})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDrawingCRUDAndSearch(t *testing.T) {
	r := newTestRepo(t)
	ctx := domain.WithUser(context.Background(), domain.User{Username: "alice"})

	d, err := r.CreateDrawing(ctx, domain.DrawingInput{DrawingNo: " D-100 ", Name: "壳体", Customer: "客户A", Material: "6061"})
	if err != nil || d.DrawingNo != "D-100" || d.ID == 0 {
		t.Fatalf("新建失败: %+v %v", d, err)
	}
	if _, err := r.CreateDrawing(ctx, domain.DrawingInput{DrawingNo: "D-100"}); !errors.Is(err, domain.ErrConflict) || err == nil || err.Error() == "" {
		t.Fatalf("重复图纸号应冲突: %v", err)
	}
	if _, err := r.CreateDrawing(ctx, domain.DrawingInput{DrawingNo: ""}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal("空图纸号应为参数错误")
	}
	seedDrawing(t, r, "D-200")
	op, _ := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: 10, OpName: "粗车"})
	if _, err := r.CreateProgram(ctx, op.ID, domain.ProgramInput{ProgramNo: "O7788"}); err != nil {
		t.Fatal(err)
	}

	page, err := r.ListDrawings(ctx, "", 1, 10)
	if err != nil || page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("列表异常: %+v %v", page, err)
	}
	for _, kw := range []string{"d-1", "壳体", "6061", "O7788"} {
		page, _ = r.ListDrawings(ctx, kw, 1, 10)
		if page.Total != 1 || page.Items[0].ID != d.ID {
			t.Errorf("关键字 %q 应命中 D-100，得到 %+v", kw, page)
		}
	}
	page, err = r.ListDrawings(ctx, "客户A", 1, 10)
	if err != nil || page.Total != 0 {
		t.Fatalf("已停用的客户字段不应参与搜索: %+v %v", page, err)
	}
	page, _ = r.ListDrawings(ctx, "D_1", 1, 10)
	if page.Total != 0 {
		t.Error("下划线应被转义，不应命中")
	}
	page, _ = r.ListDrawings(ctx, "", 2, 1)
	if page.Total != 2 || len(page.Items) != 1 || page.Page != 2 {
		t.Errorf("分页异常: %+v", page)
	}
	if page.Items[0].OperationCount+page.Items[0].ProgramCount == 0 && page.Items[0].ID == d.ID {
		t.Error("计数未统计")
	}

	got, _ := r.GetDrawing(ctx, d.ID)
	if got.OperationCount != 1 || got.ProgramCount != 1 {
		t.Fatalf("计数错误: %+v", got)
	}

	upd, err := r.UpdateDrawing(ctx, d.ID, domain.DrawingInput{DrawingNo: "D-100", Name: "新名"})
	if err != nil || upd.Name != "新名" {
		t.Fatalf("修改失败: %v", err)
	}
	if _, err := r.UpdateDrawing(ctx, 9999, domain.DrawingInput{DrawingNo: "X"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("修改不存在的图纸应 NotFound")
	}

	if err := r.DeleteDrawing(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetOperation(ctx, op.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("删除图纸应级联删除工序")
	}
	if err := r.DeleteDrawing(ctx, d.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("重复删除应 NotFound")
	}
	var actor string
	_ = r.db.Get(&actor, `SELECT actor FROM audit_log WHERE action = 'delete' AND entity_type = 'drawing'`)
	if actor != "alice" {
		t.Fatalf("日志应记录操作人，得到 %q", actor)
	}
}

func TestOperationLifecycleAndReorder(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	d := seedDrawing(t, r, "D-1")

	a, err := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: 10, OpName: "粗车", ZHeight: 25.5, Fixture: "三爪"})
	if err != nil || a.ZHeight != 25.5 || a.SortOrder != 1 {
		t.Fatalf("新增工序失败: %+v %v", a, err)
	}
	b, _ := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: 20, OpName: "精车"})
	if b.SortOrder != 2 {
		t.Fatalf("排序应递增: %d", b.SortOrder)
	}
	if _, err := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: 10}); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("同图纸重复工序号应冲突")
	}
	if _, err := r.CreateOperation(ctx, 999, domain.OperationInput{OpNo: 10}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("不存在的图纸应 NotFound")
	}
	bad := int64(12345)
	if _, err := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: 30, MachineID: &bad}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("不存在的机台应为参数错误，得到 %v", err)
	}

	if err := r.ReorderOperations(ctx, d.ID, []int64{b.ID, a.ID}); err != nil {
		t.Fatal(err)
	}
	ops, _ := r.ListOperations(ctx, d.ID)
	if ops[0].ID != b.ID || ops[1].ID != a.ID {
		t.Fatal("重排未生效")
	}
	for _, ids := range [][]int64{{a.ID}, {a.ID, a.ID}, {a.ID, 999}} {
		if err := r.ReorderOperations(ctx, d.ID, ids); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("非法排序列表 %v 应被拒绝", ids)
		}
	}

	upd, err := r.UpdateOperation(ctx, a.ID, domain.OperationInput{OpNo: 15, OpName: "改名", ZHeight: 1.0005})
	if err != nil || upd.OpNo != 15 || upd.ZHeight != 1.001 {
		t.Fatalf("修改工序失败: %+v %v", upd, err)
	}
	if err := r.DeleteOperation(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetOperation(ctx, a.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("删除后应 NotFound")
	}
}

func TestProgramLifecycleAndDetail(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	d := seedDrawing(t, r, "D-1")
	op, _ := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: 10})
	empty, _ := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: 20})

	p1, err := r.CreateProgram(ctx, op.ID, domain.ProgramInput{ProgramNo: " O1 ", Controller: "FANUC"})
	if err != nil || p1.ProgramNo != "O1" || p1.VersionCount != 0 || p1.CurrentVersionID != nil {
		t.Fatalf("新增程序失败: %+v %v", p1, err)
	}
	p2, _ := r.CreateProgram(ctx, op.ID, domain.ProgramInput{ProgramNo: "O2"})
	if _, err := r.CreateProgram(ctx, 999, domain.ProgramInput{ProgramNo: "O3"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("不存在的工序应 NotFound")
	}
	if err := r.ReorderPrograms(ctx, op.ID, []int64{p2.ID, p1.ID}); err != nil {
		t.Fatal(err)
	}

	det, err := r.DrawingDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(det.Operations) != 2 || len(det.Operations[0].Programs) != 2 || det.Operations[0].Programs[0].ID != p2.ID {
		t.Fatalf("详情结构异常: %+v", det)
	}
	if det.Operations[1].ID != empty.ID || det.Operations[1].Programs == nil || len(det.Operations[1].Programs) != 0 {
		t.Fatal("无程序的工序应返回空数组而非 null")
	}
	if _, err := r.DrawingDetail(ctx, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("不存在的图纸详情应 NotFound")
	}

	upd, err := r.UpdateProgram(ctx, p1.ID, domain.ProgramInput{ProgramNo: "O1-new", Remark: "r"})
	if err != nil || upd.ProgramNo != "O1-new" {
		t.Fatal(err)
	}
	if err := r.DeleteProgram(ctx, p1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetProgram(ctx, p1.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("删除后应 NotFound")
	}
}
