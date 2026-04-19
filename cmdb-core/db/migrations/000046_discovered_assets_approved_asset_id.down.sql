DROP INDEX IF EXISTS idx_discovered_assets_approved_asset_id;
ALTER TABLE discovered_assets DROP COLUMN IF EXISTS approved_asset_id;
