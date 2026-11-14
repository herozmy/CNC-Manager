-- 工序分中方式；已有工序保持空值，按需补填。
ALTER TABLE operation ADD COLUMN centering_method TEXT NOT NULL DEFAULT '';
