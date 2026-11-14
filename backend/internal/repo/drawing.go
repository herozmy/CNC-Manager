package repo

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"cnc-manager/internal/domain"
)

const drawingCols = `d.id, d.drawing_no, d.name, d.customer, d.material, d.drawing_version, d.remark,
	d.created_at, d.updated_at,
	(SELECT COUNT(*) FROM operation o WHERE o.drawing_id = d.id) AS operation_count,
	(SELECT COUNT(*) FROM nc_program p JOIN operation o ON o.id = p.operation_id WHERE o.drawing_id = d.id) AS program_count`

// 关键字同时匹配图纸字段与其下程序号/程序名——现场常见"只记得程序号 O1234"。
const drawingFilter = `WHERE (
	d.drawing_no LIKE ? ESCAPE '\' OR d.name LIKE ? ESCAPE '\' OR
	d.material LIKE ? ESCAPE '\' OR
	EXISTS (SELECT 1 FROM operation o JOIN nc_program p ON p.operation_id = o.id
	        WHERE o.drawing_id = d.id AND (p.program_no LIKE ? ESCAPE '\' OR p.program_name LIKE ? ESCAPE '\')))`

// ListDrawings 分页查询图纸。
func (r *Repo) ListDrawings(ctx context.Context, keyword string, page, size int) (domain.Page[domain.Drawing], error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 50
	}
	out := domain.Page[domain.Drawing]{Items: []domain.Drawing{}, Page: page, Size: size}
	pat := likePattern(keyword)
	args := []any{pat, pat, pat, pat, pat}
	if err := r.db.GetContext(ctx, &out.Total, `SELECT COUNT(*) FROM drawing d `+drawingFilter, args...); err != nil {
		return out, wrap(err)
	}
	q := `SELECT ` + drawingCols + ` FROM drawing d ` + drawingFilter + ` ORDER BY d.updated_at DESC, d.id DESC LIMIT ? OFFSET ?`
	if err := r.db.SelectContext(ctx, &out.Items, q, append(args, size, (page-1)*size)...); err != nil {
		return out, wrap(err)
	}
	return out, nil
}

// GetDrawing 按 ID 取图纸。
func (r *Repo) GetDrawing(ctx context.Context, id int64) (*domain.Drawing, error) {
	var d domain.Drawing
	if err := r.db.GetContext(ctx, &d, `SELECT `+drawingCols+` FROM drawing d WHERE d.id = ?`, id); err != nil {
		return nil, wrap(err)
	}
	return &d, nil
}

// DrawingDetail 返回图纸及其全部工序和程序（3 条 SQL，内存组装）。
func (r *Repo) DrawingDetail(ctx context.Context, id int64) (*domain.DrawingDetail, error) {
	d, err := r.GetDrawing(ctx, id)
	if err != nil {
		return nil, err
	}
	ops, err := r.ListOperations(ctx, id)
	if err != nil {
		return nil, err
	}
	out := &domain.DrawingDetail{Drawing: *d, Operations: make([]domain.OperationWithPrograms, 0, len(ops))}
	if len(ops) == 0 {
		return out, nil
	}
	opIDs := make([]int64, 0, len(ops))
	for _, o := range ops {
		opIDs = append(opIDs, o.ID)
	}
	q, args, err := sqlx.In(`SELECT `+programCols+` FROM nc_program p WHERE p.operation_id IN (?) ORDER BY p.operation_id, p.sort_order, p.id`, opIDs)
	if err != nil {
		return nil, err
	}
	progs := []domain.Program{}
	if err := r.db.SelectContext(ctx, &progs, r.db.Rebind(q), args...); err != nil {
		return nil, wrap(err)
	}
	byOp := map[int64][]domain.Program{}
	for _, p := range progs {
		byOp[p.OperationID] = append(byOp[p.OperationID], p)
	}
	for _, o := range ops {
		list := byOp[o.ID]
		if list == nil {
			list = []domain.Program{}
		}
		out.Operations = append(out.Operations, domain.OperationWithPrograms{Operation: o, Programs: list})
	}
	return out, nil
}

// CreateDrawing 新建图纸。
func (r *Repo) CreateDrawing(ctx context.Context, in domain.DrawingInput) (*domain.Drawing, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	now := domain.Now()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO drawing (drawing_no, name, customer, material, drawing_version, remark, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		in.DrawingNo, in.Name, in.Customer, in.Material, in.DrawingVersion, in.Remark, now, now)
	if err != nil {
		return nil, wrap(err)
	}
	id, _ := res.LastInsertId()
	if err := r.log(ctx, r.db, "create", "drawing", id, "新建图纸 "+in.DrawingNo); err != nil {
		return nil, err
	}
	return r.GetDrawing(ctx, id)
}

// UpdateDrawing 修改图纸。
func (r *Repo) UpdateDrawing(ctx context.Context, id int64, in domain.DrawingInput) (*domain.Drawing, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	if _, err := r.GetDrawing(ctx, id); err != nil {
		return nil, err
	}
	if _, err := r.db.ExecContext(ctx,
		`UPDATE drawing SET drawing_no = ?, name = ?, customer = ?, material = ?, drawing_version = ?, remark = ?, updated_at = ? WHERE id = ?`,
		in.DrawingNo, in.Name, in.Customer, in.Material, in.DrawingVersion, in.Remark, domain.Now(), id); err != nil {
		return nil, wrap(err)
	}
	if err := r.log(ctx, r.db, "update", "drawing", id, "修改图纸 "+in.DrawingNo); err != nil {
		return nil, err
	}
	return r.GetDrawing(ctx, id)
}

// DeleteDrawing 删除图纸，级联删除工序、程序、版本与刀具表；物理文件留给回收任务。
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

// ---- 工序 ----

const operationCols = `o.id, o.drawing_id, o.op_no, o.op_name, o.sort_order, o.machine_id,
	COALESCE(m.name, '') AS machine_name, o.fixture, o.z_height_milli, o.remark, o.created_at, o.updated_at,
	(SELECT COUNT(*) FROM nc_program p WHERE p.operation_id = o.id) AS program_count`

const operationFrom = ` FROM operation o LEFT JOIN machine m ON m.id = o.machine_id `

// ListOperations 取图纸下全部工序，按人工排序。
func (r *Repo) ListOperations(ctx context.Context, drawingID int64) ([]domain.Operation, error) {
	items := []domain.Operation{}
	if err := r.db.SelectContext(ctx, &items,
		`SELECT `+operationCols+operationFrom+`WHERE o.drawing_id = ? ORDER BY o.sort_order, o.id`, drawingID); err != nil {
		return nil, wrap(err)
	}
	for i := range items {
		items[i].ZHeight = domain.MilliToFloat(items[i].ZHeightMilli)
	}
	return items, nil
}

// GetOperation 按 ID 取工序。
func (r *Repo) GetOperation(ctx context.Context, id int64) (*domain.Operation, error) {
	var o domain.Operation
	if err := r.db.GetContext(ctx, &o, `SELECT `+operationCols+operationFrom+`WHERE o.id = ?`, id); err != nil {
		return nil, wrap(err)
	}
	o.ZHeight = domain.MilliToFloat(o.ZHeightMilli)
	return &o, nil
}

// CreateOperation 在图纸下新增工序，排在最后。
func (r *Repo) CreateOperation(ctx context.Context, drawingID int64, in domain.OperationInput) (*domain.Operation, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	if _, err := r.GetDrawing(ctx, drawingID); err != nil {
		return nil, err
	}
	now := domain.Now()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO operation (drawing_id, op_no, op_name, machine_id, sort_order, fixture, z_height_milli, remark, created_at, updated_at)
		 VALUES (?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM operation WHERE drawing_id = ?), ?, ?, ?, ?, ?)`,
		drawingID, in.OpNo, in.OpName, in.MachineID, drawingID, in.Fixture, domain.FloatToMilli(in.ZHeight), in.Remark, now, now)
	if err != nil {
		return nil, wrap(err)
	}
	id, _ := res.LastInsertId()
	if err := r.log(ctx, r.db, "create", "operation", id, fmt.Sprintf("新增工序 %d %s", in.OpNo, in.OpName)); err != nil {
		return nil, err
	}
	return r.GetOperation(ctx, id)
}

// UpdateOperation 修改工序。
func (r *Repo) UpdateOperation(ctx context.Context, id int64, in domain.OperationInput) (*domain.Operation, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	if _, err := r.GetOperation(ctx, id); err != nil {
		return nil, err
	}
	if _, err := r.db.ExecContext(ctx,
		`UPDATE operation SET op_no = ?, op_name = ?, machine_id = ?, fixture = ?, z_height_milli = ?, remark = ?, updated_at = ? WHERE id = ?`,
		in.OpNo, in.OpName, in.MachineID, in.Fixture, domain.FloatToMilli(in.ZHeight), in.Remark, domain.Now(), id); err != nil {
		return nil, wrap(err)
	}
	if err := r.log(ctx, r.db, "update", "operation", id, fmt.Sprintf("修改工序 %d %s", in.OpNo, in.OpName)); err != nil {
		return nil, err
	}
	return r.GetOperation(ctx, id)
}

// DeleteOperation 删除工序及其程序。
func (r *Repo) DeleteOperation(ctx context.Context, id int64) error {
	o, err := r.GetOperation(ctx, id)
	if err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM operation WHERE id = ?`, id); err != nil {
		return wrap(err)
	}
	return r.log(ctx, r.db, "delete", "operation", id, fmt.Sprintf("删除工序 %d %s", o.OpNo, o.OpName))
}

// ReorderOperations 按完整 ID 顺序重排图纸下的工序。
func (r *Repo) ReorderOperations(ctx context.Context, drawingID int64, ids []int64) error {
	existing, err := r.ListOperations(ctx, drawingID)
	if err != nil {
		return err
	}
	if err := checkReorder(existing, func(o domain.Operation) int64 { return o.ID }, ids, "工序"); err != nil {
		return err
	}
	return r.reorder(ctx, `UPDATE operation SET sort_order = ?, updated_at = ? WHERE id = ? AND drawing_id = ?`, drawingID, ids, "drawing", "调整工序显示顺序")
}

func checkReorder[T any](existing []T, idOf func(T) int64, ids []int64, name string) error {
	if len(existing) != len(ids) {
		return fmt.Errorf("%w: 排序列表必须包含全部%s", domain.ErrInvalid, name)
	}
	allowed := make(map[int64]bool, len(existing))
	for _, e := range existing {
		allowed[idOf(e)] = true
	}
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if !allowed[id] || seen[id] {
			return fmt.Errorf("%w: 排序列表包含无效或重复的%s", domain.ErrInvalid, name)
		}
		seen[id] = true
	}
	return nil
}

func (r *Repo) reorder(ctx context.Context, sql string, parentID int64, ids []int64, entity, detail string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return wrap(err)
	}
	defer func() { _ = tx.Rollback() }()
	now := domain.Now()
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx, sql, i+1, now, id, parentID); err != nil {
			return wrap(err)
		}
	}
	if err := r.log(ctx, tx, "reorder", entity, parentID, detail); err != nil {
		return err
	}
	return tx.Commit()
}
