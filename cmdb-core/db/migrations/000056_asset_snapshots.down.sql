-- Reverse of 000056_asset_snapshots.up.sql. Drop trigger first so the
-- table drop doesn't race with a concurrent write, then drop the
-- function, then the partitioned table (which cascades to partitions).
BEGIN;

DROP TRIGGER IF EXISTS assets_snapshot_after_write ON assets;
DROP FUNCTION IF EXISTS snapshot_asset();
DROP TABLE IF EXISTS asset_snapshots CASCADE;

COMMIT;
