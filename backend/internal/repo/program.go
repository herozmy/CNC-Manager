package repo

import (
	"context"
	"fmt"

	"cnc-manager/internal/domain"
)

const programCols = `p.id, p.revision, p.operation_id, p.program_no, p.program_name, p.controller, p.sort_order,
	p.machine_id, COALESCE((SELECT m.code FROM machine m WHERE m.id=p.machine_id),'') AS machine_code,
	COALESCE((SELECT m.name FROM machine m WHERE m.id=p.machine_id),'') AS machine_name,
	p.current_version_id, p.remark, p.created_at, p.updated_at,
	(SELECT v.version_no FROM nc_version v WHERE v.id = p.current_version_id) AS current_version_no,
	(SELECT COUNT(*) FROM nc_version v WHERE v.program_id = p.id) AS version_count`

// ListPrograms 取工序下全部程序，按人工排序。
func (r *Repo) rawListPrograms(ctx context.Context, operationID int64) ([]domain.Program, error) {
	items := []domain.Program{}
	if err := r.tx.SelectContext(ctx, &items,
		`SELECT `+programCols+` FROM nc_program p WHERE p.operation_id = ? ORDER BY p.sort_order, p.id`, operationID); err != nil {
		return nil, wrap(err)
	}
	return items, nil
}

// GetProgram 按 ID 取程序。
func (r *Repo) rawGetProgram(ctx context.Context, id int64) (*domain.Program, error) {
	var p domain.Program
	if err := r.tx.GetContext(ctx, &p, `SELECT `+programCols+` FROM nc_program p WHERE p.id = ?`, id); err != nil {
		return nil, wrap(err)
	}
	return &p, nil
}

// CreateProgram 在工序下新增程序，排在最后。
func (r *Repo) rawCreateProgram(ctx context.Context, operationID int64, in domain.ProgramInput) (*domain.Program, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	op, err := r.GetOperation(ctx, operationID)
	if err != nil {
		return nil, err
	}
	machineID := in.MachineID.ID
	if !in.MachineID.Set {
		machineID = op.MachineID
	}
	owner, err := objectOwner(ctx, r.tx, "operation", operationID)
	if err != nil {
		return nil, err
	}
	if err := r.checkOperationMachine(ctx, machineID, nil, owner); err != nil {
		return nil, err
	}
	now := domain.Now()
	res, err := r.tx.ExecContext(ctx,
		`INSERT INTO nc_program (operation_id, program_no, program_name, controller, sort_order, remark, created_at, updated_at, machine_id)
		 VALUES (?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM nc_program WHERE operation_id = ?), ?, ?, ?, ?)`,
		operationID, in.ProgramNo, in.ProgramName, in.Controller, operationID, in.Remark, now, now, machineID)
	if err != nil {
		return nil, wrap(err)
	}
	id, _ := res.LastInsertId()
	if err := r.log(ctx, r.tx, "create", "program", id, "新增程序 "+in.ProgramNo); err != nil {
		return nil, err
	}
	return r.GetProgram(ctx, id)
}

// UpdateProgram 修改程序基本信息，不影响已有版本。
func (r *Repo) rawUpdateProgram(ctx context.Context, id int64, in domain.ProgramInput) (*domain.Program, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	old, err := r.GetProgram(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.ExpectedRevision == nil {
		return nil, fmt.Errorf("%w: 缺少程序修订标识，请重新载入", domain.ErrInvalid)
	}
	if *in.ExpectedRevision != old.Revision {
		return nil, fmt.Errorf("%w: 程序信息已被修改，请重新载入后再保存", domain.ErrConflict)
	}
	machineID := old.MachineID
	if in.MachineID.Set {
		machineID = in.MachineID.ID
	}
	owner, err := objectOwner(ctx, r.tx, "program", id)
	if err != nil {
		return nil, err
	}
	if err := r.checkOperationMachine(ctx, machineID, old.MachineID, owner); err != nil {
		return nil, err
	}
	if _, err := r.tx.ExecContext(ctx,
		`UPDATE nc_program SET revision = revision + 1, program_no = ?, program_name = ?, controller = ?, remark = ?, machine_id=?, updated_at = ? WHERE id = ?`,
		in.ProgramNo, in.ProgramName, in.Controller, in.Remark, machineID, domain.Now(), id); err != nil {
		return nil, wrap(err)
	}
	if err := r.log(ctx, r.tx, "update", "program", id, "修改程序 "+in.ProgramNo); err != nil {
		return nil, err
	}
	return r.GetProgram(ctx, id)
}

// DeleteProgram 删除程序及其版本记录；物理文件不删（可能被其他版本共享）。
func (r *Repo) rawDeleteProgram(ctx context.Context, id int64) error {
	p, err := r.GetProgram(ctx, id)
	if err != nil {
		return err
	}
	if err := r.log(ctx, r.tx, "delete", "program", id, "删除程序 "+p.ProgramNo); err != nil {
		return err
	}
	if _, err := r.tx.ExecContext(ctx, `DELETE FROM nc_program WHERE id = ?`, id); err != nil {
		return wrap(err)
	}
	return nil
}

// ReorderPrograms 按完整 ID 顺序重排工序下的程序。
func (r *Repo) rawReorderPrograms(ctx context.Context, operationID int64, ids []int64) error {
	existing, err := r.ListPrograms(ctx, operationID)
	if err != nil {
		return err
	}
	if err := checkReorder(existing, func(p domain.Program) int64 { return p.ID }, ids, "程序"); err != nil {
		return err
	}
	return r.reorder(ctx, `UPDATE nc_program SET sort_order = ?, updated_at = ? WHERE id = ? AND operation_id = ?`, operationID, ids, "operation", "调整程序显示顺序")
}
