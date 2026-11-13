package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	"cnc-manager/internal/domain"
)

func TestFirstUserSetupAndLogin(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	required, err := r.SetupRequired(ctx)
	if err != nil || !required {
		t.Fatalf("空库应需要初始化: %v %v", required, err)
	}
	u, err := r.CreateFirstUser(ctx, " admin ", "", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if u.Username != "admin" || u.DisplayName != "admin" || u.Role != domain.RoleAdmin || !u.Enabled || u.ID == 0 {
		t.Fatalf("用户异常: %+v", u)
	}
	if required, _ = r.SetupRequired(ctx); required {
		t.Fatal("创建后不应再需要初始化")
	}
	if _, err := r.CreateFirstUser(ctx, "other", "", "password123"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("二次初始化应冲突，得到 %v", err)
	}

	got, err := r.Authenticate(ctx, "admin", "password123")
	if err != nil || got.ID != u.ID {
		t.Fatalf("登录失败: %+v %v", got, err)
	}
	for _, c := range [][2]string{{"admin", "wrong-pass"}, {"nobody", "password123"}, {"", ""}} {
		if _, err := r.Authenticate(ctx, c[0], c[1]); !errors.Is(err, domain.ErrUnauthorized) {
			t.Errorf("%q/%q 应返回 ErrUnauthorized，得到 %v", c[0], c[1], err)
		}
	}
}

func TestCreateFirstUserValidation(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	cases := []struct{ u, d, p string }{
		{"", "", "password123"},
		{"a b", "", "password123"},
		{"admin", "", "short"},
		{"admin", string(make([]byte, 65)), "password123"},
	}
	for _, c := range cases {
		if _, err := r.CreateFirstUser(ctx, c.u, c.d, c.p); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%+v 应为参数错误，得到 %v", c, err)
		}
	}
}

func TestDisabledUserCannotLogin(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	u, _ := r.CreateFirstUser(ctx, "admin", "", "password123")
	token, _, _ := r.CreateSession(ctx, u.ID, time.Hour)
	if _, err := r.db.Exec(`UPDATE app_user SET enabled = 0 WHERE id = ?`, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Authenticate(ctx, "admin", "password123"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatal("停用用户不应能登录")
	}
	if _, err := r.UserBySession(ctx, token); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("停用用户的已有会话应立即失效")
	}
}

func TestSessionLifecycle(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	u, _ := r.CreateFirstUser(ctx, "admin", "", "password123")

	token, expires, err := r.CreateSession(ctx, u.ID, time.Hour)
	if err != nil || token == "" || time.Until(expires) < 59*time.Minute {
		t.Fatalf("创建会话失败: %q %v %v", token, expires, err)
	}
	var stored string
	_ = r.db.Get(&stored, `SELECT token_hash FROM user_session`)
	if stored == token {
		t.Fatal("数据库不应存明文令牌")
	}
	got, err := r.UserBySession(ctx, token)
	if err != nil || got.ID != u.ID {
		t.Fatalf("按会话取用户失败: %v", err)
	}
	if _, err := r.UserBySession(ctx, "bogus"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("无效令牌应为 ErrNotFound")
	}
	if err := r.DeleteSession(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := r.UserBySession(ctx, token); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("注销后会话应失效")
	}
}

func TestExpiredSessionsAreRejectedAndCleaned(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	u, _ := r.CreateFirstUser(ctx, "admin", "", "password123")
	expired, _, _ := r.CreateSession(ctx, u.ID, -time.Minute)
	live, _, _ := r.CreateSession(ctx, u.ID, time.Hour)
	if _, err := r.UserBySession(ctx, expired); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("过期会话不应有效")
	}
	n, err := r.DeleteExpiredSessions(ctx)
	if err != nil || n != 1 {
		t.Fatalf("应清理 1 个过期会话，得到 %d %v", n, err)
	}
	if _, err := r.UserBySession(ctx, live); err != nil {
		t.Fatal("有效会话不应被清理")
	}
}
