-- name: CreateNotification :one
INSERT INTO notifications (tenant_id, user_id, type, title, body, resource_type, resource_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListUnreadNotifications :many
SELECT * FROM notifications
WHERE user_id = $1 AND is_read = false
ORDER BY created_at DESC
LIMIT $2;

-- name: MarkNotificationRead :exec
UPDATE notifications SET is_read = true WHERE id = $1 AND user_id = $2;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications SET is_read = true WHERE user_id = $1 AND is_read = false;

-- name: CountUnreadNotifications :one
SELECT count(*) FROM notifications WHERE user_id = $1 AND is_read = false;
