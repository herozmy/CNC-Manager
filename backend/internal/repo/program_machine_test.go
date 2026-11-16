package repo

import (
	"cnc-manager/internal/domain"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIndependentProgramMachinesAndDeletion(t *testing.T) {
	r := newTestRepo(t)
	ctx := testActor(t, r, "tester")
	actor, _ := domain.UserFromContext(ctx)
	m, err := r.CreateMachine(ctx, domain.MachineInput{Code: "A"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := r.CreateMachine(ctx, domain.MachineInput{Code: "B"})
	if err != nil {
		t.Fatal(err)
	}
	d, err := r.CreateDrawing(ctx, domain.DrawingInput{DrawingNo: "D", OwnerUserID: &actor.ID})
	if err != nil {
		t.Fatal(err)
	}
	op, err := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: 10, MachineID: &m.ID})
	if err != nil {
		t.Fatal(err)
	}
	p, err := r.CreateProgram(ctx, op.ID, domain.ProgramInput{ProgramNo: "OA"})
	if err != nil || p.MachineID == nil || *p.MachineID != m.ID {
		t.Fatal(p, err)
	}
	sibling, err := r.CreateProgram(ctx, op.ID, domain.ProgramInput{ProgramNo: "OB", MachineID: domain.MachineReference{Set: true, ID: &other.ID}})
	if err != nil {
		t.Fatal(err)
	}
	v, err := r.AddVersion(ctx, p.ID, blob("aa11", "utf-8"), "OA.nc", "original")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.Exec(`INSERT INTO program_tool(program_id,version_id,seq,tool_no) VALUES(?,?,1,'T01')`, p.ID, v.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.Exec(`INSERT INTO machine_tool(machine_id,pocket_no,updated_at) VALUES(?,1,'old')`, m.ID); err != nil {
		t.Fatal(err)
	}
	versions, _ := r.ListVersions(ctx, p.ID)
	tools, _ := r.GetVersionTools(ctx, v.ID)
	unlinked, err := r.CreateProgram(ctx, op.ID, domain.ProgramInput{ProgramNo: "NONE", MachineID: domain.MachineReference{Set: true}})
	if err != nil || unlinked.MachineID != nil {
		t.Fatal(unlinked, err)
	}
	if _, err = r.UpdateOperation(ctx, op.ID, domain.OperationInput{OpNo: 10, MachineID: &other.ID}); err != nil {
		t.Fatal(err)
	}
	p, err = r.GetProgram(ctx, p.ID)
	if err != nil || p.MachineID == nil || *p.MachineID != m.ID {
		t.Fatal(p, err)
	}
	for _, tc := range []struct {
		q     string
		count int
	}{{"OA", 1}, {"OB", 0}, {"NONE", 0}, {"D", 1}} {
		list, err := r.MachineDrawings(ctx, m.ID, tc.q, 1, 20)
		if err != nil || list.Total != tc.count {
			t.Fatal(tc, list, err)
		}
		if list.Total > 0 && (list.Items[0].ProgramCount != 1 || len(list.Items[0].Operations[0].Programs) != 1 || list.Items[0].Operations[0].Programs[0].ID != p.ID) {
			t.Fatal(list)
		}
	}
	if _, err = r.SetMachineEnabled(ctx, m.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CreateProgram(ctx, op.ID, domain.ProgramInput{ProgramNo: "BAD", MachineID: domain.MachineReference{Set: true, ID: &m.ID}}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal(err)
	}
	p, err = r.UpdateProgram(ctx, p.ID, domain.ProgramInput{ProgramNo: "OA", MachineID: domain.MachineReference{Set: true, ID: &m.ID}, ExpectedRevision: &p.Revision})
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "before.db")
	if err = r.BackupSnapshot(ctx, backup); err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.Exec(`CREATE TRIGGER reject_delete BEFORE INSERT ON audit_log WHEN NEW.action='delete' BEGIN SELECT RAISE(ABORT,'audit failed'); END`); err != nil {
		t.Fatal(err)
	}
	if err = r.DeleteMachine(ctx, m.ID); err == nil {
		t.Fatal("delete escaped audit rollback")
	}
	got, _ := r.GetProgram(ctx, p.ID)
	if !reflect.DeepEqual(got, p) {
		t.Fatal(got, p)
	}
	if _, err = r.db.Exec(`DROP TRIGGER reject_delete`); err != nil {
		t.Fatal(err)
	}
	if err = r.DeleteMachine(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	after, err := r.GetProgram(ctx, p.ID)
	if err != nil || after.MachineID != nil || after.Revision != p.Revision+1 || after.CurrentVersionID == nil || *after.CurrentVersionID != v.ID {
		t.Fatal(after, err)
	}
	afterVersions, _ := r.ListVersions(ctx, p.ID)
	afterTools, _ := r.GetVersionTools(ctx, v.ID)
	if !reflect.DeepEqual(versions, afterVersions) || !reflect.DeepEqual(tools, afterTools) {
		t.Fatal("history changed")
	}
	unchanged, _ := r.GetProgram(ctx, sibling.ID)
	if !reflect.DeepEqual(unchanged, sibling) {
		t.Fatal(unchanged, sibling)
	}
	var n int
	if err = r.db.Get(&n, `SELECT COUNT(*) FROM machine_tool WHERE machine_id=?`, m.ID); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	if _, err = r.UpdateProgram(ctx, p.ID, domain.ProgramInput{ProgramNo: "stale", ExpectedRevision: &p.Revision}); !errors.Is(err, domain.ErrConflict) {
		t.Fatal(err)
	}
	if err = r.DeleteMachine(ctx, other.ID); err != nil {
		t.Fatal(err)
	}
	newMachine, err := r.CreateDrawingMachine(ctx, d.ID, domain.MachineInput{Code: "NEW"})
	if err != nil || newMachine.ID <= other.ID {
		t.Fatal("machine ID reused", newMachine, err)
	}
	if _, err = r.CreateProgram(ctx, op.ID, domain.ProgramInput{ProgramNo: "deleted", MachineID: domain.MachineReference{Set: true, ID: &other.ID}}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal(err)
	}
	if err = r.RestoreSnapshot(ctx, backup); err != nil {
		t.Fatal(err)
	}
	restored, err := r.GetProgram(ctx, p.ID)
	if err != nil || !reflect.DeepEqual(restored, p) {
		t.Fatal(restored, p, err)
	}
	newest, err := r.CreateMachine(ctx, domain.MachineInput{Code: "after restore"})
	if err != nil || newest.ID <= newMachine.ID {
		t.Fatal("restore reused a deleted ID", newest, err)
	}

}

func TestProgramMachineOwnerAndAuditValidation(t *testing.T) {
	r := newTestRepo(t)
	ctx := testActor(t, r, "admin")
	actor, _ := domain.UserFromContext(ctx)
	foreign, err := r.CreateUser(ctx, domain.CreateUserInput{Username: "foreign", Password: "password123", Role: "editor"})
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.CreateMachine(ctx, domain.MachineInput{Code: "Foreign", OwnerUserID: &foreign.ID})
	if err != nil {
		t.Fatal(err)
	}
	own, err := r.CreateMachine(ctx, domain.MachineInput{Code: "Own"})
	if err != nil {
		t.Fatal(err)
	}
	d, err := r.CreateDrawing(ctx, domain.DrawingInput{DrawingNo: "D", OwnerUserID: &actor.ID})
	if err != nil {
		t.Fatal(err)
	}
	op, err := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: 10})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.CreateProgram(ctx, op.ID, domain.ProgramInput{ProgramNo: "BAD", MachineID: domain.MachineReference{Set: true, ID: &m.ID}}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal(err)
	}
	p, err := r.CreateProgram(ctx, op.ID, domain.ProgramInput{ProgramNo: "P"})
	if err != nil {
		t.Fatal(err)
	}
	input := domain.ProgramInput{ProgramNo: "P", ExpectedRevision: &p.Revision, MachineID: domain.MachineReference{Set: true, ID: &m.ID}}
	if _, err = r.UpdateProgram(ctx, p.ID, input); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = r.db.Exec(`CREATE TRIGGER reject_machine_change BEFORE INSERT ON audit_log BEGIN SELECT RAISE(ABORT,'audit failed'); END`); err != nil {
		t.Fatal(err)
	}
	input.MachineID.ID = &own.ID
	if _, err = r.UpdateProgram(ctx, p.ID, input); err == nil {
		t.Fatal("save escaped audit rollback")
	}
	got, _ := r.GetProgram(ctx, p.ID)
	if !reflect.DeepEqual(got, p) {
		t.Fatal(got, p)
	}
	if _, err = r.db.Exec(`DROP TRIGGER reject_machine_change`); err != nil {
		t.Fatal(err)
	}
	input.MachineID.ID = &own.ID
	p, err = r.UpdateProgram(ctx, p.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	// Omitted machineId preserves, explicit null clears.
	p, err = r.UpdateProgram(ctx, p.ID, domain.ProgramInput{ProgramNo: "P", ExpectedRevision: &p.Revision})
	if err != nil || p.MachineID == nil {
		t.Fatal(p, err)
	}
	p, err = r.UpdateProgram(ctx, p.ID, domain.ProgramInput{ProgramNo: "P", ExpectedRevision: &p.Revision, MachineID: domain.MachineReference{Set: true}})
	if err != nil || p.MachineID != nil {
		t.Fatal(p, err)
	}
	// Backup validation and assigning an unowned drawing also inspect program links.
	if _, err = r.db.Exec(`UPDATE nc_program SET machine_id=? WHERE id=?`, m.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "cross-owner.db")
	if err = r.BackupSnapshot(ctx, backup); err != nil {
		t.Fatal(err)
	}
	if _, _, err = r.CheckBackupDB(ctx, backup); err == nil {
		t.Fatal("cross-owner backup accepted")
	}
	if _, err = r.db.Exec(`UPDATE drawing SET owner_user_id=NULL WHERE id=?`, d.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = r.AssignDrawingOwner(ctx, d.ID, actor.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatal(err)
	}
}

func TestMachineSequenceBackupRetainsDeletedIDs(t *testing.T) {
	r := newTestRepo(t)
	ctx := testActor(t, r, "tester")
	if _, err := r.db.Exec(`UPDATE machine_id_sequence SET last_id=50`); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "high-water.db")
	if err := r.BackupSnapshot(ctx, backup); err != nil {
		t.Fatal(err)
	}
	fresh := newTestRepo(t)
	freshCtx := testActor(t, fresh, "tester")
	if _, _, err := fresh.CheckBackupDB(freshCtx, backup); err != nil {
		t.Fatal(err)
	}
	if err := fresh.RestoreSnapshot(freshCtx, backup); err != nil {
		t.Fatal(err)
	}
	m, err := fresh.CreateMachine(freshCtx, domain.MachineInput{Code: "first after restore"})
	if err != nil || m.ID != 51 {
		t.Fatal(m, err)
	}
}
