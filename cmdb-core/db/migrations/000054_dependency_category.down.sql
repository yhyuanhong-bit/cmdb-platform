BEGIN;

DROP INDEX IF EXISTS idx_asset_deps_tenant_category;

ALTER TABLE asset_dependencies
    DROP COLUMN IF EXISTS dependency_category;

DROP TYPE IF EXISTS dependency_category;

COMMIT;
