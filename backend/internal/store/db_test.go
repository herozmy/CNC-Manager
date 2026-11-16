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
	if err := db.Get(&n, `SELECT COUNT(*) FROM schema_migration`); err != nil || n != 8 {
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

func TestRoleMigrationPreservesDataAndRevokesLegacySessions(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "legacy-roles.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.Exec(`CREATE TABLE schema_migration(version TEXT PRIMARY KEY,applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"0001_init", "0002_operation_coordinates", "0003_operation_centering", "0004_drawing_price"} {
		raw, err := migrationsFS.ReadFile("migrations/" + version + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		if err := applyMigration(ctx, db, version, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	queries := []string{
		`INSERT INTO app_user(id,username,display_name,password_hash,role,enabled,created_at,updated_at) VALUES(7,'legacy','Original','preserved-hash','admin',1,'before','before'),(8,'reader','Reader','reader-hash','viewer',1,'before','before')`,
		`INSERT INTO user_session(token_hash,user_id,expires_at,created_at) VALUES('old-session',7,'2099-01-01T00:00:00Z','before')`,
		`INSERT INTO drawing(id,drawing_no,price_cents,created_at,updated_at) VALUES(42,'EXISTING',12345,'before','before')`,
		`INSERT INTO operation(id,drawing_id,op_no,created_at,updated_at) VALUES(43,42,10,'before','before')`,
		`INSERT INTO nc_program(id,operation_id,program_no,created_at,updated_at) VALUES(44,43,'O7777','before','before')`,
		`INSERT INTO nc_blob(sha256,size_bytes,rel_path,created_at) VALUES('oldsha',9,'old.nc','before')`,
		`INSERT INTO nc_version(id,program_id,version_no,sha256,file_name,created_by,created_at) VALUES(45,44,1,'oldsha','original.nc','legacy','before')`,
		`UPDATE nc_program SET current_version_id=45 WHERE id=44`,
		`INSERT INTO audit_log(action,entity_type,entity_id,actor,detail,created_at) VALUES('create','drawing',42,'old-user','historical','before')`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := Migrate(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	for _, q := range []string{`SELECT COUNT(*) FROM user_session`, `SELECT COUNT(*) FROM pragma_foreign_key_check`, `SELECT COUNT(*) FROM app_user WHERE business_role_id IS NOT NULL`, `SELECT COUNT(*) FROM audit_log WHERE actor_id IS NOT NULL OR role_id IS NOT NULL`} {
		if err := db.Get(&n, q); err != nil || n != 0 {
			t.Fatal(q, n, err)
		}
	}
	var stored string
	if err := db.Get(&stored, `SELECT drawing_no||':'||price_cents||':'||role_id FROM drawing WHERE id=42`); err != nil || stored != "EXISTING:12345:1" {
		t.Fatal(stored, err)
	}
	if err := db.Get(&stored, `SELECT password_hash||':'||role FROM app_user WHERE id=7`); err != nil || stored != "preserved-hash:admin" {
		t.Fatal(stored, err)
	}
	if err := db.Get(&stored, `SELECT p.program_no||':'||p.current_version_id||':'||p.revision||':'||v.file_name FROM nc_program p JOIN nc_version v ON v.id=p.current_version_id WHERE p.id=44`); err != nil || stored != "O7777:45:1:original.nc" {
		t.Fatal(stored, err)
	}
	if _, err := db.Exec(`UPDATE drawing SET role_id=999 WHERE id=42`); err == nil {
		t.Fatal("foreign keys not restored")
	}
}

func TestPersonalOwnershipMigrationKeepsSharedDataAndDuplicateNumbers(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "shared.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TABLE schema_migration(version TEXT PRIMARY KEY,applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"0001_init", "0002_operation_coordinates", "0003_operation_centering", "0004_drawing_price", "0005_roles"} {
		raw, err := migrationsFS.ReadFile("migrations/" + version + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(raw)); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`INSERT INTO schema_migration VALUES(?,'before')`, version); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{
		`INSERT INTO business_role(id,name,created_at,updated_at) VALUES(2,'Shared','',''),(3,'Other','','')`,
		`INSERT INTO app_user(id,username,password_hash,role,business_role_id,created_at,updated_at) VALUES(1,'admin','hash','admin',NULL,'',''),(2,'a','hash','editor',2,'',''),(3,'b','hash','viewer',2,'','')`,
		`INSERT INTO drawing(id,role_id,drawing_no,created_at,updated_at) VALUES(10,2,'DUP','',''),(11,3,'DUP','','')`,
		`INSERT INTO machine(id,role_id,code,name,created_at,updated_at) VALUES(1,2,'M','M','','')`,
		`INSERT INTO operation(id,drawing_id,op_no,machine_id,created_at,updated_at) VALUES(1,10,10,1,'','')`,
		`INSERT INTO user_session(token_hash,user_id,expires_at,created_at) VALUES('old',2,'2099-01-01','')`,
		`INSERT INTO audit_log(action,entity_type,entity_id,actor,actor_role_id,actor_role_name,role_id,role_name,created_at) VALUES('create','drawing',10,'a',2,'Shared',2,'Shared','before')`,
	} {
		if _, err = db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err = Migrate(context.Background(), db); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{`SELECT COUNT(*) FROM user_session`, `SELECT COUNT(*) FROM drawing WHERE owner_user_id IS NOT NULL`, `SELECT COUNT(*) FROM machine WHERE owner_user_id IS NOT NULL`, `SELECT COUNT(*) FROM app_user WHERE business_role_id IS NOT NULL OR role NOT IN ('admin','editor')`, `SELECT COUNT(*) FROM pragma_foreign_key_check`} {
		var n int
		if err = db.Get(&n, query); err != nil || n != 0 {
			t.Fatal(query, n, err)
		}
	}
	var n int
	if err = db.Get(&n, `SELECT COUNT(*) FROM drawing WHERE drawing_no='DUP'`); err != nil || n != 2 {
		t.Fatal(n, err)
	}
	var name string
	if err = db.Get(&name, `SELECT actor_role_name||':'||role_name FROM audit_log`); err != nil || name != "Shared:Shared" {
		t.Fatal(name, err)
	}
	if _, err = db.Exec(`UPDATE drawing SET owner_user_id=2 WHERE id=10`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE drawing SET owner_user_id=2 WHERE id=11`); err == nil {
		t.Fatal("duplicate ownership accepted")
	}
	if _, err = db.Exec(`UPDATE drawing SET owner_user_id=3 WHERE id=11`); err != nil {
		t.Fatal(err)
	}
}

func TestMachineEnabledMigrationPreservesExistingData(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "machine.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.Exec(`CREATE TABLE schema_migration(version TEXT PRIMARY KEY,applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"0001_init", "0002_operation_coordinates", "0003_operation_centering", "0004_drawing_price", "0005_roles", "0006_personal_ownership"} {
		raw, err := migrationsFS.ReadFile("migrations/" + version + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		if err := applyMigration(ctx, db, version, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	for _, sql := range []string{
		`INSERT INTO machine(id,code,name,controller,remark,created_at,updated_at) VALUES(1,'OLD','旧机台','FANUC','保留','','')`,
		`INSERT INTO drawing(id,drawing_no,created_at,updated_at) VALUES(1,'D1','','')`,
		`INSERT INTO operation(id,drawing_id,op_no,machine_id,created_at,updated_at) VALUES(1,1,10,1,'','')`,
		`INSERT INTO nc_program(id,operation_id,program_no,created_at,updated_at) VALUES(1,1,'O1','before','before'),(2,1,'O2','before','before')`,
		`INSERT INTO machine_tool(machine_id,pocket_no,tool_no,updated_at) VALUES(1,1,'T01','')`,
	} {
		if _, err := db.Exec(sql); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	var actual string
	if err := db.Get(&actual, `SELECT m.code||':'||m.controller||':'||m.remark||':'||m.enabled||':'||o.machine_id||':'||mt.tool_no FROM machine m JOIN operation o ON o.machine_id=m.id JOIN machine_tool mt ON mt.machine_id=m.id`); err != nil || actual != "OLD:FANUC:保留:1:1:T01" {
		t.Fatal(actual, err)
	}
	if _, err := db.Exec(`UPDATE machine SET enabled=2`); err == nil {
		t.Fatal("invalid enabled state accepted")
	}
}

func TestProgramMachineMigrationPreservesIndependentChanges(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "program.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err = db.Exec(`CREATE TABLE schema_migration(version TEXT PRIMARY KEY,applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"0001_init", "0002_operation_coordinates", "0003_operation_centering", "0004_drawing_price", "0005_roles", "0006_personal_ownership", "0007_machine_enabled"} {
		raw, err := migrationsFS.ReadFile("migrations/" + version + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		if err = applyMigration(ctx, db, version, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{
		`INSERT INTO machine(id,code,enabled,created_at,updated_at) VALUES(7,'old',0,'old','old')`,
		`INSERT INTO drawing(id,drawing_no,created_at,updated_at) VALUES(1,'D','old','old')`,
		`INSERT INTO operation(id,drawing_id,op_no,machine_id,created_at,updated_at) VALUES(1,1,10,7,'old','old'),(2,1,20,NULL,'old','old')`,
		`INSERT INTO nc_program(id,operation_id,program_no,revision,created_at,updated_at) VALUES(1,1,'O1',4,'old','old'),(2,1,'O2',2,'old','old'),(3,2,'O3',1,'old','old')`,
	} {
		if _, err = db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = db.Get(&n, `SELECT COUNT(*) FROM nc_program WHERE machine_id=7 AND updated_at='old' AND ((id=1 AND revision=4) OR (id=2 AND revision=2))`); err != nil || n != 2 {
		t.Fatal(n, err)
	}
	if _, err = db.Exec(`UPDATE nc_program SET machine_id=NULL WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err = db.Get(&n, `SELECT COUNT(*) FROM nc_program WHERE machine_id IS NULL`); err != nil || n != 2 {
		t.Fatal(n, err)
	}
	if _, err = db.Exec(`DELETE FROM machine WHERE id=7`); err != nil {
		t.Fatal(err)
	}
	if err = db.Get(&n, `SELECT COUNT(*) FROM nc_program WHERE machine_id IS NULL`); err != nil || n != 3 {
		t.Fatal(n, err)
	}
	if err = db.Get(&n, `SELECT last_id FROM machine_id_sequence`); err != nil || n != 7 {
		t.Fatal(n, err)
	}
}
