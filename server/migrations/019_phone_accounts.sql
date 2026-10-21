ALTER TABLE accounts ADD COLUMN phone VARCHAR(11) NULL;
ALTER TABLE accounts ADD COLUMN phone_verified_at DATETIME(3) NULL;
CREATE UNIQUE INDEX uq_accounts_verified_phone ON accounts(phone);
CREATE TABLE IF NOT EXISTS phone_identity_locks (
 phone VARCHAR(11) PRIMARY KEY
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS phone_verifications (
 phone VARCHAR(11) NOT NULL, purpose VARCHAR(32) NOT NULL, account_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 digest CHAR(64) NOT NULL DEFAULT '', salt VARCHAR(64) NOT NULL DEFAULT '',
 expires_at DATETIME(3) NOT NULL, sent_at DATETIME(3) NOT NULL,
 attempts INT NOT NULL DEFAULT 0, used TINYINT(1) NOT NULL DEFAULT 0,
 PRIMARY KEY(phone,purpose,account_id), INDEX idx_phone_verification_expiry(expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
