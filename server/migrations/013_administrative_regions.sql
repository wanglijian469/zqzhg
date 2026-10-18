ALTER TABLE regions ADD COLUMN administrative_code VARCHAR(6) NULL;
CREATE UNIQUE INDEX idx_regions_administrative_code ON regions (administrative_code);
