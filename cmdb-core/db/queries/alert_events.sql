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
