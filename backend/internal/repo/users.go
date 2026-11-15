package repo

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"cnc-manager/internal/auth"
	"cnc-manager/internal/domain"
)

func (r *Repo) ListUsers(ctx context.Context) ([]domain.User, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := requireAdmin(ctx, tx); err != nil {
		return nil, err
	}
	records := []userRecord{}
	if err := tx.SelectContext(ctx, &records, `SELECT `+userSelectColumns+userJoin+`ORDER BY u.id`); err != nil {
		return nil, err
	}
	users := make([]domain.User, 0, len(records))
	for _, rec := range records {
		user, err := rec.user()
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, tx.Commit()
}

func validateUserRole(identity string) error {
	if identity != domain.RoleAdmin && identity != domain.RoleEditor {
		return fmt.Errorf("%w: 账号身份必须为管理员或普通账号", domain.ErrInvalid)
	}
	return nil
}

func (r *Repo) CreateUser(ctx context.Context, in domain.CreateUserInput) (domain.User, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback()
	if err := requireAdmin(ctx, tx); err != nil {
		return domain.User{}, err
	}
	username, displayName, err := validateCredentials(in.Username, in.DisplayName, in.Password)
	if err != nil {
		return domain.User{}, err
	}
	if err := validateUserRole(in.Role); err != nil {
		return domain.User{}, err
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return domain.User{}, err
	}
	now := domain.Now()
	res, err := tx.ExecContext(ctx, `INSERT INTO app_user (username,display_name,password_hash,role,enabled,created_at,updated_at) VALUES (?,?,?,?,1,?,?)`, username, displayName, hash, in.Role, now, now)
	if err != nil {
		return domain.User{}, wrap(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.User{}, err
	}
	if err := r.log(ctx, tx, "create", "app_user", id, "创建账号 "+username); err != nil {
		return domain.User{}, err
	}
	user, err := loadUser(ctx, tx, id)
	if err != nil {
		return domain.User{}, err
	}
	return user, tx.Commit()
}

func (r *Repo) UpdateUser(ctx context.Context, id int64, in domain.UpdateUserInput) (domain.User, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback()
	if err := requireAdmin(ctx, tx); err != nil {
		return domain.User{}, err
	}
	old, err := loadUser(ctx, tx, id)
	if err != nil {
		return domain.User{}, err
	}
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if n := utf8.RuneCountInString(in.DisplayName); n < 1 || n > 64 {
		return domain.User{}, fmt.Errorf("%w: 显示名称应为 1~64 个字符", domain.ErrInvalid)
	}
	if err := validateUserRole(in.Role); err != nil {
		return domain.User{}, err
	}
	if old.Enabled && old.IsAdmin() && (!in.Enabled || in.Role != domain.RoleAdmin) {
		var others int
		if err := tx.GetContext(ctx, &others, `SELECT COUNT(*) FROM app_user WHERE id<>? AND role='admin' AND enabled=1`, id); err != nil {
			return domain.User{}, err
		}
		if others == 0 {
			return domain.User{}, fmt.Errorf("%w: 至少保留一个启用的管理员", domain.ErrConflict)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE app_user SET display_name=?,role=?,enabled=?,updated_at=? WHERE id=?`, in.DisplayName, in.Role, in.Enabled, domain.Now(), id); err != nil {
		return domain.User{}, wrap(err)
	}
	if old.Enabled != in.Enabled || old.Role != in.Role {
		if _, err := tx.ExecContext(ctx, `DELETE FROM user_session WHERE user_id=?`, id); err != nil {
			return domain.User{}, err
		}
	}
	detail := fmt.Sprintf("修改账号 %s；身份=%s；启用=%t", old.Username, in.Role, in.Enabled)
	if err := r.log(ctx, tx, "update", "app_user", id, detail); err != nil {
		return domain.User{}, err
	}
	user, err := loadUser(ctx, tx, id)
	if err != nil {
		return domain.User{}, err
	}
	return user, tx.Commit()
}

func (r *Repo) ResetUserPassword(ctx context.Context, id int64, password string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := requireAdmin(ctx, tx); err != nil {
		return err
	}
	if n := utf8.RuneCountInString(password); n < 8 || n > 128 {
		return fmt.Errorf("%w: 密码长度应为 8~128 个字符", domain.ErrInvalid)
	}
	if _, err := loadUser(ctx, tx, id); err != nil {
		return err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE app_user SET password_hash=?,updated_at=? WHERE id=?`, hash, domain.Now(), id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_session WHERE user_id=?`, id); err != nil {
		return err
	}
	if err := r.log(ctx, tx, "reset_password", "app_user", id, "管理员重置账号密码；该账号全部会话已撤销"); err != nil {
		return err
	}
	return tx.Commit()
}
