DROP TABLE IF EXISTS scan_targets;
DROP TABLE IF EXISTS credentials;
DROP INDEX IF EXISTS idx_assets_ip_address;
ALTER TABLE assets DROP COLUMN IF EXISTS ip_address;
