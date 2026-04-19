-- name: ListAlertRules :many
SELECT * FROM alert_rules
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListEnabledAlertRules :many
-- Used by the in-process evaluator (internal/domain/monitoring/evaluator.go).
-- Returns every enabled rule across every tenant so a single tick can fan out
-- over the whole fleet. The evaluator MUST keep each rule's tenant_id as the
-- scope for its metric aggregation and alert_event insertion — this is the
-- scheduler read, not a data read.
SELECT * FROM alert_rules
WHERE enabled = true
ORDER BY tenant_id, id;

-- name: CountAlertRules :one
SELECT count(*) FROM alert_rules
WHERE tenant_id = $1;

-- name: CreateAlertRule :one
INSERT INTO alert_rules (tenant_id, name, metric_name, condition, severity, enabled)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateAlertRule :one
UPDATE alert_rules SET
    name        = COALESCE(sqlc.narg('name'), name),
    metric_name = COALESCE(sqlc.narg('metric_name'), metric_name),
    condition   = COALESCE(sqlc.narg('condition'), condition),
    severity    = COALESCE(sqlc.narg('severity'), severity),
    enabled     = COALESCE(sqlc.narg('enabled'), enabled)
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteAlertRule :exec
DELETE FROM alert_rules WHERE id = $1;
