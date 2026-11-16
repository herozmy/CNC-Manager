package repo

import (
	"cnc-manager/internal/domain"
	"context"
	"fmt"
)

// Derive ownership from the drawing, including when an admin edits it.
func (r *Repo) DrawingMachines(ctx context.Context, drawingID int64) ([]domain.MachineOption, error) {
	return businessTransaction(ctx, r, domain.PermissionRead, "drawing", drawingID, func(t *Repo) ([]domain.MachineOption, error) {
		owner, err := objectOwner(ctx, t.tx, "drawing", drawingID)
		if err != nil {
			return nil, err
		}
		items := []domain.MachineOption{}
		err = t.tx.SelectContext(ctx, &items, `SELECT id,code,name,enabled FROM machine WHERE COALESCE(owner_user_id,0)=? ORDER BY code,id`, owner)
		return items, wrap(err)
	})
}

func (r *Repo) CreateDrawingMachine(ctx context.Context, drawingID int64, in domain.MachineInput) (*domain.MachineOption, error) {
	return businessTransaction(ctx, r, domain.PermissionCreate, "drawing", drawingID, func(t *Repo) (*domain.MachineOption, error) {
		if err := in.Normalize(); err != nil {
			return nil, err
		}
		owner, err := objectOwner(ctx, t.tx, "drawing", drawingID)
		if err != nil {
			return nil, err
		}
		// Old unassigned catalogs remain selectable; new entries need an owner.
		if owner == 0 {
			return nil, fmt.Errorf("%w: 请先分配图纸所属账号，再新增机台", domain.ErrInvalid)
		}
		if in.OwnerUserID != nil && *in.OwnerUserID != owner {
			return nil, domain.ErrForbidden
		}
		now := domain.Now()
		res, err := t.tx.ExecContext(ctx, `INSERT INTO machine(id,code,name,controller,remark,owner_user_id,created_at,updated_at) VALUES((SELECT last_id+1 FROM machine_id_sequence WHERE singleton=1),?,?,?,?,?,?,?)`, in.Code, in.Name, in.Controller, in.Remark, owner, now, now)
		if err != nil {
			return nil, wrap(err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return nil, err
		}
		if err := t.log(ctx, t.tx, "create", "machine", id, "新增机台 "+in.Code); err != nil {
			return nil, err
		}
		return &domain.MachineOption{ID: id, Code: in.Code, Name: in.Name, Enabled: true}, nil
	})
}

const machineCols = `m.id,m.code,m.name,m.enabled,m.controller,m.remark,m.created_at,m.updated_at,
 COALESCE(m.owner_user_id,0) AS owner_user_id,COALESCE(u.display_name,'待分配') AS owner_name,
 (SELECT COUNT(DISTINCT o.drawing_id) FROM operation o WHERE o.machine_id=m.id OR EXISTS(SELECT 1 FROM nc_program p WHERE p.operation_id=o.id AND p.machine_id=m.id)) AS drawing_count,
 (SELECT COUNT(*) FROM operation o WHERE o.machine_id=m.id OR EXISTS(SELECT 1 FROM nc_program p WHERE p.operation_id=o.id AND p.machine_id=m.id)) AS operation_count,
 (SELECT COUNT(*) FROM nc_program p WHERE p.machine_id=m.id) AS program_count`
const machineFrom = ` FROM machine m LEFT JOIN app_user u ON u.id=m.owner_user_id `

func pageBounds(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	return page, size
}

func (r *Repo) ListMachines(ctx context.Context, f domain.MachineFilter) (domain.Page[domain.Machine], error) {
	return businessTransaction(ctx, r, domain.PermissionRead, "", 0, func(t *Repo) (domain.Page[domain.Machine], error) {
		page, size := pageBounds(f.Page, f.Size)
		out := domain.Page[domain.Machine]{Items: []domain.Machine{}, Page: page, Size: size}
		a, err := businessScope(ctx, t.tx, domain.PermissionRead)
		if err != nil {
			return out, err
		}
		filter := ` WHERE (m.code LIKE ? ESCAPE '\' OR m.name LIKE ? ESCAPE '\' OR m.controller LIKE ? ESCAPE '\')`
		pat := likePattern(f.Keyword)
		args := []any{pat, pat, pat}
		predicate, scopeArgs := a.predicate("m.owner_user_id")
		filter += predicate
		args = append(args, scopeArgs...)
		if f.Enabled != nil {
			filter += ` AND m.enabled=?`
			args = append(args, *f.Enabled)
		}
		if err := t.tx.GetContext(ctx, &out.Total, `SELECT COUNT(*)`+machineFrom+filter, args...); err != nil {
			return out, wrap(err)
		}
		err = t.tx.SelectContext(ctx, &out.Items, `SELECT `+machineCols+machineFrom+filter+` ORDER BY m.enabled DESC,m.code,m.id LIMIT ? OFFSET ?`, append(args, size, (page-1)*size)...)
		return out, wrap(err)
	})
}
func (r *Repo) GetMachine(ctx context.Context, id int64) (*domain.Machine, error) {
	return businessTransaction(ctx, r, domain.PermissionRead, "machine", id, func(t *Repo) (*domain.Machine, error) {
		var m domain.Machine
		err := t.tx.GetContext(ctx, &m, `SELECT `+machineCols+machineFrom+` WHERE m.id=?`, id)
		return &m, wrap(err)
	})
}
func (r *Repo) CreateMachine(ctx context.Context, in domain.MachineInput) (*domain.Machine, error) {
	return businessTransaction(ctx, r, domain.PermissionCreate, "", 0, func(t *Repo) (*domain.Machine, error) {
		if err := in.Normalize(); err != nil {
			return nil, err
		}
		// Independent creation defaults to the actor; admins may choose another active owner.
		if in.OwnerUserID == nil {
			actor, _ := domain.UserFromContext(ctx)
			in.OwnerUserID = &actor.ID
		}
		owner, err := drawingCreateOwner(ctx, t.tx, in.OwnerUserID)
		if err != nil {
			return nil, err
		}
		if owner == nil {
			return nil, fmt.Errorf("%w: 请选择机台所属账号", domain.ErrInvalid)
		}
		now := domain.Now()
		res, err := t.tx.ExecContext(ctx, `INSERT INTO machine(id,code,name,controller,remark,owner_user_id,created_at,updated_at) VALUES((SELECT last_id+1 FROM machine_id_sequence WHERE singleton=1),?,?,?,?,?,?,?)`, in.Code, in.Name, in.Controller, in.Remark, *owner, now, now)
		if err != nil {
			return nil, wrap(err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return nil, err
		}
		if err := t.log(ctx, t.tx, "create", "machine", id, "新增机台 "+in.Code); err != nil {
			return nil, err
		}
		return t.GetMachine(ctx, id)
	})
}
func (r *Repo) UpdateMachine(ctx context.Context, id int64, in domain.MachineInput) (*domain.Machine, error) {
	return businessTransaction(ctx, r, domain.PermissionEdit, "machine", id, func(t *Repo) (*domain.Machine, error) {
		if err := in.Normalize(); err != nil {
			return nil, err
		}
		owner, err := objectOwner(ctx, t.tx, "machine", id)
		if err != nil {
			return nil, err
		}
		if in.OwnerUserID != nil && *in.OwnerUserID != owner {
			return nil, domain.ErrForbidden
		}
		if _, err := t.tx.ExecContext(ctx, `UPDATE machine SET code=?,name=?,controller=?,remark=?,updated_at=? WHERE id=?`, in.Code, in.Name, in.Controller, in.Remark, domain.Now(), id); err != nil {
			return nil, wrap(err)
		}
		if err := t.log(ctx, t.tx, "update", "machine", id, "修改机台 "+in.Code); err != nil {
			return nil, err
		}
		return t.GetMachine(ctx, id)
	})
}
func (r *Repo) SetMachineEnabled(ctx context.Context, id int64, enabled bool) (*domain.Machine, error) {
	return businessTransaction(ctx, r, domain.PermissionEdit, "machine", id, func(t *Repo) (*domain.Machine, error) {
		old, err := t.GetMachine(ctx, id)
		if err != nil {
			return nil, err
		}
		if old.Enabled == enabled {
			return old, nil
		}
		if _, err := t.tx.ExecContext(ctx, `UPDATE machine SET enabled=?,updated_at=? WHERE id=?`, enabled, domain.Now(), id); err != nil {
			return nil, wrap(err)
		}
		action, detail := "disable", "停用机台 "
		if enabled {
			action, detail = "enable", "启用机台 "
		}
		if err := t.log(ctx, t.tx, action, "machine", id, detail+old.Code); err != nil {
			return nil, err
		}
		return t.GetMachine(ctx, id)
	})
}
func (r *Repo) checkOperationMachine(ctx context.Context, id, previous *int64, owner int64) error {
	if err := checkRelatedOwner(ctx, r.tx, "machine", id, owner); err != nil {
		return err
	}
	if id == nil || (previous != nil && *id == *previous) {
		return nil
	}
	var enabled bool
	if err := r.tx.GetContext(ctx, &enabled, `SELECT enabled FROM machine WHERE id=?`, *id); err != nil {
		return wrap(err)
	}
	if !enabled {
		return fmt.Errorf("%w: 机台已停用，请选择启用的机台", domain.ErrInvalid)
	}
	return nil
}

func (r *Repo) DeleteMachine(ctx context.Context, id int64) error {
	_, err := businessTransaction(ctx, r, domain.PermissionDelete, "machine", id, func(t *Repo) (struct{}, error) {
		old, err := t.GetMachine(ctx, id)
		if err != nil {
			return struct{}{}, err
		}
		if err = t.log(ctx, t.tx, "delete", "machine", id, "删除机台 "+old.Code+"；解除工序及程序关联，删除机台刀库，保留程序及版本历史"); err != nil {
			return struct{}{}, err
		}
		// Revision conflicts protect open program forms from reattaching a removed machine.
		if _, err = t.tx.ExecContext(ctx, `UPDATE nc_program SET machine_id=NULL,revision=revision+1,updated_at=? WHERE machine_id=?`, domain.Now(), id); err != nil {
			return struct{}{}, wrap(err)
		}
		if _, err = t.tx.ExecContext(ctx, `UPDATE operation SET updated_at=? WHERE machine_id=?`, domain.Now(), id); err != nil {
			return struct{}{}, wrap(err)
		}
		_, err = t.tx.ExecContext(ctx, `DELETE FROM machine WHERE id=?`, id)
		return struct{}{}, wrap(err)
	})
	return err
}

// Only return operations belonging to this machine, even when a drawing is shared
// by multiple machines. Program matches never pull in another machine's drawing.
func (r *Repo) MachineDrawings(ctx context.Context, id int64, keyword string, page, size int) (domain.Page[domain.DrawingDetail], error) {
	return businessTransaction(ctx, r, domain.PermissionRead, "machine", id, func(t *Repo) (domain.Page[domain.DrawingDetail], error) {
		page, size = pageBounds(page, size)
		out := domain.Page[domain.DrawingDetail]{Items: []domain.DrawingDetail{}, Page: page, Size: size}
		owner, err := objectOwner(ctx, t.tx, "machine", id)
		if err != nil {
			return out, err
		}
		pat := likePattern(keyword)
		filter := ` WHERE COALESCE(d.owner_user_id,0)=? AND EXISTS (
   SELECT 1 FROM operation o WHERE o.drawing_id=d.id AND (o.machine_id=? OR EXISTS(SELECT 1 FROM nc_program p WHERE p.operation_id=o.id AND p.machine_id=?)) AND (
    d.drawing_no LIKE ? ESCAPE '\' OR d.name LIKE ? ESCAPE '\' OR d.material LIKE ? ESCAPE '\' OR
    CAST(o.op_no AS TEXT) LIKE ? ESCAPE '\' OR o.op_name LIKE ? ESCAPE '\' OR
    EXISTS(SELECT 1 FROM nc_program p WHERE p.operation_id=o.id AND p.machine_id=? AND (p.program_no LIKE ? ESCAPE '\' OR p.program_name LIKE ? ESCAPE '\'))))`
		args := []any{owner, id, id, pat, pat, pat, pat, pat, id, pat, pat}
		if err := t.tx.GetContext(ctx, &out.Total, `SELECT COUNT(*) FROM drawing d`+filter, args...); err != nil {
			return out, wrap(err)
		}
		ids := []int64{}
		if err := t.tx.SelectContext(ctx, &ids, `SELECT d.id FROM drawing d`+filter+` ORDER BY d.updated_at DESC,d.id DESC LIMIT ? OFFSET ?`, append(args, size, (page-1)*size)...); err != nil {
			return out, wrap(err)
		}
		for _, drawingID := range ids {
			d, err := t.DrawingDetail(ctx, drawingID)
			if err != nil {
				return out, err
			}
			ops := make([]domain.OperationWithPrograms, 0, len(d.Operations))
			programs := 0
			for _, op := range d.Operations {
				selected := make([]domain.Program, 0, len(op.Programs))
				for _, p := range op.Programs {
					if p.MachineID != nil && *p.MachineID == id {
						selected = append(selected, p)
					}
				}
				op.Programs = selected
				if (op.MachineID != nil && *op.MachineID == id) || len(selected) > 0 {
					ops = append(ops, op)
					programs += len(op.Programs)
				}
			}
			d.Operations = ops
			d.OperationCount = len(ops)
			d.ProgramCount = programs
			out.Items = append(out.Items, *d)
		}
		return out, nil
	})
}
