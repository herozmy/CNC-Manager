package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMigrateCreatesSchemaAndIsIdempotent(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := Migrate(ctx, db); err != nil {
			t.Fatalf("第 %d 次迁移: %v", i+1, err)
		}
	}
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN
		('drawing','operation','nc_program','nc_version','nc_blob','program_tool','app_user')`); err != nil {
		t.Fatal(err)
	}
	if n != 7 {
		t.Fatalf("核心表数量 = %d，期望 7", n)
	}
	if err := db.Get(&n, `SELECT COUNT(*) FROM schema_migration`); err != nil || n != 1 {
		t.Fatalf("迁移记录 = %d, err=%v", n, err)
	}
}

func TestForeignKeysCascadeOnDrawingDelete(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	const ts = "2026-01-01T00:00:00Z"
	stmts := []string{
		`INSERT INTO drawing (id, drawing_no, created_at, updated_at) VALUES (1, 'D-1', '` + ts + `', '` + ts + `')`,
		`INSERT INTO operation (id, drawing_id, op_no, created_at, updated_at) VALUES (1, 1, 10, '` + ts + `', '` + ts + `')`,
		`INSERT INTO nc_program (id, operation_id, program_no, created_at, updated_at) VALUES (1, 1, 'O1', '` + ts + `', '` + ts + `')`,
		`INSERT INTO nc_blob (sha256, size_bytes, rel_path, created_at) VALUES ('aa', 1, 'aa/aa.nc', '` + ts + `')`,
		`INSERT INTO nc_version (id, program_id, version_no, sha256, created_at) VALUES (1, 1, 1, 'aa', '` + ts + `')`,
		`INSERT INTO program_tool (program_id, version_id, seq) VALUES (1, 1, 1)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	if _, err := db.Exec(`DELETE FROM drawing WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"operation", "nc_program", "nc_version", "program_tool"} {
		var n int
		if err := db.Get(&n, `SELECT COUNT(*) FROM `+table); err != nil || n != 0 {
			t.Fatalf("%s 未级联删除: n=%d err=%v", table, n, err)
		}
	}
}

func TestRejectsInvalidRoleAndEncoding(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	const ts = "2026-01-01T00:00:00Z"
	if _, err := db.Exec(`INSERT INTO app_user (username, password_hash, role, created_at, updated_at)
		VALUES ('a','h','root','` + ts + `','` + ts + `')`); err == nil {
		t.Fatal("非法角色应被拒绝")
	}
	if _, err := db.Exec(`INSERT INTO nc_blob (sha256, size_bytes, encoding, rel_path, created_at)
		VALUES ('bb', 1, 'latin1', 'x', '` + ts + `')`); err == nil {
		t.Fatal("非法编码应被拒绝")
	}
	if _, err := db.Exec(`INSERT INTO app_user (username, password_hash, created_at, updated_at)
		VALUES ('b','h','` + ts + `','` + ts + `')`); err != nil {
		t.Fatalf("默认角色插入失败: %v", err)
	}
	var role string
	if err := db.Get(&role, `SELECT role FROM app_user WHERE username='b'`); err != nil || role != "admin" {
		t.Fatalf("默认角色 = %q, err=%v", role, err)
	}
}
