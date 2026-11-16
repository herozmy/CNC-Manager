ALTER TABLE machine ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0,1));
CREATE INDEX ix_operation_machine ON operation(machine_id, drawing_id);
