package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"cnccool/internal/domain"
)

// programCols 是查询程序时固定使用的列，附带当前版本号和版本总数。
const programCols = `p.id, p.operation_id, p.program_no, p.program_name, p.controller,
	p.current_version_id, p.remark, p.created_at, p.updated_at,
	(SELECT v.version_no FROM nc_version v WHERE v.id = p.current_version_id) AS current_version_no,
	(SELECT COUNT(*) FROM nc_version v WHERE v.program_id = p.id) AS version_count`

// ---------------------------------------------------------------------------
// 程序
// ---------------------------------------------------------------------------

// ListPrograms 取某工序下的全部程序。
func (r *Repo) ListPrograms(ctx context.Context, operationID int64) ([]domain.Program, error) {
	items := []domain.Program{}
	q := `SELECT ` + programCols + ` FROM nc_program p WHERE p.operation_id = ? ORDER BY p.program_no`
	if err := r.db.SelectContext(ctx, &items, q, operationID); err != nil {
		return nil, wrap(err)
	}
	return items, nil
}

// GetProgram 按 ID 取程序。
func (r *Repo) GetProgram(ctx context.Context, id int64) (*domain.Program, error) {
	var p domain.Program
	if err := r.db.GetContext(ctx, &p,
		`SELECT `+programCols+` FROM nc_program p WHERE p.id = ?`, id); err != nil {
		return nil, wrap(err)
	}
	return &p, nil
}

// CreateProgram 在某工序下新增程序。
func (r *Repo) CreateProgram(ctx context.Context, operationID int64, in domain.ProgramInput) (*domain.Program, error) {
	now := domain.Now()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO nc_program (operation_id, program_no, program_name, controller, remark, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		operationID, strings.TrimSpace(in.ProgramNo), in.ProgramName, in.Controller, in.Remark, now, now)
	if err != nil {
		return nil, wrap(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, wrap(err)
	}
	if err := r.log(ctx, r.db, "create", "program", id, "新增程序 "+in.ProgramNo); err != nil {
		return nil, err
	}
	return r.GetProgram(ctx, id)
}

// UpdateProgram 修改程序基本信息。注意：改这里不会影响已有版本文件。
func (r *Repo) UpdateProgram(ctx context.Context, id int64, in domain.ProgramInput) (*domain.Program, error) {
	if _, err := r.GetProgram(ctx, id); err != nil {
		return nil, err
	}
	if _, err := r.db.ExecContext(ctx,
		`UPDATE nc_program SET program_no = ?, program_name = ?, controller = ?,
		 remark = ?, updated_at = ? WHERE id = ?`,
		strings.TrimSpace(in.ProgramNo), in.ProgramName, in.Controller, in.Remark,
		domain.Now(), id); err != nil {
		return nil, wrap(err)
	}
	if err := r.log(ctx, r.db, "update", "program", id, "修改程序 "+in.ProgramNo); err != nil {
		return nil, err
	}
	return r.GetProgram(ctx, id)
}

// DeleteProgram 删除程序，连带其版本记录。
// 注意：nc_file 里的物理文件不删——同一份文件可能还被别的程序引用，
// 真正的物理清理留给后期的"孤立文件回收"任务，绝不在删除路径上做。
func (r *Repo) DeleteProgram(ctx context.Context, id int64) error {
	p, err := r.GetProgram(ctx, id)
	if err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM nc_program WHERE id = ?`, id); err != nil {
		return wrap(err)
	}
	return r.log(ctx, r.db, "delete", "program", id, "删除程序 "+p.ProgramNo)
}

// SetCurrentVersion 指定程序的当前生效版本（版本回滚就用它）。
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
	if _, err := r.db.ExecContext(ctx,
		`UPDATE nc_program SET current_version_id = ?, updated_at = ? WHERE id = ?`,
		versionID, domain.Now(), programID); err != nil {
		return nil, wrap(err)
	}
	if err := r.log(ctx, r.db, "switch", "program", programID,
		fmt.Sprintf("切换当前版本为第 %d 版", v.VersionNo)); err != nil {
		return nil, err
	}
	return r.GetProgram(ctx, programID)
}

// ---------------------------------------------------------------------------
// 版本
// ---------------------------------------------------------------------------

// versionCols 用 v.file_name（版本自己的文件名），而不是 f.original_name。
// 因为 nc_file 是按 sha256 全局共享的，一份物理文件会被多个版本引用；
// 文件名必须跟着版本走，否则第二个引用者会看到第一个上传者的文件名。
const versionCols = `v.id, v.program_id, v.version_no, v.file_id, v.change_note, v.created_by, v.created_at,
	v.file_name AS file_name, f.size_bytes AS file_size, f.sha256,
	CASE WHEN p.current_version_id = v.id THEN 1 ELSE 0 END AS is_current`

// ListVersions 取某程序的全部版本，新版本在前。
func (r *Repo) ListVersions(ctx context.Context, programID int64) ([]domain.Version, error) {
	items := []domain.Version{}
	q := `SELECT ` + versionCols + ` FROM nc_version v
	      JOIN nc_file f   ON f.id = v.file_id
	      JOIN nc_program p ON p.id = v.program_id
	      WHERE v.program_id = ? ORDER BY v.version_no DESC`
	if err := r.db.SelectContext(ctx, &items, q, programID); err != nil {
		return nil, wrap(err)
	}
	return items, nil
}

// GetVersion 按 ID 取单个版本（含物理文件信息，下载和对比都要用）。
func (r *Repo) GetVersion(ctx context.Context, id int64) (*domain.Version, error) {
	var v domain.Version
	q := `SELECT ` + versionCols + ` FROM nc_version v
	      JOIN nc_file f    ON f.id = v.file_id
	      JOIN nc_program p ON p.id = v.program_id
	      WHERE v.id = ?`
	if err := r.db.GetContext(ctx, &v, q, id); err != nil {
		return nil, wrap(err)
	}
	return &v, nil
}

// GetVersionRelPath 取版本对应物理文件在 NC 库中的相对路径。
func (r *Repo) GetVersionRelPath(ctx context.Context, versionID int64) (string, error) {
	var rel string
	if err := r.db.GetContext(ctx, &rel,
		`SELECT f.rel_path FROM nc_version v JOIN nc_file f ON f.id = v.file_id WHERE v.id = ?`,
		versionID); err != nil {
		return "", wrap(err)
	}
	return rel, nil
}

// AddVersion 上传一个新版本，并把它设为当前版本。
//
// 整个过程在一个事务里完成：文件登记 → 版本号自增 → 更新当前版本 → 记日志。
// 任何一步失败都整体回滚，不会出现"版本记录建了但当前版本没更新"的中间状态。
func (r *Repo) AddVersion(ctx context.Context, programID int64, file domain.NCFileInput, changeNote string) (*domain.Version, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, wrap(err)
	}
	defer func() { _ = tx.Rollback() }() // 提交成功后这里是空操作

	p, err := getProgramTx(ctx, tx, programID)
	if err != nil {
		return nil, err
	}

	// nc_file 按 sha256 去重：内容相同的文件全局只存一份
	fileID, err := upsertNCFile(ctx, tx, file)
	if err != nil {
		return nil, err
	}

	var nextNo int
	if err := tx.GetContext(ctx, &nextNo,
		`SELECT COALESCE(MAX(version_no), 0) + 1 FROM nc_version WHERE program_id = ?`,
		programID); err != nil {
		return nil, wrap(err)
	}

	now := domain.Now()
	// 文件名记在 nc_version 上（而不是只记在共享的 nc_file 上），
	// 这样同一份物理文件被第二个程序引用时，显示的是第二个用户自己的文件名。
	res, err := tx.ExecContext(ctx,
		`INSERT INTO nc_version (program_id, version_no, file_id, file_name, change_note, created_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		programID, nextNo, fileID, file.OriginalName, changeNote, "local", now)
	if err != nil {
		return nil, wrap(err)
	}
	versionID, err := res.LastInsertId()
	if err != nil {
		return nil, wrap(err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE nc_program SET current_version_id = ?, updated_at = ? WHERE id = ?`,
		versionID, now, programID); err != nil {
		return nil, wrap(err)
	}

	if err := r.log(ctx, tx, "upload", "program", programID,
		fmt.Sprintf("%s 上传第 %d 版（%s）", p.ProgramNo, nextNo, file.OriginalName)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, wrap(err)
	}
	return r.GetVersion(ctx, versionID)
}

// getProgramTx 在事务内读取程序。
func getProgramTx(ctx context.Context, tx *sqlx.Tx, id int64) (*domain.Program, error) {
	var p domain.Program
	if err := sqlx.GetContext(ctx, tx, &p,
		`SELECT `+programCols+` FROM nc_program p WHERE p.id = ?`, id); err != nil {
		return nil, wrap(err)
	}
	return &p, nil
}

// ---------------------------------------------------------------------------
// 程序用刀 + 刀补表
// ---------------------------------------------------------------------------

const toolRowCols = `id, program_id, seq, tool_id, tool_no, offset_no, tool_name,
	tool_dia_milli, corner_radius_milli, comp_amount_milli, spindle_speed, speed_mode,
	feed_milli, feed_mode, cut_depth_milli, coolant, machining_content, remark, custom_params_json`

func decodeToolCustomParams(item *domain.ProgramTool) error {
	item.CustomParams = []domain.ToolCustomParam{}
	if strings.TrimSpace(item.CustomParamsJSON) == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(item.CustomParamsJSON), &item.CustomParams); err != nil {
		return fmt.Errorf("解析刀具自定义参数失败: %w", err)
	}
	return nil
}

// ListProgramTools 取某程序的刀具刀补表，按行号升序。
func (r *Repo) ListProgramTools(ctx context.Context, programID int64) ([]domain.ProgramTool, error) {
	items := []domain.ProgramTool{}
	q := `SELECT ` + toolRowCols + ` FROM program_tool WHERE program_id = ? ORDER BY seq`
	if err := r.db.SelectContext(ctx, &items, q, programID); err != nil {
		return nil, wrap(err)
	}
	for i := range items {
		items[i].ApplyMilli()
		if err := decodeToolCustomParams(&items[i]); err != nil {
			return nil, err
		}
	}
	return items, nil
}

// ReplaceProgramTools 整表替换某程序的刀具刀补表。
//
// 为什么用"整表替换"而不是逐行增删改：
// 前端是一张可编辑表格，操作工改完一次提交，用整表替换语义最清晰，
// 不会出现"删了第 2 行之后第 3 行变成第 2 行"这类行号错乱的问题。
// 整表替换放在一个事务里，也不存在改到一半失败留下半张表的风险。
func (r *Repo) ReplaceProgramTools(ctx context.Context, programID int64, items []domain.ProgramToolInput) ([]domain.ProgramTool, error) {
	if _, err := r.GetProgram(ctx, programID); err != nil {
		return nil, err
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, wrap(err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM program_tool WHERE program_id = ?`, programID); err != nil {
		return nil, wrap(err)
	}

	for i, in := range items {
		seq := in.Seq
		if seq <= 0 {
			seq = i + 1
		}
		customParams := make([]domain.ToolCustomParam, 0, len(in.CustomParams))
		for _, param := range in.CustomParams {
			customParams = append(customParams, domain.ToolCustomParam{
				Name: strings.TrimSpace(param.Name), Value: strings.TrimSpace(param.Value),
			})
		}
		customJSON, err := json.Marshal(customParams)
		if err != nil {
			return nil, fmt.Errorf("序列化刀具自定义参数失败: %w", err)
		}
		t := domain.ProgramTool{
			ToolID: in.ToolID, ToolNo: strings.TrimSpace(in.ToolNo), OffsetNo: strings.TrimSpace(in.OffsetNo),
			ToolName: in.ToolName, SpindleSpeed: in.SpindleSpeed, SpeedMode: in.SpeedMode,
			FeedMode: in.FeedMode, Coolant: in.Coolant,
			MachiningContent: in.MachiningContent, Remark: in.Remark,
			ToolDia: in.ToolDia, CornerRadius: in.CornerRadius,
			CompAmount: in.CompAmount, Feed: in.Feed, CutDepth: in.CutDepth,
			CustomParams: customParams, CustomParamsJSON: string(customJSON),
		}
		t.FillMilli()

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO program_tool (program_id, seq, tool_id, tool_no, offset_no, tool_name,
			 tool_dia_milli, corner_radius_milli, comp_amount_milli, spindle_speed, speed_mode,
			 feed_milli, feed_mode, cut_depth_milli, coolant, machining_content, remark, custom_params_json)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			programID, seq, t.ToolID, t.ToolNo, t.OffsetNo, t.ToolName,
			t.ToolDiaMilli, t.CornerRadiusMilli, t.CompAmountMilli, t.SpindleSpeed, t.SpeedMode,
			t.FeedMilli, t.FeedMode, t.CutDepthMilli, t.Coolant,
			t.MachiningContent, t.Remark, t.CustomParamsJSON); err != nil {
			return nil, wrap(err)
		}
	}

	if err := r.log(ctx, tx, "update", "program", programID,
		fmt.Sprintf("更新刀具刀补表，共 %d 把刀", len(items))); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, wrap(err)
	}
	return r.ListProgramTools(ctx, programID)
}

// ---------------------------------------------------------------------------
// 操作日志
// ---------------------------------------------------------------------------

// ListLogs 取与某程序相关的操作日志（含其版本变更）。
func (r *Repo) ListLogs(ctx context.Context, programID int64, limit int) ([]domain.AuditLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	items := []domain.AuditLog{}
	if err := r.db.SelectContext(ctx, &items,
		`SELECT id, action, entity_type, entity_id, detail, actor, created_at
		 FROM audit_log WHERE entity_type = 'program' AND entity_id = ?
		 ORDER BY id DESC LIMIT ?`, programID, limit); err != nil {
		return nil, wrap(err)
	}
	return items, nil
}
