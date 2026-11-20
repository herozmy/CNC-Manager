package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cnc-manager/internal/ncstore"
	"cnc-manager/internal/repo"
	"cnc-manager/internal/store"
)

func newTestServer(t *testing.T) (http.Handler, *repo.Repo) {
	t.Helper()
	h, r, _ := newTestServerMax(t, 1<<20)
	return h, r
}

func newTestServerMax(t *testing.T, maxBytes int64) (http.Handler, *repo.Repo, *sql.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	r := repo.New(db)
	files, err := ncstore.New(filepath.Join(t.TempDir(), "nc"), maxBytes, bytes.Repeat([]byte{7}, ncstore.KeySize))
	if err != nil {
		t.Fatal(err)
	}
	h := New(Options{Version: "test", Repo: r, Files: files, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), SessionLifetime: time.Hour})
	return h, r, db.DB
}

// do 发送请求。写请求自动带同源 Origin；cookies 原样附上。
func do(t *testing.T, h http.Handler, method, target, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		req.Header.Set("Origin", "http://example.com")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == authCookieName {
			return c
		}
	}
	t.Fatalf("响应没有会话 Cookie: %s", rec.Body.String())
	return nil
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("解析响应失败: %v 原文 %s", err, rec.Body.String())
	}
}

func setupAdmin(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	rec := do(t, h, http.MethodPost, "/api/auth/setup", `{"username":"admin","password":"password123"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("初始化失败 %d: %s", rec.Code, rec.Body.String())
	}
	return sessionCookie(t, rec)
}
