BEGIN;

DROP INDEX IF EXISTS idx_quality_flags_tenant_created;
DROP INDEX IF EXISTS idx_quality_flags_open;
DROP INDEX IF EXISTS idx_quality_flags_asset_recent;

DROP TABLE IF EXISTS quality_flags;

COMMIT;
