ALTER TABLE accounts ADD COLUMN login_name VARCHAR(100) NULL, ADD UNIQUE KEY uq_account_login_name (login_name);
