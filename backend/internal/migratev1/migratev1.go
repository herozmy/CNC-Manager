// Package migratev1 把 v1（cnccool）的数据库与 NC 文件库无损导入 v2。
//
// 做法：只读 ATTACH 旧库，用 INSERT…SELECT 保持主键不变，外键关系因此原样保留；
// NC 文件逐个读出（兼容旧版硬编码密钥加密和更早的明文）、校验 sha256，
// 再用 v2 的随机密钥重新加密。整个导入在单个事务里，任何一步失败即整体回滚。
package migratev1

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmoiron/sqlx"

	"cnc-manager/internal/ncstore"
)

// legacyKey 是 v1 硬编码的 NC 文件密钥，仅用于读取旧数据。
var legacyKey = sha256.Sum256([]byte("cnccool-nc-file-storage-v1"))

// requiredV1Migration 是 v1 最后一个迁移；低于它的旧库需要先用 v1 升级。
const requiredV1Migration = "0010_machine_tool_magazine"

// Report 是导入结果，Counts 为各表导入行数。
type Report struct {
	Counts          map[string]int
	BlobsMigrated   int
	OrphanFiles     int // 旧库中没有任何版本引用、未导入的 NC 文件记录
	SkippedSession  int // 未导入的登录会话数（用户需重新登录）
	SkippedSettings int // 未导入的 v1 扩展配置
}

// Run 执行导入。newDB 必须是已完成迁移且为空的 v2 库。
func Run(ctx context.Context, oldDBPath, oldNCRoot string, newDB *sqlx.DB, store *ncstore.Store) (*Report, error) {
	if _, err := os.Stat(oldDBPath); err != nil {
		return nil, fmt.Errorf("找不到旧数据库 %s: %w", oldDBPath, err)
	}
	conn, err := newDB.Connx(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := requireEmpty(ctx, conn); err != nil {
		return nil, err
	}
	// 只读方式挂载旧库，保证不会改动旧数据。
	uri := "file:" + url.PathEscape(filepath.ToSlash(oldDBPath)) + "?mode=ro"
	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS old`, uri); err != nil {
		return nil, fmt.Errorf("挂载旧数据库失败: %w", err)
	}
	defer func() { _, _ = conn.ExecContext(context.Background(), `DETACH DATABASE old`) }()

	var last string
	if err := conn.GetContext(ctx, &last, `SELECT COALESCE(MAX(version), '') FROM old.schema_migration`); err != nil {
		return nil, fmt.Errorf("读取旧库迁移记录失败（不是 v1 数据库？）: %w", err)
	}
	if last < requiredV1Migration {
		return nil, fmt.Errorf("旧库结构版本为 %q，低于 %s，请先用 v1 程序升级后再迁移", last, requiredV1Migration)
	}
	var integrity string
	if err := conn.GetContext(ctx, &integrity, `PRAGMA old.integrity_check`); err != nil || integrity != "ok" {
		return nil, fmt.Errorf("旧数据库完整性检查失败")
	}

	tx, err := conn.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	// program ↔ version 互相引用，推迟到提交时检查外键。
	if _, err := tx.ExecContext(ctx, `PRAGMA defer_foreign_keys = ON`); err != nil {
		return nil, err
	}

	rep := &Report{Counts: map[string]int{}}
	if err := importBlobs(ctx, tx, oldNCRoot, store, rep); err != nil {
		return nil, err
	}
	for _, st := range tableCopies {
		res, err := tx.ExecContext(ctx, st.sql)
		if err != nil {
			return nil, fmt.Errorf("导入 %s 失败: %w", st.table, err)
		}
		n, _ := res.RowsAffected()
		rep.Counts[st.table] = int(n)
	}
	rep.Counts["nc_blob"] = rep.BlobsMigrated

	if err := tx.GetContext(ctx, &rep.OrphanFiles, `
		SELECT COUNT(*) FROM old.nc_file WHERE id NOT IN (SELECT file_id FROM old.nc_version)`); err != nil {
		return nil, err
	}
	if err := tx.GetContext(ctx, &rep.SkippedSession, `SELECT COUNT(*) FROM old.user_session`); err != nil {
		return nil, err
	}
	if err := tx.GetContext(ctx, &rep.SkippedSettings, `SELECT COUNT(*) FROM old.app_setting`); err != nil {
		return nil, err
	}
	if err := reconcile(ctx, tx, rep); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交导入失败: %w", err)
	}
	return rep, nil
}

func requireEmpty(ctx context.Context, conn *sqlx.Conn) error {
	for _, t := range []string{"drawing", "machine", "tool", "app_user", "nc_blob"} {
		var n int
		if err := conn.GetContext(ctx, &n, `SELECT COUNT(*) FROM `+t); err != nil {
			return fmt.Errorf("检查新库失败（是否已完成迁移？）: %w", err)
		}
		if n > 0 {
			return fmt.Errorf("新库不是空库（%s 已有 %d 行），拒绝导入以免混合数据", t, n)
		}
	}
	return nil
}

type oldBlob struct {
	SHA256   string `db:"sha256"`
	Size     int64  `db:"size_bytes"`
	RelPath  string `db:"rel_path"`
	Encoding string `db:"file_encoding"`
	Created  string `db:"created_at"`
}

func importBlobs(ctx context.Context, tx *sqlx.Tx, oldNCRoot string, store *ncstore.Store, rep *Report) error {
	var blobs []oldBlob
	if err := tx.SelectContext(ctx, &blobs, `
		SELECT sha256, size_bytes, rel_path, file_encoding, created_at FROM old.nc_file
		WHERE id IN (SELECT file_id FROM old.nc_version) ORDER BY id`); err != nil {
		return fmt.Errorf("读取旧 NC 文件表失败: %w", err)
	}
	var problems []string
	for _, b := range blobs {
		if err := ctx.Err(); err != nil {
			return err
		}
		plain, err := readOld(oldNCRoot, b.RelPath, store.MaxBytes())
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", b.RelPath, err))
			continue
		}
		sum := sha256.Sum256(plain)
		if hex.EncodeToString(sum[:]) != b.SHA256 || int64(len(plain)) != b.Size {
			problems = append(problems, fmt.Sprintf("%s: 内容哈希或大小与数据库记录不符（文件可能被改动或损坏）", b.RelPath))
			continue
		}
		saved, err := store.Save(strings.NewReader(string(plain)))
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: 写入新库失败: %v", b.RelPath, err))
			continue
		}
		enc := ncstore.NormalizeEncoding(b.Encoding)
		if b.Encoding == "" {
			enc = saved.Encoding
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO nc_blob (sha256, size_bytes, encoding, rel_path, created_at) VALUES (?, ?, ?, ?, ?)`,
			saved.SHA256, saved.SizeBytes, enc, saved.RelPath, b.Created); err != nil {
			return fmt.Errorf("导入 nc_blob 失败: %w", err)
		}
		rep.BlobsMigrated++
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d 个 NC 文件无法导入，已整体取消:\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
	return nil
}

// readOld 读取旧库中的一个 NC 文件，兼容加密与更早期的明文存储。
func readOld(root, rel string, maxBytes int64) ([]byte, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("非法路径")
	}
	f, err := os.Open(filepath.Join(root, clean))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("文件缺失")
		}
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxBytes+65))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > maxBytes+64 {
		return nil, fmt.Errorf("文件超过当前服务器的单文件上限")
	}
	if !ncstore.IsEncrypted(raw) {
		return raw, nil
	}
	return ncstore.DecryptBytes(legacyKey[:], raw)
}

type tableCopy struct{ table, sql string }

// 主键全部保持不变。顺序无关紧要（已推迟外键检查），但按依赖排列便于阅读。
var tableCopies = []tableCopy{
	{"drawing", `INSERT INTO drawing (id, drawing_no, name, customer, material, drawing_version, remark, created_at, updated_at)
		SELECT id, drawing_no, name, customer, material, drawing_version, remark, created_at, updated_at FROM old.drawing`},
	{"machine", `INSERT INTO machine (id, code, name, controller, remark, created_at, updated_at)
		SELECT id, code, name, controller, remark, created_at, updated_at FROM old.machine`},
	{"machine_tool", `INSERT INTO machine_tool (id, machine_id, pocket_no, tool_no, tool_name, spec, remark, updated_at)
		SELECT id, machine_id, pocket_no, tool_no, tool_name, spec, remark, updated_at FROM old.machine_tool`},
	{"operation", `INSERT INTO operation (id, drawing_id, op_no, op_name, machine_id, fixture, z_height_milli, remark, sort_order, created_at, updated_at)
		SELECT id, drawing_id, op_no, op_name, machine_id, fixture, z_height_milli, remark, sort_order, created_at, updated_at FROM old.operation`},
	{"tool", `INSERT INTO tool (id, tool_no, name, spec, tool_type, remark, created_at, updated_at)
		SELECT id, tool_no, name, spec, tool_type, remark, created_at, updated_at FROM old.tool`},
	{"nc_program", `INSERT INTO nc_program (id, operation_id, program_no, program_name, controller, current_version_id, remark, sort_order, created_at, updated_at,machine_id)
		SELECT p.id, p.operation_id, p.program_no, p.program_name, p.controller, p.current_version_id, p.remark, p.sort_order, p.created_at, p.updated_at,o.machine_id FROM old.nc_program p JOIN old.operation o ON o.id=p.operation_id`},
	// 文件名以版本上记录的为准；极老数据为空时回退到物理文件的首次上传名。
	{"nc_version", `INSERT INTO nc_version (id, program_id, version_no, sha256, file_name, change_note, created_by, created_at)
		SELECT v.id, v.program_id, v.version_no, f.sha256,
		       CASE WHEN v.file_name <> '' THEN v.file_name ELSE f.original_name END,
		       v.change_note, v.created_by, v.created_at
		FROM old.nc_version v JOIN old.nc_file f ON f.id = v.file_id`},
	{"program_tool", `INSERT INTO program_tool (id, program_id, version_id, seq, tool_id, tool_no, offset_no, tool_name,
			tool_dia_milli, corner_radius_milli, spindle_speed, speed_mode, feed_milli, feed_mode, cut_depth_milli,
			coolant, machining_content, remark, comp_amount_milli, custom_params_json)
		SELECT id, program_id, version_id, seq, tool_id, tool_no, offset_no, tool_name,
			tool_dia_milli, corner_radius_milli, spindle_speed, speed_mode, feed_milli, feed_mode, cut_depth_milli,
			coolant, machining_content, remark, comp_amount_milli, custom_params_json FROM old.program_tool`},
	// v1 没有角色，原有账号全部成为管理员（与 v1 中人人权限相同保持一致）。
	{"app_user", `INSERT INTO app_user (id, username, display_name, password_hash, role, enabled, created_at, updated_at)
		SELECT id, username, display_name, password_hash, 'admin', enabled, created_at, updated_at FROM old.app_user`},
	{"audit_log", `INSERT INTO audit_log (id, action, entity_type, entity_id, detail, actor, created_at)
		SELECT id, action, entity_type, entity_id, detail, actor, created_at FROM old.audit_log`},
}

// reconcile 对账：新库每张表的行数必须等于旧库（nc_blob 除外，只导入被引用的）。
func reconcile(ctx context.Context, tx *sqlx.Tx, rep *Report) error {
	for _, st := range tableCopies {
		var oldN, newN int
		if err := tx.GetContext(ctx, &oldN, `SELECT COUNT(*) FROM old.`+st.table); err != nil {
			return err
		}
		if err := tx.GetContext(ctx, &newN, `SELECT COUNT(*) FROM main.`+st.table); err != nil {
			return err
		}
		if oldN != newN {
			return fmt.Errorf("对账失败：%s 旧库 %d 行，新库 %d 行", st.table, oldN, newN)
		}
	}
	// 外键完整性（推迟的外键在这里提前暴露，便于给出明确错误）。
	rows, err := tx.QueryxContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("对账失败：导入后存在违反外键约束的数据")
	}
	return nil
}
