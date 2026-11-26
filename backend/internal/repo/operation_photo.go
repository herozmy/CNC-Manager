package repo

import (
	"cnc-manager/internal/domain"
	"context"
	"crypto/sha256"
	"fmt"
)

func (r *Repo) OperationPhotos(ctx context.Context, id int64) ([]string, error) {
	return businessTransaction(ctx, r, "read", "operation", id, func(t *Repo) ([]string, error) {
		photos := []string{}
		err := t.tx.SelectContext(ctx, &photos, "SELECT digest FROM operation_photo WHERE operation_id=? ORDER BY rowid", id)
		return photos, err
	})
}

func (r *Repo) OperationPhoto(ctx context.Context, id int64, digest string) ([]byte, error) {
	return businessTransaction(ctx, r, "read", "operation", id, func(t *Repo) ([]byte, error) {
		var data []byte
		err := t.tx.GetContext(ctx, &data, "SELECT jpeg FROM operation_photo WHERE operation_id=? AND digest=?", id, digest)
		return data, wrap(err)
	})
}

func (r *Repo) AddOperationPhoto(ctx context.Context, id int64, data []byte) (string, error) {
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	return businessTransaction(ctx, r, "edit", "operation", id, func(t *Repo) (string, error) {
		res, err := t.tx.ExecContext(ctx, "INSERT INTO operation_photo(operation_id,digest,jpeg) VALUES(?,?,?) ON CONFLICT(operation_id,digest) DO NOTHING", id, digest, data)
		if err != nil {
			return "", wrap(err)
		}
		if n, err := res.RowsAffected(); err != nil {
			return "", err
		} else if n == 0 {
			return digest, nil
		}
		return digest, t.log(ctx, t.tx, "update", "operation", id, "添加装夹照片")
	})
}

func (r *Repo) DeleteOperationPhoto(ctx context.Context, id int64, digest string) error {
	_, err := businessTransaction(ctx, r, "edit", "operation", id, func(t *Repo) (struct{}, error) {
		res, err := t.tx.ExecContext(ctx, "DELETE FROM operation_photo WHERE operation_id=? AND digest=?", id, digest)
		if err != nil {
			return struct{}{}, err
		}
		if n, err := res.RowsAffected(); err != nil {
			return struct{}{}, err
		} else if n == 0 {
			return struct{}{}, domain.ErrNotFound
		}
		return struct{}{}, t.log(ctx, t.tx, "update", "operation", id, "删除装夹照片")
	})
	return err
}
