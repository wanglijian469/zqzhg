CREATE TABLE IF NOT EXISTS operation_logs (
 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
 actor_id BIGINT UNSIGNED NOT NULL,
 target_kind VARCHAR(32) NOT NULL,
 target_id BIGINT UNSIGNED NOT NULL,
 action VARCHAR(40) NOT NULL,
 note VARCHAR(1000) NOT NULL DEFAULT '',
 source_review_log_id BIGINT UNSIGNED NULL,
 created_at DATETIME(3) NOT NULL,
 UNIQUE KEY uq_operation_review(source_review_log_id),
 INDEX idx_operation_time(created_at,id),
 INDEX idx_operation_actor(actor_id,created_at),
 INDEX idx_operation_target(target_kind,target_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
INSERT IGNORE INTO operation_logs(actor_id,target_kind,target_id,action,note,source_review_log_id,created_at)
 SELECT rl.actor_id,COALESCE(c.kind,'content'),rl.content_id,rl.action,rl.note,rl.id,rl.created_at
 FROM review_logs rl LEFT JOIN contents c ON c.id=rl.content_id;
