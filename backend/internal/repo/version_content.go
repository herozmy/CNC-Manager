package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"cnccool/internal/domain"
)

// upsertNCFile 按 sha256 找到或登记一个物理文件，返回 nc_file.id。
//
// 内容相同就复用同一条记录（秒传 + 去重）；内容不同就新增一条。
// 这样"覆盖某个版本的内容"实现上就是让它指向一个新的内容块，
// 完全不需要去改动已有文件——文件库永远只增不改，天然安全。
func upsertNCFile(ctx context.Context, tx *sqlx.Tx, file domain.NCFileInput) (int64, error) {
	var fileID int64
	err := tx.GetContext(ctx, &fileID, `SELECT id FROM nc_file WHERE sha256 = ?`, file.SHA256)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		enc := file.Encoding
		if enc == "" {
			enc = "utf-8"
		}
		res, ierr := tx.ExecContext(ctx,
			`INSERT INTO nc_file (sha256, size_bytes, original_name, rel_path, file_encoding, created_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			file.SHA256, file.SizeBytes, file.OriginalName, file.RelPath, enc, domain.Now())
		if ierr != nil {
			return 0, wrap(ierr)
		}
		id, ierr := res.LastInsertId()
		if ierr != nil {
			return 0, wrap(ierr)
		}
		return id, nil
	case err != nil:
		return 0, wrap(err)
	}
	return fileID, nil
}

// GetVersionFileInfo 取版本对应物理文件的相对路径、编码和文件名。
func (r *Repo) GetVersionFileInfo(ctx context.Context, versionID int64) (relPath, encoding, fileName string, err error) {
	var row struct {
		RelPath  string `db:"rel_path"`
		Encoding string `db:"file_encoding"`
		FileName string `db:"file_name"`
	}
	if err := r.db.GetContext(ctx, &row,
		`SELECT f.rel_path, f.file_encoding, v.file_name
		 FROM nc_version v JOIN nc_file f ON f.id = v.file_id
		 WHERE v.id = ?`, versionID); err != nil {
		return "", "", "", wrap(err)
	}
	return row.RelPath, row.Encoding, row.FileName, nil
}

// SetVersionFileEncoding 回填检测出来的文件编码。
//
// 上传时是流式写入，无法可靠判断编码（GBK 的多字节字符可能正好被缓冲区切断），
// 所以第一次读取时用完整内容检测，再把结果缓存回库。
func (r *Repo) SetVersionFileEncoding(ctx context.Context, versionID int64, enc string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE nc_file SET file_encoding = ?
		 WHERE id = (SELECT file_id FROM nc_version WHERE id = ?)`, enc, versionID)
	return wrap(err)
}

// OverwriteVersionContent 用新内容覆盖某个版本的物理文件，版本号保持不变。
//
// 适用场景：只是改个笔误、补一行注释，不值得单独占一个版本号。
//
// 实现上没有真的去改文件——文件库是按内容寻址的，所以这里是
// 「登记一个新内容块 → 把这条版本记录指向它」。
// 旧内容块如果没人引用了就变成孤儿，不做物理删除（它可能还被别的版本引用着），
// 交给以后的后台回收任务。
func (r *Repo) OverwriteVersionContent(
	ctx context.Context, versionID int64, file domain.NCFileInput, changeNote string,
) (*domain.Version, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, wrap(err)
	}
	defer func() { _ = tx.Rollback() }()

	var v domain.Version
	if err := sqlx.GetContext(ctx, tx, &v,
		`SELECT `+versionCols+` FROM nc_version v
		 JOIN nc_file f    ON f.id = v.file_id
		 JOIN nc_program p ON p.id = v.program_id
		 WHERE v.id = ?`, versionID); err != nil {
		return nil, wrap(err)
	}

	fileID, err := upsertNCFile(ctx, tx, file)
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE nc_version SET file_id = ?, change_note = ? WHERE id = ?`,
		fileID, changeNote, versionID); err != nil {
		return nil, wrap(err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE nc_program SET updated_at = ? WHERE id = ?`,
		domain.Now(), v.ProgramID); err != nil {
		return nil, wrap(err)
	}
	if err := r.log(ctx, tx, "edit", "program", v.ProgramID,
		fmt.Sprintf("直接修改第 %d 版程序内容（%s）", v.VersionNo, file.OriginalName)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, wrap(err)
	}
	return r.GetVersion(ctx, versionID)
}
