package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"cnc-manager/internal/auth"
	"cnc-manager/internal/store"
)

// Match the official v1 exporter: numeric manifest format, data/cnccool.db,
// and content-addressed files beneath data/nc/. No v2 key is present.
func legacyArchive(t *testing.T, edit func(*sqlx.DB)) []byte {
	t.Helper()
	root := t.TempDir()
	db, err := store.Open(filepath.Join(root, "cnccool.db"))
	if err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TABLE schema_migration(version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`)
	files, _ := filepath.Glob("../migratev1/testdata/v1schema/*.sql")
	sort.Strings(files)
	if len(files) != 10 {
		t.Fatal("missing v1 schemas")
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		exec(string(raw))
		exec(`INSERT INTO schema_migration VALUES (?, 'now')`, strings.TrimSuffix(filepath.Base(f), ".sql"))
	}
	password, err := auth.HashPassword("LegacyPassword123")
	if err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO app_user(id,username,display_name,password_hash,created_at,updated_at) VALUES (5,'legacyadmin','旧管理员',?,'now','now')`, password)
	exec(`INSERT INTO user_session(user_id,token_hash,expires_at,created_at) VALUES (5,'oldsession','2099-01-01T00:00:00Z','now')`)
	exec(`INSERT INTO drawing(id,drawing_no,name,created_at,updated_at) VALUES (7,'V1-7','壳体','now','now')`)
	exec(`INSERT INTO operation(id,drawing_id,op_no,z_height_milli,sort_order,created_at,updated_at) VALUES(11,7,10,25500,2,'now','now')`)
	exec(`INSERT INTO nc_program(id,operation_id,program_no,current_version_id,created_at,updated_at) VALUES(21,11,'O1000',102,'now','now')`)
	plain := [][]byte{[]byte("O1000\nG00 X0\nM30\n"), {'(', 0xbe, 0xab, 0xb3, 0xb5, ')', '\n', 'M', '3', '0', '\n'}, []byte("O3000\nM30\n"), []byte("orphan")}
	encrypted := []string{"Q05DQ05DMDEIfkOlrC/BLImGR0YISt2ymGfgvd+CIGDXAuIOjhiXJ2VQb03/vujCzaMvwnA=", "Q05DQ05DMDFnQw0fHSo4tkqz2VzPJVZ5E7JJCiz3F/PdLDh5jlZBzPjk36BZWKQ="}
	for i, content := range plain {
		sum := sha256.Sum256(content)
		sha := hex.EncodeToString(sum[:])
		rel := sha[:2] + "/" + sha[2:4] + "/" + sha + ".nc"
		path := filepath.Join(root, "nc", rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		raw := content
		if i < 2 {
			raw, err = base64.StdEncoding.DecodeString(encrypted[i])
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		encoding := "utf-8"
		if i == 1 {
			encoding = "gbk"
		}
		exec(`INSERT INTO nc_file(id,sha256,size_bytes,original_name,rel_path,file_encoding,created_at) VALUES (?,?,?,'legacy.nc',?,?,'now')`, i+1, sha, len(content), rel, encoding)
		if i < 3 {
			exec(`INSERT INTO nc_version(id,program_id,version_no,file_id,file_name,created_at) VALUES (?,21,?,?,'','now')`, 101+i, i+1, i+1)
		}
	}
	exec(`INSERT INTO program_tool(id,program_id,version_id,seq,tool_no,tool_dia_milli,comp_amount_milli,custom_params_json) VALUES(31,21,102,1,'T05',12500,150,'[{"name":"x","value":"1"}]')`)
	exec(`INSERT INTO app_setting VALUES('unused','v1 value','now')`)
	if edit != nil {
		edit(db)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	w, _ := zw.Create("manifest.json")
	w.Write([]byte(`{"format":1,"version":"1.2.13","createdAt":"2026-01-02T03:04:05Z"}`))
	if err := addFile(zw, "data/cnccool.db", filepath.Join(root, "cnccool.db")); err != nil {
		t.Fatal(err)
	}
	if err := filepath.WalkDir(filepath.Join(root, "nc"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		return addFile(zw, "data/"+filepath.ToSlash(rel), path)
	}); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestV1WebMigrationAndRecovery(t *testing.T) {
	ctx := context.Background()
	raw := legacyArchive(t, nil)
	dest, db := fixture(t, "original")
	originalNC := seed(t, dest, db)
	_, token, err := dest.Repo.Login(ctx, "original", "password123", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	beforeKey, _ := os.ReadFile(dest.Layout.KeyPath)
	stage, err := dest.Inspect(ctx, bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	defer stage.Close()
	if stage.Format != LegacyFormat || stage.Version != "1.2.13" || stage.Summary.Users != 1 || stage.Summary.Versions != 3 || stage.Summary.Files != 3 || stage.Summary.Tools != 1 {
		t.Fatal(stage.Preview)
	}
	if stage.Migration == nil || *stage.Migration != (LegacyMigration{1, 1, 1}) {
		t.Fatal(stage.Migration)
	}
	if _, err := dest.Repo.UserBySession(ctx, token); err != nil {
		t.Fatal("inspection changed account", err)
	}
	var n int
	db.Get(&n, `SELECT COUNT(*) FROM app_user WHERE username='legacyadmin'`)
	if n != 0 {
		t.Fatal("inspection imported data")
	}
	sum := sha256.Sum256(raw)
	if stage.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatal("hash does not identify original ZIP")
	}
	name, err := dest.Apply(ctx, stage)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dest.Repo.UserBySession(ctx, token); err == nil {
		t.Fatal("old session survived import")
	}
	user, _, err := dest.Repo.Login(ctx, "legacyadmin", "LegacyPassword123", time.Hour)
	if err != nil || user.Role != "admin" {
		t.Fatal(user, err)
	}
	key, _ := os.ReadFile(dest.Layout.KeyPath)
	if !bytes.Equal(key, beforeKey) {
		t.Fatal("live key changed")
	}
	var owner, height, current, comp int
	db.Get(&owner, `SELECT COALESCE(owner_user_id,0) FROM drawing WHERE id=7`)
	db.Get(&height, `SELECT z_height_milli FROM operation WHERE id=11`)
	db.Get(&current, `SELECT current_version_id FROM nc_program WHERE id=21`)
	db.Get(&comp, `SELECT comp_amount_milli FROM program_tool WHERE id=31`)
	if owner != 0 || height != 25500 || current != 102 || comp != 150 {
		t.Fatal(owner, height, current, comp)
	}
	for i, want := range [][]byte{[]byte("O1000\nG00 X0\nM30\n"), {'(', 0xbe, 0xab, 0xb3, 0xb5, ')', '\n', 'M', '3', '0', '\n'}, []byte("O3000\nM30\n")} {
		var rel string
		if err := db.Get(&rel, `SELECT b.rel_path FROM nc_version v JOIN nc_blob b ON b.sha256=v.sha256 WHERE v.id=?`, 101+i); err != nil {
			t.Fatal(err)
		}
		got, err := dest.Files.ReadAll(rel)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatal("NC not preserved", i, got, err)
		}
	}
	path, _ := dest.RecoveryPath(name)
	recoveryRaw, _ := os.ReadFile(path)
	recovery, err := dest.Inspect(ctx, bytes.NewReader(recoveryRaw), int64(len(recoveryRaw)))
	if err != nil {
		t.Fatal(err)
	}
	defer recovery.Close()
	if recovery.Format != Format || recovery.Migration != nil {
		t.Fatal("recovery is not v2")
	}
	if _, err := dest.Apply(ctx, recovery); err != nil {
		t.Fatal(err)
	}
	if _, _, err := dest.Repo.Login(ctx, "original", "password123", time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := dest.Files.ReadAll(originalNC); err != nil {
		t.Fatal("old NC recovery failed", err)
	}
}

func TestInvalidV1ArchivesNeverChangeDestination(t *testing.T) {
	ctx := context.Background()
	raw := legacyArchive(t, nil)
	dest, _ := fixture(t, "original")
	_, token, _ := dest.Repo.Login(ctx, "original", "password123", time.Hour)
	unchanged := func(n string, b []byte) (string, []byte) { return n, b }
	cases := map[string][]byte{
		"missing NC": rewrite(t, raw, func(n string, b []byte) (string, []byte) {
			if blobName.MatchString(n) {
				return "", nil
			}
			return n, b
		}, ""),
		"corrupt NC": rewrite(t, raw, func(n string, b []byte) (string, []byte) {
			if blobName.MatchString(n) {
				b[len(b)-1] ^= 1
			}
			return n, b
		}, ""),
		"oversized NC": rewrite(t, raw, func(n string, b []byte) (string, []byte) {
			if blobName.MatchString(n) {
				b = bytes.Repeat([]byte{'x'}, (1<<20)+65)
			}
			return n, b
		}, ""),
		"mixed v1 v2":    rewrite(t, raw, unchanged, "data/cnc.db"),
		"unexpected key": rewrite(t, raw, unchanged, "data/nc.key"),
		"traversal":      rewrite(t, raw, unchanged, "data/nc/../../outside"),
		"duplicate":      rewrite(t, raw, unchanged, "data/cnccool.db"),
		"wrong manifest": rewrite(t, raw, func(n string, b []byte) (string, []byte) {
			if n == "manifest.json" {
				b = []byte(`{"format":2,"createdAt":"2026-01-02T03:04:05Z"}`)
			}
			return n, b
		}, ""),
		"bad DB": rewrite(t, raw, func(n string, b []byte) (string, []byte) {
			if n == "data/cnccool.db" {
				b = []byte("not sqlite")
			}
			return n, b
		}, ""),
		"old schema":       legacyArchive(t, func(db *sqlx.DB) { db.Exec(`DELETE FROM schema_migration WHERE version='0010_machine_tool_magazine'`) }),
		"no enabled admin": legacyArchive(t, func(db *sqlx.DB) { db.Exec(`UPDATE app_user SET enabled=0`) }),
		"wrong size":       legacyArchive(t, func(db *sqlx.DB) { db.Exec(`UPDATE nc_file SET size_bytes=1 WHERE id=1`) }),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if s, err := dest.Inspect(ctx, bytes.NewReader(raw), int64(len(raw))); err == nil {
				s.Close()
				t.Fatal("invalid v1 accepted")
			}
			if _, err := dest.Repo.UserBySession(ctx, token); err != nil {
				t.Fatal("current login changed", err)
			}
			if dirs, _ := filepath.Glob(filepath.Join(dest.Layout.Root, ".import-*")); len(dirs) != 0 {
				t.Fatal("staging leaked", dirs)
			}
		})
	}
}

func TestV1ArchiveFileLimit(t *testing.T) {
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	for i := 0; i < 100001; i++ {
		if _, err := z.Create("unused"); err != nil {
			t.Fatal(err)
		}
	}
	z.Create("manifest.json")
	z.Create("data/cnccool.db")
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	dest, _ := fixture(t, "original")
	if s, err := dest.Inspect(context.Background(), bytes.NewReader(out.Bytes()), int64(out.Len())); err == nil {
		s.Close()
		t.Fatal("more than 100000 NC entries accepted")
	} else if !strings.Contains(err.Error(), "NC 文件数量过多") {
		t.Fatal("v1 file limit not enforced", err)
	}
}
