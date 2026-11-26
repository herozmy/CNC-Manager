CREATE TABLE photo_annotation (
    operation_id INTEGER NOT NULL,
    digest TEXT NOT NULL,
    revision INTEGER NOT NULL CHECK(revision > 0),
    marks TEXT NOT NULL CHECK(length(marks) <= 65536 AND json_valid(marks) AND json_type(marks) = 'array'),
    PRIMARY KEY(operation_id, digest),
    FOREIGN KEY(operation_id, digest) REFERENCES operation_photo(operation_id, digest) ON DELETE CASCADE
);
