-- 0001_init.sql —— v2 初始结构（由 v1 的 10 个迁移合并而来）
--
-- 全库约定：
--   1. 时间存 UTC RFC3339 文本。
--   2. 小数尺寸/进给存「实际值 × 1000」整数，字段名以 _milli 结尾。
--   3. 不用 SQLite 私有语法（AUTOINCREMENT、INSERT OR REPLACE、strftime）。
--   4. 外键默认 ON DELETE CASCADE。

CREATE TABLE drawing (
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
CREATE UNIQUE INDEX ux_drawing_no ON drawing (drawing_no);

CREATE TABLE machine (
    id         INTEGER PRIMARY KEY,
    code       TEXT NOT NULL,
    name       TEXT NOT NULL DEFAULT '',
    controller TEXT NOT NULL DEFAULT '',
    remark     TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX ux_machine_code ON machine (code);

-- 机床当前刀库；刀位属于机床，同一机床内不可重复。
CREATE TABLE machine_tool (
    id         INTEGER PRIMARY KEY,
    machine_id INTEGER NOT NULL REFERENCES machine (id) ON DELETE CASCADE,
    pocket_no  INTEGER NOT NULL CHECK (pocket_no > 0),
    tool_no    TEXT NOT NULL DEFAULT '',
    tool_name  TEXT NOT NULL DEFAULT '',
    spec       TEXT NOT NULL DEFAULT '',
    remark     TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX ux_machine_tool_pocket ON machine_tool (machine_id, pocket_no);

-- 工序编号用 10/20/30 以留插入空位；z_height_milli 是装夹垫块高度（影响 Z 向对刀）。
CREATE TABLE operation (
    id             INTEGER PRIMARY KEY,
    drawing_id     INTEGER NOT NULL REFERENCES drawing (id) ON DELETE CASCADE,
    op_no          INTEGER NOT NULL,
    op_name        TEXT NOT NULL DEFAULT '',
    machine_id     INTEGER REFERENCES machine (id) ON DELETE SET NULL,
    fixture        TEXT NOT NULL DEFAULT '',
    z_height_milli INTEGER NOT NULL DEFAULT 0,
    remark         TEXT NOT NULL DEFAULT '',
    sort_order     INTEGER NOT NULL DEFAULT 0,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL
);
CREATE UNIQUE INDEX ux_operation_drawing_opno ON operation (drawing_id, op_no);
CREATE INDEX ix_operation_drawing_sort ON operation (drawing_id, sort_order, id);

-- NC 内容表：按明文 sha256 寻址，同一内容全局只存一份。
-- rel_path 为相对 NC 库根目录的路径，绝不存绝对路径。文件名属于版本，不属于内容。
CREATE TABLE nc_blob (
    sha256     TEXT PRIMARY KEY,
    size_bytes INTEGER NOT NULL,
    encoding   TEXT NOT NULL DEFAULT 'utf-8' CHECK (encoding IN ('utf-8', 'gbk')),
    rel_path   TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE nc_program (
    id                 INTEGER PRIMARY KEY,
    operation_id       INTEGER NOT NULL REFERENCES operation (id) ON DELETE CASCADE,
    program_no         TEXT NOT NULL,
    program_name       TEXT NOT NULL DEFAULT '',
    controller         TEXT NOT NULL DEFAULT '',
    current_version_id INTEGER REFERENCES nc_version (id) ON DELETE SET NULL,
    remark             TEXT NOT NULL DEFAULT '',
    sort_order         INTEGER NOT NULL DEFAULT 0,
    created_at         TEXT NOT NULL,
    updated_at         TEXT NOT NULL
);
CREATE INDEX ix_nc_program_operation_sort ON nc_program (operation_id, sort_order, id);

CREATE TABLE nc_version (
    id          INTEGER PRIMARY KEY,
    program_id  INTEGER NOT NULL REFERENCES nc_program (id) ON DELETE CASCADE,
    version_no  INTEGER NOT NULL,
    sha256      TEXT NOT NULL REFERENCES nc_blob (sha256),
    file_name   TEXT NOT NULL DEFAULT '',
    change_note TEXT NOT NULL DEFAULT '',
    created_by  TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL
);
CREATE UNIQUE INDEX ux_nc_version_program_no ON nc_version (program_id, version_no);
CREATE INDEX ix_nc_version_sha ON nc_version (sha256);

CREATE TABLE tool (
    id         INTEGER PRIMARY KEY,
    tool_no    TEXT NOT NULL,
    name       TEXT NOT NULL DEFAULT '',
    spec       TEXT NOT NULL DEFAULT '',
    tool_type  TEXT NOT NULL DEFAULT '',
    remark     TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX ux_tool_no ON tool (tool_no);

-- 刀具补偿表，跟随程序版本。
-- feed_mode：0 = mm/min(G94)，1 = mm/rev(G95)；speed_mode：0 = G97 r/min，1 = G96 m/min；
-- coolant：0 无，1 冷却液，2 吹气，3 喷雾。
CREATE TABLE program_tool (
    id                  INTEGER PRIMARY KEY,
    program_id          INTEGER NOT NULL REFERENCES nc_program (id) ON DELETE CASCADE,
    version_id          INTEGER NOT NULL REFERENCES nc_version (id) ON DELETE CASCADE,
    seq                 INTEGER NOT NULL,
    tool_id             INTEGER REFERENCES tool (id) ON DELETE SET NULL,
    tool_no             TEXT NOT NULL DEFAULT '',
    offset_no           TEXT NOT NULL DEFAULT '',
    tool_name           TEXT NOT NULL DEFAULT '',
    tool_dia_milli      INTEGER NOT NULL DEFAULT 0,
    corner_radius_milli INTEGER NOT NULL DEFAULT 0,
    spindle_speed       INTEGER NOT NULL DEFAULT 0,
    speed_mode          INTEGER NOT NULL DEFAULT 0,
    feed_milli          INTEGER NOT NULL DEFAULT 0,
    feed_mode           INTEGER NOT NULL DEFAULT 0,
    cut_depth_milli     INTEGER NOT NULL DEFAULT 0,
    coolant             INTEGER NOT NULL DEFAULT 0,
    machining_content   TEXT NOT NULL DEFAULT '',
    remark              TEXT NOT NULL DEFAULT '',
    comp_amount_milli   INTEGER NOT NULL DEFAULT 0,
    custom_params_json  TEXT NOT NULL DEFAULT '[]'
);
CREATE INDEX ix_program_tool_program ON program_tool (program_id);
CREATE UNIQUE INDEX ux_program_tool_version_seq ON program_tool (version_id, seq);

-- role 为后续权限预留：admin / editor / viewer。
CREATE TABLE app_user (
    id            INTEGER PRIMARY KEY,
    username      TEXT NOT NULL COLLATE NOCASE UNIQUE,
    display_name  TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'admin' CHECK (role IN ('admin', 'editor', 'viewer')),
    enabled       INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE TABLE user_session (
    id         INTEGER PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    user_id    INTEGER NOT NULL REFERENCES app_user (id) ON DELETE CASCADE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX ix_user_session_expires_at ON user_session (expires_at);

CREATE TABLE audit_log (
    id          INTEGER PRIMARY KEY,
    action      TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id   INTEGER NOT NULL,
    detail      TEXT NOT NULL DEFAULT '',
    actor       TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL
);
CREATE INDEX ix_audit_log_entity ON audit_log (entity_type, entity_id);
