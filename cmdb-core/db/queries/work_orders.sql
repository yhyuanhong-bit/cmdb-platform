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

-- name: GetWorkOrder :one
SELECT * FROM work_orders WHERE id = $1 AND tenant_id = $2;

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
WHERE id = $1
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
WHERE id = sqlc.arg('id')
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
    approved_at = now(),
    approved_by = $2,
    sla_deadline = $3,
    status = 'approved',
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkSLAWarning :exec
UPDATE work_orders SET sla_warning_sent = true WHERE id = $1;

-- name: MarkSLABreached :exec
UPDATE work_orders SET sla_breached = true WHERE id = $1;

-- name: ListOverdueSLAOrders :many
SELECT * FROM work_orders
WHERE status = 'in_progress'
  AND sla_deadline IS NOT NULL
  AND sla_deadline < now()
  AND sla_breached = false
  AND deleted_at IS NULL;

-- name: ListSLAWarningOrders :many
SELECT * FROM work_orders
WHERE status IN ('approved', 'in_progress')
  AND sla_deadline IS NOT NULL
  AND sla_warning_sent = false
  AND sla_deadline - (sla_deadline - approved_at) * 0.25 < now()
  AND deleted_at IS NULL;
