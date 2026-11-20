CREATE TABLE IF NOT EXISTS ai_import_prompt_versions (
 id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT, rules LONGTEXT NOT NULL,
 actor_id BIGINT UNSIGNED NOT NULL DEFAULT 0, note VARCHAR(600) NOT NULL DEFAULT '', created_at DATETIME(3) NOT NULL
) ENGINE=InnoDB;
ALTER TABLE ai_import_tasks ADD COLUMN prompt_version_id BIGINT UNSIGNED NOT NULL DEFAULT 1;
ALTER TABLE ai_import_tasks ADD COLUMN recognition_outcome VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE ai_import_tasks ADD COLUMN result_version BIGINT UNSIGNED NOT NULL DEFAULT 0;
ALTER TABLE ai_import_tasks ADD COLUMN next_retry_at DATETIME(3) NULL;
ALTER TABLE ai_import_company_drafts ADD COLUMN generation_state VARCHAR(24) NOT NULL DEFAULT 'ready';
ALTER TABLE ai_import_product_drafts ADD COLUMN generation_state VARCHAR(24) NOT NULL DEFAULT 'ready';
CREATE TABLE IF NOT EXISTS ai_import_work_items (
 id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT, task_id BIGINT UNSIGNED NOT NULL,
 stage VARCHAR(128) NOT NULL, status VARCHAR(24) NOT NULL, prompt_version_id BIGINT UNSIGNED NOT NULL DEFAULT 1,
 fingerprint CHAR(64) NOT NULL DEFAULT '', attempts INT NOT NULL DEFAULT 0,
 error VARCHAR(120) NOT NULL DEFAULT '', error_field VARCHAR(300) NOT NULL DEFAULT '',
 next_retry_at DATETIME(3) NULL, updated_at DATETIME(3) NOT NULL,
 UNIQUE KEY uq_ai_work_stage(task_id,stage), INDEX idx_ai_work_status(task_id,status)
) ENGINE=InnoDB;
