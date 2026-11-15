CREATE TABLE IF NOT EXISTS vendor_product_categories (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 vendor_id BIGINT UNSIGNED NOT NULL,
 hall_id BIGINT UNSIGNED NOT NULL,
 industry_category_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 name VARCHAR(300) NOT NULL,
 name_en VARCHAR(300) NOT NULL DEFAULT '',
 name_key CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 sort_order INT NOT NULL DEFAULT 0,
 enabled BOOLEAN NOT NULL DEFAULT TRUE,
 created_at DATETIME(3) NOT NULL,
 updated_at DATETIME(3) NOT NULL,
 UNIQUE KEY uq_vendor_product_category_name (vendor_id,industry_category_id,name_key),
 KEY idx_vendor_product_category_industry (industry_category_id,vendor_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS revision_vendor_product_categories (
 revision_id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
 category_id BIGINT UNSIGNED NOT NULL,
 vendor_id BIGINT UNSIGNED NOT NULL,
 KEY idx_revision_vendor_product_category (vendor_id,category_id,revision_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS vendor_product_category_sources (
 vendor_id BIGINT UNSIGNED NOT NULL,
 source_category_id BIGINT UNSIGNED NOT NULL,
 category_id BIGINT UNSIGNED NOT NULL,
 PRIMARY KEY(vendor_id,source_category_id),
 KEY idx_vendor_product_category_source_target(category_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
