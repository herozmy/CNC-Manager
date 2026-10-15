-- 刀具补偿表必须跟随程序版本，而不是整个程序共用一份。
-- 历史数据归到迁移时的当前版本；没有版本的程序不可能展示版本刀具，因此不迁移其孤立行。
CREATE TABLE program_tool_v2 (
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

INSERT INTO program_tool_v2 (
    id, program_id, version_id, seq, tool_id, tool_no, offset_no, tool_name,
    tool_dia_milli, corner_radius_milli, spindle_speed, speed_mode,
    feed_milli, feed_mode, cut_depth_milli, coolant, machining_content,
    remark, comp_amount_milli, custom_params_json
)
SELECT pt.id, pt.program_id, p.current_version_id, pt.seq, pt.tool_id,
       pt.tool_no, pt.offset_no, pt.tool_name, pt.tool_dia_milli,
       pt.corner_radius_milli, pt.spindle_speed, pt.speed_mode,
       pt.feed_milli, pt.feed_mode, pt.cut_depth_milli, pt.coolant,
       pt.machining_content, pt.remark, pt.comp_amount_milli,
       pt.custom_params_json
FROM program_tool pt
JOIN nc_program p ON p.id = pt.program_id
WHERE p.current_version_id IS NOT NULL;

DROP TABLE program_tool;
ALTER TABLE program_tool_v2 RENAME TO program_tool;
CREATE INDEX ix_program_tool_program ON program_tool (program_id);
CREATE INDEX ix_program_tool_version ON program_tool (version_id);
CREATE UNIQUE INDEX ux_program_tool_version_seq ON program_tool (version_id, seq);
