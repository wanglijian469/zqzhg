INSERT INTO system_settings(setting_key,value,updated_at)
VALUES ('content_review_privacy_notice','accounts_and_content_review',UTC_TIMESTAMP(3))
ON DUPLICATE KEY UPDATE value=VALUES(value);
