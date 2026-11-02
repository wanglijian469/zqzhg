ALTER TABLE resources ADD COLUMN removed_at DATETIME(3) NULL;
ALTER TABLE resources ADD COLUMN video_sort_order INT NULL;
ALTER TABLE resource_revisions ADD COLUMN video_source VARCHAR(16) NOT NULL DEFAULT 'LOCAL';
ALTER TABLE resource_revisions ADD COLUMN external_video_id VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE resource_revisions ADD COLUMN rights_actor_id BIGINT UNSIGNED NOT NULL DEFAULT 0;
ALTER TABLE resource_revisions ADD COLUMN rights_confirmed_at DATETIME(3) NULL;
ALTER TABLE resource_revisions ADD COLUMN rights_version VARCHAR(32) NOT NULL DEFAULT '';
CREATE INDEX idx_resource_video_identity ON resource_revisions(video_source,external_video_id,resource_id);
CREATE TABLE IF NOT EXISTS video_link_checks (
 source VARCHAR(16) NOT NULL, video_id VARCHAR(64) NOT NULL,
 title VARCHAR(200) NOT NULL DEFAULT '', canonical_url VARCHAR(512) NOT NULL,
 player_url VARCHAR(1024) NOT NULL DEFAULT '', width INT NOT NULL DEFAULT 0, height INT NOT NULL DEFAULT 0,
 status VARCHAR(32) NOT NULL, reason VARCHAR(100) NOT NULL DEFAULT '',
 checked_at DATETIME(3) NOT NULL, expires_at DATETIME(3) NOT NULL,
 PRIMARY KEY(source,video_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS resource_cleanup_jobs (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, resource_id BIGINT UNSIGNED NOT NULL,
 owner_id BIGINT UNSIGNED NOT NULL, media_ids LONGTEXT NOT NULL,
 state VARCHAR(16) NOT NULL DEFAULT 'queued', attempts INT NOT NULL DEFAULT 0,
 last_error VARCHAR(100) NOT NULL DEFAULT '', next_attempt_at DATETIME(3) NOT NULL,
 created_at DATETIME(3) NOT NULL, updated_at DATETIME(3) NOT NULL,
 INDEX idx_resource_cleanup(state,next_attempt_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
