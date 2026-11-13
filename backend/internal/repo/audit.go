package repo

import (
	"context"

	"github.com/jmoiron/sqlx"

	"cnc-manager/internal/domain"
)

// log 写一条操作日志；actor 取自上下文中的登录用户。
func (r *Repo) log(ctx context.Context, q sqlx.ExtContext, action, entityType string, entityID int64, detail string) error {
	actor := ""
	if u, ok := domain.UserFromContext(ctx); ok {
		actor = u.Username
	}
	_, err := q.ExecContext(ctx,
		`INSERT INTO audit_log (action, entity_type, entity_id, detail, actor, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		action, entityType, entityID, detail, actor, domain.Now())
	return wrap(err)
}
