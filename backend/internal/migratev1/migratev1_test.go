package migratev1

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"

	"cnc-manager/internal/ncstore"
	"cnc-manager/internal/store"
)

const ts = "2026-01-02T03:04:05Z"

// v1Env 是一份临时搭建的 v1 数据目录。
type v1Env struct {
	dbPath string
	ncRoot string
	db     *sqlx.DB
}

func (e *v1Env) exec(t *testing.T, q string, args ...any) {
	t.Helper()
	if _, err := e.db.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// putFile 按 v1 的规则写入 NC 文件：encrypted=true 用 v1 硬编码密钥加密，否则存明文（更早版本）。
func (e *v1Env) putFile(t *testing.T, plain []byte, encrypted bool) (sum, rel string) {
	t.Helper()
	h := sha256.Sum256(plain)
	sum = hex.EncodeToString(h[:])
	rel = sum[0:2] + "/" + sum[2:4] + "/" + sum + ".nc"
	data := plain
	if encrypted {
		key := sha256.Sum256([]byte("cnccool-nc-file-storage-v1"))
		block, _ := aes.NewCipher(key[:])
		aead, _ := cipher.NewGCM(block)
		nonce := make([]byte, aead.NonceSize())
		_, _ = rand.Read(nonce)
		data = append(append([]byte("CNCCNC01"), nonce...), aead.Seal(nil, nonce, plain, nil)...)
	}
	abs := filepath.Join(e.ncRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return sum, rel
}

func newV1Env(t *testing.T) *v1Env {
	t.Helper()
	dir := t.TempDir()
	e := &v1Env{dbPath: filepath.Join(dir, "cnccool.db"), ncRoot: filepath.Join(dir, "nc")}
	db, err := sqlx.Connect("sqlite", "file:"+filepath.ToSlash(e.dbPath)+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	e.db = db
	t.Cleanup(func() { _ = db.Close() })

	e.exec(t, `CREATE TABLE schema_migration (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`)
	files, _ := filepath.Glob("testdata/v1schema/*.sql")
	sort.Strings(files)
	if len(files) != 10 {
		t.Fatalf("v1 迁移夹具应为 10 个，实际 %d", len(files))
	}
	for _, f := range files {
		sqlText, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		e.exec(t, string(sqlText))
		e.exec(t, `INSERT INTO schema_migration VALUES (?, ?)`, strings.TrimSuffix(filepath.Base(f), ".sql"), ts)
	}
	return e
}

// seed 写入覆盖各种情况的数据，返回期望的明文。
func (e *v1Env) seed(t *testing.T) (utf8Text, gbkBytes, plainText []byte) {
	t.Helper()
	utf8Text = []byte("O1000\nG00 X0\nM30\n")
	gbkBytes = []byte{'(', 0xbe, 0xab, 0xb3, 0xb5, ')', '\n', 'M', '3', '0', '\n'}
	plainText = []byte("O3000\nM30\n")

	sumA, relA := e.putFile(t, utf8Text, true)
	sumB, relB := e.putFile(t, gbkBytes, true)
	sumC, relC := e.putFile(t, plainText, false)
	sumD, relD := e.putFile(t, []byte("O9999\nM30\n"), true) // 孤儿

	e.exec(t, `INSERT INTO drawing (id, drawing_no, name, created_at, updated_at) VALUES (7, 'D-7', '壳体', ?, ?)`, ts, ts)
	e.exec(t, `INSERT INTO machine (id, code, name, created_at, updated_at) VALUES (3, 'M3', '立加', ?, ?)`, ts, ts)
	e.exec(t, `INSERT INTO machine_tool (machine_id, pocket_no, tool_no, updated_at) VALUES (3, 5, 'T05', ?)`, ts)
	e.exec(t, `INSERT INTO operation (id, drawing_id, op_no, machine_id, z_height_milli, sort_order, created_at, updated_at)
		VALUES (11, 7, 10, 3, 25500, 1, ?, ?)`, ts, ts)
	e.exec(t, `INSERT INTO tool (id, tool_no, name, created_at, updated_at) VALUES (2, 'T05', '立铣刀', ?, ?)`, ts, ts)
	// 物理文件被两个版本共享（秒传），且两处用的文件名不同。
	e.exec(t, `INSERT INTO nc_file (id, sha256, size_bytes, original_name, rel_path, created_at, file_encoding)
		VALUES (1, ?, ?, 'first.nc', ?, ?, 'utf-8'), (2, ?, ?, 'jing.nc', ?, ?, 'gbk'),
		       (3, ?, ?, 'plain.nc', ?, ?, 'utf-8'), (4, ?, 10, 'orphan.nc', ?, ?, 'utf-8')`,
		sumA, len(utf8Text), relA, ts, sumB, len(gbkBytes), relB, ts, sumC, len(plainText), relC, ts, sumD, relD, ts)
	e.exec(t, `INSERT INTO nc_program (id, operation_id, program_no, current_version_id, sort_order, created_at, updated_at)
		VALUES (21, 11, 'O1000', 102, 1, ?, ?), (22, 11, 'O2000', 103, 2, ?, ?)`, ts, ts, ts, ts)
	e.exec(t, `INSERT INTO nc_version (id, program_id, version_no, file_id, file_name, change_note, created_by, created_at) VALUES
		(101, 21, 1, 1, 'O1000_v1.nc', '首版', 'alice', ?),
		(102, 21, 2, 2, 'O1000_v2.nc', '', 'bob', ?),
		(103, 22, 1, 1, 'O2000.nc', '', '', ?),
		(104, 22, 2, 3, '', '老数据无文件名', '', ?)`, ts, ts, ts, ts)
	e.exec(t, `INSERT INTO program_tool (id, program_id, version_id, seq, tool_id, tool_no, offset_no, tool_dia_milli, comp_amount_milli, custom_params_json)
		VALUES (31, 21, 102, 1, 2, 'T05', 'D05', 12500, 150, '[{"k":"x"}]')`)
	e.exec(t, `INSERT INTO app_user (id, username, display_name, password_hash, created_at, updated_at) VALUES (1, 'admin', '管理员', 'pbkdf2$x', ?, ?)`, ts, ts)
	e.exec(t, `INSERT INTO user_session (token_hash, user_id, expires_at, created_at) VALUES ('h', 1, ?, ?)`, ts, ts)
	e.exec(t, `INSERT INTO audit_log (action, entity_type, entity_id, actor, created_at) VALUES ('create', 'drawing', 7, 'alice', ?)`, ts)
	return utf8Text, gbkBytes, plainText
}

func newV2(t *testing.T) (*sqlx.DB, *ncstore.Store) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "cnc.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	key := make([]byte, ncstore.KeySize)
	_, _ = rand.Read(key)
	s, err := ncstore.New(filepath.Join(dir, "nc"), 1<<20, key)
	if err != nil {
		t.Fatal(err)
	}
	return db, s
}

func TestRunMigratesAllDataLosslessly(t *testing.T) {
	e := newV1Env(t)
	utf8Text, gbkBytes, plainText := e.seed(t)
	db, ns := newV2(t)

	rep, err := Run(context.Background(), e.dbPath, e.ncRoot, db, ns)
	if err != nil {
		t.Fatal(err)
	}
	if rep.BlobsMigrated != 3 || rep.OrphanFiles != 1 || rep.SkippedSession != 1 {
		t.Fatalf("报告异常: %+v", rep)
	}

	// 主键与字段原样保留
	var z int
	if err := db.Get(&z, `SELECT z_height_milli FROM operation WHERE id = 11`); err != nil || z != 25500 {
		t.Fatalf("z_height_milli = %d err=%v", z, err)
	}
	var xy int
	if err := db.Get(&xy, `SELECT ABS(x_coordinate_milli) + ABS(y_coordinate_milli) FROM operation WHERE id = 11`); err != nil || xy != 0 {
		t.Fatalf("v1 导入的 X/Y 应默认为 0: %d %v", xy, err)
	}
	var cur int
	if err := db.Get(&cur, `SELECT current_version_id FROM nc_program WHERE id = 21`); err != nil || cur != 102 {
		t.Fatalf("current_version_id = %d err=%v", cur, err)
	}
	var machineID int
	if err := db.Get(&machineID, `SELECT machine_id FROM nc_program WHERE id=21`); err != nil || machineID != 3 {
		t.Fatal("program did not inherit migrated machine", machineID, err)
	}
	var tool struct {
		Dia    int    `db:"tool_dia_milli"`
		Comp   int    `db:"comp_amount_milli"`
		Custom string `db:"custom_params_json"`
	}
	if err := db.Get(&tool, `SELECT tool_dia_milli, comp_amount_milli, custom_params_json FROM program_tool WHERE id = 31`); err != nil ||
		tool.Dia != 12500 || tool.Comp != 150 || tool.Custom != `[{"k":"x"}]` {
		t.Fatalf("刀具补偿数据丢失: %+v err=%v", tool, err)
	}

	// 版本文件名：优先版本自带的；极老数据为空时回退到物理文件首次上传名
	names := map[int]string{}
	rows, _ := db.Queryx(`SELECT id, file_name FROM nc_version`)
	for rows.Next() {
		var id int
		var n string
		_ = rows.Scan(&id, &n)
		names[id] = n
	}
	if names[101] != "O1000_v1.nc" || names[103] != "O2000.nc" || names[104] != "plain.nc" {
		t.Fatalf("版本文件名异常: %v", names)
	}

	// 两个版本共享同一内容：新库只存一份
	var shared int
	if err := db.Get(&shared, `SELECT COUNT(DISTINCT sha256) FROM nc_version WHERE id IN (101, 103)`); err != nil || shared != 1 {
		t.Fatalf("共享内容应去重: %d err=%v", shared, err)
	}

	// 内容可用新密钥读回，且与原文逐字节一致；编码沿用旧库记录
	check := func(versionID int, want []byte, wantEnc string) {
		t.Helper()
		var b struct {
			Rel string `db:"rel_path"`
			Enc string `db:"encoding"`
		}
		if err := db.Get(&b, `SELECT b.rel_path, b.encoding FROM nc_version v JOIN nc_blob b ON b.sha256 = v.sha256 WHERE v.id = ?`, versionID); err != nil {
			t.Fatal(err)
		}
		got, err := ns.ReadAll(b.Rel)
		if err != nil || string(got) != string(want) {
			t.Fatalf("版本 %d 内容不一致: %q err=%v", versionID, got, err)
		}
		if b.Enc != wantEnc {
			t.Fatalf("版本 %d 编码 = %s，期望 %s", versionID, b.Enc, wantEnc)
		}
	}
	check(101, utf8Text, "utf-8")
	check(102, gbkBytes, "gbk")
	check(104, plainText, "utf-8")

	// 用户全部成为管理员；会话不导入
	var role string
	if err := db.Get(&role, `SELECT role FROM app_user WHERE username = 'admin'`); err != nil || role != "admin" {
		t.Fatalf("role = %q err=%v", role, err)
	}
	var sessions int
	_ = db.Get(&sessions, `SELECT COUNT(*) FROM user_session`)
	if sessions != 0 {
		t.Fatal("不应导入旧会话")
	}
}

func TestRunDoesNotModifyOldData(t *testing.T) {
	e := newV1Env(t)
	e.seed(t)
	before, _ := os.ReadFile(e.dbPath)
	db, ns := newV2(t)
	if _, err := Run(context.Background(), e.dbPath, e.ncRoot, db, ns); err != nil {
		t.Fatal(err)
	}
	_ = e.db.Close()
	after, _ := os.ReadFile(e.dbPath)
	if string(before) != string(after) {
		t.Fatal("迁移不应修改旧数据库文件")
	}
	if _, err := os.Stat(filepath.Join(e.ncRoot, "tmp")); err == nil {
		t.Fatal("迁移不应在旧 NC 目录里创建内容")
	}
}

func TestRunRollsBackWhenFileMissing(t *testing.T) {
	e := newV1Env(t)
	e.seed(t)
	var rel string
	if err := e.db.Get(&rel, `SELECT rel_path FROM nc_file WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(e.ncRoot, filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
	db, ns := newV2(t)
	_, err := Run(context.Background(), e.dbPath, e.ncRoot, db, ns)
	if err == nil || !strings.Contains(err.Error(), "文件缺失") {
		t.Fatalf("应因文件缺失失败，得到 %v", err)
	}
	var n int
	_ = db.Get(&n, `SELECT COUNT(*) FROM drawing`)
	if n != 0 {
		t.Fatal("失败后新库应保持为空")
	}
}

func TestRunRejectsTamperedFile(t *testing.T) {
	e := newV1Env(t)
	e.seed(t)
	var rel string
	_ = e.db.Get(&rel, `SELECT rel_path FROM nc_file WHERE id = 3`) // 明文文件
	if err := os.WriteFile(filepath.Join(e.ncRoot, filepath.FromSlash(rel)), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	db, ns := newV2(t)
	_, err := Run(context.Background(), e.dbPath, e.ncRoot, db, ns)
	if err == nil || !strings.Contains(err.Error(), "哈希") {
		t.Fatalf("应因哈希不符失败，得到 %v", err)
	}
}

func TestRunRefusesNonEmptyTarget(t *testing.T) {
	e := newV1Env(t)
	e.seed(t)
	db, ns := newV2(t)
	if _, err := db.Exec(`INSERT INTO drawing (drawing_no, created_at, updated_at) VALUES ('X', ?, ?)`, ts, ts); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), e.dbPath, e.ncRoot, db, ns); err == nil || !strings.Contains(err.Error(), "不是空库") {
		t.Fatalf("非空目标库应被拒绝，得到 %v", err)
	}
}

func TestRunRejectsOldSchemaAndMissingDB(t *testing.T) {
	e := newV1Env(t)
	e.exec(t, `DELETE FROM schema_migration WHERE version = '0010_machine_tool_magazine'`)
	db, ns := newV2(t)
	if _, err := Run(context.Background(), e.dbPath, e.ncRoot, db, ns); err == nil || !strings.Contains(err.Error(), "低于") {
		t.Fatalf("旧结构版本应被拒绝，得到 %v", err)
	}
	if _, err := Run(context.Background(), filepath.Join(t.TempDir(), "none.db"), e.ncRoot, db, ns); err == nil {
		t.Fatal("不存在的旧库应报错")
	}
}

func TestRunWithEmptyV1Database(t *testing.T) {
	e := newV1Env(t)
	db, ns := newV2(t)
	rep, err := Run(context.Background(), e.dbPath, e.ncRoot, db, ns)
	if err != nil {
		t.Fatal(err)
	}
	if rep.BlobsMigrated != 0 || rep.Counts["drawing"] != 0 {
		t.Fatalf("空库导入报告异常: %+v", rep)
	}
}
