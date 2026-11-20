-- Exceptional metadata downgrade only. Normal program rollback keeps Schema40.
-- Stop AI workers and export tasks, drafts, prompt versions and work items first.
-- Explicit acknowledgement: SET @ai_metadata_export_verified = 1;
DELIMITER //
DROP PROCEDURE IF EXISTS guard_ai_040_downgrade//
CREATE PROCEDURE guard_ai_040_downgrade()
BEGIN
 IF COALESCE(@ai_metadata_export_verified,0) <> 1 THEN
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='Export AI metadata and stop workers before destructive downgrade';
 END IF;
END//
DELIMITER ;
CALL guard_ai_040_downgrade();
DROP PROCEDURE guard_ai_040_downgrade;
DELETE FROM system_settings WHERE setting_key='ai_import_prompt_active_v1';
DROP TABLE IF EXISTS ai_import_work_items;
DROP TABLE IF EXISTS ai_import_prompt_versions;
ALTER TABLE ai_import_product_drafts DROP COLUMN generation_state;
ALTER TABLE ai_import_company_drafts DROP COLUMN generation_state;
ALTER TABLE ai_import_tasks DROP COLUMN next_retry_at;
ALTER TABLE ai_import_tasks DROP COLUMN result_version;
ALTER TABLE ai_import_tasks DROP COLUMN recognition_outcome;
ALTER TABLE ai_import_tasks DROP COLUMN prompt_version_id;
DELETE FROM schema_migrations WHERE version=40;
