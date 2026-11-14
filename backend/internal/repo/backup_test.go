package repo

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestRestoreTransactionRollsBackDataAndSessions(t *testing.T) {
	ctx := context.Background()
	source := newTestRepo(t)
	if _, err := source.CreateFirstUser(ctx, "source", "来源", "password123"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "backup.db")
	if err := source.BackupSnapshot(ctx, path); err != nil {
		t.Fatal(err)
	}
	dest := newTestRepo(t)
	if _, err := dest.CreateFirstUser(ctx, "original", "原账号", "password123"); err != nil {
		t.Fatal(err)
	}
	_, token, err := dest.Login(ctx, "original", "password123", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dest.db.Exec(`CREATE TRIGGER fail_restore BEFORE INSERT ON app_user BEGIN SELECT RAISE(ABORT,'injected write failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := dest.RestoreSnapshot(ctx, path); err == nil {
		t.Fatal("expected transaction failure")
	}
	if _, err := dest.UserBySession(ctx, token); err != nil {
		t.Fatal("rollback lost original session", err)
	}
	var name string
	if err := dest.db.Get(&name, `SELECT username FROM app_user`); err != nil || name != "original" {
		t.Fatal(name, err)
	}
	// A failed import must detach its input and leave the connection usable.
	if _, err := dest.db.Exec(`DROP TRIGGER fail_restore`); err != nil {
		t.Fatal(err)
	}
	if err := dest.RestoreSnapshot(ctx, path); err != nil {
		t.Fatal(err)
	}
}
