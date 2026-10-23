CREATE TABLE IF NOT EXISTS media_watermarks (
 media_id BIGINT UNSIGNED PRIMARY KEY,
 source_media_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 variant_key VARCHAR(80) NULL,
 purpose VARCHAR(32) NOT NULL DEFAULT 'unknown',
 state VARCHAR(16) NOT NULL DEFAULT 'pending',
 config_json TEXT NOT NULL,
 config_version BIGINT UNSIGNED NOT NULL,
 vendor_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 vendor_text VARCHAR(200) NOT NULL DEFAULT '',
 created_at DATETIME(3) NOT NULL,
 UNIQUE KEY uq_media_watermark_variant(variant_key),
 INDEX idx_watermark_source(source_media_id),
 INDEX idx_watermark_vendor(vendor_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
