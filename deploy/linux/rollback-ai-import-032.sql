-- OFFLINE ONLY. Export and retain all AI tables and disable AI_IMPORT_ENABLED first.
-- This does not remove companies, products, media or revision payloads.
-- Run explicitly after verifying no AI worker is running and the export is readable.
DROP TABLE IF EXISTS ai_import_model_calls;
DROP TABLE IF EXISTS ai_import_review_items;
DROP TABLE IF EXISTS ai_import_assets;
DROP TABLE IF EXISTS ai_import_product_drafts;
DROP TABLE IF EXISTS ai_import_company_drafts;
DROP TABLE IF EXISTS ai_import_tasks;
DELETE FROM schema_migrations WHERE version IN (32,33,34);
