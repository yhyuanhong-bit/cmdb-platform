-- name: ListAlerts :many
SELECT * FROM alert_events
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('severity')::varchar IS NULL OR severity = sqlc.narg('severity'))
  AND (sqlc.narg('asset_id')::uuid IS NULL OR asset_id = sqlc.narg('asset_id'))
ORDER BY fired_at DESC
LIMIT $2 OFFSET $3;

-- name: CountAlerts :one
SELECT count(*) FROM alert_events
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('severity')::varchar IS NULL OR severity = sqlc.narg('severity'))
  AND (sqlc.narg('asset_id')::uuid IS NULL OR asset_id = sqlc.narg('asset_id'));

-- name: AcknowledgeAlert :one
UPDATE alert_events SET
    status   = 'acknowledged',
    acked_at = now()
WHERE id = $1 AND status = 'firing'
RETURNING *;

-- name: ResolveAlert :one
UPDATE alert_events SET
    status      = 'resolved',
    resolved_at = now()
WHERE id = $1 AND status IN ('firing', 'acknowledged')
RETURNING *;

-- name: CountAlertsUnderLocation :one
-- Count firing alerts for assets under a location and all its descendants
SELECT count(*) FROM alert_events ae
JOIN assets a ON ae.asset_id = a.id
JOIN locations l ON a.location_id = l.id
WHERE a.tenant_id = $1
  AND ae.status = 'firing'
  AND l.path <@ (SELECT loc.path FROM locations loc WHERE loc.id = $2)::ltree;
