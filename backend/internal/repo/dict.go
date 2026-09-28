package repo

import (
	"context"
	"strings"

	"cnccool/internal/domain"
)

// ---------------------------------------------------------------------------
// 刀具字典
//
// 作用是让操作工从已有刀具里选，而不是每次手打刀具名称——
// 否则同一把 Ø12 立铣刀会被写成 "12立铣"、"Ø12铣刀"、"D12" 三种样子，统计就废了。
// ---------------------------------------------------------------------------

const toolCols = `id, tool_no, name, spec, tool_type, remark, created_at, updated_at`

// ListTools 查询刀具字典。
func (r *Repo) ListTools(ctx context.Context, keyword string) ([]domain.Tool, error) {
	items := []domain.Tool{}
	pat := likePattern(keyword)
	if err := r.db.SelectContext(ctx, &items,
		`SELECT `+toolCols+` FROM tool
		 WHERE (tool_no LIKE ? ESCAPE '\' OR name LIKE ? ESCAPE '\' OR spec LIKE ? ESCAPE '\')
		 ORDER BY tool_no`, pat, pat, pat); err != nil {
		return nil, wrap(err)
	}
	return items, nil
}

// CreateTool 新增刀具。
func (r *Repo) CreateTool(ctx context.Context, in domain.ToolInput) (*domain.Tool, error) {
	now := domain.Now()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO tool (tool_no, name, spec, tool_type, remark, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(in.ToolNo), in.Name, in.Spec, in.ToolType, in.Remark, now, now)
	if err != nil {
		return nil, wrap(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, wrap(err)
	}
	return r.getTool(ctx, id)
}

// UpdateTool 修改刀具。
func (r *Repo) UpdateTool(ctx context.Context, id int64, in domain.ToolInput) (*domain.Tool, error) {
	if _, err := r.getTool(ctx, id); err != nil {
		return nil, err
	}
	if _, err := r.db.ExecContext(ctx,
		`UPDATE tool SET tool_no = ?, name = ?, spec = ?, tool_type = ?, remark = ?, updated_at = ?
		 WHERE id = ?`,
		strings.TrimSpace(in.ToolNo), in.Name, in.Spec, in.ToolType, in.Remark, domain.Now(), id); err != nil {
		return nil, wrap(err)
	}
	return r.getTool(ctx, id)
}

// DeleteTool 删除刀具。
// program_tool.tool_id 是 ON DELETE SET NULL，所以已引用的刀具行不会被连带删掉，
// 只是失去字典链接——这符合现场预期：刀具淘汰了，历史程序记录必须留着。
func (r *Repo) DeleteTool(ctx context.Context, id int64) error {
	if _, err := r.getTool(ctx, id); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM tool WHERE id = ?`, id)
	return wrap(err)
}

func (r *Repo) getTool(ctx context.Context, id int64) (*domain.Tool, error) {
	var t domain.Tool
	if err := r.db.GetContext(ctx, &t, `SELECT `+toolCols+` FROM tool WHERE id = ?`, id); err != nil {
		return nil, wrap(err)
	}
	return &t, nil
}

// ---------------------------------------------------------------------------
// 机台字典
// ---------------------------------------------------------------------------

const machineCols = `id, code, name, controller, remark, created_at, updated_at`

// ListMachines 查询机台字典。
func (r *Repo) ListMachines(ctx context.Context) ([]domain.Machine, error) {
	items := []domain.Machine{}
	if err := r.db.SelectContext(ctx, &items,
		`SELECT `+machineCols+` FROM machine ORDER BY code`); err != nil {
		return nil, wrap(err)
	}
	return items, nil
}

// CreateMachine 新增机台。
func (r *Repo) CreateMachine(ctx context.Context, in domain.MachineInput) (*domain.Machine, error) {
	now := domain.Now()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO machine (code, name, controller, remark, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(in.Code), in.Name, in.Controller, in.Remark, now, now)
	if err != nil {
		return nil, wrap(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, wrap(err)
	}
	return r.getMachine(ctx, id)
}

// UpdateMachine 修改机台。
func (r *Repo) UpdateMachine(ctx context.Context, id int64, in domain.MachineInput) (*domain.Machine, error) {
	if _, err := r.getMachine(ctx, id); err != nil {
		return nil, err
	}
	if _, err := r.db.ExecContext(ctx,
		`UPDATE machine SET code = ?, name = ?, controller = ?, remark = ?, updated_at = ? WHERE id = ?`,
		strings.TrimSpace(in.Code), in.Name, in.Controller, in.Remark, domain.Now(), id); err != nil {
		return nil, wrap(err)
	}
	return r.getMachine(ctx, id)
}

// DeleteMachine 删除机台，已引用它的工序会被置为未指定机台。
func (r *Repo) DeleteMachine(ctx context.Context, id int64) error {
	if _, err := r.getMachine(ctx, id); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM machine WHERE id = ?`, id)
	return wrap(err)
}

func (r *Repo) getMachine(ctx context.Context, id int64) (*domain.Machine, error) {
	var m domain.Machine
	if err := r.db.GetContext(ctx, &m, `SELECT `+machineCols+` FROM machine WHERE id = ?`, id); err != nil {
		return nil, wrap(err)
	}
	return &m, nil
}
