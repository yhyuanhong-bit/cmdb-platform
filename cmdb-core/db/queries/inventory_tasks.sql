-- name: ListInventoryTasks :many
SELECT * FROM inventory_tasks
WHERE tenant_id = $1
  AND deleted_at IS NULL
  AND (sqlc.narg('scope_location_id')::uuid IS NULL OR scope_location_id = sqlc.narg('scope_location_id'))
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountInventoryTasks :one
SELECT count(*) FROM inventory_tasks
WHERE tenant_id = $1
  AND deleted_at IS NULL
  AND (sqlc.narg('scope_location_id')::uuid IS NULL OR scope_location_id = sqlc.narg('scope_location_id'));

-- name: GetInventoryTask :one
SELECT * FROM inventory_tasks WHERE id = $1 AND tenant_id = $2;

-- name: ListInventoryItems :many
SELECT * FROM inventory_items
WHERE task_id = $1
ORDER BY status, scanned_at
LIMIT $2 OFFSET $3;

-- name: CountInventoryItems :one
SELECT count(*) FROM inventory_items WHERE task_id = $1;

-- name: CreateInventoryTask :one
INSERT INTO inventory_tasks (tenant_id, code, name, scope_location_id, status, method, planned_date, assigned_to)
VALUES ($1, $2, $3, $4, 'planned', $5, $6, $7)
RETURNING *;

-- name: CompleteInventoryTask :one
UPDATE inventory_tasks SET status = 'completed', completed_date = now()
WHERE id = $1
RETURNING *;

-- name: ScanInventoryItem :one
UPDATE inventory_items SET
    actual = $2,
    status = $3,
    scanned_at = now(),
    scanned_by = $4
WHERE id = $1
RETURNING *;

-- name: GetInventorySummary :one
SELECT
    count(*) as total,
    count(*) FILTER (WHERE status = 'scanned') as scanned,
    count(*) FILTER (WHERE status = 'pending') as pending,
    count(*) FILTER (WHERE status = 'discrepancy') as discrepancy
FROM inventory_items
WHERE task_id = $1;

-- name: UpdateInventoryTask :one
UPDATE inventory_tasks
SET name = COALESCE(NULLIF(sqlc.narg('name')::varchar, ''), name),
    planned_date = COALESCE(sqlc.narg('planned_date')::date, planned_date),
    assigned_to = COALESCE(sqlc.narg('assigned_to')::uuid, assigned_to)
WHERE id = $1 AND tenant_id = $2 AND status != 'completed' AND deleted_at IS NULL
RETURNING *;

-- name: CreateInventoryItem :one
INSERT INTO inventory_items (task_id, asset_id, rack_id, expected, status)
VALUES ($1, $2, $3, $4, 'pending')
RETURNING *;

-- name: ActivateInventoryTask :one
UPDATE inventory_tasks SET status = 'in_progress'
WHERE id = $1 AND status = 'planned'
RETURNING *;

-- name: SoftDeleteInventoryTask :exec
UPDATE inventory_tasks
SET deleted_at = NOW()
WHERE id = $1 AND tenant_id = $2 AND status = 'planned' AND deleted_at IS NULL;
