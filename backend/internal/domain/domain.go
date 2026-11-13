// Package domain 定义领域模型与领域错误，不依赖 HTTP 和 SQL。
package domain

import (
	"context"
	"errors"
	"time"
)

// 领域错误。HTTP 层用 errors.Is 映射成状态码，映射只发生在那一处。
var (
	ErrNotFound     = errors.New("记录不存在")
	ErrConflict     = errors.New("记录已存在，或与现有数据冲突")
	ErrInvalid      = errors.New("参数不合法")
	ErrUnauthorized = errors.New("用户名或密码错误")
)

// Now 返回全库统一的时间文本：UTC RFC3339。
func Now() string { return time.Now().UTC().Format(time.RFC3339) }

// 用户角色。本轮不做权限区分，字段先行，后续加 RBAC 不改表。
const (
	RoleAdmin  = "admin"
	RoleEditor = "editor"
	RoleViewer = "viewer"
)

// User 是登录用户，不含密码哈希。
type User struct {
	ID          int64  `db:"id" json:"id"`
	Username    string `db:"username" json:"username"`
	DisplayName string `db:"display_name" json:"displayName"`
	Role        string `db:"role" json:"role"`
	Enabled     bool   `db:"enabled" json:"enabled"`
	CreatedAt   string `db:"created_at" json:"createdAt"`
	UpdatedAt   string `db:"updated_at" json:"updatedAt"`
}

type userKey struct{}

// WithUser 把当前登录用户放入请求上下文。
func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userKey{}, u)
}

// UserFromContext 取出当前登录用户。
func UserFromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userKey{}).(User)
	return u, ok
}
