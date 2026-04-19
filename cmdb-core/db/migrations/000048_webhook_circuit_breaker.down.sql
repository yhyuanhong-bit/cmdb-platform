-- Reverse 000048 in opposite order of creation.

DROP INDEX IF EXISTS idx_webhook_deliveries_dlq_subscription;
DROP INDEX IF EXISTS idx_webhook_deliveries_dlq_tenant;
DROP TABLE IF EXISTS webhook_deliveries_dlq;

DROP INDEX IF EXISTS idx_webhook_deliveries_delivered_at;

ALTER TABLE webhook_deliveries
    DROP COLUMN IF EXISTS attempt_number;

DROP INDEX IF EXISTS idx_webhook_subscriptions_disabled_at;

ALTER TABLE webhook_subscriptions
    DROP COLUMN IF EXISTS disabled_at,
    DROP COLUMN IF EXISTS last_failure_at,
    DROP COLUMN IF EXISTS consecutive_failures;
