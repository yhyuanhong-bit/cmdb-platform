-- name: ListRootLocations :many
SELECT * FROM locations
WHERE tenant_id = $1 AND parent_id IS NULL
ORDER BY sort_order, name;

-- name: GetLocation :one
SELECT * FROM locations WHERE id = $1;

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
DELETE FROM locations WHERE id = $1;

-- name: GetLocationBySlug :one
SELECT * FROM locations
WHERE tenant_id = $1 AND slug = $2 AND level = $3;
