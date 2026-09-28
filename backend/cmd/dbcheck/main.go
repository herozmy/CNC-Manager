// Command dbcheck 是数据库巡检与维护小工具。
//
// 两个用途：
//  1. 不启动服务，直接检查数据库状态（迁移版本、各表记录数、NC 文件库占用）；
//  2. 在线备份 —— 服务正在跑的时候也能做出一致性快照。
//
// 用法：
//
//	go run ./cmd/dbcheck
//	go run ./cmd/dbcheck -data "D:\cnccool\backend\data"
//	go run ./cmd/dbcheck -backup "D:\backup\cnccool-20260927.db"
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jmoiron/sqlx"

	"cnccool/internal/config"
	"cnccool/internal/store"
)

func main() {
	dataDir := flag.String("data", "", "数据目录，默认取 CNC_DATA_DIR 或 ./data")
	backupTo := flag.String("backup", "", "把数据库一致性备份到指定文件（服务运行中也可执行）")
	sqlQuery := flag.String("sql", "", "执行一条只读查询并打印结果，例如 -sql \"SELECT id, op_no, z_height_milli FROM operation\"")
	flag.Parse()

	if *dataDir != "" {
		_ = os.Setenv("CNC_DATA_DIR", *dataDir)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "读取配置失败:", err)
		os.Exit(1)
	}

	if _, err := os.Stat(cfg.DBPath); err != nil {
		fmt.Printf("数据库不存在：%s\n", cfg.DBPath)
		fmt.Println("（服务第一次启动时会自动创建，先运行 scripts\\run-backend.cmd）")
		os.Exit(1)
	}

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "打开数据库失败:", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	if *backupTo != "" {
		if err := backup(ctx, db, cfg, *backupTo); err != nil {
			fmt.Fprintln(os.Stderr, "备份失败:", err)
			os.Exit(1)
		}
		return
	}

	if *sqlQuery != "" {
		if err := runQuery(ctx, db, *sqlQuery); err != nil {
			fmt.Fprintln(os.Stderr, "查询失败:", err)
			os.Exit(1)
		}
		return
	}

	inspect(ctx, db, cfg)
}

// runQuery 执行任意只读查询并把结果打成表格。
//
// 排查问题时非常有用，比如：
//
//	-sql "SELECT id, op_no, z_height_milli FROM operation"
//	-sql "SELECT p.program_no, COUNT(v.id) FROM nc_program p LEFT JOIN nc_version v ON v.program_id=p.id GROUP BY p.id"
//
// 注意：这里不做 SQL 注入防护，因为它是给你自己在本地手工用的运维工具，
// 千万不要把它暴露成 HTTP 接口。
func runQuery(ctx context.Context, db *sqlx.DB, query string) error {
	rows, err := db.QueryxContext(ctx, query)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(cols, "\t"))

	n := 0
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		cells := make([]string, len(cols))
		for i, v := range vals {
			switch t := v.(type) {
			case nil:
				cells[i] = "NULL"
			case []byte:
				cells[i] = string(t)
			default:
				cells[i] = fmt.Sprint(t)
			}
		}
		fmt.Fprintln(w, strings.Join(cells, "\t"))
		n++
	}
	_ = w.Flush()
	if err := rows.Err(); err != nil {
		return err
	}
	fmt.Printf("\n共 %d 行\n", n)
	return nil
}

// backup 用 SQLite 的 VACUUM INTO 做在线一致性备份。
//
// 为什么不能直接拷贝 cnccool.db：
// 开了 WAL 之后，最新提交的数据可能还留在 cnccool.db-wal 里。
// 只拷主库文件会丢掉最后一批数据，得到一份"看起来正常但少了东西"的备份——
// 这种备份在真正需要恢复的时候才发现问题，是最危险的。
//
// VACUUM INTO 由 SQLite 自己保证一致性，服务正在写入也没问题。
func backup(ctx context.Context, db *sqlx.DB, cfg *config.Config, target string) error {
	abs, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("解析目标路径失败: %w", err)
	}
	if _, err := os.Stat(abs); err == nil {
		return fmt.Errorf("目标文件已存在，请换一个名字：%s", abs)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	start := time.Now()
	if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, abs); err != nil {
		return fmt.Errorf("VACUUM INTO 失败: %w", err)
	}

	fi, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("备份文件校验失败: %w", err)
	}
	fmt.Printf("数据库备份完成: %s\n", abs)
	fmt.Printf("  大小 %d 字节，耗时 %d ms\n", fi.Size(), time.Since(start).Milliseconds())
	fmt.Println()
	fmt.Println("重要：NC 程序文件在下面这个目录里，备份时必须一并拷走（整目录复制即可）：")
	fmt.Printf("  %s\n", cfg.NCDir)
	return nil
}

func inspect(ctx context.Context, db *sqlx.DB, cfg *config.Config) {
	fmt.Println("数据目录 :", cfg.DataDir)
	fmt.Println("数据库   :", cfg.DBPath)
	fmt.Println("NC 文件库:", cfg.NCDir)

	// 库文件体积。
	// 注意：WAL 模式下主库文件可能一直停在 4KB，数据实际在 -wal 里，
	// 直到 SQLite 自动检查点、或服务正常退出时才合并回主库文件。这是正常现象。
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if fi, err := os.Stat(cfg.DBPath + suffix); err == nil {
			fmt.Printf("  %-20s %10d 字节\n", filepath.Base(cfg.DBPath)+suffix, fi.Size())
		}
	}

	var versions []string
	if err := db.SelectContext(ctx, &versions, `SELECT version FROM schema_migration ORDER BY version`); err != nil {
		fmt.Fprintln(os.Stderr, "读取迁移记录失败:", err)
		os.Exit(1)
	}
	fmt.Printf("\n已应用迁移 (%d 个):\n", len(versions))
	for _, v := range versions {
		fmt.Println("  -", v)
	}

	var pageCount, pageSize int64
	var journalMode string
	_ = db.GetContext(ctx, &pageCount, `PRAGMA page_count`)
	_ = db.GetContext(ctx, &pageSize, `PRAGMA page_size`)
	_ = db.GetContext(ctx, &journalMode, `PRAGMA journal_mode`)
	fmt.Printf("\nSQLite 内部: journal_mode=%s  page_size=%d  page_count=%d  (约 %d 字节)\n",
		journalMode, pageSize, pageCount, pageSize*pageCount)

	tables := []string{
		"drawing", "operation", "nc_program", "nc_version", "nc_file",
		"program_tool", "tool", "machine", "audit_log", "app_setting",
	}
	fmt.Println("\n业务表记录数:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  表名\t记录数")
	for _, t := range tables {
		var n int64
		if err := db.GetContext(ctx, &n, `SELECT COUNT(*) FROM `+t); err != nil {
			fmt.Fprintf(w, "  %s\t读取失败: %v\n", t, err)
			continue
		}
		fmt.Fprintf(w, "  %s\t%d\n", t, n)
	}
	_ = w.Flush()

	files, bytes := 0, int64(0)
	_ = filepath.Walk(cfg.NCDir, func(_ string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		files++
		bytes += fi.Size()
		return nil
	})

	var fileRows, orphans int64
	_ = db.GetContext(ctx, &fileRows, `SELECT COUNT(*) FROM nc_file`)
	_ = db.GetContext(ctx, &orphans, `SELECT COUNT(*) FROM nc_file f
		WHERE NOT EXISTS (SELECT 1 FROM nc_version v WHERE v.file_id = f.id)`)

	fmt.Printf("\nNC 文件库: 物理文件 %d 个，共 %d 字节\n", files, bytes)
	fmt.Printf("          nc_file 登记 %d 条，其中无人引用的孤儿 %d 条\n", fileRows, orphans)
	if orphans > 0 {
		fmt.Println("          说明：删除程序时不会连带删物理文件（文件按内容共享，可能还被别的程序引用），")
		fmt.Println("                所以会留下孤儿，属预期行为，不影响使用。")
	}

	fmt.Printf("\n备份命令: go run ./cmd/dbcheck -backup \"D:\\backup\\cnccool-%s.db\"\n",
		time.Now().Format("20060102-150405"))
	fmt.Println("         备份数据库后，别忘了把 data\\nc 目录一并拷走。")
}
