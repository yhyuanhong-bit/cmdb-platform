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
INSERT INTO discovered_assets (tenant_id, source, external_id, hostname, ip_address, raw_data, status, matched_asset_id, diff_details)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ApproveDiscoveredAsset :one
-- Marks a discovered_asset as approved and links it to the newly-created
-- asset row (or the pre-existing one on an idempotent retry).
--
-- Tenant-scoped: callers must pass their tenant_id; a row owned by a
-- different tenant will not match and the handler returns 404.
UPDATE discovered_assets
   SET status            = 'approved',
       approved_asset_id = $3,
       reviewed_by       = $4,
       reviewed_at       = now()
 WHERE id = $1 AND tenant_id = $2
 RETURNING *;

-- name: IgnoreDiscoveredAsset :one
UPDATE discovered_assets SET status = 'ignored', reviewed_by = $2, reviewed_at = now()
WHERE id = $1 RETURNING *;

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
