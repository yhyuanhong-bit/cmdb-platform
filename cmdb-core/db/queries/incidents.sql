-- name: ListIncidents :many
SELECT * FROM incidents
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('severity')::varchar IS NULL OR severity = sqlc.narg('severity'))
ORDER BY started_at DESC
LIMIT $2 OFFSET $3;

-- name: CountIncidents :one
SELECT count(*) FROM incidents
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('severity')::varchar IS NULL OR severity = sqlc.narg('severity'));

-- name: GetIncident :one
SELECT * FROM incidents WHERE id = $1 AND tenant_id = $2;

-- name: CreateIncident :one
INSERT INTO incidents (tenant_id, title, status, severity, started_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateIncident :one
UPDATE incidents SET
    title       = COALESCE(sqlc.narg('title'), title),
    status      = COALESCE(sqlc.narg('status'), status),
    severity    = COALESCE(sqlc.narg('severity'), severity),
    resolved_at = COALESCE(sqlc.narg('resolved_at'), resolved_at)
WHERE id = sqlc.arg('id')
RETURNING *;
