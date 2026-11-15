package repo

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"cnc-manager/internal/auth"
	"cnc-manager/internal/domain"
)

// UpdateDisplayName changes only the signed-in user's display name.
func (r *Repo) UpdateDisplayName(ctx context.Context, id int64, name string) (domain.User, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 64 {
		return domain.User{}, fmt.Errorf("%w: 显示名称应为 1~64 个字符", domain.ErrInvalid)
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback()
	if err := requireSelf(ctx, tx, id); err != nil {
		return domain.User{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE app_user SET display_name=?,updated_at=? WHERE id=?`, name, domain.Now(), id); err != nil {
		return domain.User{}, wrap(err)
	}
	if err := r.log(ctx, tx, "update_profile", "app_user", id, "修改个人显示名称"); err != nil {
		return domain.User{}, err
	}
	user, err := loadUser(ctx, tx, id)
	if err != nil {
		return domain.User{}, err
	}
	return user, tx.Commit()
}

// ChangePassword verifies the current password and revokes all sessions in the
// same transaction. Concurrent changes must verify against the latest hash.
func (r *Repo) ChangePassword(ctx context.Context, id int64, current, next string) error {
	if current == "" || len(current) > 512 {
		return fmt.Errorf("%w: 请输入原密码", domain.ErrInvalid)
	}
	if n := utf8.RuneCountInString(next); n < 8 || n > 128 {
		return fmt.Errorf("%w: 新密码应为 8~128 个字符", domain.ErrInvalid)
	}
	if current == next {
		return fmt.Errorf("%w: 新密码不能与原密码相同", domain.ErrInvalid)
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := requireSelf(ctx, tx, id); err != nil {
		return err
	}
	var oldHash string
	if err := tx.GetContext(ctx, &oldHash, `SELECT password_hash FROM app_user WHERE id = ? AND enabled = 1`, id); err != nil {
		return wrap(err)
	}
	if !auth.CheckPassword(oldHash, current) {
		return domain.ErrUnauthorized
	}
	hash, err := auth.HashPassword(next)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE app_user SET password_hash = ?, updated_at = ? WHERE id = ?`, hash, domain.Now(), id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_session WHERE user_id = ?`, id); err != nil {
		return err
	}
	if err := r.log(ctx, tx, "change_password", "app_user", id, "修改个人密码；全部会话已撤销"); err != nil {
		return err
	}
	return tx.Commit()
}
