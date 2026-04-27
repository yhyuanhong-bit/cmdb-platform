-- name: ListMetricSources :many
SELECT * FROM metric_sources
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('kind')::varchar   IS NULL OR kind   = sqlc.narg('kind'))
ORDER BY last_heartbeat_at NULLS FIRST, name;

-- name: GetMetricSource :one
SELECT * FROM metric_sources WHERE id = $1 AND tenant_id = $2;

-- name: GetMetricSourceByName :one
SELECT * FROM metric_sources WHERE tenant_id = $1 AND name = $2;

-- name: CreateMetricSource :one
INSERT INTO metric_sources (
    tenant_id, name, kind, expected_interval_seconds, status, notes
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateMetricSource :one
UPDATE metric_sources SET
    name                      = COALESCE(sqlc.narg('name'),                      name),
    kind                      = COALESCE(sqlc.narg('kind'),                      kind),
    expected_interval_seconds = COALESCE(sqlc.narg('expected_interval_seconds'), expected_interval_seconds),
    status                    = COALESCE(sqlc.narg('status'),                    status),
    notes                     = COALESCE(sqlc.narg('notes'),                     notes)
WHERE id = sqlc.arg('id') AND tenant_id = sqlc.arg('tenant_id')
RETURNING *;

-- name: DeleteMetricSource :exec
DELETE FROM metric_sources WHERE id = $1 AND tenant_id = $2;

-- name: HeartbeatMetricSource :one
-- Bumps last_heartbeat_at to now() and adds the supplied sample count
-- to the lifetime counter. Operators don't have to compute the new
-- count themselves; the SQL adds it server-side so concurrent
-- heartbeats can't race-clobber each other.
UPDATE metric_sources SET
    last_heartbeat_at = now(),
    last_sample_count = last_sample_count + sqlc.arg('sample_delta')
WHERE id = sqlc.arg('id') AND tenant_id = sqlc.arg('tenant_id')
RETURNING *;

-- name: ListStaleMetricSources :many
-- The "data plane health" query. A source is stale when its
-- last_heartbeat_at is more than 2× expected_interval old (or it has
-- never sent a heartbeat at all). Disabled sources are excluded.
-- The 2× factor is tolerant of jitter / short restarts without
-- flapping the freshness alert.
SELECT
    s.*,
    -- sqlc generates non-nullable int32 from EXTRACT(...)::int. NULL
    -- last_heartbeat_at would scan-fail, so coerce never-heartbeated
    -- rows to -1 — the handler maps that sentinel back to a nil
    -- "seconds_since_heartbeat" in the API response.
    COALESCE(EXTRACT(EPOCH FROM (now() - s.last_heartbeat_at))::int, -1) AS seconds_since_heartbeat
FROM metric_sources s
WHERE s.tenant_id = $1
  AND s.status    = 'active'
  AND (
        s.last_heartbeat_at IS NULL
     OR s.last_heartbeat_at < now() - (s.expected_interval_seconds * 2 || ' seconds')::interval
      )
ORDER BY s.last_heartbeat_at NULLS FIRST, s.name;
