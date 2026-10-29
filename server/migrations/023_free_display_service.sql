CREATE TABLE IF NOT EXISTS service_documents (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, doc_key VARCHAR(40) NOT NULL, version BIGINT UNSIGNED NOT NULL,
 title TEXT NOT NULL, title_en TEXT NOT NULL, body LONGTEXT NOT NULL, body_en LONGTEXT NOT NULL,
 digest CHAR(64) NOT NULL, status VARCHAR(16) NOT NULL DEFAULT 'draft', effective_at DATETIME(3) NOT NULL,
 updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0, created_at DATETIME(3) NOT NULL, published_at DATETIME(3) NULL,
 UNIQUE KEY uq_service_document(doc_key,version), INDEX idx_service_document_active(doc_key,status,effective_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS service_acceptances (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, account_id BIGINT UNSIGNED NOT NULL, document_id BIGINT UNSIGNED NOT NULL,
 digest CHAR(64) NOT NULL, source VARCHAR(16) NOT NULL, accepted_at DATETIME(3) NOT NULL,
 UNIQUE KEY uq_service_acceptance(account_id,document_id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS business_verifications (
 account_id BIGINT UNSIGNED PRIMARY KEY, company_name VARCHAR(200) NOT NULL, credit_code VARCHAR(18) NOT NULL,
 representative VARCHAR(100) NOT NULL, agent_name VARCHAR(100) NOT NULL, contact VARCHAR(200) NOT NULL,
 license_id BIGINT UNSIGNED NOT NULL DEFAULT 0, authorization_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 status VARCHAR(16) NOT NULL DEFAULT 'draft', version BIGINT UNSIGNED NOT NULL DEFAULT 1,
 review_note TEXT NOT NULL, reviewed_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
 created_at DATETIME(3) NOT NULL, updated_at DATETIME(3) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS service_requests (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, owner_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 kind VARCHAR(20) NOT NULL, target_url VARCHAR(1000) NOT NULL DEFAULT '', subject VARCHAR(200) NOT NULL,
 description TEXT NOT NULL, contact VARCHAR(200) NOT NULL DEFAULT '', attachments TEXT NOT NULL,
 status VARCHAR(20) NOT NULL DEFAULT 'received', reply TEXT NOT NULL, version BIGINT UNSIGNED NOT NULL DEFAULT 1,
 credential_hash CHAR(64) NOT NULL, idempotency_hash CHAR(64) NOT NULL, updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
 created_at DATETIME(3) NOT NULL, updated_at DATETIME(3) NOT NULL,
 UNIQUE KEY uq_service_request_dedup(idempotency_hash), INDEX idx_service_request_owner(owner_id,id), INDEX idx_service_request_state(status,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS service_evidence (
 id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, owner_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
 credential_hash CHAR(64) NOT NULL DEFAULT '', purpose VARCHAR(20) NOT NULL,
 object_key VARCHAR(255) NOT NULL, name VARCHAR(200) NOT NULL, mime VARCHAR(80) NOT NULL, size BIGINT NOT NULL,
 created_at DATETIME(3) NOT NULL, INDEX idx_service_evidence_owner(owner_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS home_rotation_pool (
 vendor_id BIGINT UNSIGNED PRIMARY KEY, hall_id BIGINT UNSIGNED NOT NULL, state VARCHAR(16) NOT NULL,
 reason TEXT NOT NULL, updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0, updated_at DATETIME(3) NOT NULL,
 INDEX idx_home_pool(hall_id,state)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS home_rotation_days (
 hall_id BIGINT UNSIGNED NOT NULL, day VARCHAR(10) NOT NULL, created_at DATETIME(3) NOT NULL,
 PRIMARY KEY(hall_id,day)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS home_rotation_slots (
 hall_id BIGINT UNSIGNED NOT NULL, day VARCHAR(10) NOT NULL, position INT NOT NULL,
 vendor_id BIGINT UNSIGNED NOT NULL, PRIMARY KEY(hall_id,day,position),
 UNIQUE KEY uq_home_rotation_vendor(hall_id,day,vendor_id), INDEX idx_home_rotation_history(vendor_id,day)
) ENGINE=InnoDB;
