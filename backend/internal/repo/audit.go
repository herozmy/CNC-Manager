package repo

import (
	"cnc-manager/internal/domain"
	"context"
	"github.com/jmoiron/sqlx"
	"strings"
)

func (r *Repo) log(ctx context.Context, q sqlx.ExtContext, action, entity string, id int64, detail string) error {
	return r.logResult(ctx, q, action, entity, id, detail, "success")
}

func (r *Repo) logResult(ctx context.Context, q sqlx.ExtContext, action, entity string, id int64, detail, result string) error {
	e := domain.AuditEntry{Action: action, EntityType: entity, EntityID: id, Detail: detail, Result: result}
	if u, ok := domain.UserFromContext(ctx); ok {
		current := u
		e.Actor = current.Username
		e.ActorID = &current.ID

		if current.IsAdmin() {
			e.ActorRoleName = "管理员"
		} else {
			e.ActorRoleName = "普通账号"
		}
	}
	names := map[string]string{"drawing": `SELECT drawing_no FROM drawing WHERE id=?`, "operation": `SELECT op_name FROM operation WHERE id=?`, "program": `SELECT program_no FROM nc_program WHERE id=?`, "version": `SELECT file_name FROM nc_version WHERE id=?`, "app_user": `SELECT username FROM app_user WHERE id=?`}
	if query := names[entity]; query != "" {
		if err := sqlx.GetContext(ctx, q, &e.EntityName, query, id); err != nil {
			return wrap(err)
		}
	}
	if entity == "drawing" || entity == "operation" || entity == "program" || entity == "version" {
		ownerID, err := objectOwner(ctx, q, entity, id)
		if err != nil {
			return err
		}
		if ownerID > 0 {
			e.OwnerUserID = &ownerID
			if err := sqlx.GetContext(ctx, q, &e.OwnerName, `SELECT display_name FROM app_user WHERE id=?`, ownerID); err != nil {
				return wrap(err)
			}
		} else {
			e.OwnerName = "待分配"
		}
	}
	return insertAudit(ctx, q, e)
}
func insertAudit(ctx context.Context, q sqlx.ExecerContext, e domain.AuditEntry) error {
	_, err := q.ExecContext(ctx, `INSERT INTO audit_log(action,entity_type,entity_id,entity_name,detail,actor,actor_id,actor_role_id,actor_role_name,role_id,role_name,owner_user_id,owner_name,result,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, e.Action, e.EntityType, e.EntityID, e.EntityName, e.Detail, e.Actor, e.ActorID, e.ActorRoleID, e.ActorRoleName, e.RoleID, e.RoleName, e.OwnerUserID, e.OwnerName, e.Result, domain.Now())
	return wrap(err)
}

// Rejections contain no looked-up object information or request bodies.
func (r *Repo) RecordEvent(ctx context.Context, e domain.AuditEntry) error {
	return insertAudit(ctx, r.db, e)
}
func (r *Repo) ListAuditLogs(ctx context.Context, f domain.AuditFilter) (domain.Page[domain.AuditEntry], error) {
	return inTransaction(ctx, r, func(tx *sqlx.Tx) (domain.Page[domain.AuditEntry], error) {
		out := domain.Page[domain.AuditEntry]{Items: []domain.AuditEntry{}}
		if err := requireAdmin(ctx, tx); err != nil {
			return out, err
		}
		if f.Page < 1 {
			f.Page = 1
		}
		if f.Size < 1 || f.Size > 200 {
			f.Size = 50
		}
		out.Page = f.Page
		out.Size = f.Size
		conditions := []string{"1=1"}
		args := []any{}
		for _, x := range []struct {
			column string
			value  *int64
		}{{"actor_id", f.UserID}} {
			if x.value != nil {
				conditions = append(conditions, x.column+"=?")
				args = append(args, *x.value)
			}
		}
		for _, x := range []struct{ column, value string }{{"action", f.Action}, {"result", f.Result}, {"entity_type", f.EntityType}} {
			if x.value != "" {
				conditions = append(conditions, x.column+"=?")
				args = append(args, x.value)
			}
		}
		if f.From != "" {
			conditions = append(conditions, "julianday(created_at)>=julianday(?)")
			args = append(args, f.From)
		}
		if f.To != "" {
			conditions = append(conditions, "julianday(created_at)<=julianday(?)")
			args = append(args, f.To)
		}
		where := " WHERE " + strings.Join(conditions, " AND ")
		if err := tx.GetContext(ctx, &out.Total, `SELECT COUNT(*) FROM audit_log`+where, args...); err != nil {
			return out, err
		}
		err := tx.SelectContext(ctx, &out.Items, `SELECT id,action,entity_type,COALESCE(entity_id,0) AS entity_id,entity_name,detail,actor,actor_id,actor_role_id,actor_role_name,role_id,role_name,owner_user_id,owner_name,result,created_at FROM audit_log`+where+` ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?`, append(args, f.Size, (f.Page-1)*f.Size)...)
		return out, err
	})
}
func (r *Repo) LogDownload(ctx context.Context, id int64) error {
	_, err := businessTransaction(ctx, r, "download", "version", id, func(t *Repo) (struct{}, error) {
		return struct{}{}, t.logResult(ctx, t.tx, "download", "version", id, "服务端开始处理 NC 文件下载", "started")
	})
	return err
}
