-- Complete system-setting before/after snapshots exceed the original 1000 characters.
-- MODIFY is idempotent and retains historical notes without truncation.
ALTER TABLE operation_logs MODIFY note TEXT NOT NULL;
