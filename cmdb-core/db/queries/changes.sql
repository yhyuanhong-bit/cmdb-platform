-- name: ListChanges :many
SELECT * FROM changes
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('type')::varchar   IS NULL OR type   = sqlc.narg('type'))
  AND (sqlc.narg('risk')::varchar   IS NULL OR risk   = sqlc.narg('risk'))
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountChanges :one
SELECT count(*) FROM changes
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('type')::varchar   IS NULL OR type   = sqlc.narg('type'))
  AND (sqlc.narg('risk')::varchar   IS NULL OR risk   = sqlc.narg('risk'));

-- name: GetChange :one
SELECT * FROM changes WHERE id = $1 AND tenant_id = $2;

-- name: CreateChange :one
INSERT INTO changes (
    tenant_id, title, description, type, risk, status,
    approval_threshold, requested_by, assignee_user_id,
    planned_start, planned_end, rollback_plan, impact_summary
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: UpdateChange :one
-- Partial update for non-status fields. Status routes through dedicated
-- transition queries.
UPDATE changes SET
    title              = COALESCE(sqlc.narg('title'),              title),
    description        = COALESCE(sqlc.narg('description'),        description),
    type               = COALESCE(sqlc.narg('type'),               type),
    risk               = COALESCE(sqlc.narg('risk'),               risk),
    approval_threshold = COALESCE(sqlc.narg('approval_threshold'), approval_threshold),
    assignee_user_id   = COALESCE(sqlc.narg('assignee_user_id'),   assignee_user_id),
    planned_start      = COALESCE(sqlc.narg('planned_start'),      planned_start),
    planned_end        = COALESCE(sqlc.narg('planned_end'),        planned_end),
    rollback_plan      = COALESCE(sqlc.narg('rollback_plan'),      rollback_plan),
    impact_summary     = COALESCE(sqlc.narg('impact_summary'),     impact_summary)
WHERE id = sqlc.arg('id') AND tenant_id = sqlc.arg('tenant_id')
RETURNING *;

-- ---------------------------------------------------------------------------
-- Lifecycle transitions. WHERE status guards make illegal transitions
-- a zero-row result; the domain layer maps that to ErrInvalidStateTransition.
-- ---------------------------------------------------------------------------

-- name: SubmitChange :one
-- draft → submitted. submitted_at stamped.
UPDATE changes SET
    status       = 'submitted',
    submitted_at = now()
WHERE id = sqlc.arg('id')
  AND tenant_id = sqlc.arg('tenant_id')
  AND status = 'draft'
RETURNING *;

-- name: ApproveChangeAuto :one
-- Used by the domain layer after a CAB vote pushes approve count past the
-- threshold. submitted → approved.
UPDATE changes SET
    status      = 'approved',
    approved_at = now()
WHERE id = sqlc.arg('id')
  AND tenant_id = sqlc.arg('tenant_id')
  AND status = 'submitted'
RETURNING *;

-- name: RejectChangeAuto :one
-- Used after a single CAB reject. submitted → rejected.
UPDATE changes SET
    status      = 'rejected',
    rejected_at = now()
WHERE id = sqlc.arg('id')
  AND tenant_id = sqlc.arg('tenant_id')
  AND status = 'submitted'
RETURNING *;

-- name: StartChange :one
-- approved → in_progress. actual_start stamped.
UPDATE changes SET
    status       = 'in_progress',
    actual_start = now()
WHERE id = sqlc.arg('id')
  AND tenant_id = sqlc.arg('tenant_id')
  AND status = 'approved'
RETURNING *;

-- name: MarkChangeSucceeded :one
-- in_progress → succeeded. actual_end stamped.
UPDATE changes SET
    status     = 'succeeded',
    actual_end = now()
WHERE id = sqlc.arg('id')
  AND tenant_id = sqlc.arg('tenant_id')
  AND status = 'in_progress'
RETURNING *;

-- name: MarkChangeFailed :one
-- in_progress → failed.
UPDATE changes SET
    status     = 'failed',
    actual_end = now()
WHERE id = sqlc.arg('id')
  AND tenant_id = sqlc.arg('tenant_id')
  AND status = 'in_progress'
RETURNING *;

-- name: MarkChangeRolledBack :one
-- {in_progress | failed} → rolled_back. We allow rollback from failed too
-- because operators sometimes mark failed first and then trigger the
-- rollback procedure separately.
UPDATE changes SET
    status     = 'rolled_back',
    actual_end = COALESCE(actual_end, now())
WHERE id = sqlc.arg('id')
  AND tenant_id = sqlc.arg('tenant_id')
  AND status IN ('in_progress', 'failed')
RETURNING *;

-- ---------------------------------------------------------------------------
-- CAB approvals (per-voter votes).
-- ---------------------------------------------------------------------------

-- name: UpsertChangeApproval :one
-- INSERT … ON CONFLICT UPDATE — voters can change their vote, but each
-- voter has at most one row per change. The audit trail is per-voter,
-- not per-vote-attempt; if you want a history of vote changes, the
-- change_comments timeline is where that lives.
INSERT INTO change_approvals (tenant_id, change_id, voter_id, vote, note)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (change_id, voter_id) DO UPDATE
    SET vote = EXCLUDED.vote,
        note = EXCLUDED.note,
        voted_at = now()
RETURNING *;

-- name: ListChangeApprovals :many
SELECT a.id, a.tenant_id, a.change_id, a.voter_id, a.vote, a.note, a.voted_at,
       u.username AS voter_username
FROM change_approvals a
LEFT JOIN users u ON u.id = a.voter_id
WHERE a.change_id = sqlc.arg('change_id')
  AND a.tenant_id = sqlc.arg('tenant_id')
ORDER BY a.voted_at ASC;

-- name: CountChangeApprovalsByVote :one
-- Helper used by the auto-approve / auto-reject logic. Returns
-- (approve_count, reject_count) for a given change.
SELECT
    COUNT(*) FILTER (WHERE vote = 'approve')::int AS approve_count,
    COUNT(*) FILTER (WHERE vote = 'reject')::int  AS reject_count
FROM change_approvals
WHERE change_id = sqlc.arg('change_id')
  AND tenant_id = sqlc.arg('tenant_id');

-- ---------------------------------------------------------------------------
-- Linkage. ON CONFLICT DO NOTHING for idempotent re-link.
-- ---------------------------------------------------------------------------

-- name: LinkChangeAsset :exec
INSERT INTO change_assets (change_id, asset_id, tenant_id)
VALUES ($1, $2, $3)
ON CONFLICT (change_id, asset_id) DO NOTHING;

-- name: UnlinkChangeAsset :exec
DELETE FROM change_assets
WHERE change_id = $1 AND asset_id = $2 AND tenant_id = $3;

-- name: ListAssetsForChange :many
SELECT a.*
FROM assets a
JOIN change_assets l ON l.asset_id = a.id
WHERE l.change_id = sqlc.arg('change_id')
  AND l.tenant_id = sqlc.arg('tenant_id')
  AND a.tenant_id = sqlc.arg('tenant_id')
  AND a.deleted_at IS NULL
ORDER BY a.name;

-- name: LinkChangeService :exec
INSERT INTO change_services (change_id, service_id, tenant_id)
VALUES ($1, $2, $3)
ON CONFLICT (change_id, service_id) DO NOTHING;

-- name: UnlinkChangeService :exec
DELETE FROM change_services
WHERE change_id = $1 AND service_id = $2 AND tenant_id = $3;

-- name: ListServicesForChange :many
SELECT s.*
FROM services s
JOIN change_services l ON l.service_id = s.id
WHERE l.change_id = sqlc.arg('change_id')
  AND l.tenant_id = sqlc.arg('tenant_id')
  AND s.tenant_id = sqlc.arg('tenant_id')
ORDER BY s.name;

-- name: LinkChangeProblem :exec
INSERT INTO change_problems (change_id, problem_id, tenant_id)
VALUES ($1, $2, $3)
ON CONFLICT (change_id, problem_id) DO NOTHING;

-- name: UnlinkChangeProblem :exec
DELETE FROM change_problems
WHERE change_id = $1 AND problem_id = $2 AND tenant_id = $3;

-- name: ListProblemsForChange :many
SELECT p.*
FROM problems p
JOIN change_problems l ON l.problem_id = p.id
WHERE l.change_id = sqlc.arg('change_id')
  AND l.tenant_id = sqlc.arg('tenant_id')
  AND p.tenant_id = sqlc.arg('tenant_id')
ORDER BY p.created_at DESC;

-- name: ListChangesForProblem :many
SELECT c.*
FROM changes c
JOIN change_problems l ON l.change_id = c.id
WHERE l.problem_id = sqlc.arg('problem_id')
  AND l.tenant_id = sqlc.arg('tenant_id')
  AND c.tenant_id = sqlc.arg('tenant_id')
ORDER BY c.created_at DESC;

-- ---------------------------------------------------------------------------
-- Comments.
-- ---------------------------------------------------------------------------

-- name: ListChangeComments :many
SELECT c.id, c.tenant_id, c.change_id, c.author_id, c.kind, c.body, c.created_at,
       u.username AS author_username
FROM change_comments c
LEFT JOIN users u ON u.id = c.author_id
WHERE c.change_id = sqlc.arg('change_id')
  AND c.tenant_id = sqlc.arg('tenant_id')
ORDER BY c.created_at ASC;

-- name: CreateChangeComment :one
INSERT INTO change_comments (tenant_id, change_id, author_id, kind, body)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;
