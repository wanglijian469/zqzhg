CREATE TABLE IF NOT EXISTS showroom_addresses (
 slug VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
 vendor_id BIGINT UNSIGNED NOT NULL,
 created_at DATETIME(3) NOT NULL,
 INDEX idx_showroom_address_vendor(vendor_id)
) ENGINE=InnoDB;
INSERT IGNORE INTO showroom_addresses(slug,vendor_id,created_at) SELECT showroom_slug,id,UTC_TIMESTAMP(3) FROM contents WHERE kind='vendor' AND showroom_slug IS NOT NULL;
