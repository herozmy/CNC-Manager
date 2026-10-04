package domain

import "context"

type User struct {
	ID          int64  `db:"id" json:"id"`
	Username    string `db:"username" json:"username"`
	DisplayName string `db:"display_name" json:"displayName"`
	Enabled     bool   `db:"enabled" json:"enabled"`
	CreatedAt   string `db:"created_at" json:"createdAt"`
	UpdatedAt   string `db:"updated_at" json:"updatedAt"`
}

type authUserKey struct{}

func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, authUserKey{}, user)
}

func UserFromContext(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(authUserKey{}).(User)
	return user, ok
}
