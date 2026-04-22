-- D9-P1 (review-2026-04-21-v2): assets need a routing label so auto-created
-- work orders can target the owning team instead of going unassigned.
--
-- Why: the low-quality auto-WO loop (000055 / auto_workorders_quality.go)
-- already creates a data-correction work order when an asset has scored
-- below threshold for a week, but the order currently has no owner — it
-- lands on a generic triage queue and sits. Adding `owner_team` to the
-- asset row lets the WO generator stamp the responsible team onto each
-- order at creation time, which is what the dashboard routing needs.
--
-- Why VARCHAR(100) and not a foreign key to a teams table:
-- there is no teams table today. A free-text label is the minimum viable
-- carrier for the routing signal; when a proper teams entity lands, this
-- column becomes the backfill source for the FK.
--
-- The partial index covers the common "list my team's assets" query; a
-- full B-tree would bloat on the (majority) unassigned rows.

BEGIN;

ALTER TABLE assets
    ADD COLUMN IF NOT EXISTS owner_team VARCHAR(100) NULL;

CREATE INDEX IF NOT EXISTS idx_assets_owner_team
    ON assets (tenant_id, owner_team)
    WHERE owner_team IS NOT NULL AND deleted_at IS NULL;

-- The 000058 update-snapshot trigger whitelists which columns are part
-- of the "state" that warrants a new asset_snapshots row. owner_team
-- belongs in that list: a team reassignment is a meaningful change that
-- the point-in-time history should record. Recreate the trigger with
-- owner_team folded into the WHEN condition.
DROP TRIGGER IF EXISTS assets_snapshot_after_update ON assets;

CREATE TRIGGER assets_snapshot_after_update
    AFTER UPDATE ON assets
    FOR EACH ROW
    WHEN (
        (NEW.name, NEW.asset_tag, NEW.status, NEW.bia_level,
         NEW.location_id, NEW.rack_id,
         NEW.vendor, NEW.model, NEW.serial_number,
         NEW.attributes, NEW.tags, NEW.owner_team)
        IS DISTINCT FROM
           (OLD.name, OLD.asset_tag, OLD.status, OLD.bia_level,
            OLD.location_id, OLD.rack_id,
            OLD.vendor, OLD.model, OLD.serial_number,
            OLD.attributes, OLD.tags, OLD.owner_team)
    )
    EXECUTE FUNCTION snapshot_asset();

COMMIT;
