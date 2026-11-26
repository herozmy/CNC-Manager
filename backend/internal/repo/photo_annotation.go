package repo

import (
	"cnc-manager/internal/domain"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

type PhotoMark struct {
	Kind string `json:"kind"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
	X2   int    `json:"x2"`
	Y2   int    `json:"y2"`
	Text string `json:"text,omitempty"`
}

type PhotoAnnotations struct {
	Revision int64       `json:"revision"`
	Marks    []PhotoMark `json:"marks"`
}

func (a PhotoAnnotations) valid() bool {
	if a.Revision < 0 || a.Marks == nil || len(a.Marks) > 80 {
		return false
	}
	for _, m := range a.Marks {
		for _, n := range []int{m.X, m.Y, m.X2, m.Y2} {
			if n < 0 || n > 1000 {
				return false
			}
		}
		switch m.Kind {
		case "text":
			if strings.TrimSpace(m.Text) == "" || utf8.RuneCountInString(m.Text) > 40 {
				return false
			}
		case "arrow", "ellipse":
			if m.Text != "" || (m.X == m.X2 && m.Y == m.Y2) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func (r *Repo) readAnnotations(ctx context.Context, id int64, digest string) (PhotoAnnotations, error) {
	var a PhotoAnnotations
	var raw string
	err := r.tx.QueryRowContext(ctx, `SELECT coalesce(a.revision,0),coalesce(a.marks,'[]') FROM operation_photo p LEFT JOIN photo_annotation a USING(operation_id,digest) WHERE p.operation_id=? AND p.digest=?`, id, digest).Scan(&a.Revision, &raw)
	if err != nil {
		return a, wrap(err)
	}
	err = json.Unmarshal([]byte(raw), &a.Marks)
	if err == nil && !a.valid() {
		err = fmt.Errorf("照片标注数据无效")
	}
	return a, err
}

func (r *Repo) GetAnnotations(ctx context.Context, id int64, digest string) (PhotoAnnotations, error) {
	return businessTransaction(ctx, r, "read", "operation", id, func(t *Repo) (PhotoAnnotations, error) { return t.readAnnotations(ctx, id, digest) })
}

func (r *Repo) SaveAnnotations(ctx context.Context, id int64, digest string, a PhotoAnnotations) (PhotoAnnotations, error) {
	return businessTransaction(ctx, r, "edit", "operation", id, func(t *Repo) (PhotoAnnotations, error) {
		current, err := t.readAnnotations(ctx, id, digest)
		if err != nil {
			return current, err
		}
		if !a.valid() {
			return current, fmt.Errorf("%w: 标注格式无效，每张照片最多 80 处标注，文字最多 40 字", domain.ErrInvalid)
		}
		if current.Revision != a.Revision {
			return current, fmt.Errorf("%w: 照片标注已更新，请取消编辑后重新打开", domain.ErrConflict)
		}
		raw, err := json.Marshal(a.Marks)
		if err != nil {
			return current, err
		}
		a.Revision++
		_, err = t.tx.ExecContext(ctx, `INSERT INTO photo_annotation(operation_id,digest,marks,revision) VALUES(?,?,?,?) ON CONFLICT(operation_id,digest) DO UPDATE SET marks=excluded.marks,revision=excluded.revision`, id, digest, string(raw), a.Revision)
		if err != nil {
			return current, err
		}
		return a, t.log(ctx, t.tx, "update", "operation", id, "修改装夹照片标注")
	})
}
