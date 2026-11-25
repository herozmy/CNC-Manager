package httpapi

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"cnc-manager/internal/backup"
	"cnc-manager/internal/config"
	"cnc-manager/internal/ncstore"
	"cnc-manager/internal/repo"
	"cnc-manager/internal/store"
)

func backupServer(t *testing.T) (*Server, *sqlx.DB) {
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
	return New(Options{DataDir: layout.Root, Version: "test", Repo: repo.New(db), Files: files}), db
}

func uploadBackup(t *testing.T, h http.Handler, endpoint string, raw []byte, hash, confirm string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	w, _ := mw.CreateFormFile("file", "backup.zip")
	w.Write(raw)
	mw.WriteField("sha256", hash)
	mw.WriteField("confirmation", confirm)
	mw.Close()
	req := httptest.NewRequest("POST", endpoint, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Origin", "http://example.com")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestBackupHTTPWorkflowAndAuthorization(t *testing.T) {
	h, db := backupServer(t)
	admin := setupAdmin(t, h)
	for _, endpoint := range []string{"/api/backup/export", "/api/backup/recoveries"} {
		if rec := do(t, h, "GET", endpoint, ""); rec.Code != 401 {
			t.Fatal("unauthenticated", rec.Code)
		}
	}
	if _, err := db.Exec(`UPDATE app_user SET role='viewer'`); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{"/api/backup/inspect", "/api/backup/import"} {
		if rec := uploadBackup(t, h, endpoint, []byte("bad"), "", "", admin); rec.Code != 403 {
			t.Fatal("viewer accepted", rec.Code)
		}
	}
	if rec := do(t, h, "GET", "/api/backup/export", "", admin); rec.Code != 403 {
		t.Fatal(rec.Code)
	}
	db.Exec(`UPDATE app_user SET role='admin'`)
	export := do(t, h, "GET", "/api/backup/export", "", admin)
	if export.Code != 200 || export.Header().Get("Content-Type") != "application/zip" {
		t.Fatal(export.Code, export.Body.String())
	}
	if export.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("backup cached")
	}
	raw := export.Body.Bytes()
	var saved []backup.Recovery
	decode(t, do(t, h, "GET", "/api/backup/recoveries", "", admin), &saved)
	if len(saved) != 1 {
		t.Fatal("export not retained", saved)
	}
	downloaded := do(t, h, "GET", "/api/backup/recoveries/"+saved[0].Name, "", admin)
	if downloaded.Code != 200 || !bytes.Equal(downloaded.Body.Bytes(), raw) {
		t.Fatal("retained export differs")
	}

	inspect := uploadBackup(t, h, "/api/backup/inspect", raw, "", "", admin)
	if inspect.Code != 200 {
		t.Fatal(inspect.Code, inspect.Body.String())
	}
	var preview backup.Preview
	decode(t, inspect, &preview)
	if rec := uploadBackup(t, h, "/api/backup/import", raw, preview.SHA256, "", admin); rec.Code != 400 {
		t.Fatal("missing confirmation accepted")
	}
	if rec := uploadBackup(t, h, "/api/backup/import", raw, "wrong", "覆盖导入", admin); rec.Code != 400 {
		t.Fatal("changed archive accepted")
	}
	if rec := do(t, h, "PUT", "/api/auth/me", `{"displayName":"导入后应该恢复"}`, admin); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	restored := uploadBackup(t, h, "/api/backup/import", raw, preview.SHA256, "覆盖导入", admin)
	if restored.Code != 200 {
		t.Fatal(restored.Code, restored.Body.String())
	}
	if rec := do(t, h, "GET", "/api/auth/me", "", admin); rec.Code != 401 {
		t.Fatal("old session survived")
	}
	login := do(t, h, "POST", "/api/auth/login", `{"username":"admin","password":"password123"}`)
	if login.Code != 200 {
		t.Fatal(login.Code, login.Body.String())
	}
	cookie := sessionCookie(t, login)
	var result map[string]string
	decode(t, restored, &result)
	list := do(t, h, "GET", "/api/backup/recoveries", "", cookie)
	var items []backup.Recovery
	decode(t, list, &items)
	if len(items) != 2 {
		t.Fatal(items, result)
	}
	recovery := do(t, h, "GET", "/api/backup/recoveries/"+result["recoveryName"], "", cookie)
	if recovery.Code != 200 {
		t.Fatal(recovery.Code)
	}
	if rec := do(t, h, "GET", "/api/backup/recoveries/not-allowed.zip", "", cookie); rec.Code != 400 {
		t.Fatal("bad path allowed", rec.Code)
	}
	// same-origin guard also applies to import requests before parsing their body.
	req := httptest.NewRequest("POST", "/api/backup/import", io.NopCloser(bytes.NewReader(raw)))
	req.AddCookie(cookie)
	req.Header.Set("Origin", "http://evil.example")
	response := httptest.NewRecorder()
	h.ServeHTTP(response, req)
	if response.Code != 403 {
		t.Fatal("cross origin accepted")
	}
	var dirs []string
	db.Select(&dirs, `SELECT file FROM pragma_database_list WHERE name='main'`)
	if len(dirs) != 1 || filepath.Base(dirs[0]) != "cnc.db" {
		t.Fatal("live DB was replaced", dirs)
	}
}

func TestDeleteBackup(t *testing.T) {
	h, db := backupServer(t)
	admin := setupAdmin(t, h)
	do(t, h, "GET", "/api/backup/export", "", admin)
	do(t, h, "GET", "/api/backup/export", "", admin)
	var items []backup.Recovery
	decode(t, do(t, h, "GET", "/api/backup/recoveries", "", admin), &items)
	if len(items) != 2 {
		t.Fatal(items)
	}
	path := "/api/backup/recoveries/" + items[0].Name
	if r := do(t, h, "DELETE", path, ""); r.Code != 401 {
		t.Fatal(r.Code)
	}
	db.Exec(`UPDATE app_user SET role='viewer'`)
	if r := do(t, h, "DELETE", path, "", admin); r.Code != 403 {
		t.Fatal(r.Code)
	}
	db.Exec(`UPDATE app_user SET role='admin'`)
	if r := do(t, h, "DELETE", "/api/backup/recoveries/cnc.db", "", admin); r.Code != 400 {
		t.Fatal(r.Code)
	}
	req := httptest.NewRequest("DELETE", path, nil)
	req.AddCookie(admin)
	req.Header.Set("Origin", "http://evil.example")
	response := httptest.NewRecorder()
	h.ServeHTTP(response, req)
	if response.Code != 403 {
		t.Fatal(response.Code)
	}
	if r := do(t, h, "DELETE", path, "", admin); r.Code != 204 {
		t.Fatal(r.Code, r.Body.String())
	}
	if r := do(t, h, "GET", path, "", admin); r.Code != 404 {
		t.Fatal(r.Code)
	}
	if r := do(t, h, "DELETE", path, "", admin); r.Code != 404 {
		t.Fatal(r.Code)
	}
	var left []backup.Recovery
	decode(t, do(t, h, "GET", "/api/backup/recoveries", "", admin), &left)
	if len(left) != 1 || left[0].Name != items[1].Name {
		t.Fatal(left)
	}
	if r := do(t, h, "GET", "/api/auth/me", "", admin); r.Code != 200 {
		t.Fatal(r.Code)
	}
}

func TestCreateBackupWithoutDownload(t *testing.T) {
	h, db := backupServer(t)
	admin := setupAdmin(t, h)
	if r := do(t, h, "POST", "/api/backup/create", "{}"); r.Code != 401 {
		t.Fatal(r.Code)
	}
	db.Exec(`UPDATE app_user SET role='viewer'`)
	if r := do(t, h, "POST", "/api/backup/create", "{}", admin); r.Code != 403 {
		t.Fatal(r.Code)
	}
	db.Exec(`UPDATE app_user SET role='admin'`)
	created := do(t, h, "POST", "/api/backup/create", "{}", admin)
	if created.Code != 201 || created.Header().Get("Content-Disposition") != "" {
		t.Fatal(created.Code, created.Header())
	}
	var result map[string]string
	decode(t, created, &result)
	var items []backup.Recovery
	decode(t, do(t, h, "GET", "/api/backup/recoveries", "", admin), &items)
	if len(items) != 1 || items[0].Name != result["name"] {
		t.Fatal(result, items)
	}
	downloaded := do(t, h, "GET", "/api/backup/recoveries/"+result["name"], "", admin)
	if downloaded.Code != 200 || downloaded.Header().Get("Content-Type") != "application/zip" {
		t.Fatal(downloaded.Code)
	}
	if r := uploadBackup(t, h, "/api/backup/inspect", downloaded.Body.Bytes(), "", "", admin); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	req := httptest.NewRequest("POST", "/api/backup/create", nil)
	req.AddCookie(admin)
	req.Header.Set("Origin", "http://evil.example")
	response := httptest.NewRecorder()
	h.ServeHTTP(response, req)
	if response.Code != 403 {
		t.Fatal(response.Code)
	}
}

func TestBackupSchedule(t *testing.T) {
	h, db := backupServer(t)
	admin := setupAdmin(t, h)
	const endpoint = "/api/backup/schedule"
	const weekly = `{"frequency":"weekly","weekday":1,"time":"08:30"}`
	var p backupSchedule
	decode(t, do(t, h, "GET", endpoint, "", admin), &p)
	if p.Frequency != "off" || p.Next != nil {
		t.Fatal(p)
	}
	if got := do(t, h, "PUT", endpoint, `{"frequency":"daily","time":"25:00"}`, admin); got.Code != 400 {
		t.Fatal(got.Code)
	}
	rec := do(t, h, "PUT", endpoint, weekly, admin)
	if rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	decode(t, rec, &p)
	if p.Next == nil || p.Next.In(beijing).Weekday() != time.Monday || p.Next.In(beijing).Format("15:04") != "08:30" {
		t.Fatal(p)
	}
	due := *p.Next
	for _, at := range []time.Time{due.Add(-time.Second), due, due.Add(time.Minute)} {
		if err := h.runDueBackup(context.Background(), at); err != nil {
			t.Fatal(err)
		}
	}
	items, err := h.backup.Recoveries()
	if err != nil || len(items) != 1 || !strings.HasPrefix(items[0].Name, "scheduled-") {
		t.Fatal(items, err)
	}
	saved, err := h.readSchedule()
	if err != nil || !saved.Next.Equal(due.AddDate(0, 0, 7)) {
		t.Fatal(saved, err)
	}
	if rec := do(t, h, "PUT", endpoint, `{"frequency":"off","weekday":1,"time":"08:30"}`, admin); rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	if err := h.runDueBackup(context.Background(), *saved.Next); err != nil {
		t.Fatal(err)
	}
	if items, _ := h.backup.Recoveries(); len(items) != 1 {
		t.Fatal(items)
	}
	if _, err := db.Exec("UPDATE app_user SET role='viewer'"); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"GET", "PUT"} {
		if rec := do(t, h, method, endpoint, weekly, admin); rec.Code != 403 {
			t.Fatal(rec.Code)
		}
	}
}

func TestScheduleRetention(t *testing.T) {
	h, _ := backupServer(t)
	admin := setupAdmin(t, h)
	ctx := context.Background()
	const endpoint = "/api/backup/schedule"
	const legacy = `{"frequency":"daily","weekday":1,"time":"08:00"}`
	if err := os.WriteFile(filepath.Join(h.backup.Layout.Root, "backup-schedule.json"), []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := h.readSchedule()
	if err != nil || p.Retention != 7 {
		t.Fatal(p, err)
	}
	for _, value := range []string{"0", "366", "1.5"} {
		body := strings.TrimSuffix(legacy, "}") + `,"retention":` + value + "}"
		if rec := do(t, h, "PUT", endpoint, body, admin); rec.Code != 400 {
			t.Fatal(rec.Code)
		}
	}
	dir := filepath.Join(h.backup.Layout.Root, "backups")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	names := []string{"scheduled-20260101-000000-1.zip", "scheduled-20260102-000000-1.zip", "manual-20260101-000000-1.zip", "pre-import-20260101-000000-1.zip"}
	for i, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("existing"), 0600); err != nil {
			t.Fatal(err)
		}
		stamp := time.Unix(int64(i+1), 0)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	if rec := do(t, h, "PUT", endpoint, `{"frequency":"daily","weekday":1,"time":"08:00","retention":2}`, admin); rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	decode(t, do(t, h, "PUT", endpoint, legacy, admin), &p)
	if p.Retention != 2 {
		t.Fatal(p)
	}
	key := h.backup.Layout.KeyPath
	h.backup.Layout.KeyPath = key + ".missing"
	if err := h.runDueBackup(ctx, *p.Next); err == nil {
		t.Fatal("expected archive failure")
	}
	before, err := h.backup.Recoveries()
	if err != nil || len(before) != 4 {
		t.Fatal(before, err)
	}
	h.backup.Layout.KeyPath = key
	p, err = h.readSchedule()
	if err != nil {
		t.Fatal(err)
	}
	if err = h.runDueBackup(ctx, *p.Next); err != nil {
		t.Fatal(err)
	}
	after, err := h.backup.Recoveries()
	if err != nil || len(after) != 4 {
		t.Fatal(after, err)
	}
	if _, err := os.Stat(filepath.Join(dir, names[0])); !os.IsNotExist(err) {
		t.Fatal("oldest scheduled archive remained", err)
	}
	for _, name := range names[1:] {
		if data, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(data) != "existing" {
			t.Fatal(name, err)
		}
	}
}
