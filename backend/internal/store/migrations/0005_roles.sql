-- Existing business rows retain an explicit, disabled holding role until an
-- administrator assigns them. Account identity remains admin/editor/viewer.
CREATE TABLE business_role (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL COLLATE NOCASE UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0,1)),
    permissions_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(permissions_json) AND json_type(permissions_json) = 'array'),
    is_system INTEGER NOT NULL DEFAULT 0 CHECK (is_system IN (0,1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
INSERT INTO business_role (id,name,description,enabled,permissions_json,is_system,created_at,updated_at)
VALUES (1,'待分配','升级前的数据，等待管理员分配到业务角色',0,'[]',1,'','');
ALTER TABLE app_user ADD COLUMN business_role_id INTEGER REFERENCES business_role (id);
ALTER TABLE drawing ADD COLUMN role_id INTEGER NOT NULL DEFAULT 1 REFERENCES business_role (id);
ALTER TABLE machine ADD COLUMN role_id INTEGER NOT NULL DEFAULT 1 REFERENCES business_role (id);
ALTER TABLE tool ADD COLUMN role_id INTEGER NOT NULL DEFAULT 1 REFERENCES business_role (id);
DROP INDEX ux_drawing_no;
DROP INDEX ux_machine_code;
DROP INDEX ux_tool_no;
CREATE UNIQUE INDEX ux_drawing_no ON drawing (role_id,drawing_no);
CREATE UNIQUE INDEX ux_machine_code ON machine (role_id,code);
CREATE UNIQUE INDEX ux_tool_no ON tool (role_id,tool_no);
CREATE INDEX ix_app_user_business_role ON app_user (business_role_id);
ALTER TABLE nc_program ADD COLUMN revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0);
ALTER TABLE audit_log ADD COLUMN actor_id INTEGER;
ALTER TABLE audit_log ADD COLUMN actor_role_id INTEGER;
ALTER TABLE audit_log ADD COLUMN actor_role_name TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_log ADD COLUMN role_id INTEGER;
ALTER TABLE audit_log ADD COLUMN role_name TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_log ADD COLUMN entity_name TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_log ADD COLUMN result TEXT NOT NULL DEFAULT 'success';
CREATE INDEX ix_audit_log_created_at ON audit_log (created_at,id);
CREATE INDEX ix_audit_log_role ON audit_log (role_id,created_at,id);
CREATE INDEX ix_audit_log_actor ON audit_log (actor_id,created_at,id);

-- Permission and identity changes must not preserve pre-upgrade sessions.
DELETE FROM user_session;
