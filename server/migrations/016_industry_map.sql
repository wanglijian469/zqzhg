ALTER TABLE revisions ADD COLUMN province_code VARCHAR(6) NULL;
ALTER TABLE revisions ADD COLUMN city_code VARCHAR(6) NULL;
ALTER TABLE revisions ADD COLUMN district_code VARCHAR(6) NULL;
CREATE INDEX idx_revision_administrative_public ON revisions(province_code,city_code,district_code,status,category_id,content_id);
CREATE INDEX idx_content_map_public ON contents(hall_id,kind,visible,published_revision_id,id);
