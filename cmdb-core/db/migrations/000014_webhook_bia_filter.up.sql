ALTER TABLE webhook_subscriptions ADD COLUMN IF NOT EXISTS filter_bia TEXT[] DEFAULT '{}';
