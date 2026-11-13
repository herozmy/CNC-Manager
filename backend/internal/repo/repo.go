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
		return fmt.Errorf("%w: %s", domain.ErrConflict, msg)
	case strings.Contains(msg, "FOREIGN KEY constraint failed"):
		return fmt.Errorf("%w: 关联记录不存在", domain.ErrInvalid)
	case strings.Contains(msg, "CHECK constraint failed"):
		return fmt.Errorf("%w: %s", domain.ErrInvalid, msg)
	}
	return err
}
