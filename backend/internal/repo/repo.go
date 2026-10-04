// Package repo 是数据访问层，所有 SQL 都集中在这里。
//
// 刻意不使用 ORM：SQL 全部手写、可见、可改，换数据库时只需改这一层，
// 上层（service / httpapi）完全无感。这也是"方便迁移"最实在的保障。
package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"cnccool/internal/domain"
)

// Repo 聚合所有数据访问方法。
type Repo struct {
	db *sqlx.DB
}

// New 创建数据访问层。
func New(db *sqlx.DB) *Repo { return &Repo{db: db} }

// DB 暴露底层连接，供需要自定义事务的场景使用。
func (r *Repo) DB() *sqlx.DB { return r.db }

// Ping 探活，启动时用来确认数据库可用。
func (r *Repo) Ping(ctx context.Context) error { return r.db.PingContext(ctx) }

// wrap 把底层数据库错误翻译成领域错误，让 HTTP 层能给出准确的中文提示。
func wrap(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	msg := err.Error()
	if strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "constraint failed") {
		return fmt.Errorf("%w: %s", domain.ErrConflict, humanizeConstraint(msg))
	}
	return err
}

// humanizeConstraint 把 SQLite 的约束错误翻译成人能看懂的中文。
// 这些提示会直接弹给操作工看，所以要具体到"是哪个号重复了"。
func humanizeConstraint(msg string) string {
	switch {
	case strings.Contains(msg, "drawing.drawing_no"):
		return "该图纸号已存在"
	case strings.Contains(msg, "operation.drawing_id") && strings.Contains(msg, "op_no"):
		return "该图纸下已存在相同工序号"
	case strings.Contains(msg, "machine.code"):
		return "该机台编号已存在"
	case strings.Contains(msg, "tool.tool_no"):
		return "该刀具号已存在"
	case strings.Contains(msg, "program_tool.program_id"):
		return "刀具表存在重复行号"
	case strings.Contains(msg, "nc_version.program_id"):
		return "版本号重复"
	}
	return "数据与现有记录冲突"
}

// likePattern 构造 LIKE 模式串，并转义用户输入里的 % 和 _。
//
// 不转义的话，操作工搜 "A_1" 会因为 _ 匹配任意单字符而搜出 "AB1" 这类无关结果。
func likePattern(keyword string) string {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return "%"
	}
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(keyword) + "%"
}

func (r *Repo) log(ctx context.Context, q sqlx.ExtContext, action, entityType string, entityID int64, detail string) error {
	actor := "local"
	if user, ok := domain.UserFromContext(ctx); ok {
		actor = user.Username
	}
	_, err := q.ExecContext(ctx,
		`INSERT INTO audit_log (action, entity_type, entity_id, detail, actor, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		action, entityType, entityID, detail, actor, domain.Now())
	return wrap(err)
}
