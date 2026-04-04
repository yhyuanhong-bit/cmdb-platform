-- name: ListInventoryTasks :many
SELECT * FROM inventory_tasks
WHERE tenant_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountInventoryTasks :one
SELECT count(*) FROM inventory_tasks WHERE tenant_id = $1;

-- name: GetInventoryTask :one
SELECT * FROM inventory_tasks WHERE id = $1;

-- name: ListInventoryItems :many
SELECT * FROM inventory_items
WHERE task_id = $1
ORDER BY status, scanned_at;

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
