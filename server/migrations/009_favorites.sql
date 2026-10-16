CREATE TABLE IF NOT EXISTS favorites (
 account_id BIGINT UNSIGNED NOT NULL,
 kind VARCHAR(16) NOT NULL,
 content_id BIGINT UNSIGNED NOT NULL,
 active TINYINT(1) NOT NULL DEFAULT 1,
 collected_at DATETIME(3) NOT NULL,
 updated_at DATETIME(3) NOT NULL,
 PRIMARY KEY(account_id,kind,content_id),
 INDEX idx_favorite_list(account_id,active,collected_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS favorite_operations (
 account_id BIGINT UNSIGNED NOT NULL,
 operation_id VARCHAR(80) NOT NULL,
 fingerprint CHAR(64) NOT NULL,
 created_at DATETIME(3) NOT NULL,
 PRIMARY KEY(account_id,operation_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
