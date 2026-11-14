package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"cnc-manager/internal/config"
	"cnc-manager/internal/ncstore"
	"cnc-manager/internal/repo"
	"cnc-manager/internal/store"
)

func fixture(t *testing.T, username string) (*Manager, *sqlx.DB) {
	t.Helper()
	layout := config.NewLayout(t.TempDir())
	db, err := store.Open(layout.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	key, err := ncstore.LoadOrCreateKey(layout.KeyPath)
	if err != nil {
		t.Fatal(err)
	}
	files, err := ncstore.New(layout.NCDir, 1<<20, key)
	if err != nil {
		t.Fatal(err)
	}
	m := &Manager{Repo: repo.New(db), Files: files, Layout: layout, Version: "2.1.0-test"}
	if _, err := m.Repo.CreateFirstUser(context.Background(), username, "迁移账号", "password123"); err != nil {
		t.Fatal(err)
	}
	return m, db
}

func seed(t *testing.T, m *Manager, db *sqlx.DB) string {
	t.Helper()
	blob, err := m.Files.Save(strings.NewReader("O1000\nG01 X1.5\nM30\n"))
	if err != nil {
		t.Fatal(err)
	}
	queries := []string{
		`INSERT INTO drawing(id,drawing_no,price_cents,created_at,updated_at) VALUES (7,'迁移图纸',12345,'now','now')`,
		`INSERT INTO operation(id,drawing_id,op_no,centering_method,x_coordinate_milli,created_at,updated_at) VALUES (8,7,10,'四面分中',1250,'now','now')`,
		`INSERT INTO nc_program(id,operation_id,program_no,created_at,updated_at) VALUES (9,8,'O1000','now','now')`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO nc_blob VALUES (?,?,?,?,?)`, blob.SHA256, blob.SizeBytes, blob.Encoding, blob.RelPath, "now"); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO nc_version(id,program_id,version_no,sha256,file_name,created_at) SELECT 10,9,1,sha256,'程序.nc','now' FROM nc_blob`,
		`UPDATE nc_program SET current_version_id=10 WHERE id=9`,
		`INSERT INTO program_tool(id,program_id,version_id,seq,tool_no,tool_name,comp_amount_milli) VALUES(11,9,10,1,'T01','铣刀',-125)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return blob.RelPath
}

func archive(t *testing.T, m *Manager) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "backup.zip")
	if err := m.Create(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestMigrationRoundTripAcrossKeysAndRecovery(t *testing.T) {
	ctx := context.Background()
	source, srcDB := fixture(t, "sourceadmin")
	rel := seed(t, source, srcDB)
	srcUser, _ := source.Repo.Authenticate(ctx, "sourceadmin", "password123")
	source.Repo.CreateSession(ctx, srcUser.ID, time.Hour)
	raw := archive(t, source)
	dest, dstDB := fixture(t, "destination")
	keyBefore, _ := os.ReadFile(dest.Layout.KeyPath)
	_, oldToken, err := dest.Repo.Login(ctx, "destination", "password123", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	stage, err := dest.Inspect(ctx, bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	defer stage.Close()
	if stage.Summary.Versions != 1 || stage.Summary.Tools != 1 || stage.Summary.Admins[0] != "sourceadmin" {
		t.Fatalf("bad preview: %+v", stage.Preview)
	}
	if _, err := dest.Repo.UserBySession(ctx, oldToken); err != nil {
		t.Fatal("inspect changed current login", err)
	}
	name, err := dest.Apply(ctx, stage)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dest.Repo.UserBySession(ctx, oldToken); err == nil {
		t.Fatal("old session survived")
	}
	if _, _, err := dest.Repo.Login(ctx, "sourceadmin", "password123", time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, _, err := dest.Repo.Login(ctx, "destination", "password123", time.Hour); err == nil {
		t.Fatal("destination account survived overwrite")
	}
	got, err := dest.Files.ReadAll(rel)
	if err != nil || string(got) != "O1000\nG01 X1.5\nM30\n" {
		t.Fatalf("NC not restored: %q %v", got, err)
	}
	keyAfter, _ := os.ReadFile(dest.Layout.KeyPath)
	if !bytes.Equal(keyBefore, keyAfter) {
		t.Fatal("destination key changed")
	}
	var coord, comp, current int
	var price int64
	if err := dstDB.Get(&price, `SELECT price_cents FROM drawing WHERE id=7`); err != nil || price != 12345 {
		t.Fatal("drawing price lost", price, err)
	}
	dstDB.Get(&coord, `SELECT x_coordinate_milli FROM operation WHERE id=8`)
	dstDB.Get(&comp, `SELECT comp_amount_milli FROM program_tool WHERE id=11`)
	dstDB.Get(&current, `SELECT current_version_id FROM nc_program WHERE id=9`)
	if coord != 1250 || comp != -125 || current != 10 {
		t.Fatal("relationships or values lost", coord, comp, current)
	}
	items, err := dest.Recoveries()
	if err != nil || len(items) != 1 || items[0].Name != name {
		t.Fatal(items, err)
	}
	path, _ := dest.RecoveryPath(name)
	recoveryRaw, _ := os.ReadFile(path)
	recovery, err := dest.Inspect(ctx, bytes.NewReader(recoveryRaw), int64(len(recoveryRaw)))
	if err != nil {
		t.Fatal(err)
	}
	defer recovery.Close()
	if _, err := dest.Apply(ctx, recovery); err != nil {
		t.Fatal(err)
	}
	if _, _, err := dest.Repo.Login(ctx, "destination", "password123", time.Hour); err != nil {
		t.Fatal(err)
	}
	var drawings int
	dstDB.Get(&drawings, `SELECT COUNT(*) FROM drawing`)
	if drawings != 0 {
		t.Fatal("recovery did not restore original contents")
	}
}

func rewrite(t *testing.T, raw []byte, edit func(string, []byte) (string, []byte), extra string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, f := range zr.File {
		in, _ := f.Open()
		data, _ := io.ReadAll(in)
		in.Close()
		name, data := edit(f.Name, data)
		if name == "" {
			continue
		}
		w, _ := zw.Create(name)
		w.Write(data)
	}
	if extra != "" {
		w, _ := zw.Create(extra)
		w.Write([]byte("bad"))
	}
	zw.Close()
	return out.Bytes()
}

func TestInvalidArchivesNeverChangeDestination(t *testing.T) {
	source, db := fixture(t, "source")
	seed(t, source, db)
	raw := archive(t, source)
	dest, dstDB := fixture(t, "original")
	_, token, _ := dest.Repo.Login(context.Background(), "original", "password123", time.Hour)
	unchanged := func(name string, b []byte) (string, []byte) { return name, b }
	cases := map[string][]byte{
		"not zip":   []byte("bad"),
		"traversal": rewrite(t, raw, unchanged, "../outside"),
		"duplicate": rewrite(t, raw, unchanged, "manifest.json"),
		"missing key": rewrite(t, raw, func(n string, b []byte) (string, []byte) {
			if n == "data/nc.key" {
				return "", nil
			}
			return n, b
		}, ""),
		"wrong key": rewrite(t, raw, func(n string, b []byte) (string, []byte) {
			if n == "data/nc.key" {
				return n, bytes.Repeat([]byte{0}, 32)
			}
			return n, b
		}, ""),
		"corrupt file": rewrite(t, raw, func(n string, b []byte) (string, []byte) {
			if blobName.MatchString(n) {
				b[len(b)-1] ^= 1
			}
			return n, b
		}, ""),
		"bad database": rewrite(t, raw, func(n string, b []byte) (string, []byte) {
			if n == "data/cnc.db" {
				return n, []byte("not sqlite")
			}
			return n, b
		}, ""),
		"wrong format": rewrite(t, raw, func(n string, b []byte) (string, []byte) {
			if n == "manifest.json" {
				return n, []byte(`{"format":1}`)
			}
			return n, b
		}, ""),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if s, err := dest.Inspect(context.Background(), bytes.NewReader(data), int64(len(data))); err == nil {
				s.Close()
				t.Fatal("accepted invalid archive")
			}
			if _, err := dest.Repo.UserBySession(context.Background(), token); err != nil {
				t.Fatal("original session changed")
			}
			var count int
			dstDB.Get(&count, `SELECT COUNT(*) FROM app_user WHERE username='original'`)
			if count != 1 {
				t.Fatal("original user lost")
			}
		})
	}
	if _, err := dest.Inspect(context.Background(), bytes.NewReader(raw), MaxArchiveBytes+1); err == nil {
		t.Fatal("oversize accepted")
	}
	paths, _ := filepath.Glob(filepath.Join(dest.Layout.Root, ".import-*"))
	if len(paths) != 0 {
		t.Fatal("temporary imports leaked", paths)
	}
}

func TestRecoveryFailureStopsImport(t *testing.T) {
	source, _ := fixture(t, "source")
	raw := archive(t, source)
	dest, _ := fixture(t, "original")
	stage, err := dest.Inspect(context.Background(), bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	defer stage.Close()
	if err := os.WriteFile(filepath.Join(dest.Layout.Root, "backups"), []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := dest.Apply(context.Background(), stage); err == nil {
		t.Fatal("import succeeded without recovery backup")
	}
	if _, _, err := dest.Repo.Login(context.Background(), "original", "password123", time.Hour); err != nil {
		t.Fatal(err)
	}
}

func TestRejectForeignSchemaAndBrokenReferences(t *testing.T) {
	source, db := fixture(t, "source")
	seed(t, source, db)
	dest, _ := fixture(t, "dest")
	for _, query := range []string{`CREATE TRIGGER malicious AFTER INSERT ON drawing BEGIN DELETE FROM app_user; END`, `DROP TRIGGER malicious`, `PRAGMA foreign_keys=OFF`, `UPDATE operation SET drawing_id=999`} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(query, "CREATE") || strings.HasPrefix(query, "UPDATE") {
			path := filepath.Join(t.TempDir(), "snapshot.db")
			if err := source.Repo.BackupSnapshot(context.Background(), path); err != nil {
				t.Fatal(err)
			}
			if _, _, err := dest.Repo.CheckBackupDB(context.Background(), path); err == nil {
				t.Fatal("accepted incompatible data")
			}
		}
	}
}

func TestRetainedBackupsPublishOnlyOnSuccess(t *testing.T) {
	m, _ := fixture(t, "admin")
	ctx := context.Background()
	first, err := m.Retain(ctx, "manual")
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.Retain(ctx, "manual")
	if err != nil || first == second {
		t.Fatal(second, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := m.Retain(canceled, "manual"); err == nil {
		t.Fatal("canceled backup succeeded")
	}
	if err := os.Remove(m.Layout.KeyPath); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Retain(ctx, "manual"); err == nil {
		t.Fatal("missing key accepted")
	}
	items, err := m.Recoveries()
	if err != nil || len(items) != 2 {
		t.Fatal(items, err)
	}
	entries, err := os.ReadDir(filepath.Join(m.Layout.Root, "backups"))
	if err != nil || len(entries) != 2 {
		t.Fatal("temporary files left", entries, err)
	}
	for _, item := range items {
		p, err := m.RecoveryPath(item.Name)
		if err != nil {
			t.Fatal(err)
		}
		st, err := os.Stat(p)
		if err != nil || st.Mode().Perm() != 0600 {
			t.Fatal("archive permissions", err)
		}
	}
}
