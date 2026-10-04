package repo

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cnccool/internal/domain"
)

const (
	passwordIterations = 120000
	passwordSaltBytes  = 16
	sessionTokenBytes  = 32
)

var ErrInvalidCredentials = errors.New("用户名或密码错误")

type userWithPassword struct {
	domain.User
	PasswordHash string `db:"password_hash"`
}

func (r *Repo) AuthSetupRequired(ctx context.Context) (bool, error) {
	var count int
	if err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM app_user`); err != nil {
		return false, err
	}
	return count == 0, nil
}

func (r *Repo) CreateFirstUser(ctx context.Context, username, displayName, password string) (domain.User, error) {
	username = strings.TrimSpace(username)
	displayName = strings.TrimSpace(displayName)
	if username == "" || len(username) > 64 {
		return domain.User{}, fmt.Errorf("%w: 用户名长度应为 1~64 个字符", domain.ErrInvalid)
	}
	if len(password) < 8 || len(password) > 128 {
		return domain.User{}, fmt.Errorf("%w: 密码长度应为 8~128 个字符", domain.ErrInvalid)
	}
	if displayName == "" {
		displayName = username
	}
	if len(displayName) > 64 {
		return domain.User{}, fmt.Errorf("%w: 显示名称不能超过 64 个字符", domain.ErrInvalid)
	}
	hash, err := hashPassword(password)
	if err != nil {
		return domain.User{}, err
	}
	now := domain.Now()
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return domain.User{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var count int
	if err := tx.GetContext(ctx, &count, `SELECT COUNT(*) FROM app_user`); err != nil {
		return domain.User{}, err
	}
	if count != 0 {
		return domain.User{}, fmt.Errorf("%w: 初始管理员已经创建", domain.ErrConflict)
	}
	result, err := tx.ExecContext(ctx,
		`INSERT INTO app_user (username, display_name, password_hash, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, 1, ?, ?)`, username, displayName, hash, now, now)
	if err != nil {
		return domain.User{}, wrap(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.User{}, err
	}
	return domain.User{ID: id, Username: username, DisplayName: displayName, Enabled: true, CreatedAt: now, UpdatedAt: now}, nil
}

func (r *Repo) Authenticate(ctx context.Context, username, password string) (domain.User, error) {
	var record userWithPassword
	err := r.db.GetContext(ctx, &record,
		`SELECT id, username, display_name, password_hash, enabled, created_at, updated_at
		 FROM app_user WHERE username = ?`, strings.TrimSpace(username))
	if errors.Is(err, sql.ErrNoRows) || err == nil && (!record.Enabled || !checkPassword(record.PasswordHash, password)) {
		return domain.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return domain.User{}, err
	}
	return record.User, nil
}

func (r *Repo) CreateSession(ctx context.Context, userID int64, lifetime time.Duration) (string, time.Time, error) {
	tokenBytes := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	expires := time.Now().UTC().Add(lifetime)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO user_session (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		hashToken(token), userID, expires.Format(time.RFC3339Nano), domain.Now())
	return token, expires, err
}

func (r *Repo) UserBySession(ctx context.Context, token string) (domain.User, error) {
	var user domain.User
	err := r.db.GetContext(ctx, &user,
		`SELECT u.id, u.username, u.display_name, u.enabled, u.created_at, u.updated_at
		 FROM user_session s JOIN app_user u ON u.id = s.user_id
		 WHERE s.token_hash = ? AND s.expires_at > ? AND u.enabled = 1`,
		hashToken(token), time.Now().UTC().Format(time.RFC3339Nano))
	return user, wrap(err)
}

func (r *Repo) DeleteSession(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_session WHERE token_hash = ?`, hashToken(token))
	return err
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	derived := pbkdf2SHA256([]byte(password), salt, passwordIterations, 32)
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", passwordIterations,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(derived)), nil
}

func checkPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations < 1 || iterations > 1000000 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(want) == 0 {
		return false
	}
	got := pbkdf2SHA256([]byte(password), salt, iterations, len(want))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func pbkdf2SHA256(password, salt []byte, iterations, keyLength int) []byte {
	result := make([]byte, 0, keyLength)
	for block := 1; len(result) < keyLength; block++ {
		mac := hmac.New(sha256.New, password)
		_, _ = mac.Write(salt)
		_, _ = mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for index := 1; index < iterations; index++ {
			mac = hmac.New(sha256.New, password)
			_, _ = mac.Write(u)
			u = mac.Sum(nil)
			for offset := range t {
				t[offset] ^= u[offset]
			}
		}
		result = append(result, t...)
	}
	return result[:keyLength]
}
