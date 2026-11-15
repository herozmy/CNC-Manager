package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jmoiron/sqlx"

	"cnc-manager/internal/auth"
	"cnc-manager/internal/domain"
)

const userSelectColumns = `u.id, u.username, u.display_name, u.role, u.enabled, u.created_at, u.updated_at`
const userJoin = ` FROM app_user u `

type userRecord struct {
	domain.User
	PasswordHash string `db:"password_hash"`
}

func (rec userRecord) user() (domain.User, error) { return rec.User, nil }

func loadUser(ctx context.Context, q sqlx.QueryerContext, id int64) (domain.User, error) {
	var rec userRecord
	if err := sqlx.GetContext(ctx, q, &rec, `SELECT `+userSelectColumns+userJoin+`WHERE u.id = ?`, id); err != nil {
		return domain.User{}, wrap(err)
	}
	return rec.user()
}

func requireAdmin(ctx context.Context, q sqlx.QueryerContext) error {
	actor, ok := domain.UserFromContext(ctx)
	if !ok || actor.ID <= 0 || !actor.IsAdmin() {
		return domain.ErrForbidden
	}
	current, err := loadUser(ctx, q, actor.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ErrForbidden
	}
	if err != nil {
		return err
	}
	if !current.Enabled || !current.IsAdmin() {
		return domain.ErrForbidden
	}
	return nil
}

func requireSelf(ctx context.Context, q sqlx.QueryerContext, id int64) error {
	actor, ok := domain.UserFromContext(ctx)
	if !ok || actor.ID != id || id <= 0 {
		return domain.ErrForbidden
	}
	current, err := loadUser(ctx, q, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ErrForbidden
	}
	if err != nil {
		return err
	}
	if !current.Enabled {
		return domain.ErrForbidden
	}
	return nil
}

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
	if utf8.RuneCountInString(password) < 8 || utf8.RuneCountInString(password) > 128 {
		return "", "", fmt.Errorf("%w: 密码长度应为 8~128 个字符", domain.ErrInvalid)
	}
	if displayName == "" {
		displayName = username
	}
	if utf8.RuneCountInString(displayName) > 64 {
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
	u, err := loadUser(ctx, tx, id)
	if err != nil {
		return domain.User{}, err
	}
	if err := r.log(domain.WithUser(ctx, u), tx, "create", "app_user", id, "创建初始管理员"); err != nil {
		return domain.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return u, nil
}

// Authenticate 校验用户名密码。用户不存在、被停用、密码错误一律返回 ErrUnauthorized。
func (r *Repo) Authenticate(ctx context.Context, username, password string) (domain.User, error) {
	return authenticate(ctx, r.db, username, password)
}

func authenticate(ctx context.Context, db sqlx.QueryerContext, username, password string) (domain.User, error) {
	var rec userRecord
	err := sqlx.GetContext(ctx, db, &rec,
		`SELECT `+userSelectColumns+`, u.password_hash`+userJoin+`WHERE u.username = ?`, strings.TrimSpace(username))
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
	return rec.user()
}

// Login verifies the password and creates its session in a single transaction,
// so an in-flight login cannot restore a session after a password change.
func (r *Repo) Login(ctx context.Context, username, password string, lifetime time.Duration) (domain.User, string, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return domain.User{}, "", err
	}
	defer func() { _ = tx.Rollback() }()
	u, err := authenticate(ctx, tx, username, password)
	if err != nil {
		return domain.User{}, "", err
	}
	token, _, err := createSession(ctx, tx, u.ID, lifetime)
	if err != nil {
		return domain.User{}, "", err
	}
	if err := r.log(domain.WithUser(ctx, u), tx, "login", "app_user", u.ID, "账号登录"); err != nil {
		return domain.User{}, "", err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, "", err
	}
	return u, token, nil
}

// CreateSession 创建会话，返回明文令牌与过期时间；数据库只存令牌哈希。
func (r *Repo) CreateSession(ctx context.Context, userID int64, lifetime time.Duration) (string, time.Time, error) {
	return createSession(ctx, r.db, userID, lifetime)
}

func createSession(ctx context.Context, db sqlx.ExecerContext, userID int64, lifetime time.Duration) (string, time.Time, error) {
	token, err := auth.NewToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := time.Now().UTC().Add(lifetime)
	_, err = db.ExecContext(ctx,
		`INSERT INTO user_session (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		auth.HashToken(token), userID, expires.Format(time.RFC3339Nano), domain.Now())
	return token, expires, wrap(err)
}

// UserBySession 按令牌取有效会话对应的启用用户，无效时返回 ErrNotFound。
func (r *Repo) UserBySession(ctx context.Context, token string) (domain.User, error) {
	var rec userRecord
	err := r.db.GetContext(ctx, &rec,
		`SELECT `+userSelectColumns+userJoin+`JOIN user_session s ON s.user_id = u.id
   WHERE s.token_hash = ? AND s.expires_at > ? AND u.enabled = 1`,
		auth.HashToken(token), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return domain.User{}, wrap(err)
	}
	return rec.user()
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
