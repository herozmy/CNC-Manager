-- 0001_init.sql —— 初始表结构
--
-- 全库统一约定（务必遵守，否则后期换 PostgreSQL 会返工）：
--   1. 时间统一存 UTC 的 RFC3339 文本，字典序即时间序，任何库都能原样搬运。
--   2. 所有带小数的尺寸/进给一律存「实际值 × 1000」的整数，字段名以 _milli 结尾。
--      SQLite 没有精确小数类型，用浮点存刀补迟早会出现 12.499999 这种脏数据。
--   3. 不用 SQLite 私有语法：主键写 INTEGER PRIMARY KEY（rowid 别名，自动编号），
--      不用 AUTOINCREMENT；不用 INSERT OR REPLACE；不用 strftime() 等函数。
--   4. 外键统一 ON DELETE CASCADE，删图纸即连带删工序、程序、版本引用。

-- ---------------------------------------------------------------------------
-- 图纸
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS drawing (
    id              INTEGER PRIMARY KEY,
    drawing_no      TEXT NOT NULL,
    name            TEXT NOT NULL DEFAULT '',
    customer        TEXT NOT NULL DEFAULT '',
    material        TEXT NOT NULL DEFAULT '',
    drawing_version TEXT NOT NULL DEFAULT '',
    remark          TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_drawing_no ON drawing (drawing_no);

-- ---------------------------------------------------------------------------
-- 机台字典
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS machine (
    id         INTEGER PRIMARY KEY,
    code       TEXT NOT NULL,
    name       TEXT NOT NULL DEFAULT '',
    controller TEXT NOT NULL DEFAULT '',
    remark     TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_machine_code ON machine (code);

-- ---------------------------------------------------------------------------
-- 工序（一序 / 二序 …），编号用 10 / 20 / 30 以留插入空位
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS operation (
    id         INTEGER PRIMARY KEY,
    drawing_id INTEGER NOT NULL REFERENCES drawing (id) ON DELETE CASCADE,
    op_no      INTEGER NOT NULL,
    op_name    TEXT NOT NULL DEFAULT '',
    machine_id INTEGER REFERENCES machine (id) ON DELETE SET NULL,
    fixture    TEXT NOT NULL DEFAULT '',
    remark     TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_operation_drawing_opno ON operation (drawing_id, op_no);

-- ---------------------------------------------------------------------------
-- NC 物理文件库：按内容 sha256 寻址，同一份文件全局只存一份
-- rel_path 是相对 NC 库根目录的路径（如 a3/f9/a3f9....nc），
-- 绝不存绝对路径——否则数据目录一搬家或挂进容器，全库路径失效。
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS nc_file (
    id            INTEGER PRIMARY KEY,
    sha256        TEXT NOT NULL,
    size_bytes    INTEGER NOT NULL,
    original_name TEXT NOT NULL,
    rel_path      TEXT NOT NULL,
    created_at    TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_nc_file_sha ON nc_file (sha256);

-- ---------------------------------------------------------------------------
-- 程序
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS nc_program (
    id                 INTEGER PRIMARY KEY,
    operation_id       INTEGER NOT NULL REFERENCES operation (id) ON DELETE CASCADE,
    program_no         TEXT NOT NULL,
    program_name       TEXT NOT NULL DEFAULT '',
    controller         TEXT NOT NULL DEFAULT '',
    current_version_id INTEGER,
    remark             TEXT NOT NULL DEFAULT '',
    created_at         TEXT NOT NULL,
    updated_at         TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS ix_nc_program_operation ON nc_program (operation_id);

-- ---------------------------------------------------------------------------
-- 程序版本：每次上传 NC 产生一个新版本，可回滚、可对比
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS nc_version (
    id          INTEGER PRIMARY KEY,
    program_id  INTEGER NOT NULL REFERENCES nc_program (id) ON DELETE CASCADE,
    version_no  INTEGER NOT NULL,
    file_id     INTEGER NOT NULL REFERENCES nc_file (id),
    change_note TEXT NOT NULL DEFAULT '',
    created_by  TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_nc_version_program_no ON nc_version (program_id, version_no);

-- ---------------------------------------------------------------------------
-- 刀具字典
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tool (
    id         INTEGER PRIMARY KEY,
    tool_no    TEXT NOT NULL,
    name       TEXT NOT NULL DEFAULT '',
    spec       TEXT NOT NULL DEFAULT '',
    tool_type  TEXT NOT NULL DEFAULT '',
    remark     TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_tool_no ON tool (tool_no);

-- ---------------------------------------------------------------------------
-- 程序用刀 + 刀补参数（核心表）
--
-- *_milli 字段 = 实际值 × 1000。例如直径 Ø12.5mm 存 12500，切深 0.5mm 存 500。
-- feed_milli 根据 feed_mode 解释：0 = mm/min（G94），1 = mm/rev（G95）。
-- speed_mode：0 = G97 恒转速 r/min，1 = G96 恒线速 m/min。
-- coolant：0 = 无，1 = 冷却液，2 = 吹气，3 = 喷雾。
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS program_tool (
    id                INTEGER PRIMARY KEY,
    program_id        INTEGER NOT NULL REFERENCES nc_program (id) ON DELETE CASCADE,
    seq               INTEGER NOT NULL,
    tool_id           INTEGER REFERENCES tool (id) ON DELETE SET NULL,
    tool_no           TEXT NOT NULL DEFAULT '',
    offset_no         TEXT NOT NULL DEFAULT '',
    tool_name         TEXT NOT NULL DEFAULT '',
    tool_dia_milli    INTEGER NOT NULL DEFAULT 0,
    corner_radius_milli INTEGER NOT NULL DEFAULT 0,
    spindle_speed     INTEGER NOT NULL DEFAULT 0,
    speed_mode        INTEGER NOT NULL DEFAULT 0,
    feed_milli        INTEGER NOT NULL DEFAULT 0,
    feed_mode         INTEGER NOT NULL DEFAULT 0,
    cut_depth_milli   INTEGER NOT NULL DEFAULT 0,
    coolant           INTEGER NOT NULL DEFAULT 0,
    machining_content TEXT NOT NULL DEFAULT '',
    remark            TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_program_tool_seq ON program_tool (program_id, seq);

-- ---------------------------------------------------------------------------
-- 操作日志
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS audit_log (
    id          INTEGER PRIMARY KEY,
    action      TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id   INTEGER NOT NULL,
    detail      TEXT NOT NULL DEFAULT '',
    actor       TEXT NOT NULL DEFAULT 'local',
    created_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS ix_audit_log_entity ON audit_log (entity_type, entity_id);

-- ---------------------------------------------------------------------------
-- 通用配置（留作后期扩展，如默认数控系统、编号规则等）
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS app_setting (
    key_name   TEXT PRIMARY KEY,
    value_text TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL
);
