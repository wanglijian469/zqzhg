-- Normal application rollback keeps TEXT, which remains compatible with old code.
-- Export and review notes first. Strict mode refuses shrinking if any note exceeds
-- 1000 characters, preserving data rather than silently truncating audit history.
SET SESSION sql_mode = CONCAT_WS(',', @@SESSION.sql_mode, 'STRICT_ALL_TABLES');
ALTER TABLE operation_logs MODIFY note VARCHAR(1000) NOT NULL DEFAULT '';
DELETE FROM schema_migrations WHERE version = 38;
