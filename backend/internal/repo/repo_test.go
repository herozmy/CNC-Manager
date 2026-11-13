package repo

import (
	"context"
	"path/filepath"
	"testing"

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
