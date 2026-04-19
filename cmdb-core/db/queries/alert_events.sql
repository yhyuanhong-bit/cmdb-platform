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
WHERE id = $1 AND tenant_id = $2 AND status = 'firing'
RETURNING *;

-- name: ResolveAlert :one
UPDATE alert_events SET
    status      = 'resolved',
    resolved_at = now()
WHERE id = $1 AND tenant_id = $2 AND status IN ('firing', 'acknowledged')
RETURNING *;

-- name: UpsertAlertEventByDedup :one
-- Idempotent per-hour alert insertion used by the evaluator. The unique
-- index on alert_events.dedup_key enforces "one row per rule + asset + hour";
-- on conflict we refresh trigger_value + status + updated_at but keep
-- fired_at pinned to the first firing of the window so operators can see
-- the original time the breach started.
INSERT INTO alert_events (
    tenant_id, rule_id, asset_id, status, severity, message, trigger_value, dedup_key, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
ON CONFLICT (dedup_key) DO UPDATE SET
    status        = EXCLUDED.status,
    trigger_value = EXCLUDED.trigger_value,
    message       = EXCLUDED.message,
    updated_at    = now()
RETURNING id, tenant_id, rule_id, asset_id, status, severity, message, trigger_value, fired_at, acked_at, resolved_at, sync_version, dedup_key, updated_at, (xmax = 0) AS inserted;

-- name: CountAlertsUnderLocation :one
-- Count firing alerts for assets under a location and all its descendants
SELECT count(*) FROM alert_events ae
JOIN assets a ON ae.asset_id = a.id
JOIN locations l ON a.location_id = l.id
WHERE a.tenant_id = $1
  AND ae.status = 'firing'
  AND l.path <@ (SELECT loc.path FROM locations loc WHERE loc.id = $2)::ltree;
