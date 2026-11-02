CREATE TABLE IF NOT EXISTS content_create_requests (
 owner_id BIGINT UNSIGNED NOT NULL,
 request_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
 content_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 created_at DATETIME(3) NOT NULL,
 PRIMARY KEY(owner_id,request_id),
 INDEX idx_content_create_request_content(content_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
