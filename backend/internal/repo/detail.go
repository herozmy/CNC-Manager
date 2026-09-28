package repo

import (
	"context"

	"github.com/jmoiron/sqlx"

	"cnccool/internal/domain"
)

// DrawingDetail 一次返回一张图纸的完整内容：图纸 → 工序 → 程序 → 刀具补偿表。
//
// 为什么专门做一个接口：
// 界面是"选中图纸就整页渲染"的线性表单，如果让前端分四级去请求
// （图纸、工序、程序、每个程序的刀具表），一个图纸就要发十几个请求，
// 界面还会一段一段地跳出来。这里用 4 条 SQL 一次查完，在内存里组装。
func (r *Repo) DrawingDetail(ctx context.Context, drawingID int64) (*domain.DrawingDetail, error) {
	d, err := r.GetDrawing(ctx, drawingID)
	if err != nil {
		return nil, err
	}

	ops, err := r.ListOperations(ctx, drawingID)
	if err != nil {
		return nil, err
	}
	out := &domain.DrawingDetail{Drawing: *d, Operations: []domain.DetailOperation{}}
	if len(ops) == 0 {
		return out, nil
	}

	// 一次把所有工序下的程序查出来
	opIDs := make([]int64, 0, len(ops))
	for _, o := range ops {
		opIDs = append(opIDs, o.ID)
	}
	progs := []domain.Program{}
	pq, pargs, err := sqlx.In(`SELECT `+programCols+` FROM nc_program p
		WHERE p.operation_id IN (?) ORDER BY p.operation_id, p.program_no`, opIDs)
	if err != nil {
		return nil, wrap(err)
	}
	if err := r.db.SelectContext(ctx, &progs, r.db.Rebind(pq), pargs...); err != nil {
		return nil, wrap(err)
	}

	// 一次把所有程序的刀具补偿表查出来
	toolsByProgram := map[int64][]domain.ProgramTool{}
	if len(progs) > 0 {
		progIDs := make([]int64, 0, len(progs))
		for _, p := range progs {
			progIDs = append(progIDs, p.ID)
		}
		allTools := []domain.ProgramTool{}
		tq, targs, err := sqlx.In(`SELECT `+toolRowCols+` FROM program_tool
			WHERE program_id IN (?) ORDER BY program_id, seq`, progIDs)
		if err != nil {
			return nil, wrap(err)
		}
		if err := r.db.SelectContext(ctx, &allTools, r.db.Rebind(tq), targs...); err != nil {
			return nil, wrap(err)
		}
		for i := range allTools {
			allTools[i].ApplyMilli()
			toolsByProgram[allTools[i].ProgramID] = append(toolsByProgram[allTools[i].ProgramID], allTools[i])
		}
	}

	// 组装
	progsByOp := map[int64][]domain.DetailProgram{}
	for _, p := range progs {
		tools := toolsByProgram[p.ID]
		if tools == nil {
			tools = []domain.ProgramTool{}
		}
		progsByOp[p.OperationID] = append(progsByOp[p.OperationID], domain.DetailProgram{
			ID: p.ID, ProgramNo: p.ProgramNo, ProgramName: p.ProgramName,
			Controller: p.Controller, CurrentVersionID: p.CurrentVersionID,
			CurrentVersionNo: p.CurrentVersionNo, VersionCount: p.VersionCount,
			Remark: p.Remark, Tools: tools,
		})
	}

	for _, o := range ops {
		list := progsByOp[o.ID]
		if list == nil {
			list = []domain.DetailProgram{}
		}
		out.Operations = append(out.Operations, domain.DetailOperation{
			ID: o.ID, OpNo: o.OpNo, OpName: o.OpName,
			MachineID: o.MachineID, MachineName: o.MachineName,
			Fixture: o.Fixture, ZHeight: o.ZHeight, Remark: o.Remark,
			Programs: list,
		})
	}
	return out, nil
}
