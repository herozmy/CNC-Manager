package repo

import (
	"context"
	"path/filepath"
	"testing"

	"cnccool/internal/domain"
	"cnccool/internal/store"
)

func TestReorderOperationsPersistsManualOrder(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "operation-order.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	r := New(db)
	d, err := r.CreateDrawing(ctx, domain.DrawingInput{DrawingNo: "D-ORDER", Name: "order"})
	if err != nil {
		t.Fatal(err)
	}
	o1, _ := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: 10, OpName: "10"})
	o2, _ := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: 20, OpName: "20"})
	o3, _ := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: 30, OpName: "30"})
	if err := r.ReorderOperations(ctx, d.ID, []int64{o3.ID, o1.ID, o2.ID}); err != nil {
		t.Fatal(err)
	}
	items, err := r.ListOperations(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].ID != o3.ID || items[1].ID != o1.ID || items[2].ID != o2.ID {
		t.Fatalf("工序顺序 = %+v，期望 30, 10, 20", items)
	}
	if err := r.ReorderOperations(ctx, d.ID, []int64{o1.ID, o1.ID, o2.ID}); err == nil {
		t.Fatal("重复工序 ID 的排序应当被拒绝")
	}
}
