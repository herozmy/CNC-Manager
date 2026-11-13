-- 程序列表支持人工拖动排序。历史数据先按主键顺序稳定排列。
ALTER TABLE nc_program ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;
UPDATE nc_program SET sort_order = id WHERE sort_order = 0;
CREATE INDEX ix_nc_program_operation_sort ON nc_program (operation_id, sort_order, id);
