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
