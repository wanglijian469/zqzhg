-- Disable AI and stop workers before explicit downgrade. Export task metadata first.
-- Encrypted administrator configuration and all business/draft data are retained.
ALTER TABLE ai_import_tasks DROP COLUMN review_model;
ALTER TABLE ai_import_tasks DROP COLUMN extraction_model;
ALTER TABLE ai_import_tasks DROP COLUMN max_confirmed_at;
ALTER TABLE ai_import_tasks DROP COLUMN model_mode;
DELETE FROM schema_migrations WHERE version=34;
