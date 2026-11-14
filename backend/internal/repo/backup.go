package repo

import (
	"context"
	"fmt"
	"reflect"

	"github.com/jmoiron/sqlx"

	"cnc-manager/internal/domain"
	"cnc-manager/internal/store"
)

// The list is explicit: archive SQL is never executed against the live database.
var backupTables = []string{"drawing", "machine", "machine_tool", "tool", "operation", "nc_blob", "nc_program", "nc_version", "program_tool", "app_user", "audit_log"}

type BackupSummary struct {
	Users      int      `json:"users"`
	Drawings   int      `json:"drawings"`
	Operations int      `json:"operations"`
	Programs   int      `json:"programs"`
	Versions   int      `json:"versions"`
	Tools      int      `json:"tools"`
	Files      int      `json:"files"`
	Admins     []string `json:"admins"`
}

type BackupBlob struct {
	SHA256    string `db:"sha256"`
	RelPath   string `db:"rel_path"`
	SizeBytes int64  `db:"size_bytes"`
}

// BackupSnapshot takes a consistent SQLite snapshot including uncheckpointed WAL data.
// Login sessions are removed and vacuumed out of the portable copy.
func (r *Repo) BackupSnapshot(ctx context.Context, path string) error {
	if _, err := r.db.ExecContext(ctx, `VACUUM INTO ?`, path); err != nil {
		return err
	}
	db, err := store.Open(path)
	if err != nil {
		return err
	}
	defer db.Close()
	for _, query := range []string{`PRAGMA secure_delete=ON`, `DELETE FROM user_session`, `VACUUM`, `PRAGMA wal_checkpoint(TRUNCATE)`} {
		if _, err := db.ExecContext(ctx, query); err != nil {
			return err
		}
	}
	return nil
}

// CheckBackupDB requires the same schema, including constraints and indexes, as this
// server. Unexpected triggers/views and newer schemas are rejected before any writes.
func (r *Repo) CheckBackupDB(ctx context.Context, path string) (BackupSummary, []BackupBlob, error) {
	var summary BackupSummary
	db, err := store.Open(path)
	if err != nil {
		return summary, nil, err
	}
	defer db.Close()
	var integrity string
	if err := db.GetContext(ctx, &integrity, `PRAGMA integrity_check`); err != nil || integrity != "ok" {
		return summary, nil, fmt.Errorf("数据库完整性检查失败")
	}
	const schemaQuery = `SELECT type || ':' || name || ':' || COALESCE(sql, '') FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' ORDER BY type, name`
	var expected, actual []string
	if err := r.db.SelectContext(ctx, &expected, schemaQuery); err != nil {
		return summary, nil, err
	}
	if err := db.SelectContext(ctx, &actual, schemaQuery); err != nil {
		return summary, nil, err
	}
	if !reflect.DeepEqual(expected, actual) {
		return summary, nil, fmt.Errorf("备份与当前数据库结构不兼容，请使用与备份相同版本的程序导入")
	}
	var expectedMigrations, actualMigrations []string
	if err := r.db.SelectContext(ctx, &expectedMigrations, `SELECT version FROM schema_migration ORDER BY version`); err != nil {
		return summary, nil, err
	}
	if err := db.SelectContext(ctx, &actualMigrations, `SELECT version FROM schema_migration ORDER BY version`); err != nil {
		return summary, nil, err
	}
	if !reflect.DeepEqual(expectedMigrations, actualMigrations) {
		return summary, nil, fmt.Errorf("备份的数据库迁移版本不兼容")
	}
	if err := checkForeignKeys(ctx, db); err != nil {
		return summary, nil, err
	}
	for table, count := range map[string]*int{"app_user": &summary.Users, "drawing": &summary.Drawings, "operation": &summary.Operations, "nc_program": &summary.Programs, "nc_version": &summary.Versions, "program_tool": &summary.Tools, "nc_blob": &summary.Files} {
		if err := db.GetContext(ctx, count, `SELECT COUNT(*) FROM `+table); err != nil {
			return summary, nil, err
		}
	}
	if err := db.SelectContext(ctx, &summary.Admins, `SELECT username FROM app_user WHERE role='admin' AND enabled=1 ORDER BY username`); err != nil {
		return summary, nil, err
	}
	if len(summary.Admins) == 0 {
		return summary, nil, fmt.Errorf("备份中没有启用的管理员账号，无法导入")
	}
	var bad int
	if err := db.GetContext(ctx, &bad, `SELECT COUNT(*) FROM nc_program p JOIN nc_version v ON p.current_version_id=v.id WHERE p.id<>v.program_id`); err != nil {
		return summary, nil, err
	}
	if bad != 0 {
		return summary, nil, fmt.Errorf("备份中程序与当前版本不匹配")
	}
	var blobs []BackupBlob
	if err := db.SelectContext(ctx, &blobs, `SELECT sha256, rel_path, size_bytes FROM nc_blob ORDER BY sha256`); err != nil {
		return summary, nil, err
	}
	return summary, blobs, nil
}

func checkForeignKeys(ctx context.Context, q sqlx.QueryerContext) error {
	rows, err := q.QueryxContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("数据库关联记录检查失败")
	}
	return rows.Err()
}

// RestoreSnapshot replaces rows in one transaction, retaining this server's database
// handle, schema and encryption key. All old sessions are invalidated atomically.
// Caller must validate the snapshot and install its NC files under the maintenance lock.
func (r *Repo) RestoreSnapshot(ctx context.Context, path string) error {
	conn, err := r.db.Connx(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS incoming`, path); err != nil {
		return err
	}
	defer conn.ExecContext(context.Background(), `DETACH DATABASE incoming`)
	tx, err := conn.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `PRAGMA defer_foreign_keys=ON`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_session`); err != nil {
		return err
	}
	for i := len(backupTables) - 1; i >= 0; i-- {
		if _, err := tx.ExecContext(ctx, `DELETE FROM main.`+backupTables[i]); err != nil {
			return err
		}
	}
	for _, table := range backupTables {
		if _, err := tx.ExecContext(ctx, `INSERT INTO main.`+table+` SELECT * FROM incoming.`+table); err != nil {
			return err
		}
	}
	if err := checkForeignKeys(ctx, tx); err != nil {
		return err
	}
	if err := r.log(ctx, tx, "restore", "system", 0, "从整站备份导入数据；所有登录会话已撤销"); err != nil {
		return err
	}
	return tx.Commit()
}

// BackupError keeps archive validation failures readable at the HTTP boundary.
func BackupError(err error) error { return fmt.Errorf("%w: %v", domain.ErrInvalid, err) }
