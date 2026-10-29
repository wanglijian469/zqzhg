ALTER TABLE phone_verifications ADD COLUMN send_id CHAR(64) NOT NULL DEFAULT '';
CREATE INDEX idx_phone_verification_send ON phone_verifications(send_id);
CREATE TABLE IF NOT EXISTS sms_send_attempts (
 id CHAR(64) PRIMARY KEY, request_key CHAR(64) NOT NULL,
 phone_key CHAR(64) NOT NULL, ip_key CHAR(64) NOT NULL, purpose VARCHAR(32) NOT NULL,
 account_id BIGINT UNSIGNED NOT NULL DEFAULT 0, state VARCHAR(16) NOT NULL,
 provider_code VARCHAR(80) NOT NULL DEFAULT '', provider_message VARCHAR(255) NOT NULL DEFAULT '',
 request_id VARCHAR(128) NOT NULL DEFAULT '', biz_id VARCHAR(128) NOT NULL DEFAULT '',
 created_at DATETIME(3) NOT NULL, updated_at DATETIME(3) NOT NULL,
 UNIQUE KEY uq_sms_request(request_key), KEY idx_sms_phone_time(phone_key,created_at), KEY idx_sms_ip_time(ip_key,created_at), KEY idx_sms_created(created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS sms_guards (
 id VARCHAR(96) PRIMARY KEY, updated_at DATETIME(3) NOT NULL, KEY idx_sms_guard_updated(updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS sms_challenges (
 id CHAR(64) PRIMARY KEY, phone_key CHAR(64) NOT NULL, ip_key CHAR(64) NOT NULL,
 purpose VARCHAR(32) NOT NULL, digest CHAR(64) NOT NULL, used TINYINT(1) NOT NULL DEFAULT 0,
 created_at DATETIME(3) NOT NULL, expires_at DATETIME(3) NOT NULL,
 KEY idx_sms_challenge_ip(ip_key,created_at), KEY idx_sms_challenge_created(created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
UPDATE phone_verifications SET used=1;
UPDATE sms_codes SET used=1;
