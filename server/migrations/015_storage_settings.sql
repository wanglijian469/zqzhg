ALTER TABLE storage_profiles ADD COLUMN root_key VARCHAR(100) NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS stored_object_replicas (
 object_key VARCHAR(255) NOT NULL, store_id BIGINT UNSIGNED NOT NULL,
 local_root VARCHAR(1000) NOT NULL DEFAULT '', size BIGINT NOT NULL, sha256 CHAR(64) NOT NULL,
 verified_at DATETIME(3) NOT NULL, PRIMARY KEY(object_key,store_id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS media_object_owners (
 object_key VARCHAR(255) PRIMARY KEY, owner_id BIGINT UNSIGNED NOT NULL,
 size BIGINT NOT NULL, INDEX idx_media_object_owner(owner_id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS merchant_storage_quotas (
 account_id BIGINT UNSIGNED PRIMARY KEY, quota_bytes BIGINT NULL, initialized BOOLEAN NOT NULL DEFAULT FALSE,
 updated_at DATETIME(3) NOT NULL
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS media_upload_reservations (
 id VARCHAR(64) PRIMARY KEY, owner_id BIGINT UNSIGNED NOT NULL,
 bytes BIGINT NOT NULL, expires_at DATETIME(3) NOT NULL,
 INDEX idx_media_reservation_owner(owner_id,expires_at)
) ENGINE=InnoDB;
