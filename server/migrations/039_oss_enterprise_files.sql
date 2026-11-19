ALTER TABLE storage_profiles ADD COLUMN auth_mode VARCHAR(32) NOT NULL DEFAULT 'static';
ALTER TABLE storage_profiles ADD COLUMN role_name VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE storage_profiles ADD COLUMN purpose VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE storage_profiles ADD COLUMN public_base_url VARCHAR(1000) NOT NULL DEFAULT '';
ALTER TABLE media_upload_reservations ADD COLUMN hold_until_reclaimed BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE stored_objects ADD COLUMN remote_key VARCHAR(255) NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS enterprise_files (
 media_id BIGINT UNSIGNED PRIMARY KEY, owner_id BIGINT UNSIGNED NOT NULL,
 enterprise_id BIGINT UNSIGNED NOT NULL DEFAULT 0, business_type VARCHAR(40) NOT NULL,
 business_id BIGINT UNSIGNED NOT NULL DEFAULT 0, original_filename VARCHAR(255) NOT NULL,
 mime_type VARCHAR(100) NOT NULL, visibility VARCHAR(16) NOT NULL DEFAULT 'private',
 file_status VARCHAR(24) NOT NULL, version BIGINT UNSIGNED NOT NULL DEFAULT 1,
 created_at DATETIME(3) NOT NULL, updated_at DATETIME(3) NOT NULL,
 INDEX idx_enterprise_file_owner(owner_id,media_id), INDEX idx_enterprise_file_vendor(enterprise_id,media_id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS enterprise_file_objects (
 object_key VARCHAR(255) PRIMARY KEY, media_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 owner_id BIGINT UNSIGNED NOT NULL, enterprise_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 store_id BIGINT UNSIGNED NOT NULL, bucket VARCHAR(100) NOT NULL DEFAULT '',
 source_key VARCHAR(255) NOT NULL DEFAULT '', physical_key VARCHAR(255) NOT NULL DEFAULT '', object_role VARCHAR(32) NOT NULL,
 business_type VARCHAR(40) NOT NULL, file_size BIGINT NOT NULL,
 sha256 CHAR(64) NOT NULL, object_state VARCHAR(24) NOT NULL,
 created_at DATETIME(3) NOT NULL, updated_at DATETIME(3) NOT NULL,
 INDEX idx_file_object_media(media_id), INDEX idx_file_object_owner(owner_id),
 INDEX idx_file_object_source(source_key,object_state)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS file_upload_sessions (
 id VARCHAR(64) PRIMARY KEY, owner_id BIGINT UNSIGNED NOT NULL, actor_id BIGINT UNSIGNED NOT NULL,
 enterprise_id BIGINT UNSIGNED NOT NULL DEFAULT 0, business_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 business_type VARCHAR(40) NOT NULL, business_scope VARCHAR(24) NOT NULL DEFAULT '', purpose VARCHAR(40) NOT NULL, original_filename VARCHAR(255) NOT NULL,
 mime_type VARCHAR(100) NOT NULL, expected_bytes BIGINT NOT NULL, store_id BIGINT UNSIGNED NOT NULL,
 object_key VARCHAR(255) NOT NULL, idempotency_key VARCHAR(100) NOT NULL, fingerprint CHAR(64) NOT NULL,
 upload_mode VARCHAR(16) NOT NULL, multipart_id VARCHAR(256) NOT NULL DEFAULT '',
 upload_state VARCHAR(24) NOT NULL, media_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 error VARCHAR(100) NOT NULL DEFAULT '', expires_at DATETIME(3) NOT NULL,
 signature_expires_at DATETIME(3) NOT NULL, lease_token VARCHAR(64) NOT NULL DEFAULT '',
 lease_until DATETIME(3) NULL, created_at DATETIME(3) NOT NULL, updated_at DATETIME(3) NOT NULL,
 UNIQUE KEY uq_file_upload_idempotency(owner_id,idempotency_key),
 INDEX idx_file_upload_worker(upload_state,lease_until), INDEX idx_file_upload_owner(owner_id,created_at)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS file_storage_jobs (
 id VARCHAR(64) PRIMARY KEY, job_kind VARCHAR(32) NOT NULL, owner_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 enterprise_id BIGINT UNSIGNED NOT NULL DEFAULT 0, resource_kind VARCHAR(24) NOT NULL DEFAULT '',
 resource_id BIGINT UNSIGNED NOT NULL DEFAULT 0, revision_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 expected_epoch BIGINT UNSIGNED NOT NULL DEFAULT 0, status VARCHAR(24) NOT NULL,
 checkpoint LONGTEXT NOT NULL, error VARCHAR(100) NOT NULL DEFAULT '',
 lease_token VARCHAR(64) NOT NULL DEFAULT '', lease_until DATETIME(3) NULL,
 created_at DATETIME(3) NOT NULL, updated_at DATETIME(3) NOT NULL,
 INDEX idx_file_storage_worker(status,lease_until), INDEX idx_file_storage_resource(resource_kind,resource_id,revision_id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS file_storage_job_items (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, job_id VARCHAR(64) NOT NULL,
 source_key VARCHAR(255) NOT NULL, target_key VARCHAR(255) NOT NULL,
 source_store_id BIGINT UNSIGNED NOT NULL, target_store_id BIGINT UNSIGNED NOT NULL,
 media_id BIGINT UNSIGNED NOT NULL DEFAULT 0, file_size BIGINT NOT NULL, sha256 CHAR(64) NOT NULL,
 status VARCHAR(24) NOT NULL, reservation_id VARCHAR(64) NOT NULL DEFAULT '', error VARCHAR(100) NOT NULL DEFAULT '', updated_at DATETIME(3) NOT NULL,
 UNIQUE KEY uq_file_transfer_item(job_id,source_key,target_key)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS file_backup_pins (
 snapshot_id VARCHAR(64) NOT NULL, object_key VARCHAR(255) NOT NULL,
 expires_at DATETIME(3) NOT NULL, PRIMARY KEY(snapshot_id,object_key), INDEX idx_file_pin_key(object_key,expires_at)
) ENGINE=InnoDB;
