// Package store 负责数据库连接与表结构迁移。
package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"

	// 纯 Go 实现的 SQLite 驱动，不需要 cgo / gcc。
	// 这一点很关键：本机没有 gcc，用 mattn/go-sqlite3 会直接编译失败；
	// 而且纯 Go 意味着可以 CGO_ENABLED=0 编出静态单文件 exe，
	// 后期进 Docker 时能直接用 scratch/alpine 镜像，体积压到 15MB 左右。
	_ "modernc.org/sqlite"

	"cnccool/internal/domain"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open 打开（必要时创建）SQLite 数据库，并设置关键 PRAGMA。
func Open(dbPath string) (*sqlx.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	// 四个 PRAGMA 缺一不可：
	//   journal_mode=WAL   读写不互相阻塞，是多人同时使用的前提
	//   busy_timeout=5000  遇到锁时等待而不是立刻抛 database is locked
	//   foreign_keys=1     启用外键级联删除（SQLite 默认是关闭的！）
	//   synchronous=NORMAL WAL 模式下的安全/性能平衡点
	dsn := "file:" + filepath.ToSlash(dbPath) +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=synchronous(NORMAL)"

	db, err := sqlx.Connect("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// SQLite 同一时刻只允许一个写事务。把连接池限制为 1 条连接，
	// 从根上消除 "database is locked"，代价是并发度降低——
	// 对编程组这个规模（几个人、几千条记录）完全够用。
	// 将来真需要提升并发，改成「1 条写连接 + N 条读连接」两个池即可，业务代码不动。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	return db, nil
}

// Migrate 应用内嵌的迁移脚本。
//
// 迁移文件命名为 NNNN_描述.sql，按序号升序执行，执行过的记入 schema_migration。
// 脚本内嵌在二进制里，所以单个 exe 拷到任何机器上、首次运行就会自动建表，
// 不需要任何手工步骤——这也是后期进 Docker 时"容器一启动库就好了"的基础。
func Migrate(ctx context.Context, db *sqlx.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migration (
			version    TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`); err != nil {
		return fmt.Errorf("创建迁移记录表失败: %w", err)
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return err
	}

	files, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("枚举迁移脚本失败: %w", err)
	}
	sort.Strings(files)

	for _, name := range files {
		version := strings.TrimSuffix(filepath.Base(name), ".sql")
		if applied[version] {
			continue
		}
		content, err := migrationsFS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("读取迁移 %s 失败: %w", version, err)
		}

		// SQLite 支持事务化 DDL，所以整个脚本要么全成功要么全回滚，
		// 不会出现"建了一半表"的中间状态。
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
			version, domain.Now()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("记录迁移 %s 失败: %w", version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("提交迁移 %s 失败: %w", version, err)
		}
	}
	return nil
}

// appliedVersions 读出已经执行过的迁移版本号。
func appliedVersions(ctx context.Context, db *sqlx.DB) (map[string]bool, error) {
	var versions []string
	if err := db.SelectContext(ctx, &versions, `SELECT version FROM schema_migration`); err != nil {
		return nil, fmt.Errorf("读取迁移记录失败: %w", err)
	}
	out := make(map[string]bool, len(versions))
	for _, v := range versions {
		out[v] = true
	}
	return out, nil
}
