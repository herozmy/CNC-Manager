// Package store 负责数据库连接与表结构迁移。
package store

import (
	"context"
	"embed"
	"errors"
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
		if err := applyMigration(ctx, db, version, string(content)); err != nil {
			return err
		}
	}
	return nil
}

// applyMigration pins one connection so SQLite's per-connection foreign-key
// setting cannot escape to another operation. Adding a non-null REFERENCES
// column to populated tables requires this setting to be disabled outside the
// transaction; all links are verified before committing the new schema.
func applyMigration(ctx context.Context, db *sqlx.DB, version, content string) (result error) {
	conn, err := db.Connx(ctx)
	if err != nil {
		return fmt.Errorf("取得迁移连接失败: %w", err)
	}
	defer conn.Close()
	relaxed := version == "0005_roles"
	if relaxed {
		if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys=OFF`); err != nil {
			return err
		}
		defer func() {
			if _, err := conn.ExecContext(context.Background(), `PRAGMA foreign_keys=ON`); err != nil {
				result = errors.Join(result, fmt.Errorf("恢复外键检查失败: %w", err))
			}
		}()
	}
	tx, err := conn.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启迁移事务失败: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, content); err != nil {
		return fmt.Errorf("执行迁移 %s 失败: %w", version, err)
	}
	if relaxed {
		var broken int
		if err := tx.GetContext(ctx, &broken, `SELECT COUNT(*) FROM pragma_foreign_key_check`); err != nil {
			return err
		}
		if broken != 0 {
			return fmt.Errorf("迁移 %s 存在无效关联记录", version)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migration (version, applied_at) VALUES (?, ?)`, version, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("记录迁移 %s 失败: %w", version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交迁移 %s 失败: %w", version, err)
	}
	return nil
}
