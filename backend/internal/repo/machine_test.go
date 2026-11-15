package repo

import (
	"cnc-manager/internal/domain"
	"testing"
)

func TestMachineCreationAuditRollback(t *testing.T) {
	r := newTestRepo(t)
	ctx := testActor(t, r, "admin")
	u, _ := domain.UserFromContext(ctx)
	d, err := r.CreateDrawing(ctx, domain.DrawingInput{DrawingNo: "M", OwnerUserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.Exec("CREATE TRIGGER reject_machine_audit BEFORE INSERT ON audit_log BEGIN SELECT RAISE(ABORT,'audit failed'); END"); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CreateDrawingMachine(ctx, d.ID, domain.MachineInput{Code: "rollback"}); err == nil {
		t.Fatal("creation must fail")
	}
	items, err := r.DrawingMachines(ctx, d.ID)
	if err != nil || len(items) != 0 {
		t.Fatal(items, err)
	}
}
