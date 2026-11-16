package repo

import (
	"cnc-manager/internal/domain"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
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

func TestMachineLifecyclePreservesHistoryAndBackup(t *testing.T) {
	r := newTestRepo(t)
	ctx := testActor(t, r, "tester")
	actor, _ := domain.UserFromContext(ctx)
	m, err := r.CreateMachine(ctx, domain.MachineInput{Code: "CNC-01", Controller: "FANUC", Remark: "原备注"})
	if err != nil {
		t.Fatal(err)
	}
	d, err := r.CreateDrawing(ctx, domain.DrawingInput{DrawingNo: "D1", OwnerUserID: &actor.ID})
	if err != nil {
		t.Fatal(err)
	}
	op, err := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: 10, MachineID: &m.ID})
	if err != nil {
		t.Fatal(err)
	}
	p, err := r.CreateProgram(ctx, op.ID, domain.ProgramInput{ProgramNo: "O100"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := r.AddVersion(ctx, p.ID, blob("aa11", "utf-8"), "O100.nc", "初版")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.Exec(`INSERT INTO program_tool(program_id,version_id,seq,tool_no) VALUES(?,?,1,'T01')`, p.ID, v.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = r.db.Exec(`INSERT INTO machine_tool(machine_id,pocket_no,tool_no,updated_at) VALUES(?,1,'T01','before')`, m.ID); err != nil {
		t.Fatal(err)
	}
	beforeP, _ := r.GetProgram(ctx, p.ID)
	beforeV, _ := r.ListVersions(ctx, p.ID)
	beforeTools, _ := r.GetVersionTools(ctx, v.ID)
	m, err = r.UpdateMachine(ctx, m.ID, domain.MachineInput{Code: "CNC-renamed", Name: "精铣", Controller: "SIEMENS", Remark: "修改备注"})
	if err != nil {
		t.Fatal(err)
	}
	m, err = r.SetMachineEnabled(ctx, m.ID, false)
	if err != nil || m.Enabled || m.DrawingCount != 1 || m.OperationCount != 1 || m.ProgramCount != 1 {
		t.Fatal(m, err)
	}
	choices, err := r.DrawingMachines(ctx, d.ID)
	if err != nil || len(choices) != 1 || choices[0].Enabled {
		t.Fatal(choices, err)
	}
	if _, err := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: 20, MachineID: &m.ID}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal("disabled creation", err)
	}
	if _, err := r.UpdateOperation(ctx, op.ID, domain.OperationInput{OpNo: 10, OpName: "保留关联", MachineID: &m.ID}); err != nil {
		t.Fatal(err)
	}
	afterP, _ := r.GetProgram(ctx, p.ID)
	afterV, _ := r.ListVersions(ctx, p.ID)
	afterTools, _ := r.GetVersionTools(ctx, v.ID)
	beforeP.MachineCode, beforeP.MachineName = afterP.MachineCode, afterP.MachineName
	if !reflect.DeepEqual(beforeP, afterP) || !reflect.DeepEqual(beforeV, afterV) || !reflect.DeepEqual(beforeTools, afterTools) {
		t.Fatal("machine changes altered program history/tools")
	}
	path := filepath.Join(t.TempDir(), "machines.db")
	if err := r.BackupSnapshot(ctx, path); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.CheckBackupDB(ctx, path); err != nil {
		t.Fatal(err)
	}
	if _, err := r.SetMachineEnabled(ctx, m.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := r.RestoreSnapshot(ctx, path); err != nil {
		t.Fatal(err)
	}
	restored, err := r.GetMachine(ctx, m.ID)
	if err != nil || restored.Enabled || restored.Controller != "SIEMENS" || restored.Remark != "修改备注" {
		t.Fatal(restored, err)
	}
	var pockets int
	if err := r.db.Get(&pockets, `SELECT COUNT(*) FROM machine_tool WHERE machine_id=?`, m.ID); err != nil || pockets != 1 {
		t.Fatal(pockets, err)
	}
	if _, err := r.UpdateOperation(ctx, op.ID, domain.OperationInput{OpNo: 10}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.UpdateOperation(ctx, op.ID, domain.OperationInput{OpNo: 10, MachineID: &m.ID}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal("disabled reassociation", err)
	}
	if _, err := r.SetMachineEnabled(ctx, m.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := r.UpdateOperation(ctx, op.ID, domain.OperationInput{OpNo: 10, MachineID: &m.ID}); err != nil {
		t.Fatal(err)
	}
}

func TestMachineMutationAuditRollback(t *testing.T) {
	r := newTestRepo(t)
	ctx := testActor(t, r, "tester")
	m, err := r.CreateMachine(ctx, domain.MachineInput{Code: "original"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec(`CREATE TRIGGER fail_machine_log BEFORE INSERT ON audit_log BEGIN SELECT RAISE(ABORT,'audit failed'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateMachine(ctx, domain.MachineInput{Code: "failed"}); err == nil {
		t.Fatal("create escaped rollback")
	}
	if _, err := r.UpdateMachine(ctx, m.ID, domain.MachineInput{Code: "changed"}); err == nil {
		t.Fatal("update escaped rollback")
	}
	if _, err := r.SetMachineEnabled(ctx, m.ID, false); err == nil {
		t.Fatal("disable escaped rollback")
	}
	list, err := r.ListMachines(ctx, domain.MachineFilter{})
	if err != nil || len(list.Items) != 1 || list.Items[0].Code != "original" || !list.Items[0].Enabled {
		t.Fatal(list, err)
	}
}

func TestMachineRelatedSearchAndPagination(t *testing.T) {
	r := newTestRepo(t)
	ctx := testActor(t, r, "tester")
	actor, _ := domain.UserFromContext(ctx)
	m, err := r.CreateMachine(ctx, domain.MachineInput{Code: "M"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := r.CreateMachine(ctx, domain.MachineInput{Code: "other"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		d, err := r.CreateDrawing(ctx, domain.DrawingInput{DrawingNo: fmt.Sprintf("D%d", i), OwnerUserID: &actor.ID})
		if err != nil {
			t.Fatal(err)
		}
		for j, mid := range []int64{m.ID, other.ID} {
			op, err := r.CreateOperation(ctx, d.ID, domain.OperationInput{OpNo: (j + 1) * 10, MachineID: &mid, OpName: fmt.Sprintf("op%d", j)})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := r.CreateProgram(ctx, op.ID, domain.ProgramInput{ProgramNo: fmt.Sprintf("O%d%d", i, j)}); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, tc := range []struct {
		q     string
		total int
	}{{"", 2}, {"D1", 1}, {"O10", 1}, {"O11", 0}, {"op0", 2}, {"op1", 0}, {"%", 0}} {
		list, err := r.MachineDrawings(ctx, m.ID, tc.q, 1, 1)
		if err != nil || list.Total != tc.total {
			t.Fatalf("%q: %+v %v", tc.q, list, err)
		}
		for _, d := range list.Items {
			if len(d.Operations) != 1 || d.Operations[0].MachineID == nil || *d.Operations[0].MachineID != m.ID || d.ProgramCount != 1 || d.OperationCount != 1 {
				t.Fatal(d)
			}
		}
	}
	first, _ := r.MachineDrawings(ctx, m.ID, "", 1, 1)
	second, _ := r.MachineDrawings(ctx, m.ID, "", 2, 1)
	if first.Items[0].ID == second.Items[0].ID {
		t.Fatal("pagination repeated drawing")
	}
}
