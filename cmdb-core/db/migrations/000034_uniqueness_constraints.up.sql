-- 1. Racks: unique name per location
CREATE UNIQUE INDEX IF NOT EXISTS idx_racks_unique_name_per_location
ON racks (tenant_id, location_id, name) WHERE deleted_at IS NULL;

-- 2. Alert rules: unique name per tenant
CREATE UNIQUE INDEX IF NOT EXISTS idx_alert_rules_unique_name
ON alert_rules (tenant_id, name);

-- 3. Discovered assets: unique external_id per source per tenant
CREATE UNIQUE INDEX IF NOT EXISTS idx_discovered_assets_unique_external
ON discovered_assets (tenant_id, source, external_id);
