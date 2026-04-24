-- name: ListDiscoveredAssets :many
SELECT * FROM discovered_assets
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
ORDER BY discovered_at DESC
LIMIT $2 OFFSET $3;

-- name: CountDiscoveredAssets :one
SELECT count(*) FROM discovered_assets
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'));

-- name: GetDiscoveredAsset :one
SELECT * FROM discovered_assets WHERE id = $1 AND tenant_id = $2;

-- name: CreateDiscoveredAsset :one
-- Ingestion pipeline entry point. match_confidence + match_strategy may
-- be NULL when no match was attempted (raw ingest, no known CI yet).
INSERT INTO discovered_assets (
    tenant_id, source, external_id, hostname, ip_address, raw_data,
    status, matched_asset_id, diff_details, match_confidence, match_strategy
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ApproveDiscoveredAsset :one
-- Marks a discovered_asset as approved and links it to the newly-created
-- asset row (or the pre-existing one on an idempotent retry).
--
-- Tenant-scoped: callers must pass their tenant_id; a row owned by a
-- different tenant will not match and the handler returns 404.
-- reason is required by the Wave-3 review gate — the UI never sends an
-- empty string, it's a mandatory input.
UPDATE discovered_assets
   SET status            = 'approved',
       approved_asset_id = $3,
       reviewed_by       = $4,
       reviewed_at       = now(),
       review_reason     = $5
 WHERE id = $1 AND tenant_id = $2
 RETURNING *;

-- name: IgnoreDiscoveredAsset :one
-- Tenant-scoped (fixed Wave 3: the pre-3 query only filtered by id,
-- which allowed cross-tenant rejection attacks).
UPDATE discovered_assets
   SET status        = 'ignored',
       reviewed_by   = $3,
       reviewed_at   = now(),
       review_reason = $4
 WHERE id = $1 AND tenant_id = $2
 RETURNING *;

-- name: GetDiscoveryStats :one
SELECT
    count(*) as total,
    count(*) FILTER (WHERE status = 'pending') as pending,
    count(*) FILTER (WHERE status = 'conflict') as conflict,
    count(*) FILTER (WHERE status = 'approved') as approved,
    count(*) FILTER (WHERE status = 'ignored') as ignored,
    count(*) FILTER (WHERE matched_asset_id IS NOT NULL) as matched
FROM discovered_assets
WHERE tenant_id = $1
  AND discovered_at > now() - interval '24 hours';

-- name: FindAssetByIP :one
SELECT * FROM assets WHERE tenant_id = $1 AND (ip_address = $2 OR bmc_ip = $2) AND deleted_at IS NULL LIMIT 1;

-- name: ListUnreviewedOverdue :many
-- Wave 3: picks up discovered_assets that have sat in pending/conflict
-- status for > N hours so the governance auto-work-order scheduler can
-- open a ticket. Returns one row per tenant+discovery so the scheduler
-- fans out correctly.
SELECT id, tenant_id, source, hostname, ip_address, status, discovered_at
FROM discovered_assets
WHERE status IN ('pending', 'conflict')
  AND discovered_at < now() - make_interval(hours => $1)
ORDER BY discovered_at ASC
LIMIT $2;
