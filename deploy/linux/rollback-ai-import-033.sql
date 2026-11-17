-- Disable AI_IMPORT_ENABLED and export drafts before explicit rollback.
-- Business products, revisions and media are retained.
ALTER TABLE ai_import_product_drafts DROP COLUMN publish_mode;
ALTER TABLE ai_import_product_drafts DROP COLUMN target_content_id;
DELETE FROM schema_migrations WHERE version=33;
