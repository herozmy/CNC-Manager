package repo

import (
	"cnc-manager/internal/domain"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

func TestProgramRevisionAndAuditRollback(t *testing.T) {
	r := newTestRepo(t)
	ctx := testActor(t, r, "admin")
	p := newProgram(t, r, "CONCURRENT")
	if _, err := r.UpdateProgram(ctx, p.ID, domain.ProgramInput{ProgramNo: "missing"}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := r.UpdateProgram(ctx, p.ID, domain.ProgramInput{ProgramNo: fmt.Sprint("P", i), ExpectedRevision: &p.Revision})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	ok, conflicts := 0, 0
	for err := range results {
		if err == nil {
			ok++
		} else if errors.Is(err, domain.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || conflicts != 1 {
		t.Fatal(ok, conflicts)
	}
	saved, err := r.GetProgram(ctx, p.ID)
	if err != nil || saved.Revision != 2 {
		t.Fatal(saved, err)
	}
	if _, err := r.db.Exec(`CREATE TRIGGER reject_audit BEFORE INSERT ON audit_log BEGIN SELECT RAISE(ABORT,'audit failed'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.UpdateProgram(ctx, p.ID, domain.ProgramInput{ProgramNo: "must roll back", ExpectedRevision: &saved.Revision}); err == nil {
		t.Fatal("audit failure accepted")
	}
	if err := r.DeleteProgram(ctx, p.ID); err == nil {
		t.Fatal("delete escaped audit rollback")
	}
	if _, err := r.CreateDrawing(ctx, domain.DrawingInput{DrawingNo: "must not exist"}); err == nil {
		t.Fatal("create escaped audit rollback")
	}
	got, err := r.GetProgram(ctx, p.ID)
	if err != nil || got.ProgramNo != saved.ProgramNo || got.Revision != saved.Revision {
		t.Fatal(got, err)
	}
	var n int
	r.db.Get(&n, `SELECT COUNT(*) FROM drawing WHERE drawing_no='must not exist'`)
	if n != 0 {
		t.Fatal(n)
	}
}
func TestLastAdminConcurrentProtection(t *testing.T) {
	r := newTestRepo(t)
	ctx := testActor(t, r, "one")
	u1, _ := domain.UserFromContext(ctx)
	u2, err := r.CreateUser(ctx, domain.CreateUserInput{Username: "two", Password: "password123", Role: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, u := range []domain.User{u1, u2} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = r.UpdateUser(domain.WithUser(context.Background(), u), u.ID, domain.UpdateUserInput{DisplayName: u.DisplayName, Role: "admin", Enabled: false})
		}()
	}
	wg.Wait()
	var n int
	if err := r.db.Get(&n, `SELECT COUNT(*) FROM app_user WHERE role='admin' AND enabled=1`); err != nil || n != 1 {
		t.Fatal(n, err)
	}
}
func TestAssignmentAuditSnapshotsAndOwnerBackup(t *testing.T) {
	r := newTestRepo(t)
	ctx := testActor(t, r, "owner")
	a, err := r.CreateUser(ctx, domain.CreateUserInput{Username: "member", DisplayName: "旧名称", Password: "password123", Role: "editor"})
	if err != nil {
		t.Fatal(err)
	}
	d := seedDrawing(t, r, "LEGACY")
	if d.OwnerUserID != 0 {
		t.Fatal(d)
	}
	assigned, err := r.AssignDrawingOwner(ctx, d.ID, a.ID)
	if err != nil || assigned.OwnerUserID != a.ID {
		t.Fatal(assigned, err)
	}
	if _, err := r.AssignDrawingOwner(ctx, d.ID, a.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatal(err)
	}
	_, err = r.UpdateUser(ctx, a.ID, domain.UpdateUserInput{DisplayName: "新名称", Role: "editor", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteDrawing(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	logs, err := r.ListAuditLogs(ctx, domain.AuditFilter{Action: "assign_owner"})
	if err != nil || logs.Total != 1 || logs.Items[0].OwnerName != "旧名称" || logs.Items[0].EntityName != "LEGACY" {
		t.Fatal(logs, err)
	}
	kept, err := r.CreateDrawing(ctx, domain.DrawingInput{DrawingNo: "BACKUP-OWNER", OwnerUserID: &a.ID})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "snapshot.db")
	if err := r.BackupSnapshot(ctx, path); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.CheckBackupDB(ctx, path); err != nil {
		t.Fatal(err)
	}
	dest := newTestRepo(t)
	other := testActor(t, dest, "different-actor")
	if err := dest.RestoreSnapshot(other, path); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := dest.db.Get(&name, `SELECT display_name FROM app_user WHERE id=?`, a.ID); err != nil || name != "新名称" {
		t.Fatal(name, err)
	}
	memberCtx := domain.WithUser(context.Background(), a)
	restored, err := dest.GetDrawing(memberCtx, kept.ID)
	if err != nil || restored.OwnerUserID != a.ID {
		t.Fatal(restored, err)
	}
	var actor string
	if err := dest.db.Get(&actor, `SELECT actor FROM audit_log WHERE action='restore'`); err != nil || actor != "different-actor" {
		t.Fatal(actor, err)
	}
}

func TestRelatedIDsCannotCrossOwners(t *testing.T) {
	r := newTestRepo(t)
	ctx := testActor(t, r, "admin")
	a, err := r.CreateUser(ctx, domain.CreateUserInput{Username: "a", Password: "password123", Role: "editor"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.CreateUser(ctx, domain.CreateUserInput{Username: "b", Password: "password123", Role: "editor"})
	if err != nil {
		t.Fatal(err)
	}
	d, err := r.CreateDrawing(ctx, domain.DrawingInput{DrawingNo: "A", OwnerUserID: &a.ID})
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.db.Exec(`INSERT INTO machine(code,name,owner_user_id,created_at,updated_at) VALUES('B-machine','Private B',?,'','')`, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	mid, _ := res.LastInsertId()
	if _, err := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: 10, MachineID: &mid}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal(err)
	}
	op, err := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: 10})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.UpdateOperation(ctx, op.ID, domain.OperationInput{OpNo: 10, MachineID: &mid}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal(err)
	}
	// Even a structurally valid backup must reject cross-owner links.
	if _, err := r.db.Exec(`UPDATE operation SET machine_id=? WHERE id=?`, mid, op.ID); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "cross-owner.db")
	if err := r.BackupSnapshot(ctx, path); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.CheckBackupDB(ctx, path); err == nil {
		t.Fatal("cross-owner backup accepted")
	}
}

func TestOwnerAssignmentConflictsRollbackAndStaleAccountIsDenied(t *testing.T) {
	r := newTestRepo(t)
	admin := testActor(t, r, "admin")
	u, err := r.CreateUser(admin, domain.CreateUserInput{Username: "worker", Password: "password123", Role: "editor"})
	if err != nil {
		t.Fatal(err)
	}
	member := domain.WithUser(context.Background(), u)
	own, err := r.CreateDrawing(member, domain.DrawingInput{DrawingNo: "DUP"})
	if err != nil {
		t.Fatal(err)
	}
	legacy := seedDrawing(t, r, "DUP")
	if _, err = r.AssignDrawingOwner(member, legacy.ID, u.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal(err)
	}
	if _, err = r.AssignDrawingOwner(admin, legacy.ID, u.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatal(err)
	}
	got, err := r.GetDrawing(admin, legacy.ID)
	if err != nil || got.OwnerUserID != 0 {
		t.Fatal(got, err)
	}
	if _, err = r.GetDrawing(member, legacy.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	logs, err := r.ListAuditLogs(admin, domain.AuditFilter{Action: "assign_owner"})
	if err != nil || logs.Total != 0 {
		t.Fatal(logs, err)
	}
	zero := int64(0)
	if _, err = r.UpdateDrawing(member, own.ID, domain.DrawingInput{DrawingNo: "DUP", OwnerUserID: &zero}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal(err)
	}
	list, err := r.ListDrawings(WithOwnerFilter(admin, &zero), "", 1, 20)
	if err != nil || list.Total != 1 || list.Items[0].ID != legacy.ID {
		t.Fatal(list, err)
	}
	if _, err = r.UpdateUser(admin, u.ID, domain.UpdateUserInput{DisplayName: u.DisplayName, Role: "editor", Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if _, err = r.ListDrawings(member, "", 1, 20); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal(err)
	}
	if _, err = r.CreateDrawing(member, domain.DrawingInput{DrawingNo: "stale"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal(err)
	}
	if _, err = r.AssignDrawingOwner(admin, legacy.ID, u.ID); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal(err)
	}
}
