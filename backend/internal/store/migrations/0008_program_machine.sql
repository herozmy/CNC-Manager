ALTER TABLE nc_program ADD COLUMN machine_id INTEGER REFERENCES machine(id) ON DELETE SET NULL;
UPDATE nc_program SET machine_id=(SELECT o.machine_id FROM operation o WHERE o.id=nc_program.operation_id);
CREATE INDEX ix_nc_program_machine ON nc_program(machine_id,operation_id);

-- Keep deleted machine IDs reserved so an old form cannot select a different
-- machine subsequently created with the same SQLite rowid.
CREATE TABLE machine_id_sequence (singleton INTEGER PRIMARY KEY CHECK(singleton=1), last_id INTEGER NOT NULL);
INSERT INTO machine_id_sequence SELECT 1,COALESCE(MAX(id),0) FROM machine;
CREATE TRIGGER machine_id_advance AFTER INSERT ON machine BEGIN
 UPDATE machine_id_sequence SET last_id=MAX(last_id,NEW.id) WHERE singleton=1;
END;
