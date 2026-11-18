-- The migration adds no business columns. Preserve published document versions and truthful acceptance records.
-- Program rollback restores the prior UI. Never delete history or pretend that a user accepted an older version.
DELETE FROM schema_migrations WHERE version=36;
