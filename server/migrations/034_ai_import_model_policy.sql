ALTER TABLE ai_import_tasks ADD COLUMN model_mode VARCHAR(16) NOT NULL DEFAULT 'flash';
ALTER TABLE ai_import_tasks ADD COLUMN max_confirmed_at DATETIME(3) NULL;
ALTER TABLE ai_import_tasks ADD COLUMN extraction_model VARCHAR(100) NOT NULL DEFAULT '';
ALTER TABLE ai_import_tasks ADD COLUMN review_model VARCHAR(100) NOT NULL DEFAULT '';
