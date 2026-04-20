-- sync_state: per-node × entity-type sync watermark.
--
-- A unique (node_id, entity_type) row tracks the last_sync_version that a
-- given edge node has successfully applied for a given entity_type within
-- a specific tenant. Rows are scoped by tenant_id except where the central
-- reconciler must sweep across every tenant (ListStaleSyncStates), which
-- is explicitly documented as cross-tenant.

-- name: ListSyncStatesByTenant :many
SELECT node_id, entity_type, last_sync_version, last_sync_at, status, error_message
FROM sync_state
WHERE tenant_id = $1
ORDER BY node_id, entity_type;

-- name: ListSyncStatesByTenantEntity :many
SELECT node_id, last_sync_version
FROM sync_state
WHERE tenant_id = $1 AND entity_type = $2;

-- name: CountSyncStateByNode :one
-- cross-tenant: agent.Start probe — checks whether ANY sync_state row exists
-- for this edge node so it can detect a first-boot. The node_id namespace is
-- operator-controlled, not tenant-controlled.
SELECT count(*) FROM sync_state WHERE node_id = $1;

-- name: ListStaleSyncStates :many
-- cross-tenant: the central reconciler fans out to every tenant to find
-- stale edges. Returned rows still include tenant_id so downstream logic
-- can scope its follow-up writes when needed.
SELECT node_id, tenant_id, entity_type, last_sync_version, last_sync_at
FROM sync_state
WHERE status = 'active' AND last_sync_at < now() - interval '1 hour';

-- name: UpdateSyncStateHeartbeat :exec
-- Resets last_sync_at when the reconciler confirms a node is caught up.
-- Keyed by (node_id, entity_type) which is the table's unique key.
UPDATE sync_state
SET last_sync_at = now()
WHERE node_id = $1 AND entity_type = $2;

-- name: MarkSyncStateError :exec
UPDATE sync_state
SET status = 'error', error_message = $3
WHERE node_id = $1 AND entity_type = $2;

-- name: UpsertSyncState :exec
-- Used by the edge agent after applying an envelope. GREATEST ensures the
-- watermark only moves forward even if an older envelope is re-played.
INSERT INTO sync_state (node_id, tenant_id, entity_type, last_sync_version, last_sync_at, status)
VALUES ($1, $2, $3, $4, now(), 'active')
ON CONFLICT (node_id, entity_type) DO UPDATE SET
  last_sync_version = GREATEST(sync_state.last_sync_version, EXCLUDED.last_sync_version),
  last_sync_at = now(),
  status = 'active';

-- name: UpsertSyncStateAbsolute :exec
-- Variant used by updateSyncState: writes the current MAX(sync_version)
-- directly without GREATEST. When the agent probes the local max, that
-- value is already the latest, so "forward-only" is guaranteed by the
-- caller — using the explicit assignment avoids masking drift.
INSERT INTO sync_state (node_id, tenant_id, entity_type, last_sync_version, last_sync_at, status)
VALUES ($1, $2, $3, $4, now(), 'active')
ON CONFLICT (node_id, entity_type) DO UPDATE SET
  last_sync_version = EXCLUDED.last_sync_version,
  last_sync_at = now();
