.bail on

BEGIN;

ALTER TABLE settings ADD COLUMN partial TEXT NOT NULL DEFAULT '';
SELECT no_such_function();

COMMIT;
