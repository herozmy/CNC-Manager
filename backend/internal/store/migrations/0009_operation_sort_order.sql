ALTER TABLE operation ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;
UPDATE operation SET sort_order = op_no WHERE sort_order = 0;
CREATE INDEX ix_operation_drawing_sort ON operation (drawing_id, sort_order, id);
