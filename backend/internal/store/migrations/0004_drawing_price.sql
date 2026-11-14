-- 金额按人民币分保存；NULL 表示未填写，0 表示明确填写零元。
ALTER TABLE drawing ADD COLUMN price_cents INTEGER
    CHECK (price_cents IS NULL OR (typeof(price_cents) = 'integer' AND price_cents BETWEEN 0 AND 99999999999));
