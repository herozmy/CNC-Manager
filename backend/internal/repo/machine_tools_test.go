package repo

import (
	"context"
	"path/filepath"
	"testing"

	"cnccool/internal/domain"
	"cnccool/internal/store"
)

func TestReplaceMachineTools(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "machine-tools.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	r := New(db)
	m, err := r.CreateMachine(ctx, domain.MachineInput{Code: "MC01", Name: "机床1"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.ReplaceMachineTools(ctx, m.ID, []domain.MachineToolInput{
		{PocketNo: 8, ToolNo: "T08", ToolName: "钻头"},
		{PocketNo: 2, ToolNo: "T02", ToolName: "立铣刀"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].PocketNo != 2 || got[1].ToolNo != "T08" {
		t.Fatalf("刀库保存或排序错误：%+v", got)
	}
	got, err = r.ReplaceMachineTools(ctx, m.ID, []domain.MachineToolInput{{PocketNo: 3, ToolNo: "T03"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].PocketNo != 3 {
		t.Fatalf("整表替换后刀库 = %+v", got)
	}
}

func TestDeleteMachineCascadesMagazine(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "cascade.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	r := New(db)
	m, err := r.CreateMachine(ctx, domain.MachineInput{Code: "MC02", Name: "机床2"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.ReplaceMachineTools(ctx, m.ID, []domain.MachineToolInput{{PocketNo: 1}}); err != nil {
		t.Fatal(err)
	}
	if err = r.DeleteMachine(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.Get(&count, `SELECT COUNT(*) FROM machine_tool WHERE machine_id = ?`, m.ID); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("删除机床后仍有 %d 条刀库记录", count)
	}
}
