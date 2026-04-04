-- name: ListAdapters :many
SELECT * FROM integration_adapters WHERE tenant_id = $1 ORDER BY name;

-- name: CreateAdapter :one
INSERT INTO integration_adapters (tenant_id, name, type, direction, endpoint, config, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING *;

-- name: ListWebhooks :many
SELECT * FROM webhook_subscriptions WHERE tenant_id = $1 ORDER BY name;

-- name: CreateWebhook :one
INSERT INTO webhook_subscriptions (tenant_id, name, url, secret, events, enabled)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;

-- name: ListDeliveries :many
SELECT * FROM webhook_deliveries WHERE subscription_id = $1 ORDER BY delivered_at DESC LIMIT $2;
