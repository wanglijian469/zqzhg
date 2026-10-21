ALTER TABLE categories ADD COLUMN category_kind VARCHAR(16) NOT NULL DEFAULT 'legacy';
ALTER TABLE categories ADD COLUMN code VARCHAR(100) NULL;
ALTER TABLE categories ADD COLUMN image_media_id BIGINT UNSIGNED NOT NULL DEFAULT 0;
ALTER TABLE categories ADD COLUMN search_aliases TEXT NULL;
ALTER TABLE categories ADD COLUMN deleted_at DATETIME(3) NULL;
CREATE UNIQUE INDEX idx_category_code ON categories(code);
CREATE INDEX idx_category_kind ON categories(hall_id,category_kind,parent_id,enabled);
ALTER TABLE contents ADD COLUMN taxonomy_required TINYINT(1) NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS revision_categories (
 revision_id BIGINT UNSIGNED NOT NULL, category_id BIGINT UNSIGNED NOT NULL,
 role VARCHAR(16) NOT NULL, source VARCHAR(16) NOT NULL DEFAULT 'explicit',
 PRIMARY KEY(revision_id,category_id), INDEX idx_revision_category_v2(category_id,revision_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS taxonomy_mappings (
 revision_id BIGINT UNSIGNED PRIMARY KEY, content_id BIGINT UNSIGNED NOT NULL,
 category_kind VARCHAR(16) NOT NULL, legacy_category_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 target_id BIGINT UNSIGNED NOT NULL DEFAULT 0, status VARCHAR(16) NOT NULL DEFAULT 'pending',
 updated_at DATETIME(3) NOT NULL, INDEX idx_mapping_status(status,content_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS industrial_belts (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, hall_id BIGINT UNSIGNED NOT NULL DEFAULT 1,
 name VARCHAR(300) NOT NULL, name_en VARCHAR(300) NOT NULL DEFAULT '',
 sort_order INT NOT NULL DEFAULT 0, enabled TINYINT(1) NOT NULL DEFAULT 1
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS vendor_industrial_belts (
 vendor_id BIGINT UNSIGNED NOT NULL, belt_id BIGINT UNSIGNED NOT NULL,
 PRIMARY KEY(vendor_id,belt_id), INDEX idx_belt_vendor(belt_id,vendor_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
