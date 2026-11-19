-- Exceptional destructive downgrade only, AFTER exporting metadata and verifying
-- there are no OSS-only business objects. Ordinary rollback keeps schema 039.
-- Explicit operator acknowledgement: SET @oss_metadata_export_verified = 1;
DELIMITER //
DROP PROCEDURE IF EXISTS guard_oss_039_downgrade//
CREATE PROCEDURE guard_oss_039_downgrade()
BEGIN
 IF COALESCE(@oss_metadata_export_verified,0) <> 1 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Export and verify OSS metadata before destructive downgrade';
 END IF;
 IF EXISTS(SELECT 1 FROM stored_objects o JOIN storage_profiles p ON p.id=o.store_id WHERE p.auth_mode='ecs_role')
 OR EXISTS(SELECT 1 FROM stored_objects WHERE remote_key<>'')
 OR EXISTS(SELECT 1 FROM media_upload_reservations WHERE hold_until_reclaimed=1)
 OR EXISTS(SELECT 1 FROM file_upload_sessions)
 OR EXISTS(SELECT 1 FROM file_backup_pins WHERE expires_at>UTC_TIMESTAMP())
 OR EXISTS(SELECT 1 FROM file_storage_jobs WHERE status IN ('processing','queued','failed')) THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='OSS objects, sessions, reservations or jobs still require schema 039';
 END IF;
END//
DELIMITER ;
CALL guard_oss_039_downgrade();
DROP PROCEDURE guard_oss_039_downgrade;
DELETE FROM system_settings WHERE setting_key IN ('oss_readiness_v1','oss_quota_300mib_v1');
DROP TABLE file_backup_pins;
DROP TABLE file_storage_job_items;
DROP TABLE file_storage_jobs;
DROP TABLE file_upload_sessions;
DROP TABLE enterprise_file_objects;
DROP TABLE enterprise_files;
DELETE FROM storage_profiles WHERE auth_mode='ecs_role';
ALTER TABLE stored_objects DROP COLUMN remote_key;
ALTER TABLE media_upload_reservations DROP COLUMN hold_until_reclaimed;
ALTER TABLE storage_profiles DROP COLUMN public_base_url, DROP COLUMN purpose, DROP COLUMN role_name, DROP COLUMN auth_mode;
DELETE FROM schema_migrations WHERE version=39;
