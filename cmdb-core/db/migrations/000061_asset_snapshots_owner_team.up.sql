-- D10-P2 (review-2026-04-21-v2): the asset history diff API needs to
-- surface owner_team changes — reassigning an asset to a new team is one
-- of the most frequently audited state transitions. The 000056 snapshot
-- table doesn't carry owner_team (the column didn't exist yet), so even
-- though 000060's UPDATE trigger WHEN clause already fires a snapshot
-- on team reassignment, the snapshot row itself lacks the new value.
--
-- This migration extends asset_snapshots and the snapshot_asset()
-- trigger to capture owner_team, and backfills every existing row with
-- the current asset's owner_team (which is NULL for all rows today, so
-- the backfill is a no-op for correctness but keeps the column
-- consistent across history).

BEGIN;

ALTER TABLE asset_snapshots
    ADD COLUMN IF NOT EXISTS owner_team VARCHAR(100) NULL;

-- Rewrite the trigger function so subsequent snapshots capture owner_team.
-- CREATE OR REPLACE keeps the function OID stable so the existing
-- trigger binding (assets_snapshot_after_insert + assets_snapshot_after_update
-- from 000060) stays attached without a DROP/CREATE dance.
CREATE OR REPLACE FUNCTION snapshot_asset() RETURNS trigger AS $$
BEGIN
    INSERT INTO asset_snapshots (
        asset_id, tenant_id, valid_at,
        name, asset_tag, status, bia_level,
        location_id, rack_id,
        vendor, model, serial_number,
        attributes, tags, owner_team
    ) VALUES (
        NEW.id, NEW.tenant_id, now(),
        NEW.name, NEW.asset_tag, NEW.status, NEW.bia_level,
        NEW.location_id, NEW.rack_id,
        NEW.vendor, NEW.model, NEW.serial_number,
        NEW.attributes, NEW.tags, NEW.owner_team
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Backfill existing snapshots from the current asset row. Every
-- pre-000060 asset has owner_team IS NULL so this is a correctness
-- no-op, but it keeps the column aligned for anyone who seeds
-- owner_team before re-running tests.
UPDATE asset_snapshots s
   SET owner_team = a.owner_team
  FROM assets a
 WHERE s.asset_id = a.id
   AND s.owner_team IS DISTINCT FROM a.owner_team;

COMMIT;
