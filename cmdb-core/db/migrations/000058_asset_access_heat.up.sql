-- D9-P1 (review-2026-04-21-v2): track asset read-access heat so quality
-- scoring can weight hot assets more than cold ones.
--
-- Why: review called out that assets contribute equally to the tenant
-- quality score regardless of whether anyone ever reads them. A stale
-- server nobody looks at should not drag the dashboard down as much as
-- a bad record that 50 operators hit daily.
--
-- access_count_24h is a rolling counter; a nightly decay job zeroes it
-- so it doesn't grow unbounded. last_accessed_at is the absolute clock
-- and can be used for cold-asset detection (assets untouched >90d are
-- candidates for deprecation workflows — out of scope here, but the
-- field carries no extra cost for the heat signal itself).
--
-- Index (tenant_id, access_count_24h DESC) powers the "top 20 hot
-- assets" query shape used by the admin dashboard and the quality
-- weighting worker.

BEGIN;

ALTER TABLE assets
    ADD COLUMN IF NOT EXISTS access_count_24h INT         NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_accessed_at TIMESTAMPTZ NULL;

-- Partial index: cold assets (count=0) are the majority and don't need
-- to be in this B-tree. The index only helps rank-by-heat queries, and
-- those always filter count>0 implicitly via ORDER BY DESC LIMIT N
-- reaching the first non-zero rows fast.
CREATE INDEX IF NOT EXISTS idx_assets_access_heat
    ON assets (tenant_id, access_count_24h DESC)
    WHERE access_count_24h > 0 AND deleted_at IS NULL;

-- The 000056 snapshot trigger fires on every UPDATE. Read-path counter
-- bumps must NOT create snapshot rows — they'd flood asset_snapshots
-- and lose the history signal. Split the trigger into INSERT and
-- UPDATE variants so the UPDATE variant can use a WHEN clause that
-- skips no-op state changes (OLD is not accessible from the INSERT
-- trigger's WHEN condition, which is why one trigger can't do both).
--
-- Every business-relevant column is listed explicitly; if a new column
-- is added later and it is part of the "state" we want snapshotted,
-- it must be added here.
DROP TRIGGER IF EXISTS assets_snapshot_after_write ON assets;

CREATE TRIGGER assets_snapshot_after_insert
    AFTER INSERT ON assets
    FOR EACH ROW EXECUTE FUNCTION snapshot_asset();

CREATE TRIGGER assets_snapshot_after_update
    AFTER UPDATE ON assets
    FOR EACH ROW
    WHEN (
        (NEW.name, NEW.asset_tag, NEW.status, NEW.bia_level,
         NEW.location_id, NEW.rack_id,
         NEW.vendor, NEW.model, NEW.serial_number,
         NEW.attributes, NEW.tags)
        IS DISTINCT FROM
           (OLD.name, OLD.asset_tag, OLD.status, OLD.bia_level,
            OLD.location_id, OLD.rack_id,
            OLD.vendor, OLD.model, OLD.serial_number,
            OLD.attributes, OLD.tags)
    )
    EXECUTE FUNCTION snapshot_asset();

COMMIT;
