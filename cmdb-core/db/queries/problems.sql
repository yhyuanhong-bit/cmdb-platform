-- name: ListProblems :many
SELECT * FROM problems
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('priority')::varchar IS NULL OR priority = sqlc.narg('priority'))
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountProblems :one
SELECT count(*) FROM problems
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('priority')::varchar IS NULL OR priority = sqlc.narg('priority'));

-- name: GetProblem :one
SELECT * FROM problems WHERE id = $1 AND tenant_id = $2;

-- name: CreateProblem :one
INSERT INTO problems (
    tenant_id, title, description, status, priority, severity,
    workaround, assignee_user_id, created_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateProblem :one
-- Partial update for non-status fields. Status routes through dedicated
-- transition queries so the state-machine guard lives in one place.
UPDATE problems SET
    title            = COALESCE(sqlc.narg('title'), title),
    description      = COALESCE(sqlc.narg('description'), description),
    severity         = COALESCE(sqlc.narg('severity'), severity),
    priority         = COALESCE(sqlc.narg('priority'), priority),
    workaround       = COALESCE(sqlc.narg('workaround'), workaround),
    root_cause       = COALESCE(sqlc.narg('root_cause'), root_cause),
    resolution       = COALESCE(sqlc.narg('resolution'), resolution),
    assignee_user_id = COALESCE(sqlc.narg('assignee_user_id'), assignee_user_id)
WHERE id = sqlc.arg('id') AND tenant_id = sqlc.arg('tenant_id')
RETURNING *;

-- ---------------------------------------------------------------------------
-- Lifecycle transitions. Each WHERE clause encodes the allowed source
-- state — the row count tells the domain layer whether the transition was
-- legal. Mirror of incidents (000065 / Wave 5.1).
-- ---------------------------------------------------------------------------

-- name: StartInvestigatingProblem :one
-- open → investigating
UPDATE problems SET status = 'investigating'
WHERE id = sqlc.arg('id')
  AND tenant_id = sqlc.arg('tenant_id')
  AND status = 'open'
RETURNING *;

-- name: MarkProblemKnownError :one
-- investigating → known_error. Workaround is required at this point so the
-- caller is forced to capture how operators are working around the bug
-- until the permanent fix lands.
UPDATE problems SET
    status     = 'known_error',
    workaround = COALESCE(sqlc.narg('workaround'), workaround)
WHERE id = sqlc.arg('id')
  AND tenant_id = sqlc.arg('tenant_id')
  AND status = 'investigating'
RETURNING *;

-- name: ResolveProblem :one
-- {investigating | known_error} → resolved. Skipping straight from open is
-- intentionally blocked — we want an investigation step recorded even if
-- it's instantaneous, otherwise the audit trail is meaningless.
UPDATE problems SET
    status      = 'resolved',
    resolved_at = now(),
    resolved_by = sqlc.arg('user_id'),
    root_cause  = COALESCE(sqlc.narg('root_cause'), root_cause),
    resolution  = COALESCE(sqlc.narg('resolution'), resolution)
WHERE id = sqlc.arg('id')
  AND tenant_id = sqlc.arg('tenant_id')
  AND status IN ('investigating', 'known_error')
RETURNING *;

-- name: CloseProblem :one
-- resolved → closed. Post-mortem lock.
UPDATE problems SET
    status    = 'closed',
    closed_at = now()
WHERE id = sqlc.arg('id')
  AND tenant_id = sqlc.arg('tenant_id')
  AND status = 'resolved'
RETURNING *;

-- name: ReopenProblem :one
-- resolved → investigating. Clears resolution metadata. Closed problems
-- stay closed (post-mortem lock).
UPDATE problems SET
    status      = 'investigating',
    resolved_at = NULL,
    resolved_by = NULL,
    resolution  = NULL
WHERE id = sqlc.arg('id')
  AND tenant_id = sqlc.arg('tenant_id')
  AND status = 'resolved'
RETURNING *;

-- ---------------------------------------------------------------------------
-- Linkage with incidents (M:N).
-- ---------------------------------------------------------------------------

-- name: LinkIncidentToProblem :exec
-- ON CONFLICT DO NOTHING: re-linking is idempotent. The PK on
-- (incident_id, problem_id) catches dupes; we don't want to error on a
-- second click in the UI.
INSERT INTO incident_problem_links (incident_id, problem_id, tenant_id, created_by)
VALUES ($1, $2, $3, $4)
ON CONFLICT (incident_id, problem_id) DO NOTHING;

-- name: UnlinkIncidentFromProblem :exec
DELETE FROM incident_problem_links
WHERE incident_id = $1 AND problem_id = $2 AND tenant_id = $3;

-- name: ListIncidentsForProblem :many
-- All incidents this problem covers. Tenant guarded on both incident and
-- link rows (defence in depth).
SELECT i.*
FROM incidents i
JOIN incident_problem_links l ON l.incident_id = i.id
WHERE l.problem_id = sqlc.arg('problem_id')
  AND l.tenant_id = sqlc.arg('tenant_id')
  AND i.tenant_id = sqlc.arg('tenant_id')
ORDER BY i.started_at DESC;

-- name: ListProblemsForIncident :many
-- All problems linked to a given incident.
SELECT p.*
FROM problems p
JOIN incident_problem_links l ON l.problem_id = p.id
WHERE l.incident_id = sqlc.arg('incident_id')
  AND l.tenant_id = sqlc.arg('tenant_id')
  AND p.tenant_id = sqlc.arg('tenant_id')
ORDER BY p.created_at DESC;

-- ---------------------------------------------------------------------------
-- Comments (mirror of incident_comments).
-- ---------------------------------------------------------------------------

-- name: ListProblemComments :many
SELECT c.id, c.tenant_id, c.problem_id, c.author_id, c.kind, c.body, c.created_at,
       u.username AS author_username
FROM problem_comments c
LEFT JOIN users u ON u.id = c.author_id
WHERE c.problem_id = sqlc.arg('problem_id')
  AND c.tenant_id = sqlc.arg('tenant_id')
ORDER BY c.created_at ASC;

-- name: CreateProblemComment :one
INSERT INTO problem_comments (tenant_id, problem_id, author_id, kind, body)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;
