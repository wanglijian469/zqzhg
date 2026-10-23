CREATE TABLE IF NOT EXISTS buyer_profiles (
 account_id BIGINT UNSIGNED PRIMARY KEY, company VARCHAR(200) NOT NULL DEFAULT '', contact_name VARCHAR(100) NOT NULL,
 region_code VARCHAR(6) NOT NULL DEFAULT '', suspended TINYINT(1) NOT NULL DEFAULT 0,
 version BIGINT UNSIGNED NOT NULL DEFAULT 1, updated_at DATETIME(3) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS procurement_projects (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, buyer_id BIGINT UNSIGNED NOT NULL, hall_id BIGINT UNSIGNED NOT NULL,
 source_product_id BIGINT UNSIGNED NOT NULL DEFAULT 0, source_vendor_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 state VARCHAR(16) NOT NULL DEFAULT 'draft', audience VARCHAR(16) NOT NULL DEFAULT 'direct',
 draft_revision_id BIGINT UNSIGNED NOT NULL DEFAULT 0, published_revision_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 version BIGINT UNSIGNED NOT NULL DEFAULT 1, suspended TINYINT(1) NOT NULL DEFAULT 0,
 created_at DATETIME(3) NOT NULL, updated_at DATETIME(3) NOT NULL,
 INDEX idx_proc_buyer(buyer_id,updated_at), INDEX idx_proc_public(hall_id,state,audience,published_revision_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS procurement_revisions (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, project_id BIGINT UNSIGNED NOT NULL, status VARCHAR(16) NOT NULL DEFAULT 'draft',
 requested_audience VARCHAR(16) NOT NULL DEFAULT 'public', vendor_ids TEXT NOT NULL,
 title VARCHAR(200) NOT NULL, delivery_region VARCHAR(6) NOT NULL DEFAULT '', expected_delivery VARCHAR(100) NOT NULL DEFAULT '',
 deadline DATETIME(3) NOT NULL, notes TEXT NOT NULL, contact_name VARCHAR(100) NOT NULL, phone VARCHAR(32) NOT NULL,
 created_at DATETIME(3) NOT NULL, INDEX idx_proc_revision(project_id,status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS procurement_items (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, revision_id BIGINT UNSIGNED NOT NULL, category_id BIGINT UNSIGNED NOT NULL,
 name VARCHAR(200) NOT NULL, specification TEXT NOT NULL, quantity DECIMAL(18,3) NOT NULL,
 unit VARCHAR(30) NOT NULL, INDEX idx_proc_item(revision_id,category_id), INDEX idx_proc_category(category_id,revision_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS procurement_reviews (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, project_id BIGINT UNSIGNED NOT NULL, revision_id BIGINT UNSIGNED NOT NULL,
 action VARCHAR(16) NOT NULL, audience VARCHAR(16) NOT NULL, vendor_ids TEXT NOT NULL,
 status VARCHAR(16) NOT NULL DEFAULT 'pending', reviewer_id BIGINT UNSIGNED NOT NULL DEFAULT 0, reason VARCHAR(1000) NOT NULL DEFAULT '',
 created_at DATETIME(3) NOT NULL, reviewed_at DATETIME(3) NULL, INDEX idx_proc_review(status,created_at), INDEX idx_proc_review_project(project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS procurement_participants (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, project_id BIGINT UNSIGNED NOT NULL, vendor_id BIGINT UNSIGNED NOT NULL,
 source VARCHAR(16) NOT NULL, state VARCHAR(16) NOT NULL DEFAULT 'invited', intent TINYINT(1) NOT NULL DEFAULT 0,
 match_reason VARCHAR(1000) NOT NULL DEFAULT '', created_at DATETIME(3) NOT NULL, updated_at DATETIME(3) NOT NULL,
 UNIQUE KEY uq_proc_participant(project_id,vendor_id), INDEX idx_proc_vendor(vendor_id,updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS procurement_quotes (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, project_id BIGINT UNSIGNED NOT NULL, vendor_id BIGINT UNSIGNED NOT NULL,
 submitted_revision_id BIGINT UNSIGNED NOT NULL DEFAULT 0, draft_revision_id BIGINT UNSIGNED NOT NULL DEFAULT 0, version BIGINT UNSIGNED NOT NULL DEFAULT 1,
 withdrawn TINYINT(1) NOT NULL DEFAULT 0, excluded TINYINT(1) NOT NULL DEFAULT 0, updated_at DATETIME(3) NOT NULL,
 UNIQUE KEY uq_proc_quote(project_id,vendor_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS procurement_quote_revisions (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, quote_id BIGINT UNSIGNED NOT NULL, demand_revision_id BIGINT UNSIGNED NOT NULL, status VARCHAR(16) NOT NULL DEFAULT 'draft',
 tax_mode VARCHAR(16) NOT NULL, freight_mode VARCHAR(16) NOT NULL, freight DECIMAL(18,2) NOT NULL DEFAULT 0,
 valid_until DATETIME(3) NOT NULL, notes TEXT NOT NULL, created_at DATETIME(3) NOT NULL,
 INDEX idx_proc_quote_history(quote_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS procurement_quote_items (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, quote_revision_id BIGINT UNSIGNED NOT NULL, item_id BIGINT UNSIGNED NOT NULL,
 status VARCHAR(16) NOT NULL, unit_price DECIMAL(18,4) NOT NULL DEFAULT 0, minimum_quantity DECIMAL(18,3) NOT NULL DEFAULT 0,
 delivery_days INT UNSIGNED NOT NULL DEFAULT 0, notes VARCHAR(1000) NOT NULL DEFAULT '',
 UNIQUE KEY uq_proc_quote_line(quote_revision_id,item_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS procurement_messages (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, project_id BIGINT UNSIGNED NOT NULL, participant_id BIGINT UNSIGNED NOT NULL,
 author_id BIGINT UNSIGNED NOT NULL, body TEXT NOT NULL, created_at DATETIME(3) NOT NULL,
 INDEX idx_proc_messages(participant_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS procurement_attachments (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, project_id BIGINT UNSIGNED NOT NULL, owner_id BIGINT UNSIGNED NOT NULL,
 scope VARCHAR(16) NOT NULL DEFAULT '', scope_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 object_key VARCHAR(300) NOT NULL, name VARCHAR(200) NOT NULL, content_type VARCHAR(150) NOT NULL,
 size BIGINT NOT NULL, created_at DATETIME(3) NOT NULL,
 INDEX idx_proc_attachment_scope(scope,scope_id), INDEX idx_proc_attachment_owner(owner_id), INDEX idx_proc_attachment_project(project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS procurement_events (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, project_id BIGINT UNSIGNED NOT NULL DEFAULT 0, actor_id BIGINT UNSIGNED NOT NULL,
 action VARCHAR(50) NOT NULL, operation_id VARCHAR(80) NULL, fingerprint CHAR(64) NOT NULL DEFAULT '', result LONGTEXT NULL,
 created_at DATETIME(3) NOT NULL, UNIQUE KEY uq_proc_operation(actor_id,operation_id), INDEX idx_proc_event(project_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS procurement_notifications (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, account_id BIGINT UNSIGNED NOT NULL, project_id BIGINT UNSIGNED NOT NULL,
 event_key VARCHAR(150) NOT NULL, kind VARCHAR(30) NOT NULL, title VARCHAR(250) NOT NULL,
 available_at DATETIME(3) NOT NULL, read_at DATETIME(3) NULL, created_at DATETIME(3) NOT NULL,
 UNIQUE KEY uq_proc_notice(account_id,event_key), INDEX idx_proc_notice(account_id,available_at,read_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS procurement_preferences (
 account_id BIGINT UNSIGNED PRIMARY KEY, opportunity_mode VARCHAR(16) NOT NULL DEFAULT 'instant',
 supplier_suspended TINYINT(1) NOT NULL DEFAULT 0, updated_at DATETIME(3) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
