// Package repo 是数据访问层，所有 SQL 集中在这里，不用 ORM。
package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"cnc-manager/internal/domain"
)

// Repo 聚合所有数据访问方法。
type Repo struct {
	db *sqlx.DB
}

// New 创建数据访问层。
func New(db *sqlx.DB) *Repo { return &Repo{db: db} }

// Ping 探活。
func (r *Repo) Ping(ctx context.Context) error { return r.db.PingContext(ctx) }

// wrap 把数据库错误翻译成领域错误。
func wrap(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "UNIQUE constraint failed"):
		return fmt.Errorf("%w: %s", domain.ErrConflict, humanizeConstraint(msg))
	case strings.Contains(msg, "FOREIGN KEY constraint failed"):
		return fmt.Errorf("%w: 关联记录不存在", domain.ErrInvalid)
	case strings.Contains(msg, "CHECK constraint failed"):
		return fmt.Errorf("%w: %s", domain.ErrInvalid, msg)
	}
	return err
}

// humanizeConstraint 把唯一约束错误翻译成操作工能看懂的提示。
func humanizeConstraint(msg string) string {
	switch {
	case strings.Contains(msg, "drawing.drawing_no"):
		return "该图纸号已存在"
	case strings.Contains(msg, "operation.drawing_id"):
		return "该图纸下已存在相同工序号"
	case strings.Contains(msg, "machine.code"):
		return "该机台编号已存在"
	case strings.Contains(msg, "tool.tool_no"):
		return "该刀具号已存在"
	case strings.Contains(msg, "machine_tool.machine_id"):
		return "同一机床不能有重复刀位"
	case strings.Contains(msg, "program_tool.version_id"):
		return "刀具表存在重复行号"
	case strings.Contains(msg, "nc_version.program_id"):
		return "版本号重复"
	case strings.Contains(msg, "app_user.username"):
		return "该用户名已存在"
	}
	return "数据与现有记录冲突"
}

// likePattern 构造 LIKE 模式并转义 % 和 _，避免 "A_1" 搜出 "AB1"。
func likePattern(keyword string) string {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return "%"
	}
	return "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(keyword) + "%"
}
