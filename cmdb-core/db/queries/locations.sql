-- name: ListRootLocations :many
SELECT * FROM locations
WHERE tenant_id = $1 AND parent_id IS NULL
ORDER BY sort_order, name;

-- name: GetLocation :one
SELECT * FROM locations WHERE id = $1 AND tenant_id = $2;

-- name: ListChildren :many
SELECT * FROM locations
WHERE parent_id = $1
ORDER BY sort_order, name;

-- name: ListDescendants :many
SELECT * FROM locations
WHERE tenant_id = $1 AND path <@ $2::ltree
ORDER BY path;

-- name: ListAncestors :many
SELECT * FROM locations
WHERE tenant_id = $1 AND path @> $2::ltree
ORDER BY path;

-- name: CreateLocation :one
INSERT INTO locations (
    tenant_id, name, name_en, slug, level,
    parent_id, path, status, metadata, sort_order
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7::ltree, $8, $9, $10
) RETURNING *;

-- name: UpdateLocation :one
UPDATE locations SET
    name       = COALESCE(sqlc.narg('name'), name),
    name_en    = COALESCE(sqlc.narg('name_en'), name_en),
    slug       = COALESCE(sqlc.narg('slug'), slug),
    level      = COALESCE(sqlc.narg('level'), level),
    status     = COALESCE(sqlc.narg('status'), status),
    metadata   = COALESCE(sqlc.narg('metadata'), metadata),
    sort_order = COALESCE(sqlc.narg('sort_order'), sort_order),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteLocation :exec
DELETE FROM locations WHERE id = $1 AND tenant_id = $2;

-- name: GetLocationBySlug :one
SELECT * FROM locations
WHERE tenant_id = $1 AND slug = $2 AND level = $3;

-- name: CountAssetsByLocation :many
SELECT l.id, COUNT(a.id)::bigint AS total_assets
FROM locations l
LEFT JOIN assets a ON a.location_id IN (
  SELECT sub.id FROM locations sub WHERE sub.path <@ l.path AND sub.deleted_at IS NULL
) AND a.deleted_at IS NULL
WHERE l.tenant_id = $1 AND l.deleted_at IS NULL
GROUP BY l.id;

-- name: CountAlertsByLocation :many
SELECT a.location_id AS id, COUNT(ae.id)::bigint AS critical_alerts
FROM assets a
JOIN alert_events ae ON ae.asset_id = a.id AND ae.status = 'firing' AND ae.severity = 'critical'
WHERE a.tenant_id = $1 AND a.deleted_at IS NULL AND a.location_id IS NOT NULL
GROUP BY a.location_id;
