-- Phase 2.5: Webhook circuit breaker + DLQ + attempt tracking.
--
-- Mirrors the adapter failure-state model (see integration_adapters) but
-- scoped to outbound webhook subscriptions. Three consecutive delivery
-- failures flip disabled_at and park the payload in the DLQ so operators
-- can retry by hand without losing data.

-- Per-subscription failure state (mirrors integration_adapters shape).
ALTER TABLE webhook_subscriptions
    ADD COLUMN IF NOT EXISTS consecutive_failures INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_failure_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS disabled_at TIMESTAMPTZ;

-- Needed to find tripped subscriptions quickly for ops dashboards and
-- the "re-enable after manual fix" flow. Partial index keeps it small:
-- only a handful of broken subscriptions will ever match.
CREATE INDEX IF NOT EXISTS idx_webhook_subscriptions_disabled_at
    ON webhook_subscriptions (disabled_at)
    WHERE disabled_at IS NOT NULL;

-- Attempt tracking: every retry now inserts a NEW delivery row so the full
-- retry history is visible in webhook_deliveries instead of being squashed
-- into a single final-status row.
ALTER TABLE webhook_deliveries
    ADD COLUMN IF NOT EXISTS attempt_number INT NOT NULL DEFAULT 1;

-- Retention scan index. The existing idx_webhook_deliveries_sub_delivered
-- is leading on subscription_id, which is fine for "show me this hook's
-- history" but useless for a cross-subscription "DELETE WHERE delivered_at
-- < threshold" sweep. This index backs the daily retention DELETE.
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_delivered_at
    ON webhook_deliveries (delivered_at);

-- Dead-letter queue: when a subscription trips the circuit breaker the
-- originating payload lands here so operators can replay or audit after
-- fixing the receiver. tenant_id is REQUIRED (not nullable) — every DLQ
-- query must filter by tenant. A null tenant column would be a
-- cross-tenant leak waiting to happen.
CREATE TABLE IF NOT EXISTS webhook_deliveries_dlq (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID REFERENCES webhook_subscriptions(id) ON DELETE CASCADE,
    event_type      TEXT NOT NULL,
    payload         JSONB NOT NULL,
    last_error      TEXT NOT NULL,
    attempt_count   INT NOT NULL,
    tenant_id       UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Tenant-scoped DLQ listing for ops-admin UIs.
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_dlq_tenant
    ON webhook_deliveries_dlq (tenant_id, created_at DESC);

-- Per-subscription DLQ lookup (cascade-drop safe).
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_dlq_subscription
    ON webhook_deliveries_dlq (subscription_id);
