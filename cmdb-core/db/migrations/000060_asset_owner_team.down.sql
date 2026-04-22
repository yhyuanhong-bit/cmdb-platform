BEGIN;

-- Revert the trigger WHEN clause to the 000058 shape (without owner_team).
DROP TRIGGER IF EXISTS assets_snapshot_after_update ON assets;

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

DROP INDEX IF EXISTS idx_assets_owner_team;
ALTER TABLE assets DROP COLUMN IF EXISTS owner_team;

COMMIT;
