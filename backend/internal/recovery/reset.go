// Package recovery implements offline, administrator-controlled password recovery.
package recovery

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"cnc-manager/internal/auth"
	"cnc-manager/internal/datalock"
	_ "modernc.org/sqlite"
)

type Account struct {
	ID                    int64
	Username, DisplayName string
	Enabled               bool
}

type Service interface {
	Running(context.Context) (bool, error)
	Stop(context.Context) error
	Start(context.Context) error
}

func openDB(data, mode string) (*sql.DB, error) {
	path, err := filepath.Abs(filepath.Join(data, "cnc.db"))
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("cnc.db 必须是已有的普通文件")
	}
	u := url.URL{Path: filepath.ToSlash(path)}
	q := url.Values{"mode": {mode}, "_pragma": {"foreign_keys(1)", "busy_timeout(15000)"}}
	db, err := sql.Open("sqlite", "file:"+u.EscapedPath()+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func Accounts(ctx context.Context, data string) ([]Account, error) {
	db, err := openDB(data, "ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, "SELECT id, username, display_name, enabled FROM app_user ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Username, &a.DisplayName, &a.Enabled); err != nil {
			return nil, err
		}
		users = append(users, a)
	}
	return users, rows.Err()
}

type Result struct {
	Backup                   string
	Revoked                  int64
	Changed, ServiceRestored bool
}

// Reset never starts an originally stopped service. All cleanup uses a fresh context,
// so cancellation cannot skip ownership repair or service recovery.
func Reset(ctx context.Context, data string, account Account, password string, service Service, out io.Writer) (result Result, err error) {
	if n := utf8.RuneCountInString(password); n < 8 || n > 128 {
		return result, errors.New("密码应为 8~128 个字符")
	}
	encoded, err := auth.HashPassword(password)
	if err != nil {
		return result, err
	}
	password = ""
	data, err = filepath.EvalSymlinks(data)
	if err != nil {
		return result, err
	}
	data, err = filepath.Abs(data)
	if err != nil {
		return result, err
	}
	owner, err := os.Lstat(filepath.Join(data, "cnc.db"))
	if err != nil {
		return result, err
	}
	if !owner.Mode().IsRegular() {
		return result, errors.New("cnc.db 必须是普通文件")
	}
	backupRoot := filepath.Join(filepath.Dir(data), "backups")
	// A directory named backups would otherwise recursively copy its own backup.
	if backupRoot == data {
		return result, errors.New("数据目录不能与维护备份目录相同")
	}
	if err = os.MkdirAll(backupRoot, 0700); err != nil {
		return result, err
	}
	backupRoot, err = filepath.EvalSymlinks(backupRoot)
	if err != nil {
		return result, err
	}
	if rel, e := filepath.Rel(data, backupRoot); e != nil || rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return result, errors.New("维护备份目录不能位于数据目录内")
	}
	maintenance, err := datalock.Acquire(filepath.Join(backupRoot, ".password-reset.lock"))
	if err != nil {
		return result, err
	}
	defer maintenance.Close()
	running := false
	if service != nil {
		running, err = service.Running(ctx)
		if err != nil {
			return result, err
		}
	}
	var lock *os.File
	defer func() {
		// The service needs its data lock back before it can restart.
		if lock != nil {
			err = errors.Join(err, restoreOwner(data, owner), lock.Close())
		}
		if running {
			cleanup, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			if e := service.Start(cleanup); e != nil {
				state := "密码未修改"
				if result.Changed {
					state = "密码已重置"
				}
				err = errors.Join(err, fmt.Errorf("%s，但服务恢复失败，请检查 systemctl status；备份：%s: %w", state, result.Backup, e))
			} else {
				result.ServiceRestored = true
			}
		}
	}()
	if err = ctx.Err(); err != nil {
		return result, err
	}
	if running {
		fmt.Fprintln(out, "正在暂停服务并备份数据…")
		if err = service.Stop(ctx); err != nil {
			return result, err
		}
	} else {
		fmt.Fprintln(out, "正在备份离线数据…")
	}
	lock, err = datalock.Acquire(filepath.Join(data, ".cnc-manager.lock"))
	if err != nil {
		return result, err
	}
	result.Backup, err = os.MkdirTemp(backupRoot, "pre-password-reset-"+time.Now().Format("20060102-150405")+"-")
	if err != nil {
		return result, err
	}
	if err = copyData(ctx, data, filepath.Join(result.Backup, "data")); err != nil {
		return result, fmt.Errorf("备份未完成，密码未修改（不完整副本：%s）: %w", result.Backup, err)
	}
	fmt.Fprintf(out, "备份已保存：%s\n", result.Backup)
	result.Revoked, err = updatePassword(ctx, data, account, encoded)
	result.Changed = err == nil
	return result, err
}

func updatePassword(ctx context.Context, data string, a Account, encoded string) (int64, error) {
	db, err := openDB(data, "rw")
	if err != nil {
		return 0, err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	r, err := tx.ExecContext(ctx, "UPDATE app_user SET password_hash = ?, updated_at = ? WHERE id = ? AND username = ? AND enabled = 1", encoded, now, a.ID, a.Username)
	if err != nil {
		return 0, err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n != 1 {
		return 0, errors.New("账号状态已变化，没有重置密码")
	}
	r, err = tx.ExecContext(ctx, "DELETE FROM user_session WHERE user_id = ?", a.ID)
	if err != nil {
		return 0, err
	}
	n, err = r.RowsAffected()
	if err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO audit_log (action, entity_type, entity_id, detail, actor, created_at) VALUES ('reset_password', 'app_user', ?, ?, 'server-admin', ?)", a.ID, fmt.Sprintf("管理员重置密码，撤销 %d 个会话", n), now); err != nil {
		return 0, err
	}
	return n, tx.Commit()
}

func copyData(ctx context.Context, source, target string) error {
	return filepath.WalkDir(source, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == ".cnc-manager.lock" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		dest := filepath.Join(target, rel)
		if d.IsDir() {
			return os.Mkdir(dest, info.Mode().Perm()|0700)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("备份拒绝符号链接或特殊文件：%s", rel)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, &contextReader{ctx, in})
		return errors.Join(copyErr, out.Sync(), out.Close())
	})
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
