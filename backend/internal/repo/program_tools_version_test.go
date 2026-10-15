package repo

import (
	"context"
	"path/filepath"
	"testing"

	"cnccool/internal/domain"
	"cnccool/internal/store"
)

func TestProgramToolsFollowCurrentVersion(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "tools.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	r := New(db)
	d, err := r.CreateDrawing(ctx, domain.DrawingInput{DrawingNo: "D1", Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	o, err := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: 10, OpName: "op"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := r.CreateProgram(ctx, o.ID, domain.ProgramInput{ProgramNo: "O1"})
	if err != nil {
		t.Fatal(err)
	}
	file1 := domain.NCFileInput{SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SizeBytes: 1, OriginalName: "v1.nc", RelPath: "a/v1.nc"}
	v1, err := r.AddVersion(ctx, p.ID, file1, "v1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReplaceVersionTools(ctx, p.ID, v1.ID, []domain.ProgramToolInput{{Seq: 1, ToolNo: "T01"}}); err != nil {
		t.Fatal(err)
	}
	file2 := domain.NCFileInput{SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", SizeBytes: 1, OriginalName: "v2.nc", RelPath: "b/v2.nc"}
	v2, err := r.AddVersion(ctx, p.ID, file2, "v2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReplaceVersionTools(ctx, p.ID, v2.ID, []domain.ProgramToolInput{{Seq: 1, ToolNo: "T02"}}); err != nil {
		t.Fatal(err)
	}

	current, err := r.ListProgramTools(ctx, p.ID)
	if err != nil || len(current) != 1 || current[0].ToolNo != "T02" {
		t.Fatalf("当前版本刀具 = %+v, err=%v，期望 T02", current, err)
	}
	if _, err := r.SetCurrentVersion(ctx, p.ID, v1.ID); err != nil {
		t.Fatal(err)
	}
	detail, err := r.DrawingDetail(ctx, d.ID)
	if err != nil {
		t.Fatalf("读取图纸详情失败（JOIN 列名必须无歧义）：%v", err)
	}
	if got := detail.Operations[0].Programs[0].Tools; len(got) != 1 || got[0].ToolNo != "T01" {
		t.Fatalf("图纸详情中的当前版本刀具 = %+v，期望 T01", got)
	}
	current, err = r.ListProgramTools(ctx, p.ID)
	if err != nil || len(current) != 1 || current[0].ToolNo != "T01" {
		t.Fatalf("切回第一版后的刀具 = %+v, err=%v，期望 T01", current, err)
	}
}

func TestReorderProgramsPersistsManualOrder(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "program-order.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	r := New(db)
	d, err := r.CreateDrawing(ctx, domain.DrawingInput{DrawingNo: "D2", Name: "order"})
	if err != nil {
		t.Fatal(err)
	}
	o, err := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: 10, OpName: "op"})
	if err != nil {
		t.Fatal(err)
	}
	p1, _ := r.CreateProgram(ctx, o.ID, domain.ProgramInput{ProgramNo: "O10"})
	p2, _ := r.CreateProgram(ctx, o.ID, domain.ProgramInput{ProgramNo: "O20"})
	p3, _ := r.CreateProgram(ctx, o.ID, domain.ProgramInput{ProgramNo: "O30"})
	if err := r.ReorderPrograms(ctx, o.ID, []int64{p3.ID, p1.ID, p2.ID}); err != nil {
		t.Fatal(err)
	}
	items, err := r.ListPrograms(ctx, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].ID != p3.ID || items[1].ID != p1.ID || items[2].ID != p2.ID {
		t.Fatalf("程序顺序 = %+v，期望 O30, O10, O20", items)
	}
	if err := r.ReorderPrograms(ctx, o.ID, []int64{p1.ID, p1.ID, p2.ID}); err == nil {
		t.Fatal("重复程序 ID 的排序应当被拒绝")
	}
}
