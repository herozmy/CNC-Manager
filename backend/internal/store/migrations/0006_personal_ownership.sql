-- Legacy shared data stays administrator-only until explicitly assigned.
-- Keep legacy role columns and snapshots for lossless upgrades and backups.
ALTER TABLE drawing ADD COLUMN owner_user_id INTEGER REFERENCES app_user(id);
ALTER TABLE machine ADD COLUMN owner_user_id INTEGER REFERENCES app_user(id);
ALTER TABLE tool ADD COLUMN owner_user_id INTEGER REFERENCES app_user(id);
DROP INDEX ux_drawing_no;
DROP INDEX ux_machine_code;
DROP INDEX ux_tool_no;
CREATE UNIQUE INDEX ux_drawing_no ON drawing(owner_user_id,drawing_no) WHERE owner_user_id IS NOT NULL;
CREATE UNIQUE INDEX ux_machine_code ON machine(owner_user_id,code) WHERE owner_user_id IS NOT NULL;
CREATE UNIQUE INDEX ux_tool_no ON tool(owner_user_id,tool_no) WHERE owner_user_id IS NOT NULL;
CREATE UNIQUE INDEX ux_unassigned_drawing_no ON drawing(role_id,drawing_no) WHERE owner_user_id IS NULL;
CREATE UNIQUE INDEX ux_unassigned_machine_code ON machine(role_id,code) WHERE owner_user_id IS NULL;
CREATE UNIQUE INDEX ux_unassigned_tool_no ON tool(role_id,tool_no) WHERE owner_user_id IS NULL;
ALTER TABLE audit_log ADD COLUMN owner_user_id INTEGER;
ALTER TABLE audit_log ADD COLUMN owner_name TEXT NOT NULL DEFAULT '';
UPDATE app_user SET role='editor' WHERE role='viewer';
UPDATE app_user SET business_role_id=NULL;
DELETE FROM user_session;
