package repo

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"cnc-manager/internal/domain"
)

func TestPasswordChangeRollbackAndIsolation(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	u, _ := r.CreateFirstUser(ctx, "admin", "Original", "password123")
	ctx = domain.WithUser(ctx, u)
	_, err := r.db.Exec(`INSERT INTO app_user (id, username, display_name, password_hash, role, enabled, created_at, updated_at)
		SELECT 2, 'other', 'Other', password_hash, 'editor', 1, created_at, updated_at FROM app_user WHERE id = ?`, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	one, _, _ := r.CreateSession(ctx, u.ID, time.Hour)
	other, _, _ := r.CreateSession(ctx, 2, time.Hour)
	_, err = r.db.Exec(`CREATE TRIGGER reject_session_delete BEFORE DELETE ON user_session BEGIN SELECT RAISE(ABORT, 'test failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.ChangePassword(ctx, u.ID, "password123", "newpassword456"); err == nil {
		t.Fatal("expected rollback")
	}
	if _, err := r.Authenticate(ctx, "admin", "password123"); err != nil {
		t.Fatal("password must roll back", err)
	}
	if _, err := r.UserBySession(ctx, one); err != nil {
		t.Fatal("session must roll back", err)
	}
	if _, err := r.db.Exec(`DROP TRIGGER reject_session_delete`); err != nil {
		t.Fatal(err)
	}
	if err := r.ChangePassword(ctx, u.ID, "password123", "newpassword456"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.UserBySession(ctx, other); err != nil {
		t.Fatal("other user session affected", err)
	}
	if _, err := r.Authenticate(ctx, "other", "password123"); err != nil {
		t.Fatal("other user password affected", err)
	}
}

func TestConcurrentPasswordChangesOnlyOneSucceeds(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	u, _ := r.CreateFirstUser(ctx, "admin", "", "password123")
	ctx = domain.WithUser(ctx, u)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, next := range []string{"newpassword456", "newpassword789"} {
		wg.Add(1)
		go func() { defer wg.Done(); results <- r.ChangePassword(ctx, u.ID, "password123", next) }()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("got %d successful changes", successes)
	}
}

func TestNewPasswordCharacterBounds(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	u, _ := r.CreateFirstUser(ctx, "admin", "", "password123")
	ctx = domain.WithUser(ctx, u)
	for _, next := range []string{strings.Repeat("密", 7), strings.Repeat("密", 129)} {
		if !errors.Is(r.ChangePassword(ctx, u.ID, "password123", next), domain.ErrInvalid) {
			t.Fatal("invalid password accepted")
		}
	}
	next := strings.Repeat("密", 128)
	if err := r.ChangePassword(ctx, u.ID, "password123", next); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Authenticate(ctx, "admin", next); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentOldPasswordLoginCannotSurviveChange(t *testing.T) {
	for range 6 {
		r := newTestRepo(t)
		ctx := context.Background()
		u, _ := r.CreateFirstUser(ctx, "admin", "", "password123")
		ctx = domain.WithUser(ctx, u)
		start := make(chan struct{})
		var wg sync.WaitGroup
		var token string
		var loginErr, changeErr error
		wg.Add(2)
		go func() { defer wg.Done(); <-start; _, token, loginErr = r.Login(ctx, "admin", "password123", time.Hour) }()
		go func() {
			defer wg.Done()
			<-start
			changeErr = r.ChangePassword(ctx, u.ID, "password123", "newpassword456")
		}()
		close(start)
		wg.Wait()
		if changeErr != nil {
			t.Fatal(changeErr)
		}
		if loginErr != nil && !errors.Is(loginErr, domain.ErrUnauthorized) {
			t.Fatal(loginErr)
		}
		if token != "" {
			if _, err := r.UserBySession(ctx, token); !errors.Is(err, domain.ErrNotFound) {
				t.Fatal("old-password login survived password change", err)
			}
		}
	}
}
