-- name: ListWorkOrders :many
SELECT * FROM work_orders
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('asset_id')::uuid IS NULL OR asset_id = sqlc.narg('asset_id'))
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountWorkOrders :one
SELECT count(*) FROM work_orders
WHERE tenant_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('asset_id')::uuid IS NULL OR asset_id = sqlc.narg('asset_id'));

-- name: GetWorkOrder :one
SELECT * FROM work_orders WHERE id = $1;

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
