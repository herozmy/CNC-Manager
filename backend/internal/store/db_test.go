package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestDrawingPriceMigrationPreservesExistingData(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE schema_migration (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"0001_init", "0002_operation_coordinates", "0003_operation_centering"} {
		migration, err := migrationsFS.ReadFile("migrations/" + version + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(migration)); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migration VALUES (?, 'before')`, version); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO drawing (id,drawing_no,name,material,remark,created_at,updated_at) VALUES (1,'LEGACY','壳体','6061','保留','before','before')`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := Migrate(context.Background(), db); err != nil {
			t.Fatal(err)
		}
	}
	var price sql.NullInt64
	if err := db.Get(&price, `SELECT price_cents FROM drawing WHERE id=1`); err != nil || price.Valid {
		t.Fatal(price, err)
	}
	var text string
	if err := db.Get(&text, `SELECT drawing_no || name || material || remark || created_at || updated_at FROM drawing WHERE id=1`); err != nil || text != "LEGACY壳体6061保留beforebefore" {
		t.Fatal(text, err)
	}
	for _, value := range []string{"-1", "1.5", "100000000000", "'invalid'"} {
		if _, err := db.Exec(`UPDATE drawing SET price_cents=` + value + ` WHERE id=1`); err == nil {
			t.Fatal("invalid stored price accepted", value)
		}
	}
}

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
	if err := db.Get(&n, `SELECT COUNT(*) FROM schema_migration`); err != nil || n != 4 {
		t.Fatalf("迁移记录 = %d, err=%v", n, err)
	}
}

func TestCoordinateMigrationPreservesExistingZ(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	initial, err := migrationsFS.ReadFile("migrations/0001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(initial)); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`CREATE TABLE schema_migration (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`,
		`INSERT INTO schema_migration VALUES ('0001_init', '2026-11-14T00:00:00Z')`,
		`INSERT INTO drawing (id, drawing_no, created_at, updated_at) VALUES (1, 'LEGACY', '', '')`,
		`INSERT INTO operation (id, drawing_id, op_no, z_height_milli, fixture, created_at, updated_at) VALUES (1, 1, 10, 25500, '虎钳', '', '')`,
		`INSERT INTO nc_program (id, operation_id, program_no, created_at, updated_at) VALUES (1, 1, 'O1', '', '')`,
	} {
		if _, err := db.Exec(sql); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := Migrate(context.Background(), db); err != nil {
			t.Fatal(err)
		}
	}
	var coords struct {
		X               int64  `db:"x_coordinate_milli"`
		Y               int64  `db:"y_coordinate_milli"`
		Z               int64  `db:"z_height_milli"`
		Fixture         string `db:"fixture"`
		CenteringMethod string `db:"centering_method"`
	}
	if err := db.Get(&coords, `SELECT x_coordinate_milli, y_coordinate_milli, z_height_milli, fixture, centering_method FROM operation WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if coords.X != 0 || coords.Y != 0 || coords.Z != 25500 || coords.Fixture != "虎钳" || coords.CenteringMethod != "" {
		t.Fatalf("迁移不应改变原值: %+v", coords)
	}
	var operationID int64
	if err := db.Get(&operationID, `SELECT operation_id FROM nc_program WHERE id = 1`); err != nil || operationID != 1 {
		t.Fatalf("迁移不应破坏程序关联: %d %v", operationID, err)
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
