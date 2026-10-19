-- 每台机床的当前刀库。刀位属于机床，同一台机床内不可重复。
CREATE TABLE machine_tool (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    machine_id  INTEGER NOT NULL REFERENCES machine (id) ON DELETE CASCADE,
    pocket_no   INTEGER NOT NULL CHECK (pocket_no > 0),
    tool_no     TEXT NOT NULL DEFAULT '',
    tool_name   TEXT NOT NULL DEFAULT '',
    spec        TEXT NOT NULL DEFAULT '',
    remark      TEXT NOT NULL DEFAULT '',
    updated_at  TEXT NOT NULL
);

CREATE UNIQUE INDEX ux_machine_tool_pocket ON machine_tool (machine_id, pocket_no);
CREATE INDEX ix_machine_tool_machine ON machine_tool (machine_id);
