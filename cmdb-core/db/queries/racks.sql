-- name: GetRack :one
SELECT * FROM racks WHERE id = $1 AND tenant_id = $2;

-- name: ListRacksByLocation :many
-- Returns racks at this location AND all descendant locations (via ltree)
SELECT r.* FROM racks r
JOIN locations l ON r.location_id = l.id
WHERE l.path <@ (SELECT loc.path FROM locations loc WHERE loc.id = $1)::ltree
ORDER BY r.name;

-- name: CreateRack :one
INSERT INTO racks (
    tenant_id, location_id, name, row_label,
    total_u, power_capacity_kw, status, tags
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8
) RETURNING *;

-- name: UpdateRack :one
UPDATE racks SET
    name              = COALESCE(sqlc.narg('name'), name),
    row_label         = COALESCE(sqlc.narg('row_label'), row_label),
    total_u           = COALESCE(sqlc.narg('total_u'), total_u),
    power_capacity_kw = COALESCE(sqlc.narg('power_capacity_kw'), power_capacity_kw),
    status            = COALESCE(sqlc.narg('status'), status),
    tags              = COALESCE(sqlc.narg('tags'), tags)
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteRack :exec
DELETE FROM racks WHERE id = $1 AND tenant_id = $2;

-- name: ListAssetsByRack :many
SELECT a.* FROM assets a
WHERE a.rack_id = $1
ORDER BY a.name;

-- name: GetRackOccupancy :one
SELECT r.total_u, COALESCE(SUM(rs.end_u - rs.start_u + 1), 0)::int AS used_u
FROM racks r
LEFT JOIN rack_slots rs ON rs.rack_id = r.id
WHERE r.id = $1
GROUP BY r.id;

-- name: CountRacksUnderLocation :one
-- Count all racks under a location and all its descendants (using ltree)
SELECT count(*) FROM racks r
JOIN locations l ON r.location_id = l.id
WHERE r.tenant_id = $1
  AND l.path <@ (SELECT loc.path FROM locations loc WHERE loc.id = $2)::ltree;
