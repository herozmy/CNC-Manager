-- 工序增加 X/Y 坐标，单位沿用 0.001 mm；旧 Z 值保留在原列中。
ALTER TABLE operation ADD COLUMN x_coordinate_milli INTEGER NOT NULL DEFAULT 0;
ALTER TABLE operation ADD COLUMN y_coordinate_milli INTEGER NOT NULL DEFAULT 0;
