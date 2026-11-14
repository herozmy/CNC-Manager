package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"cnc-manager/internal/domain"
)

const versionCols = `v.id, v.program_id, v.version_no, v.sha256, v.file_name, b.size_bytes AS file_size, b.encoding,
	v.change_note, v.created_by, v.created_at,
	CASE WHEN p.current_version_id = v.id THEN 1 ELSE 0 END AS is_current`

const versionFrom = ` FROM nc_version v JOIN nc_blob b ON b.sha256 = v.sha256 JOIN nc_program p ON p.id = v.program_id `

// ListVersions 取程序全部版本，新版本在前。
func (r *Repo) ListVersions(ctx context.Context, programID int64) ([]domain.Version, error) {
	if _, err := r.GetProgram(ctx, programID); err != nil {
		return nil, err
	}
	items := []domain.Version{}
	if err := r.db.SelectContext(ctx, &items,
		`SELECT `+versionCols+versionFrom+`WHERE v.program_id = ? ORDER BY v.version_no DESC`, programID); err != nil {
		return nil, wrap(err)
	}
	return items, nil
}

// GetVersion 按 ID 取版本。
func (r *Repo) GetVersion(ctx context.Context, id int64) (*domain.Version, error) {
	return getVersion(ctx, r.db, id)
}

func getVersion(ctx context.Context, q sqlx.QueryerContext, id int64) (*domain.Version, error) {
	var v domain.Version
	if err := sqlx.GetContext(ctx, q, &v, `SELECT `+versionCols+versionFrom+`WHERE v.id = ?`, id); err != nil {
		return nil, wrap(err)
	}
	return &v, nil
}

// GetVersionFile 取读取版本内容所需的信息。
func (r *Repo) GetVersionFile(ctx context.Context, id int64) (*domain.VersionFile, error) {
	var f domain.VersionFile
	err := r.db.GetContext(ctx, &f,
		`SELECT v.id, v.program_id, v.version_no, v.sha256, v.file_name, b.rel_path, b.encoding,
		        CASE WHEN p.current_version_id = v.id THEN 1 ELSE 0 END AS is_current`+versionFrom+`WHERE v.id = ?`, id)
	if err != nil {
		return nil, wrap(err)
	}
	return &f, nil
}

func upsertBlob(ctx context.Context, tx *sqlx.Tx, b domain.BlobInput) error {
	// 内容相同则复用已有记录（秒传）。ON CONFLICT 是 SQLite 与 PostgreSQL 通用语法。
	_, err := tx.ExecContext(ctx,
		`INSERT INTO nc_blob (sha256, size_bytes, encoding, rel_path, created_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (sha256) DO NOTHING`, b.SHA256, b.SizeBytes, b.Encoding, b.RelPath, domain.Now())
	return wrap(err)
}

// AddVersion 新增版本并设为当前版本。整个过程在一个事务里，要么全成功要么全回滚。
func (r *Repo) AddVersion(ctx context.Context, programID int64, blob domain.BlobInput, fileName, changeNote string) (*domain.Version, error) {
	if len(changeNote) > maxNote {
		return nil, fmt.Errorf("%w: 变更说明不能超过 %d 字节", domain.ErrInvalid, maxNote)
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, wrap(err)
	}
	defer func() { _ = tx.Rollback() }()

	var programNo string
	if err := tx.GetContext(ctx, &programNo, `SELECT program_no FROM nc_program WHERE id = ?`, programID); err != nil {
		return nil, wrap(err)
	}
	if err := upsertBlob(ctx, tx, blob); err != nil {
		return nil, err
	}
	actor := ""
	if u, ok := domain.UserFromContext(ctx); ok {
		actor = u.Username
	}
	now := domain.Now()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO nc_version (program_id, version_no, sha256, file_name, change_note, created_by, created_at)
		 VALUES (?, (SELECT COALESCE(MAX(version_no), 0) + 1 FROM nc_version WHERE program_id = ?), ?, ?, ?, ?, ?)`,
		programID, programID, blob.SHA256, fileName, strings.TrimSpace(changeNote), actor, now)
	if err != nil {
		return nil, wrap(err)
	}
	vid, _ := res.LastInsertId()
	if _, err := tx.ExecContext(ctx, `UPDATE nc_program SET current_version_id = ?, updated_at = ? WHERE id = ?`, vid, now, programID); err != nil {
		return nil, wrap(err)
	}
	v, err := getVersion(ctx, tx, vid)
	if err != nil {
		return nil, err
	}
	if err := r.log(ctx, tx, "upload", "program", programID, fmt.Sprintf("%s 上传第 %d 版（%s）", programNo, v.VersionNo, fileName)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, wrap(err)
	}
	return v, nil
}

// OverwriteVersion 用新内容覆盖某个版本（不新增版本号）。
// 只改这个版本指向的内容；原内容若不再被引用，由孤儿回收任务清理。
func (r *Repo) OverwriteVersion(ctx context.Context, versionID int64, blob domain.BlobInput, expectedSHA256 *string) (*domain.Version, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, wrap(err)
	}
	defer func() { _ = tx.Rollback() }()
	old, err := getVersion(ctx, tx, versionID)
	if err != nil {
		return nil, err
	}
	if expectedSHA256 != nil && old.SHA256 != *expectedSHA256 {
		return nil, fmt.Errorf("%w: 此版本已被其他操作修改，请载入最新内容或另存新版本", domain.ErrConflict)
	}
	if err := upsertBlob(ctx, tx, blob); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE nc_version SET sha256 = ? WHERE id = ?`, blob.SHA256, versionID); err != nil {
		return nil, wrap(err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE nc_program SET updated_at = ? WHERE id = ?`, domain.Now(), old.ProgramID); err != nil {
		return nil, wrap(err)
	}
	if err := r.log(ctx, tx, "overwrite", "program", old.ProgramID, fmt.Sprintf("覆盖第 %d 版内容", old.VersionNo)); err != nil {
		return nil, err
	}
	v, err := getVersion(ctx, tx, versionID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, wrap(err)
	}
	return v, nil
}

// SetCurrentVersion 指定当前生效版本（回滚就用它）。
func (r *Repo) SetCurrentVersion(ctx context.Context, programID, versionID int64) (*domain.Program, error) {
	if _, err := r.GetProgram(ctx, programID); err != nil {
		return nil, err
	}
	v, err := r.GetVersion(ctx, versionID)
	if err != nil {
		return nil, err
	}
	if v.ProgramID != programID {
		return nil, fmt.Errorf("%w: 该版本不属于这个程序", domain.ErrInvalid)
	}
	if _, err := r.db.ExecContext(ctx, `UPDATE nc_program SET current_version_id = ?, updated_at = ? WHERE id = ?`, versionID, domain.Now(), programID); err != nil {
		return nil, wrap(err)
	}
	if err := r.log(ctx, r.db, "switch", "program", programID, fmt.Sprintf("切换当前版本为第 %d 版", v.VersionNo)); err != nil {
		return nil, err
	}
	return r.GetProgram(ctx, programID)
}

const maxNote = 500
