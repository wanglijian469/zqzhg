-- Export submission batches first. Close AI_IMPORT_ENABLED and stop its worker.
-- This optional destructive downgrade only removes new submission metadata, not business content/media/drafts.
DROP TABLE IF EXISTS ai_import_submission_items;
DROP TABLE IF EXISTS ai_import_submission_batches;
ALTER TABLE ai_import_product_drafts DROP COLUMN active_batch_id;
ALTER TABLE ai_import_company_drafts DROP COLUMN active_batch_id;
ALTER TABLE ai_import_tasks DROP COLUMN submission_mode;
DELETE FROM schema_migrations WHERE version=35;
