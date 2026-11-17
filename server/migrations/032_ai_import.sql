CREATE TABLE IF NOT EXISTS ai_import_tasks (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 owner_id BIGINT UNSIGNED NOT NULL, vendor_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 request_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 file_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 file_name VARCHAR(500) NOT NULL, source_ids LONGTEXT NOT NULL,
 status VARCHAR(32) NOT NULL, stage VARCHAR(64) NOT NULL DEFAULT '',
 progress INT NOT NULL DEFAULT 0, total INT NOT NULL DEFAULT 0,
 checkpoint LONGTEXT NOT NULL, error VARCHAR(120) NOT NULL DEFAULT '',
 lease VARCHAR(64) NOT NULL DEFAULT '', locked_until DATETIME(3) NULL,
 publish_key VARCHAR(128) NOT NULL DEFAULT '', publish_manifest LONGTEXT NOT NULL,
 created_at DATETIME(3) NOT NULL, updated_at DATETIME(3) NOT NULL,
 UNIQUE KEY uq_ai_request(owner_id,request_id), KEY idx_ai_owner_hash(owner_id,file_hash),
 KEY idx_ai_queue(status,locked_until,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS ai_import_company_drafts (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY, task_id BIGINT UNSIGNED NOT NULL,
 owner_id BIGINT UNSIGNED NOT NULL, temp_id VARCHAR(128) NOT NULL,
 version BIGINT UNSIGNED NOT NULL DEFAULT 1, reviewed_version BIGINT UNSIGNED NOT NULL DEFAULT 0,
 data LONGTEXT NOT NULL, source_pages TEXT NOT NULL, field_sources LONGTEXT NOT NULL,
 review_issues LONGTEXT NOT NULL, selected_fields TEXT NOT NULL,
 base_revision_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 content_id BIGINT UNSIGNED NOT NULL DEFAULT 0, revision_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 deleted BOOLEAN NOT NULL DEFAULT FALSE, error VARCHAR(120) NOT NULL DEFAULT '',
 created_at DATETIME(3) NOT NULL, updated_at DATETIME(3) NOT NULL,
 UNIQUE KEY uq_ai_company(task_id,temp_id), KEY idx_ai_company_owner(owner_id,task_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS ai_import_product_drafts LIKE ai_import_company_drafts;
CREATE TABLE IF NOT EXISTS ai_import_assets (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY, task_id BIGINT UNSIGNED NOT NULL,
 owner_id BIGINT UNSIGNED NOT NULL, media_id BIGINT UNSIGNED NOT NULL,
 source_asset_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 page INT NOT NULL DEFAULT 0, panel VARCHAR(12) NOT NULL DEFAULT '', printed_page VARCHAR(32) NOT NULL DEFAULT '',
 image_type VARCHAR(32) NOT NULL, crop_region TEXT NOT NULL,
 width INT NOT NULL DEFAULT 0, height INT NOT NULL DEFAULT 0,
 sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 created_at DATETIME(3) NOT NULL,
 KEY idx_ai_asset_task(task_id,owner_id), KEY idx_ai_asset_media(media_id), KEY idx_ai_asset_source(source_asset_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS ai_import_review_items (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY, task_id BIGINT UNSIGNED NOT NULL,
 owner_id BIGINT UNSIGNED NOT NULL, draft_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 draft_kind VARCHAR(12) NOT NULL, draft_version BIGINT UNSIGNED NOT NULL,
 issue TEXT NOT NULL, resolution TEXT NOT NULL, actor_id BIGINT UNSIGNED NOT NULL,
 created_at DATETIME(3) NOT NULL, KEY idx_ai_review(task_id,draft_kind,draft_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS ai_import_model_calls (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY, task_id BIGINT UNSIGNED NOT NULL,
 stage VARCHAR(128) NOT NULL, fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 model VARCHAR(100) NOT NULL, request_id VARCHAR(200) NOT NULL, attempt INT NOT NULL,
 http_status INT NOT NULL, error VARCHAR(120) NOT NULL, usage_json TEXT NOT NULL,
 estimated_cost DECIMAL(20,8) NULL, price_basis TEXT NOT NULL, response_json LONGTEXT NOT NULL,
 duration_ms BIGINT NOT NULL, created_at DATETIME(3) NOT NULL,
 KEY idx_ai_call_cache(task_id,stage,fingerprint), KEY idx_ai_call_task(task_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
