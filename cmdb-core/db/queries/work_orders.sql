-- name: ListWorkOrders :many
SELECT * FROM work_orders
WHERE tenant_id = $1
  AND deleted_at IS NULL
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('asset_id')::uuid IS NULL OR asset_id = sqlc.narg('asset_id'))
  AND (sqlc.narg('location_id')::uuid IS NULL OR location_id = sqlc.narg('location_id'))
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountWorkOrders :one
SELECT count(*) FROM work_orders
WHERE tenant_id = $1
  AND deleted_at IS NULL
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('asset_id')::uuid IS NULL OR asset_id = sqlc.narg('asset_id'))
  AND (sqlc.narg('location_id')::uuid IS NULL OR location_id = sqlc.narg('location_id'));

-- name: CountPendingWorkOrders :one
-- Counts work orders in the approval-gated states (submitted or approved)
-- that have not yet moved to in_progress. Used by the dashboard "pending
-- work orders" tile so operators can see the backlog waiting on action.
SELECT count(*) FROM work_orders
WHERE tenant_id = $1
  AND deleted_at IS NULL
  AND status IN ('submitted', 'approved');

-- name: GetWorkOrder :one
SELECT * FROM work_orders WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- name: CreateWorkOrder :one
INSERT INTO work_orders (
    tenant_id, code, title, type, status, priority,
    location_id, asset_id, requestor_id, assignee_id,
    description, reason, prediction_id,
    scheduled_start, scheduled_end
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10,
    $11, $12, $13,
    $14, $15
) RETURNING *;

-- name: UpdateWorkOrderStatus :one
UPDATE work_orders SET
    status     = $2,
    updated_at = now()
WHERE id = $1 AND tenant_id = $3 AND status = $4 AND deleted_at IS NULL
RETURNING *;

-- name: CreateWorkOrderLog :one
INSERT INTO work_order_logs (
    order_id, action, from_status, to_status,
    operator_id, comment
) VALUES (
    $1, $2, $3, $4,
    $5, $6
) RETURNING *;

-- name: UpdateWorkOrder :one
UPDATE work_orders SET
    title           = COALESCE(sqlc.narg('title'), title),
    description     = COALESCE(sqlc.narg('description'), description),
    priority        = COALESCE(sqlc.narg('priority'), priority),
    assignee_id     = COALESCE(sqlc.narg('assignee_id'), assignee_id),
    scheduled_start = COALESCE(sqlc.narg('scheduled_start'), scheduled_start),
    scheduled_end   = COALESCE(sqlc.narg('scheduled_end'), scheduled_end),
    updated_at      = now()
WHERE id = sqlc.arg('id') AND tenant_id = sqlc.arg('tenant_id') AND deleted_at IS NULL
RETURNING *;

-- name: ListWorkOrderLogs :many
SELECT * FROM work_order_logs
WHERE order_id = $1
ORDER BY created_at;

-- name: SoftDeleteWorkOrder :exec
UPDATE work_orders
SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- name: StampWorkOrderApproval :one
UPDATE work_orders SET
    approved_at       = now(),
    approved_by       = $2,
    sla_deadline      = $3,
    status            = 'approved',
    governance_status = 'approved',
    updated_at        = now()
WHERE id = $1 AND tenant_id = $4 AND governance_status = 'submitted' AND deleted_at IS NULL
RETURNING *;

-- name: MarkSLAWarning :exec
UPDATE work_orders SET sla_warning_sent = true WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- name: MarkSLABreached :exec
UPDATE work_orders SET sla_breached = true WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL;

-- name: BreachSLAWorkOrders :many
-- Atomic cross-tenant SLA breach sweep: flips sla_breached=true and
-- RETURNs every row that transitioned, in one round-trip. The
-- `NOT sla_breached` guard inside the UPDATE is what makes this
-- TOCTOU-safe: a second concurrent caller finds the already-flipped
-- flag and gets zero rows back. Called by the scheduler only; has no
-- tenant filter by design (see workflows/sla.go).
UPDATE work_orders
SET sla_breached = true, updated_at = now()
WHERE tenant_id IS NOT NULL
  AND status IN ('approved', 'in_progress')
  AND sla_deadline IS NOT NULL
  AND sla_deadline < now()
  AND NOT sla_breached
  AND deleted_at IS NULL
RETURNING id, tenant_id, code, assignee_id;

-- name: ListOverdueSLAOrders :many
SELECT * FROM work_orders
WHERE tenant_id = $1
  AND status IN ('approved', 'in_progress')
  AND sla_deadline IS NOT NULL
  AND sla_deadline < now()
  AND sla_breached = false
  AND deleted_at IS NULL;

-- name: ListSLAWarningOrders :many
SELECT * FROM work_orders
WHERE tenant_id = $1
  AND status IN ('approved', 'in_progress')
  AND sla_deadline IS NOT NULL
  AND sla_warning_sent = false
  AND approved_at IS NOT NULL
  AND sla_deadline - (sla_deadline - approved_at) * 0.25 < now()
  AND deleted_at IS NULL;

-- name: UpdateExecutionStatus :one
UPDATE work_orders SET
    execution_status = $2,
    status           = $3,
    updated_at       = now()
WHERE id = $1 AND tenant_id = $4 AND execution_status = $5 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateGovernanceStatus :one
UPDATE work_orders SET
    governance_status = $2,
    status            = $3,
    updated_at        = now()
WHERE id = $1 AND tenant_id = $4 AND governance_status = $5 AND deleted_at IS NULL
RETURNING *;

-- name: AssignWorkOrder :one
-- Reassigns a work order to a different operator. Unlike UpdateWorkOrder
-- which restricts edits to submitted/rejected, reassignment is a workflow
-- action that must work on in-flight states too — submitted, rejected,
-- approved, and in_progress. Completed and verified orders stay immutable.
-- 0 rows returned = order missing, cross-tenant, soft-deleted, or in a
-- frozen state; the caller distinguishes these via a follow-up GetWorkOrder.
UPDATE work_orders SET
    assignee_id = $2,
    updated_at  = now()
WHERE id = $1
  AND tenant_id = $3
  AND status IN ('submitted', 'rejected', 'approved', 'in_progress')
  AND deleted_at IS NULL
RETURNING *;

-- name: TransitionEmergencyWorkOrder :one
-- Atomically approves-and-starts an emergency work order in a single UPDATE.
-- Replaces the previous two-step flow (approve, then in_progress) which could
-- strand a WO half-approved if the process crashed, timed out, or retried
-- between the two statements.
--
-- Guards (all required, all enforced by the WHERE clause):
--   - tenant_id: defense-in-depth isolation
--   - type = 'emergency': only emergency orders skip the approval queue
--   - governance_status = 'submitted': idempotent; a second caller gets 0 rows
--   - execution_status = 'pending': prevents double-start
--   - deleted_at IS NULL: don't resurrect tombstoned rows
--
-- 0 rows returned = idempotent success (already transitioned, stale retry, or
-- wrong type/tenant). The caller treats pgx.ErrNoRows as (nil, nil).
UPDATE work_orders SET
    governance_status = 'approved',
    execution_status  = 'working',
    status            = 'in_progress',
    approved_at       = now(),
    approved_by       = $3,
    sla_deadline      = $4,
    updated_at        = now()
WHERE id = $1
  AND tenant_id = $2
  AND type = 'emergency'
  AND governance_status = 'submitted'
  AND execution_status = 'pending'
  AND deleted_at IS NULL
RETURNING *;
