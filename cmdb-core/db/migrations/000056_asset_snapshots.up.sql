-- D10-P0 (review-2026-04-21-v2): queryable point-in-time asset state.
--
-- The audit log tells you "who changed what" but can't answer "what did
-- this asset look like three months ago?" — the diffs are partial and
-- there's no mechanism to replay them into a full state. This table
-- captures a complete snapshot of every asset write so service.GetStateAt
-- can answer historical questions with one indexed lookup instead of a
-- diff replay.
--
-- Partitioning by month on valid_at because:
--   1. Queries are always "state at T" — range-prunes cleanly
--   2. Archival follows the same monthly cadence as audit_events (000053)
--   3. A hot tenant produces ~1 row per asset write; monthly partitions
--      keep any individual partition small enough to index efficiently
--
-- Snapshot is taken via trigger on the assets table (not via app code)
-- so the write and the snapshot commit in the same transaction. This
-- closes the race where an app-layer snapshot could miss a state after
-- a concurrent update.

BEGIN;

-- 1. Partitioned parent. Column set mirrors the fields queryable via
--    GetStateAt: status/location/rack/attributes plus name+asset_tag
--    so the historical view is human-readable without a join back to
--    assets (which may have been renamed or deleted since).
--
--    PK includes valid_at because PG requires the partition key to be
--    part of every UNIQUE constraint on a partitioned table.
CREATE TABLE asset_snapshots (
    id              UUID         NOT NULL DEFAULT gen_random_uuid(),
    asset_id        UUID         NOT NULL,
    tenant_id       UUID         NOT NULL REFERENCES tenants(id),
    valid_at        TIMESTAMPTZ  NOT NULL,
    name            VARCHAR(255) NOT NULL,
    asset_tag       VARCHAR(100) NOT NULL,
    status          VARCHAR(30)  NOT NULL,
    bia_level       VARCHAR(20)  NOT NULL,
    location_id     UUID,
    rack_id         UUID,
    vendor          VARCHAR(255),
    model           VARCHAR(255),
    serial_number   VARCHAR(255),
    attributes      JSONB        NOT NULL DEFAULT '{}',
    tags            TEXT[],
    PRIMARY KEY (id, valid_at)
) PARTITION BY RANGE (valid_at);

-- 2. Indexes propagate to every partition automatically.
--    idx_asset_snapshots_lookup is the workhorse for GetStateAt:
--    WHERE asset_id=$1 AND valid_at<=$2 ORDER BY valid_at DESC LIMIT 1
--    uses this exact shape.
CREATE INDEX idx_asset_snapshots_lookup
    ON asset_snapshots (asset_id, valid_at DESC);

CREATE INDEX idx_asset_snapshots_tenant_valid
    ON asset_snapshots (tenant_id, valid_at DESC);

-- 3. Catch-all legacy partition for anything strictly before the current
--    month — covers the backfill below and any clock-skewed late write.
CREATE TABLE asset_snapshots_legacy_partition
    PARTITION OF asset_snapshots
    FOR VALUES FROM (MINVALUE) TO (date_trunc('month', now()));

-- 4. Pre-create the current month and the next three months. The archive
--    CronJob that rolls audit_events should extend to this table; for
--    now four months of forward coverage buys time for the operator to
--    wire in rolling creation.
DO $$
DECLARE
    m       date := date_trunc('month', now())::date;
    i       int;
    p_name  text;
    p_from  text;
    p_to    text;
BEGIN
    FOR i IN 0..3 LOOP
        p_name := 'asset_snapshots_' || to_char(m + (i || ' month')::interval, 'YYYY_MM');
        p_from := to_char(m + (i     || ' month')::interval, 'YYYY-MM-DD');
        p_to   := to_char(m + ((i+1) || ' month')::interval, 'YYYY-MM-DD');
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS %I PARTITION OF asset_snapshots
             FOR VALUES FROM (%L) TO (%L)',
            p_name, p_from, p_to
        );
    END LOOP;
END $$;

-- 5. Snapshot trigger. Fires AFTER INSERT OR UPDATE on assets so the
--    snapshot reflects the committed row exactly (BEFORE triggers would
--    capture NEW that could still be mutated by subsequent BEFORE
--    triggers in the same statement).
--
--    valid_at = now() rather than NEW.updated_at because updated_at may
--    be set by the application and could lag wall-clock — the snapshot
--    chain must be monotonic in real time for GetStateAt's "<=" to be
--    well-ordered.
CREATE OR REPLACE FUNCTION snapshot_asset() RETURNS trigger AS $$
BEGIN
    INSERT INTO asset_snapshots (
        asset_id, tenant_id, valid_at,
        name, asset_tag, status, bia_level,
        location_id, rack_id,
        vendor, model, serial_number,
        attributes, tags
    ) VALUES (
        NEW.id, NEW.tenant_id, now(),
        NEW.name, NEW.asset_tag, NEW.status, NEW.bia_level,
        NEW.location_id, NEW.rack_id,
        NEW.vendor, NEW.model, NEW.serial_number,
        NEW.attributes, NEW.tags
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER assets_snapshot_after_write
    AFTER INSERT OR UPDATE ON assets
    FOR EACH ROW EXECUTE FUNCTION snapshot_asset();

-- 6. Backfill: insert one snapshot per existing asset at its updated_at.
--    Without this, GetStateAt for any time before the first post-deploy
--    write would return 404 for every asset — a broken cut-over
--    experience for the first month after deploy. The legacy partition
--    created in step 3 absorbs all backfill rows.
INSERT INTO asset_snapshots (
    asset_id, tenant_id, valid_at,
    name, asset_tag, status, bia_level,
    location_id, rack_id,
    vendor, model, serial_number,
    attributes, tags
)
SELECT
    id, tenant_id, updated_at,
    name, asset_tag, status, bia_level,
    location_id, rack_id,
    vendor, model, serial_number,
    attributes, tags
FROM assets;

COMMIT;
