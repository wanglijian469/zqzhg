ALTER TABLE ai_import_tasks ADD COLUMN submission_mode VARCHAR(16) NOT NULL DEFAULT 'legacy';
ALTER TABLE ai_import_company_drafts ADD COLUMN active_batch_id BIGINT UNSIGNED NOT NULL DEFAULT 0;
ALTER TABLE ai_import_product_drafts ADD COLUMN active_batch_id BIGINT UNSIGNED NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS ai_import_submission_batches (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 task_id BIGINT UNSIGNED NOT NULL, owner_id BIGINT UNSIGNED NOT NULL,
 kind VARCHAR(16) NOT NULL, idempotency_key VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL, request_json LONGTEXT NOT NULL,
 status VARCHAR(16) NOT NULL, created_at DATETIME(3) NOT NULL, updated_at DATETIME(3) NOT NULL,
 UNIQUE KEY uq_ai_batch_key(owner_id,task_id,idempotency_key), KEY idx_ai_batch_task(task_id,owner_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS ai_import_submission_items (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 batch_id BIGINT UNSIGNED NOT NULL, task_id BIGINT UNSIGNED NOT NULL, owner_id BIGINT UNSIGNED NOT NULL,
 kind VARCHAR(16) NOT NULL, draft_id BIGINT UNSIGNED NOT NULL, version BIGINT UNSIGNED NOT NULL,
 request_json LONGTEXT NOT NULL, status VARCHAR(16) NOT NULL, error VARCHAR(120) NOT NULL DEFAULT '',
 fields_json TEXT NOT NULL, content_id BIGINT UNSIGNED NOT NULL DEFAULT 0, revision_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 created_at DATETIME(3) NOT NULL, updated_at DATETIME(3) NOT NULL,
 UNIQUE KEY uq_ai_batch_item(batch_id,kind,draft_id), KEY idx_ai_submission_draft(task_id,kind,draft_id),
 KEY idx_ai_submission_batch(batch_id,owner_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
