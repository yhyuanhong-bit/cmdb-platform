-- sync_conflicts: manual-intervention channel.
--
-- Rows are inserted by operator tooling when a human dispute needs to be
-- arbitrated; automatic sync uses last-write-wins and never writes here.
-- See docs/SYNC_CONFLICT.md for the full policy.

-- name: ListPendingSyncConflicts :many
SELECT id, entity_type, entity_id, local_version, remote_version,
       local_diff, remote_diff, created_at
FROM sync_conflicts
WHERE tenant_id = $1 AND resolution = 'pending'
ORDER BY created_at;

-- name: GetPendingSyncConflict :one
-- Used by the resolve handler. Tenant-scoped so a resolve attempt against
-- another tenant's conflict yields ErrNoRows (maps to HTTP 404 in the
-- handler, which is how cross-tenant IDOR is blocked).
SELECT entity_type, entity_id, remote_diff
FROM sync_conflicts
WHERE id = $1 AND tenant_id = $2 AND resolution = 'pending';

-- name: ResolveSyncConflict :exec
-- Marks a pending conflict as resolved. Tenant-scoped as a second guard
-- after GetPendingSyncConflict — even if a caller bypasses the read path,
-- this UPDATE still filters by tenant_id.
UPDATE sync_conflicts
SET resolution = $1, resolved_by = $2, resolved_at = now()
WHERE id = $3 AND tenant_id = $4;

-- name: CountPendingConflictsByTenantNearSLA :many
-- Scans conflicts that crossed the 3-day SLA warning threshold but haven't
-- yet hit the 7-day auto-expire. Used by the workflow subscriber to ping
-- ops-admins. Cross-tenant on purpose — the caller then looks up each
-- tenant's ops-admins and files per-tenant notifications.
SELECT tenant_id, count(*)::bigint AS count
FROM sync_conflicts
WHERE resolution = 'pending'
  AND created_at < now() - interval '3 days'
  AND created_at >= now() - interval '4 days'
GROUP BY tenant_id;

-- name: AutoExpireStaleSyncConflicts :execrows
-- cross-tenant: runs on a background timer to mark all conflicts older than
-- 7 days as auto-expired. The tenant dimension is preserved on each row but
-- the sweep itself is intentionally tenant-wide.
UPDATE sync_conflicts
SET resolution = 'auto_expired', resolved_at = now()
WHERE resolution = 'pending' AND created_at < now() - interval '7 days';
