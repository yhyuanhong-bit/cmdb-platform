BEGIN;

-- Restore the 000056 trigger function body (without owner_team).
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

ALTER TABLE asset_snapshots DROP COLUMN IF EXISTS owner_team;

COMMIT;
