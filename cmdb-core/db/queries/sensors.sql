-- sensors: per-tenant sensor registry. The table has no soft-delete
-- column (see 000020_phase4_group2.up.sql); deletes are hard deletes.

-- name: ListSensorsByTenant :many
-- Left-joins assets to surface asset_name; the join is filtered on
-- assets.deleted_at IS NULL so a soft-deleted asset is treated as
-- "no asset" rather than surfacing a stale name.
SELECT s.id, s.asset_id, a.name AS asset_name, s.name, s.type, s.location,
       s.polling_interval, s.enabled, s.status, s.last_heartbeat
FROM sensors s
LEFT JOIN assets a ON s.asset_id = a.id AND a.deleted_at IS NULL
WHERE s.tenant_id = $1
ORDER BY s.name;

-- name: CreateSensor :exec
INSERT INTO sensors (id, tenant_id, asset_id, name, type, location,
                     polling_interval, enabled, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'offline', now(), now());

-- name: DeleteSensor :execrows
DELETE FROM sensors WHERE id = $1 AND tenant_id = $2;

-- name: UpdateSensorHeartbeat :exec
UPDATE sensors
SET last_heartbeat = now(), status = $3, updated_at = now()
WHERE id = $1 AND tenant_id = $2;
