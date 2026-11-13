// Package store 负责数据库连接与表结构迁移。
package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	// 纯 Go SQLite 驱动，无需 cgo，可 CGO_ENABLED=0 编出静态单文件。
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open 打开（必要时创建）SQLite 数据库。
//
// WAL 让读写互不阻塞；busy_timeout 遇锁等待；foreign_keys 启用级联（SQLite 默认关闭）。
// 连接池限制为 1 条，从根上避免 database is locked。
func Open(dbPath string) (*sqlx.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}
	dsn := "file:" + filepath.ToSlash(dbPath) +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=synchronous(NORMAL)"
	db, err := sqlx.Connect("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	return db, nil
}

// Migrate 按序号升序应用内嵌迁移，已执行的记入 schema_migration。每个脚本在单独事务里执行。
func Migrate(ctx context.Context, db *sqlx.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migration (
			version    TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`); err != nil {
		return fmt.Errorf("创建迁移记录表失败: %w", err)
	}
	var done []string
	if err := db.SelectContext(ctx, &done, `SELECT version FROM schema_migration`); err != nil {
		return fmt.Errorf("读取迁移记录失败: %w", err)
	}
	applied := make(map[string]bool, len(done))
	for _, v := range done {
		applied[v] = true
	}

	files, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("枚举迁移脚本失败: %w", err)
	}
	sort.Strings(files)

	for _, name := range files {
		version := strings.TrimSuffix(path.Base(name), ".sql")
		if applied[version] {
			continue
		}
		content, err := migrationsFS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("读取迁移 %s 失败: %w", version, err)
		}
		tx, err := db.BeginTxx(ctx, nil)
		if err != nil {
			return fmt.Errorf("开启迁移事务失败: %w", err)
		}
		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("执行迁移 %s 失败: %w", version, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migration (version, applied_at) VALUES (?, ?)`,
			version, time.Now().UTC().Format(time.RFC3339)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("记录迁移 %s 失败: %w", version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("提交迁移 %s 失败: %w", version, err)
		}
	}
	return nil
}
