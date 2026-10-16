CREATE TABLE IF NOT EXISTS category_suggestions (
 category_id BIGINT UNSIGNED PRIMARY KEY, owner_id BIGINT UNSIGNED NOT NULL,
 status VARCHAR(16) NOT NULL DEFAULT 'pending', created_at DATETIME(3) NOT NULL,
 updated_at DATETIME(3) NOT NULL, INDEX idx_category_suggestion_owner(owner_id,status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS category_aliases (
 source_id BIGINT UNSIGNED PRIMARY KEY, target_id BIGINT UNSIGNED NOT NULL,
 created_at DATETIME(3) NOT NULL, INDEX idx_category_alias_target(target_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
