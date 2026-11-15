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
		err = t.tx.SelectContext(ctx, &items, `SELECT id,code,name FROM machine WHERE COALESCE(owner_user_id,0)=? ORDER BY code,id`, owner)
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
		now := domain.Now()
		res, err := t.tx.ExecContext(ctx, `INSERT INTO machine(code,name,owner_user_id,created_at,updated_at) VALUES(?,?,?,?,?)`, in.Code, in.Name, owner, now, now)
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
		return &domain.MachineOption{ID: id, Code: in.Code, Name: in.Name}, nil
	})
}
