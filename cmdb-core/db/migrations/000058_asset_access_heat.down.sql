-- Reverse of 000058. Drops the index before the columns so the index
-- rebuild on other connections never races a DROP on the underlying
-- column.
--
-- Restores the 000056-era trigger without the WHEN clause, so the
-- snapshot behavior at version 57 is fully recovered. Safe even if
-- the WHEN-filtered trigger isn't present (DROP IF EXISTS).
BEGIN;

DROP INDEX IF EXISTS idx_assets_access_heat;

DROP TRIGGER IF EXISTS assets_snapshot_after_insert ON assets;
DROP TRIGGER IF EXISTS assets_snapshot_after_update ON assets;
CREATE TRIGGER assets_snapshot_after_write
    AFTER INSERT OR UPDATE ON assets
    FOR EACH ROW
    EXECUTE FUNCTION snapshot_asset();

ALTER TABLE assets
    DROP COLUMN IF EXISTS last_accessed_at,
    DROP COLUMN IF EXISTS access_count_24h;

COMMIT;
