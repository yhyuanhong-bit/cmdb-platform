-- name: ListAdapters :many
SELECT * FROM integration_adapters WHERE tenant_id = $1 ORDER BY name;

-- name: GetAdapterByID :one
SELECT * FROM integration_adapters WHERE id = $1 AND tenant_id = $2;

-- name: CreateAdapter :one
INSERT INTO integration_adapters (tenant_id, name, type, direction, endpoint, config, config_encrypted, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING *;

-- name: UpdateAdapter :one
-- Partial update: NULL-valued params preserve the existing column value via
-- COALESCE. Callers must supply both `config` and `config_encrypted` together
-- (or neither) to keep the dual-write columns consistent.
UPDATE integration_adapters SET
    name             = COALESCE(sqlc.narg('name'), name),
    type             = COALESCE(sqlc.narg('type'), type),
    direction        = COALESCE(sqlc.narg('direction'), direction),
    endpoint         = COALESCE(sqlc.narg('endpoint'), endpoint),
    config           = COALESCE(sqlc.narg('config'), config),
    config_encrypted = COALESCE(sqlc.narg('config_encrypted'), config_encrypted),
    enabled          = COALESCE(sqlc.narg('enabled'), enabled)
WHERE id = sqlc.arg('id') AND tenant_id = sqlc.arg('tenant_id')
RETURNING *;

-- name: DeleteAdapter :exec
DELETE FROM integration_adapters WHERE id = $1 AND tenant_id = $2;

-- name: ListDuePullAdapters :many
-- Returns inbound adapters eligible to be polled right now:
-- enabled, and either never-failed (next_attempt_at IS NULL) or past their
-- backoff window. Used by the metrics puller; backoff is enforced in SQL
-- so restarts cannot silently re-poll a broken adapter.
SELECT id, tenant_id, name, type, endpoint, config, config_encrypted, consecutive_failures
FROM integration_adapters
WHERE direction = 'inbound'
  AND enabled = true
  AND (next_attempt_at IS NULL OR next_attempt_at <= now());

-- name: RecordAdapterSuccess :exec
-- Clears failure state after a successful pull.
UPDATE integration_adapters
SET consecutive_failures = 0,
    last_failure_at      = NULL,
    last_failure_reason  = NULL,
    next_attempt_at      = NULL
WHERE id = $1 AND tenant_id = $2;

-- name: RecordAdapterFailure :one
-- Atomically increments consecutive_failures, stamps the reason (truncated
-- to 500 chars by the caller via $3), and computes next_attempt_at using
-- the exponential backoff schedule (30s / 2m / 10m / 30m cap).
-- Returns the post-update row so the workflow layer can decide whether to
-- auto-disable without a follow-up SELECT (avoids TOCTOU races).
UPDATE integration_adapters
SET consecutive_failures = consecutive_failures + 1,
    last_failure_at      = now(),
    last_failure_reason  = $3,
    next_attempt_at      = now() + CASE
        WHEN consecutive_failures + 1 = 1 THEN INTERVAL '30 seconds'
        WHEN consecutive_failures + 1 = 2 THEN INTERVAL '2 minutes'
        WHEN consecutive_failures + 1 = 3 THEN INTERVAL '10 minutes'
        ELSE INTERVAL '30 minutes'
    END
WHERE id = $1 AND tenant_id = $2
RETURNING id, tenant_id, name, consecutive_failures, next_attempt_at;

-- name: DisableAdapter :exec
-- Auto-disable after threshold reached. Separate from RecordAdapterFailure
-- so the workflow can emit an audit event atomically with the decision.
UPDATE integration_adapters
SET enabled = false
WHERE id = $1 AND tenant_id = $2;

-- name: ListWebhooks :many
SELECT * FROM webhook_subscriptions WHERE tenant_id = $1 ORDER BY name;

-- name: GetWebhookByID :one
SELECT * FROM webhook_subscriptions WHERE id = $1 AND tenant_id = $2;

-- name: CreateWebhook :one
INSERT INTO webhook_subscriptions (tenant_id, name, url, secret, secret_encrypted, events, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING *;

-- name: UpdateWebhook :one
-- Partial update: NULL-valued params preserve the existing column value via
-- COALESCE. Callers must supply both `secret` and `secret_encrypted` together
-- (or neither) to keep the dual-write columns consistent.
UPDATE webhook_subscriptions SET
    name             = COALESCE(sqlc.narg('name'), name),
    url              = COALESCE(sqlc.narg('url'), url),
    secret           = COALESCE(sqlc.narg('secret'), secret),
    secret_encrypted = COALESCE(sqlc.narg('secret_encrypted'), secret_encrypted),
    events           = COALESCE(sqlc.narg('events'), events),
    enabled          = COALESCE(sqlc.narg('enabled'), enabled)
WHERE id = sqlc.arg('id') AND tenant_id = sqlc.arg('tenant_id')
RETURNING *;

-- name: DeleteWebhook :exec
DELETE FROM webhook_subscriptions WHERE id = $1 AND tenant_id = $2;

-- name: CreateDelivery :exec
INSERT INTO webhook_deliveries (subscription_id, event_type, payload, status_code, response_body)
VALUES ($1, $2, $3, $4, $5);

-- name: ListWebhooksByEvent :many
SELECT * FROM webhook_subscriptions
WHERE tenant_id = $1
  AND enabled = true
  AND $2::text = ANY(events);

-- name: ListDeliveries :many
SELECT * FROM webhook_deliveries WHERE subscription_id = $1 ORDER BY delivered_at DESC LIMIT $2;
