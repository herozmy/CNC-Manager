package repo

import (
	"context"
	"errors"
	"fmt"

	"cnc-manager/internal/domain"
	"github.com/jmoiron/sqlx"
)

type ownerFilterKey struct{}

// Owner zero selects unassigned legacy data; nil selects all data for admins.
func WithOwnerFilter(ctx context.Context, ownerID *int64) context.Context {
	var copyID *int64
	if ownerID != nil {
		id := *ownerID
		copyID = &id
	}
	return context.WithValue(ctx, ownerFilterKey{}, copyID)
}

type businessAccess struct {
	admin   bool
	ownerID *int64
}

// Reload identity inside the business transaction to avoid stale authorization.
func businessScope(ctx context.Context, q sqlx.QueryerContext, permission string) (businessAccess, error) {
	actor, ok := domain.UserFromContext(ctx)
	if !ok || actor.ID <= 0 {
		return businessAccess{}, domain.ErrForbidden
	}
	current, err := loadUser(ctx, q, actor.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return businessAccess{}, domain.ErrForbidden
	}
	if err != nil {
		return businessAccess{}, err
	}
	if !current.Can(permission) {
		return businessAccess{}, domain.ErrForbidden
	}
	filter, _ := ctx.Value(ownerFilterKey{}).(*int64)
	if filter != nil && *filter < 0 {
		return businessAccess{}, domain.ErrInvalid
	}
	if current.IsAdmin() {
		return businessAccess{admin: true, ownerID: filter}, nil
	}
	if filter != nil && *filter != current.ID {
		return businessAccess{}, domain.ErrForbidden
	}
	return businessAccess{ownerID: &current.ID}, nil
}
func (a businessAccess) predicate(column string) (string, []any) {
	if a.ownerID == nil {
		return "", nil
	}
	return " AND COALESCE(" + column + ",0) = ?", []any{*a.ownerID}
}
func requireActiveOwner(ctx context.Context, q sqlx.QueryerContext, id int64) error {
	var n int
	if err := sqlx.GetContext(ctx, q, &n, `SELECT COUNT(*) FROM app_user WHERE id=? AND enabled=1`, id); err != nil {
		return wrap(err)
	}
	if n != 1 {
		return fmt.Errorf("%w: 请选择启用的账号", domain.ErrInvalid)
	}
	return nil
}
func drawingCreateOwner(ctx context.Context, q sqlx.QueryerContext, requested *int64) (*int64, error) {
	a, err := businessScope(ctx, q, domain.PermissionCreate)
	if err != nil {
		return nil, err
	}
	if !a.admin {
		if requested != nil && *requested != *a.ownerID {
			return nil, domain.ErrForbidden
		}
		return a.ownerID, nil
	}
	if requested == nil {
		requested = a.ownerID
	}
	if requested == nil || *requested == 0 {
		return nil, nil
	}
	if a.ownerID != nil && *a.ownerID != *requested {
		return nil, domain.ErrForbidden
	}
	return requested, requireActiveOwner(ctx, q, *requested)
}

func checkRelatedOwner(ctx context.Context, q sqlx.QueryerContext, table string, id *int64, ownerID int64) error {
	if id == nil {
		return nil
	}
	if table != "machine" && table != "tool" {
		return domain.ErrInvalid
	}
	var n int
	if err := sqlx.GetContext(ctx, q, &n, `SELECT COUNT(*) FROM `+table+` WHERE id=? AND COALESCE(owner_user_id,0)=?`, *id, ownerID); err != nil {
		return wrap(err)
	}
	if n != 1 {
		return fmt.Errorf("%w: 关联记录不存在或不属于当前账号", domain.ErrInvalid)
	}
	return nil
}

// Every authorization decision, mutation and success audit entry share a
// transaction. This also gives multi-query reads a consistent identity snapshot.
func inTransaction[T any](ctx context.Context, r *Repo, fn func(*sqlx.Tx) (T, error)) (T, error) {
	var zero T
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return zero, wrap(err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := fn(tx)
	if err != nil {
		return zero, err
	}
	if err = tx.Commit(); err != nil {
		return zero, wrap(err)
	}
	return result, nil
}

func objectOwner(ctx context.Context, q sqlx.QueryerContext, entity string, id int64) (int64, error) {
	queries := map[string]string{
		"machine":   `SELECT COALESCE(owner_user_id,0) FROM machine WHERE id=?`,
		"drawing":   `SELECT COALESCE(owner_user_id,0) FROM drawing WHERE id=?`,
		"operation": `SELECT COALESCE(d.owner_user_id,0) FROM operation o JOIN drawing d ON d.id=o.drawing_id WHERE o.id=?`,
		"program":   `SELECT COALESCE(d.owner_user_id,0) FROM nc_program p JOIN operation o ON o.id=p.operation_id JOIN drawing d ON d.id=o.drawing_id WHERE p.id=?`,
		"version":   `SELECT COALESCE(d.owner_user_id,0) FROM nc_version v JOIN nc_program p ON p.id=v.program_id JOIN operation o ON o.id=p.operation_id JOIN drawing d ON d.id=o.drawing_id WHERE v.id=?`,
	}
	query, ok := queries[entity]
	if !ok {
		return 0, domain.ErrInvalid
	}
	var ownerID int64
	err := sqlx.GetContext(ctx, q, &ownerID, query, id)
	return ownerID, wrap(err)
}
func authorizeObject(ctx context.Context, q sqlx.QueryerContext, permission, entity string, id int64) error {
	if entity != "" {
		ownerID, err := objectOwner(ctx, q, entity, id)
		if err != nil {
			return err
		}
		actor, ok := domain.UserFromContext(ctx)
		if !ok {
			return domain.ErrForbidden
		}
		user, err := loadUser(ctx, q, actor.ID)
		if err != nil {
			return domain.ErrForbidden
		}
		if !user.IsAdmin() && (user.ID != ownerID) {
			return domain.ErrNotFound
		}
	}
	access, err := businessScope(ctx, q, permission)
	if err != nil {
		return err
	}
	if entity != "" && access.ownerID != nil {
		ownerID, err := objectOwner(ctx, q, entity, id)
		if err != nil {
			return err
		}
		if ownerID != *access.ownerID {
			return domain.ErrNotFound
		}
	}
	return nil
}

// Only internal repository calls can reuse this already-authorized transaction.
func businessTransaction[T any](ctx context.Context, r *Repo, permission, entity string, id int64, fn func(*Repo) (T, error)) (T, error) {
	if r.tx != nil {
		return fn(r)
	}
	return inTransaction(ctx, r, func(tx *sqlx.Tx) (T, error) {
		var zero T
		if err := authorizeObject(ctx, tx, permission, entity, id); err != nil {
			return zero, err
		}
		return fn(&Repo{db: r.db, tx: tx})
	})
}
func (r *Repo) CheckAccess(ctx context.Context, permission, entity string, id int64) error {
	_, err := businessTransaction(ctx, r, permission, entity, id, func(*Repo) (struct{}, error) { return struct{}{}, nil })
	return err
}
func (r *Repo) AssignDrawingOwner(ctx context.Context, id, ownerID int64) (*domain.Drawing, error) {
	return inTransaction(ctx, r, func(tx *sqlx.Tx) (*domain.Drawing, error) {
		if err := requireAdmin(ctx, tx); err != nil {
			return nil, err
		}
		if err := requireActiveOwner(ctx, tx, ownerID); err != nil {
			return nil, err
		}
		oldOwner, err := objectOwner(ctx, tx, "drawing", id)
		if err != nil {
			return nil, err
		}
		if oldOwner != 0 {
			return nil, fmt.Errorf("%w: 仅可分配待分配图纸", domain.ErrConflict)
		}
		var links int
		if err := tx.GetContext(ctx, &links, `SELECT (SELECT COUNT(*) FROM operation o JOIN machine m ON m.id=o.machine_id WHERE o.drawing_id=? AND COALESCE(m.owner_user_id,0)<>?) + (SELECT COUNT(*) FROM program_tool pt JOIN tool t ON t.id=pt.tool_id JOIN nc_program p ON p.id=pt.program_id JOIN operation o ON o.id=p.operation_id WHERE o.drawing_id=? AND COALESCE(t.owner_user_id,0)<>?)`, id, ownerID, id, ownerID); err != nil {
			return nil, err
		}
		if links > 0 {
			return nil, fmt.Errorf("%w: 图纸关联了其他归属的机台或刀具，请先解除关联", domain.ErrConflict)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE drawing SET owner_user_id=?,updated_at=? WHERE id=?`, ownerID, domain.Now(), id); err != nil {
			return nil, wrap(err)
		}
		if err := r.log(ctx, tx, "assign_owner", "drawing", id, fmt.Sprintf("数据归属从待分配调整为账号 %d", ownerID)); err != nil {
			return nil, err
		}
		return (&Repo{db: r.db, tx: tx}).rawGetDrawing(ctx, id)
	})
}
