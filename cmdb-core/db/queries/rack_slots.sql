-- name: ListRackSlots :many
SELECT rs.id, rs.rack_id, rs.asset_id, rs.start_u, rs.end_u, rs.side,
       a.name as asset_name, a.asset_tag, a.type as asset_type, a.bia_level
FROM rack_slots rs
LEFT JOIN assets a ON rs.asset_id = a.id
WHERE rs.rack_id = $1
ORDER BY rs.start_u;

-- name: CreateRackSlot :one
INSERT INTO rack_slots (rack_id, asset_id, start_u, end_u, side)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: DeleteRackSlot :exec
DELETE FROM rack_slots
WHERE id = $1
  AND rack_id IN (SELECT r.id FROM racks r WHERE r.tenant_id = $2);

-- name: CheckSlotConflict :one
SELECT count(*) FROM rack_slots
WHERE rack_id = $1
  AND side = $2
  AND NOT (end_u < $3 OR start_u > $4);
