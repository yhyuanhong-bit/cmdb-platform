-- 000073_drop_sync_tables.up.sql
--
-- Edge-Central deployment was cancelled (2026-04-27). Drop the
-- replication-state tables and the per-tenant RBAC sync grants. The
-- per-table sync_version columns and their indexes are intentionally
-- LEFT IN PLACE: ~5 non-sync packages still increment them on every
-- CRUD, and the columns are cheap (BIGINT, default 0). Removing them
-- would touch every UPDATE in asset / maintenance / topology /
-- location_detect / impl_qr — high blast radius, near-zero benefit.

DROP TABLE IF EXISTS sync_conflicts;
DROP TABLE IF EXISTS sync_state;

-- Strip the sync key from RBAC role permissions (added in 000030).
-- Using `-` because the JSONB value is an array; `?-` would only
-- match top-level keys. `-` strips the key cleanly.
UPDATE roles SET permissions = permissions - 'sync'
WHERE permissions ? 'sync';
