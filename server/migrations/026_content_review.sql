INSERT INTO system_settings(setting_key,value,updated_at)
VALUES ('company_verification_mode','content_review',UTC_TIMESTAMP(3))
ON DUPLICATE KEY UPDATE value=VALUES(value);
