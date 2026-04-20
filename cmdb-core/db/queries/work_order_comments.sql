-- work_order_comments: per-work-order comment thread. The table has
-- NO tenant_id column — tenancy is inherited from the parent
-- work_orders row, which IS tenant-scoped. The pre-sqlc handlers
-- relied on the caller having been authorized for the order_id
-- before reaching the query (and the ON DELETE CASCADE means the
-- comment row cannot outlive the order). Behavior preserved here.

-- name: ListWorkOrderComments :many
-- cross-tenant: no tenant_id; order_id is the isolation boundary.
-- The LEFT JOIN on users is purely for the display name — author_id
-- is nullable (ON DELETE SET NULL) so display_name collapses to the
-- pre-migration "*string" shape downstream.
SELECT wc.id, u.display_name AS author_name, wc.text, wc.created_at
FROM work_order_comments wc
LEFT JOIN users u ON wc.author_id = u.id
WHERE wc.order_id = $1
ORDER BY wc.created_at ASC;

-- name: CreateWorkOrderComment :exec
-- cross-tenant: no tenant_id. Caller is expected to own order_id
-- authorization. author_id is the RBAC-checked user_id from the
-- request context.
INSERT INTO work_order_comments (id, order_id, author_id, text, created_at)
VALUES ($1, $2, $3, $4, now());
