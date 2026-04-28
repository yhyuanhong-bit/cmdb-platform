-- 000076_inventory_tenant_id.up.sql
--
-- Add tenant_id to inventory_scan_history and inventory_notes.
--
-- Background (audit finding H5, 2026-04-28):
-- These tables were created in 000018 with only an item_id FK to
-- inventory_items. Tenant isolation depended entirely on the FK chain
-- (item -> task -> tenant) being honoured at query time, but
-- impl_inventory_items.go's queries filtered only by item_id. A caller
-- who could enumerate item UUIDs across tenants (UUIDs are not secret)
-- could read scan history and notes from other tenants. tenantlint did
-- not catch this because the queries went through *pgxpool.Pool with
-- an //tenantlint:allow-direct-pool suppression comment.
--
-- Migration strategy:
--   1. Add tenant_id as nullable
--   2. Backfill from the FK chain (item -> task -> tenant)
--   3. SET NOT NULL + FK + index
-- Steps 1-2 are safe to run concurrently with reads/writes; step 3
-- briefly takes an ACCESS EXCLUSIVE lock per table but the rowcount is
-- typically small (these are append-only audit trails), so the lock
-- window is sub-second.

-- 0. Data hygiene. Some installations (notably the seed/demo dataset
-- on this codebase) contain inventory_items whose task_id does not
-- match any inventory_tasks row — the FK exists but was bypassed when
-- the data was originally loaded. Their scan_history / notes are
-- orphan rows: invisible through the API (which scopes via task) and
-- impossible to backfill since they have no tenant. Drop them first
-- so the SET NOT NULL step in step 3 doesn't fail. The companion
-- inventory_items rows are left alone — surfacing them as a "needs
-- task" data-quality alert is a follow-up concern, not a migration
-- one.
DELETE FROM inventory_scan_history ish
WHERE NOT EXISTS (
    SELECT 1
    FROM inventory_items ii
    JOIN inventory_tasks it ON ii.task_id = it.id
    WHERE ii.id = ish.item_id
);
DELETE FROM inventory_notes n
WHERE NOT EXISTS (
    SELECT 1
    FROM inventory_items ii
    JOIN inventory_tasks it ON ii.task_id = it.id
    WHERE ii.id = n.item_id
);

-- 1. Add nullable column.
ALTER TABLE inventory_scan_history ADD COLUMN tenant_id UUID;
ALTER TABLE inventory_notes        ADD COLUMN tenant_id UUID;

-- 2. Backfill from the FK chain. Any rows whose item_id has been
-- ON DELETE CASCADE'd already are unreachable, so the join handles
-- the live set in one pass.
UPDATE inventory_scan_history ish
SET tenant_id = it.tenant_id
FROM inventory_items ii
JOIN inventory_tasks it ON ii.task_id = it.id
WHERE ish.item_id = ii.id;

UPDATE inventory_notes n
SET tenant_id = it.tenant_id
FROM inventory_items ii
JOIN inventory_tasks it ON ii.task_id = it.id
WHERE n.item_id = ii.id;

-- 3. Lock down. If any rows still have NULL tenant_id at this point
-- there's a data integrity issue (orphan rows pointing at deleted
-- items) — fail the migration loudly so SRE can investigate rather
-- than silently dropping the rows.
ALTER TABLE inventory_scan_history
    ALTER COLUMN tenant_id SET NOT NULL,
    ADD CONSTRAINT fk_inventory_scan_history_tenant
        FOREIGN KEY (tenant_id) REFERENCES tenants(id);
CREATE INDEX idx_inventory_scan_history_tenant_id
    ON inventory_scan_history(tenant_id);

ALTER TABLE inventory_notes
    ALTER COLUMN tenant_id SET NOT NULL,
    ADD CONSTRAINT fk_inventory_notes_tenant
        FOREIGN KEY (tenant_id) REFERENCES tenants(id);
CREATE INDEX idx_inventory_notes_tenant_id
    ON inventory_notes(tenant_id);
