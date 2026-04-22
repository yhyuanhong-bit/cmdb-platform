-- Reverse of 000057. Restore the hard UNIQUE, drop the partial index
-- and validity columns. Any rows with valid_to set (i.e. soft-closed
-- edges) would duplicate-key under the hard UNIQUE restoration if the
-- source/target/type tuple is still in the open set; the DELETE step
-- prunes those historical rows first to keep the down migration safe.
BEGIN;

DELETE FROM asset_dependencies WHERE valid_to IS NOT NULL;

DROP INDEX IF EXISTS uq_asset_deps_open;
DROP INDEX IF EXISTS idx_asset_deps_validity;

ALTER TABLE asset_dependencies
    ADD CONSTRAINT asset_dependencies_source_asset_id_target_asset_id_dependen_key
    UNIQUE (source_asset_id, target_asset_id, dependency_type);

ALTER TABLE asset_dependencies
    DROP COLUMN IF EXISTS valid_to,
    DROP COLUMN IF EXISTS valid_from;

COMMIT;
