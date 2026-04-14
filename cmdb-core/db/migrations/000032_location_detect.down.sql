DROP INDEX IF EXISTS idx_mac_cache_switch;
DROP INDEX IF EXISTS idx_mac_cache_asset;
DROP TABLE IF EXISTS mac_address_cache;

DROP INDEX IF EXISTS idx_location_history_tenant;
DROP INDEX IF EXISTS idx_location_history_asset;
DROP TABLE IF EXISTS asset_location_history;

DROP INDEX IF EXISTS idx_switch_port_rack;
DROP TABLE IF EXISTS switch_port_mapping;
