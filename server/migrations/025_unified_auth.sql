CREATE TABLE IF NOT EXISTS phone_action_proofs (
 token_hash CHAR(64) PRIMARY KEY, phone VARCHAR(11) NOT NULL, purpose VARCHAR(24) NOT NULL,
 expires_at DATETIME(3) NOT NULL, used TINYINT(1) NOT NULL DEFAULT 0, created_at DATETIME(3) NOT NULL,
 INDEX idx_phone_action_expiry(expires_at), INDEX idx_phone_action_subject(phone,purpose)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS account_welcomes (
 account_id BIGINT UNSIGNED PRIMARY KEY, pending TINYINT(1) NOT NULL DEFAULT 1,
 choice VARCHAR(24) NOT NULL DEFAULT '', updated_at DATETIME(3) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
