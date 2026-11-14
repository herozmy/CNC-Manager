package repo

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"cnc-manager/internal/domain"
	"github.com/jmoiron/sqlx"
)

const toolRowCols = `id, program_id, version_id, seq, tool_id, tool_no, offset_no, tool_name,
 tool_dia_milli, corner_radius_milli, comp_amount_milli, spindle_speed, speed_mode,
 feed_milli, feed_mode, cut_depth_milli, coolant, machining_content, remark, custom_params_json`

// Read the version and its rows in one snapshot, including for empty tables.
func readToolTable(ctx context.Context, tx *sqlx.Tx, versionID int64) (*domain.ToolTable, int64, error) {
	var programID int64
	if err := tx.GetContext(ctx, &programID, `SELECT program_id FROM nc_version WHERE id=?`, versionID); err != nil {
		return nil, 0, wrap(err)
	}
	table := &domain.ToolTable{Items: []domain.ProgramTool{}}
	if err := tx.SelectContext(ctx, &table.Items, `SELECT `+toolRowCols+` FROM program_tool WHERE version_id=? ORDER BY seq`, versionID); err != nil {
		return nil, 0, wrap(err)
	}
	for i := range table.Items {
		row := &table.Items[i]
		row.ApplyMilli()
		if strings.TrimSpace(row.CustomParamsJSON) != "" {
			if err := json.Unmarshal([]byte(row.CustomParamsJSON), &row.CustomParams); err != nil {
				return nil, 0, fmt.Errorf("解析刀具自定义参数失败: %w", err)
			}
		}
		if row.CustomParams == nil {
			row.CustomParams = []domain.ToolCustomParam{}
		}
	}
	raw, err := json.Marshal(table.Items)
	if err != nil {
		return nil, 0, err
	}
	table.Revision = fmt.Sprintf("%x", sha256.Sum256(append([]byte(fmt.Sprintf("%d:", versionID)), raw...)))
	return table, programID, nil
}

func (r *Repo) GetVersionTools(ctx context.Context, versionID int64) (*domain.ToolTable, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, wrap(err)
	}
	defer func() { _ = tx.Rollback() }()
	table, _, err := readToolTable(ctx, tx, versionID)
	if err != nil {
		return nil, err
	}
	return table, wrap(tx.Commit())
}

// Compare and replace in one transaction so failures cannot leave a partial table.
func (r *Repo) ReplaceVersionTools(ctx context.Context, versionID int64, in domain.ToolTableInput) (*domain.ToolTable, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, wrap(err)
	}
	defer func() { _ = tx.Rollback() }()
	old, programID, err := readToolTable(ctx, tx, versionID)
	if err != nil {
		return nil, err
	}
	if in.ExpectedRevision != old.Revision {
		return nil, fmt.Errorf("%w: 刀具表已被修改，请载入最新数据后再保存", domain.ErrConflict)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM program_tool WHERE version_id=?`, versionID); err != nil {
		return nil, wrap(err)
	}
	for _, row := range in.Items {
		custom, err := json.Marshal(row.CustomParams)
		if err != nil {
			return nil, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO program_tool
   (program_id,version_id,seq,tool_id,tool_no,offset_no,tool_name,
    tool_dia_milli,corner_radius_milli,comp_amount_milli,spindle_speed,speed_mode,
    feed_milli,feed_mode,cut_depth_milli,coolant,machining_content,remark,custom_params_json)
   VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			programID, versionID, row.Seq, row.ToolID, row.ToolNo, row.OffsetNo, row.ToolName,
			domain.FloatToMilli(row.ToolDia), domain.FloatToMilli(row.CornerRadius), domain.FloatToMilli(row.CompAmount), row.SpindleSpeed, row.SpeedMode,
			domain.FloatToMilli(row.Feed), row.FeedMode, domain.FloatToMilli(row.CutDepth), row.Coolant, row.MachiningContent, row.Remark, string(custom))
		if err != nil {
			return nil, wrap(err)
		}
	}
	if err := r.log(ctx, tx, "update", "version", versionID, fmt.Sprintf("更新刀具补偿表，共 %d 行", len(in.Items))); err != nil {
		return nil, err
	}
	result, _, err := readToolTable(ctx, tx, versionID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, wrap(err)
	}
	return result, nil
}
