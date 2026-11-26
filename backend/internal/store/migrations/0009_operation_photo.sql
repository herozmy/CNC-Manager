CREATE TABLE operation_photo (
    operation_id INTEGER NOT NULL REFERENCES operation(id) ON DELETE CASCADE,
    digest TEXT NOT NULL,
    jpeg BLOB NOT NULL CHECK (length(jpeg) BETWEEN 1 AND 4194304),
    PRIMARY KEY (operation_id, digest)
);
