ALTER TABLE ai_import_product_drafts ADD COLUMN target_content_id BIGINT UNSIGNED NOT NULL DEFAULT 0;
ALTER TABLE ai_import_product_drafts ADD COLUMN publish_mode VARCHAR(16) NOT NULL DEFAULT 'create';
