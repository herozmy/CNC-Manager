package repo

import (
	"context"

	"github.com/jmoiron/sqlx"

	"cnccool/internal/domain"
)

// treeDrawingLimit 限制一次返回的图纸数量。
//
// 编程组的数据规模下（几百到一两千个图纸号）一次全量返回体验最好：
// 不需要前端逐级请求，展开零延迟。真超过这个量再考虑按需加载。
const treeDrawingLimit = 500

// Tree 一次性返回「图纸 → 工序 → 程序」三级轻量结构，供前端左侧树使用。
//
// 为什么单独做一个接口：如果让前端每展开一层就请求一次，
// 打开一个工程要发几十个请求，局域网里也会明显卡顿。
func (r *Repo) Tree(ctx context.Context, keyword string) ([]domain.TreeDrawing, error) {
	pat := likePattern(keyword)

	// 1) 命中的图纸。除了图纸自身字段，还支持用程序号/程序名反查图纸——
	//    现场最常见的用法就是"我只记得程序号 O1234，帮我找出是哪个图纸"。
	type drawingRow struct {
		ID        int64  `db:"id"`
		DrawingNo string `db:"drawing_no"`
		Name      string `db:"name"`
		Material  string `db:"material"`
		Customer  string `db:"customer"`
	}
	drawings := []drawingRow{}
	if err := r.db.SelectContext(ctx, &drawings,
		`SELECT d.id, d.drawing_no, d.name, d.material, d.customer
		 FROM drawing d
		 WHERE (
			d.drawing_no LIKE ? ESCAPE '\' OR d.name LIKE ? ESCAPE '\' OR
			d.customer   LIKE ? ESCAPE '\' OR d.material LIKE ? ESCAPE '\' OR
			EXISTS (SELECT 1 FROM operation o JOIN nc_program p ON p.operation_id = o.id
			        WHERE o.drawing_id = d.id
			          AND (p.program_no LIKE ? ESCAPE '\' OR p.program_name LIKE ? ESCAPE '\'))
		 )
		 ORDER BY d.drawing_no
		 LIMIT ?`,
		pat, pat, pat, pat, pat, pat, treeDrawingLimit); err != nil {
		return nil, wrap(err)
	}
	if len(drawings) == 0 {
		return []domain.TreeDrawing{}, nil
	}

	drawingIDs := make([]int64, 0, len(drawings))
	for _, d := range drawings {
		drawingIDs = append(drawingIDs, d.ID)
	}

	// 2) 这些图纸下的全部工序
	var ops []domain.Operation
	oq, oargs, err := sqlx.In(`SELECT `+operationCols+` FROM operation o
		LEFT JOIN machine m ON m.id = o.machine_id
		WHERE o.drawing_id IN (?) ORDER BY o.drawing_id, o.op_no`, drawingIDs)
	if err != nil {
		return nil, wrap(err)
	}
	if err := r.db.SelectContext(ctx, &ops, r.db.Rebind(oq), oargs...); err != nil {
		return nil, wrap(err)
	}

	// 3) 这些工序下的全部程序
	opIDs := make([]int64, 0, len(ops))
	for _, o := range ops {
		opIDs = append(opIDs, o.ID)
	}
	progs := []domain.Program{}
	if len(opIDs) > 0 {
		pq, pargs, err := sqlx.In(`SELECT `+programCols+` FROM nc_program p
			WHERE p.operation_id IN (?) ORDER BY p.operation_id, p.program_no`, opIDs)
		if err != nil {
			return nil, wrap(err)
		}
		if err := r.db.SelectContext(ctx, &progs, r.db.Rebind(pq), pargs...); err != nil {
			return nil, wrap(err)
		}
	}

	// 4) 在内存里组装成树（数据量小，三次查询 + 一次组装远快于递归查询）
	progsByOp := make(map[int64][]domain.TreeProgram, len(ops))
	for _, p := range progs {
		progsByOp[p.OperationID] = append(progsByOp[p.OperationID], domain.TreeProgram{
			ID: p.ID, ProgramNo: p.ProgramNo, ProgramName: p.ProgramName,
			Controller: p.Controller, CurrentVersionNo: p.CurrentVersionNo,
			VersionCount: p.VersionCount,
		})
	}
	opsByDrawing := make(map[int64][]domain.TreeOperation, len(drawings))
	for _, o := range ops {
		list := progsByOp[o.ID]
		if list == nil {
			list = []domain.TreeProgram{}
		}
		opsByDrawing[o.DrawingID] = append(opsByDrawing[o.DrawingID], domain.TreeOperation{
			ID: o.ID, OpNo: o.OpNo, OpName: o.OpName, MachineID: o.MachineID,
			MachineName: o.MachineName, ProgramCount: o.ProgramCount, Programs: list,
		})
	}

	out := make([]domain.TreeDrawing, 0, len(drawings))
	for _, d := range drawings {
		list := opsByDrawing[d.ID]
		if list == nil {
			list = []domain.TreeOperation{}
		}
		out = append(out, domain.TreeDrawing{
			ID: d.ID, DrawingNo: d.DrawingNo, Name: d.Name, Material: d.Material,
			Customer: d.Customer, OperationCount: len(list), Operations: list,
		})
	}
	return out, nil
}
