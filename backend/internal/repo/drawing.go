package repo

import (
	"context"
	"strings"

	"cnccool/internal/domain"
)

// drawingCols 是查询图纸时固定使用的列，带上工序数量统计。
// 所有返回 Drawing 的查询都必须用它，否则 sqlx 会因为缺列而扫描失败。
const drawingCols = `d.id, d.drawing_no, d.name, d.customer, d.material,
	d.drawing_version, d.remark, d.created_at, d.updated_at,
	(SELECT COUNT(*) FROM operation o WHERE o.drawing_id = d.id) AS operation_count`

// drawingFilter 按关键字过滤图纸。
//
// 除了图纸自身字段，还会通过程序号/程序名反查——
// 现场最常见的用法就是"我只记得程序号 O1234，帮我找出是哪个图纸"。
const drawingFilter = `WHERE (
	d.drawing_no LIKE ? ESCAPE '\' OR d.name LIKE ? ESCAPE '\' OR
	d.customer    LIKE ? ESCAPE '\' OR d.material LIKE ? ESCAPE '\' OR
	EXISTS (SELECT 1 FROM operation o JOIN nc_program p ON p.operation_id = o.id
	        WHERE o.drawing_id = d.id
	          AND (p.program_no LIKE ? ESCAPE '\' OR p.program_name LIKE ? ESCAPE '\'))
)`

// ListDrawings 分页查询图纸列表。
func (r *Repo) ListDrawings(ctx context.Context, keyword string, page, size int) ([]domain.Drawing, int, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	pat := likePattern(keyword)
	args := []any{pat, pat, pat, pat, pat, pat}

	var total int
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM drawing d `+drawingFilter, args...); err != nil {
		return nil, 0, wrap(err)
	}

	items := []domain.Drawing{}
	q := `SELECT ` + drawingCols + ` FROM drawing d ` + drawingFilter +
		` ORDER BY d.drawing_no LIMIT ? OFFSET ?`
	if err := r.db.SelectContext(ctx, &items, q, append(args, size, (page-1)*size)...); err != nil {
		return nil, 0, wrap(err)
	}
	return items, total, nil
}

// GetDrawing 按 ID 取图纸。
func (r *Repo) GetDrawing(ctx context.Context, id int64) (*domain.Drawing, error) {
	var d domain.Drawing
	if err := r.db.GetContext(ctx, &d,
		`SELECT `+drawingCols+` FROM drawing d WHERE d.id = ?`, id); err != nil {
		return nil, wrap(err)
	}
	return &d, nil
}

// CreateDrawing 新建图纸。
func (r *Repo) CreateDrawing(ctx context.Context, in domain.DrawingInput) (*domain.Drawing, error) {
	now := domain.Now()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO drawing (drawing_no, name, customer, material, drawing_version, remark, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(in.DrawingNo), in.Name, in.Customer, in.Material,
		in.DrawingVersion, in.Remark, now, now)
	if err != nil {
		return nil, wrap(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, wrap(err)
	}
	if err := r.log(ctx, r.db, "create", "drawing", id, "新建图纸 "+in.DrawingNo); err != nil {
		return nil, err
	}
	return r.GetDrawing(ctx, id)
}

// UpdateDrawing 修改图纸。
func (r *Repo) UpdateDrawing(ctx context.Context, id int64, in domain.DrawingInput) (*domain.Drawing, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE drawing SET drawing_no = ?, name = ?, customer = ?, material = ?,
		 drawing_version = ?, remark = ?, updated_at = ? WHERE id = ?`,
		strings.TrimSpace(in.DrawingNo), in.Name, in.Customer, in.Material,
		in.DrawingVersion, in.Remark, domain.Now(), id)
	if err != nil {
		return nil, wrap(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// 注意：内容完全没变时 RowsAffected 也是 0，所以要先确认记录是否存在。
		if _, err := r.GetDrawing(ctx, id); err != nil {
			return nil, err
		}
	}
	if err := r.log(ctx, r.db, "update", "drawing", id, "修改图纸 "+in.DrawingNo); err != nil {
		return nil, err
	}
	return r.GetDrawing(ctx, id)
}

// DeleteDrawing 删除图纸。外键是 ON DELETE CASCADE，会连带删除工序、程序、版本引用。
func (r *Repo) DeleteDrawing(ctx context.Context, id int64) error {
	d, err := r.GetDrawing(ctx, id)
	if err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM drawing WHERE id = ?`, id); err != nil {
		return wrap(err)
	}
	return r.log(ctx, r.db, "delete", "drawing", id, "删除图纸 "+d.DrawingNo)
}

// ---------------------------------------------------------------------------
// 工序
// ---------------------------------------------------------------------------

const operationCols = `o.id, o.drawing_id, o.op_no, o.op_name, o.machine_id,
	COALESCE(m.name, '') AS machine_name, o.fixture, o.z_height_milli, o.remark,
	o.created_at, o.updated_at,
	(SELECT COUNT(*) FROM nc_program p WHERE p.operation_id = o.id) AS program_count`

// ListOperations 取某图纸下的全部工序，按工序号升序。
func (r *Repo) ListOperations(ctx context.Context, drawingID int64) ([]domain.Operation, error) {
	items := []domain.Operation{}
	q := `SELECT ` + operationCols + ` FROM operation o
	      LEFT JOIN machine m ON m.id = o.machine_id
	      WHERE o.drawing_id = ? ORDER BY o.op_no`
	if err := r.db.SelectContext(ctx, &items, q, drawingID); err != nil {
		return nil, wrap(err)
	}
	for i := range items {
		items[i].ApplyMilli()
	}
	return items, nil
}

// GetOperation 按 ID 取工序。
func (r *Repo) GetOperation(ctx context.Context, id int64) (*domain.Operation, error) {
	var o domain.Operation
	q := `SELECT ` + operationCols + ` FROM operation o
	      LEFT JOIN machine m ON m.id = o.machine_id WHERE o.id = ?`
	if err := r.db.GetContext(ctx, &o, q, id); err != nil {
		return nil, wrap(err)
	}
	o.ApplyMilli()
	return &o, nil
}

// CreateOperation 在指定图纸下新增工序。
func (r *Repo) CreateOperation(ctx context.Context, drawingID int64, in domain.OperationInput) (*domain.Operation, error) {
	op := domain.Operation{
		OpNo: in.OpNo, OpName: in.OpName, MachineID: in.MachineID,
		Fixture: in.Fixture, ZHeight: in.ZHeight, Remark: in.Remark,
	}
	op.FillMilli()

	now := domain.Now()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO operation (drawing_id, op_no, op_name, machine_id, fixture, z_height_milli, remark, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		drawingID, op.OpNo, op.OpName, op.MachineID, op.Fixture, op.ZHeightMilli, op.Remark, now, now)
	if err != nil {
		return nil, wrap(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, wrap(err)
	}
	if err := r.log(ctx, r.db, "create", "operation", id, "新增工序 "+in.OpName); err != nil {
		return nil, err
	}
	return r.GetOperation(ctx, id)
}

// UpdateOperation 修改工序。装夹方式、Z 轴垫高、备注都在这里更新。
func (r *Repo) UpdateOperation(ctx context.Context, id int64, in domain.OperationInput) (*domain.Operation, error) {
	if _, err := r.GetOperation(ctx, id); err != nil {
		return nil, err
	}
	op := domain.Operation{Fixture: in.Fixture, ZHeight: in.ZHeight}
	op.FillMilli()

	if _, err := r.db.ExecContext(ctx,
		`UPDATE operation SET op_no = ?, op_name = ?, machine_id = ?, fixture = ?,
		 z_height_milli = ?, remark = ?, updated_at = ? WHERE id = ?`,
		in.OpNo, in.OpName, in.MachineID, in.Fixture, op.ZHeightMilli,
		in.Remark, domain.Now(), id); err != nil {
		return nil, wrap(err)
	}
	if err := r.log(ctx, r.db, "update", "operation", id, "修改工序 "+in.OpName); err != nil {
		return nil, err
	}
	return r.GetOperation(ctx, id)
}

// DeleteOperation 删除工序，连带其下程序与版本引用。
func (r *Repo) DeleteOperation(ctx context.Context, id int64) error {
	o, err := r.GetOperation(ctx, id)
	if err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM operation WHERE id = ?`, id); err != nil {
		return wrap(err)
	}
	return r.log(ctx, r.db, "delete", "operation", id, "删除工序 "+o.OpName)
}
