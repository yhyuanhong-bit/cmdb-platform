-- name: QueryAuditEvents :many
SELECT * FROM audit_events
WHERE tenant_id = $1
  AND (sqlc.narg('module')::varchar IS NULL OR module = sqlc.narg('module'))
  AND (sqlc.narg('target_type')::varchar IS NULL OR target_type = sqlc.narg('target_type'))
  AND (sqlc.narg('target_id')::uuid IS NULL OR target_id = sqlc.narg('target_id'))
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountAuditEvents :one
SELECT count(*) FROM audit_events
WHERE tenant_id = $1
  AND (sqlc.narg('module')::varchar IS NULL OR module = sqlc.narg('module'))
  AND (sqlc.narg('target_type')::varchar IS NULL OR target_type = sqlc.narg('target_type'))
  AND (sqlc.narg('target_id')::uuid IS NULL OR target_id = sqlc.narg('target_id'));

-- name: CreateAuditEvent :one
INSERT INTO audit_events (
    tenant_id, action, module, target_type,
    target_id, operator_type, operator_id, diff, source
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8, $9
) RETURNING *;
