-- name: GetRack :one
SELECT * FROM racks WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- name: ListRacksByLocation :many
-- Returns racks at this location AND all descendant locations (via ltree)
SELECT r.* FROM racks r
JOIN locations l ON r.location_id = l.id
WHERE l.tenant_id = $1
  AND l.path <@ (SELECT loc.path FROM locations loc WHERE loc.id = $2)::ltree
  AND r.deleted_at IS NULL
ORDER BY r.row_label, r.name;

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
    tags              = COALESCE(sqlc.narg('tags'), tags),
    location_id       = COALESCE(sqlc.narg('location_id'), location_id),
    updated_at        = now()
WHERE id = sqlc.arg('id') AND tenant_id = sqlc.arg('tenant_id') AND deleted_at IS NULL
RETURNING *;

-- name: DeleteRack :exec
UPDATE racks SET deleted_at = now() WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- name: ListAssetsByRack :many
SELECT a.* FROM assets a
WHERE a.rack_id = $1 AND a.tenant_id = $2 AND a.deleted_at IS NULL
ORDER BY a.name;

-- name: GetRackOccupancy :one
SELECT r.total_u, COALESCE(SUM(rs.end_u - rs.start_u + 1), 0)::int AS used_u
FROM racks r
LEFT JOIN rack_slots rs ON rs.rack_id = r.id
WHERE r.id = $1
GROUP BY r.id;

-- name: GetRackOccupanciesByLocation :many
SELECT rs.rack_id, COALESCE(SUM(rs.end_u - rs.start_u + 1), 0)::int AS used_u
FROM rack_slots rs
JOIN racks r ON rs.rack_id = r.id
JOIN locations l ON r.location_id = l.id
WHERE l.tenant_id = $1
  AND l.path <@ (SELECT loc.path FROM locations loc WHERE loc.id = $2)::ltree
  AND r.deleted_at IS NULL
GROUP BY rs.rack_id;

-- name: CountRacksUnderLocation :one
-- Count all racks under a location and all its descendants (using ltree)
SELECT count(*) FROM racks r
JOIN locations l ON r.location_id = l.id
WHERE r.tenant_id = $1
  AND l.path <@ (SELECT loc.path FROM locations loc WHERE loc.id = $2)::ltree
  AND r.deleted_at IS NULL;

-- name: TenantRackUtilizationPct :one
-- Percentage of rack U capacity currently occupied across a tenant's
-- non-deleted racks: sum(slot_u) / sum(rack.total_u) * 100. Two separate
-- aggregates, because a naive JOIN + SUM double-counts total_u once per
-- slot. Returns 0 when the tenant has no racks (NULLIF guards division
-- by zero). rack_slots.asset_id is NOT NULL so every slot row is a real
-- occupancy; no asset-state filter needed.
SELECT COALESCE(
    (SELECT SUM(rs.end_u - rs.start_u + 1)::float8
       FROM rack_slots rs
       JOIN racks r2 ON r2.id = rs.rack_id
      WHERE r2.tenant_id = $1 AND r2.deleted_at IS NULL)
    / NULLIF(
        (SELECT SUM(r.total_u)::float8
           FROM racks r
          WHERE r.tenant_id = $1 AND r.deleted_at IS NULL),
        0)
    * 100,
    0
)::float8;
