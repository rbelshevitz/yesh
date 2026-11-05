.bail on

BEGIN;

CREATE TABLE schema_migrations (
    version TEXT PRIMARY KEY
);

ALTER TABLE settings ADD COLUMN description TEXT NOT NULL DEFAULT '';

INSERT INTO schema_migrations(version) VALUES ('001-add-description');

COMMIT;
