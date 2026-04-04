-- name: ListAlertRules :many
SELECT * FROM alert_rules
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountAlertRules :one
SELECT count(*) FROM alert_rules
WHERE tenant_id = $1;

-- name: CreateAlertRule :one
INSERT INTO alert_rules (tenant_id, name, metric_name, condition, severity, enabled)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;
