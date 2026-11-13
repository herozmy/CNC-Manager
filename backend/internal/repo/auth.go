package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"cnc-manager/internal/auth"
	"cnc-manager/internal/domain"
)

const userColumns = `id, username, display_name, role, enabled, created_at, updated_at`

// SetupRequired 在还没有任何用户时为 true。
func (r *Repo) SetupRequired(ctx context.Context) (bool, error) {
	var n int
	if err := r.db.GetContext(ctx, &n, `SELECT COUNT(*) FROM app_user`); err != nil {
		return false, err
	}
	return n == 0, nil
}

func validateCredentials(username, displayName, password string) (string, string, error) {
	username = strings.TrimSpace(username)
	displayName = strings.TrimSpace(displayName)
	if username == "" || len(username) > 64 || strings.ContainsFunc(username, unicode.IsSpace) {
		return "", "", fmt.Errorf("%w: 用户名应为 1~64 个字符且不含空白", domain.ErrInvalid)
	}
	if len(password) < 8 || len(password) > 128 {
		return "", "", fmt.Errorf("%w: 密码长度应为 8~128 个字符", domain.ErrInvalid)
	}
	if displayName == "" {
		displayName = username
	}
	if len(displayName) > 64 {
		return "", "", fmt.Errorf("%w: 显示名称不能超过 64 个字符", domain.ErrInvalid)
	}
	return username, displayName, nil
}

// CreateFirstUser 创建初始管理员，只允许在没有任何用户时执行一次。
func (r *Repo) CreateFirstUser(ctx context.Context, username, displayName, password string) (domain.User, error) {
	username, displayName, err := validateCredentials(username, displayName, password)
	if err != nil {
		return domain.User{}, err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return domain.User{}, err
	}
	now := domain.Now()
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return domain.User{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var n int
	if err := tx.GetContext(ctx, &n, `SELECT COUNT(*) FROM app_user`); err != nil {
		return domain.User{}, err
	}
	if n != 0 {
		return domain.User{}, fmt.Errorf("%w: 初始管理员已经创建", domain.ErrConflict)
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO app_user (username, display_name, password_hash, role, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 1, ?, ?)`, username, displayName, hash, domain.RoleAdmin, now, now)
	if err != nil {
		return domain.User{}, wrap(err)
	}
	id, _ := res.LastInsertId()
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return domain.User{ID: id, Username: username, DisplayName: displayName, Role: domain.RoleAdmin,
		Enabled: true, CreatedAt: now, UpdatedAt: now}, nil
}

// Authenticate 校验用户名密码。用户不存在、被停用、密码错误一律返回 ErrUnauthorized。
func (r *Repo) Authenticate(ctx context.Context, username, password string) (domain.User, error) {
	var rec struct {
		domain.User
		PasswordHash string `db:"password_hash"`
	}
	err := r.db.GetContext(ctx, &rec,
		`SELECT `+userColumns+`, password_hash FROM app_user WHERE username = ?`, strings.TrimSpace(username))
	if errors.Is(err, sql.ErrNoRows) {
		auth.CheckDummy(password) // 用户不存在也做一次等价开销的计算，避免通过耗时枚举用户名
		return domain.User{}, domain.ErrUnauthorized
	}
	if err != nil {
		return domain.User{}, err
	}
	if !auth.CheckPassword(rec.PasswordHash, password) || !rec.Enabled {
		return domain.User{}, domain.ErrUnauthorized
	}
	return rec.User, nil
}

// CreateSession 创建会话，返回明文令牌与过期时间；数据库只存令牌哈希。
func (r *Repo) CreateSession(ctx context.Context, userID int64, lifetime time.Duration) (string, time.Time, error) {
	token, err := auth.NewToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := time.Now().UTC().Add(lifetime)
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO user_session (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		auth.HashToken(token), userID, expires.Format(time.RFC3339Nano), domain.Now())
	return token, expires, wrap(err)
}

// UserBySession 按令牌取有效会话对应的启用用户，无效时返回 ErrNotFound。
func (r *Repo) UserBySession(ctx context.Context, token string) (domain.User, error) {
	var u domain.User
	err := r.db.GetContext(ctx, &u,
		`SELECT u.id, u.username, u.display_name, u.role, u.enabled, u.created_at, u.updated_at
		 FROM user_session s JOIN app_user u ON u.id = s.user_id
		 WHERE s.token_hash = ? AND s.expires_at > ? AND u.enabled = 1`,
		auth.HashToken(token), time.Now().UTC().Format(time.RFC3339Nano))
	return u, wrap(err)
}

// DeleteSession 注销一个会话。
func (r *Repo) DeleteSession(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_session WHERE token_hash = ?`, auth.HashToken(token))
	return err
}

// DeleteExpiredSessions 清理过期会话，返回删除数量。由后台定时任务调用。
func (r *Repo) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM user_session WHERE expires_at <= ?`,
		time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
