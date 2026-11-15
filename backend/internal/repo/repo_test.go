package repo

import (
	"context"
	"path/filepath"
	"testing"

	"cnc-manager/internal/domain"
	"cnc-manager/internal/store"
)

func newTestRepo(t *testing.T) *Repo {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return New(db)
}

// Business tests use a persisted account, just like an authenticated request.
func testActor(t *testing.T, r *Repo, name string) context.Context {
	t.Helper()
	var id int64
	if err := r.db.Get(&id, `SELECT id FROM app_user WHERE username=?`, name); err != nil {
		now := domain.Now()
		result, err := r.db.Exec(`INSERT INTO app_user(username,display_name,password_hash,role,enabled,created_at,updated_at) VALUES(?,?,?,'admin',1,?,?)`, name, name, "test-only", now, now)
		if err != nil {
			t.Fatal(err)
		}
		id, _ = result.LastInsertId()
	}
	u, err := loadUser(context.Background(), r.db, id)
	if err != nil {
		t.Fatal(err)
	}
	return domain.WithUser(context.Background(), u)
}
