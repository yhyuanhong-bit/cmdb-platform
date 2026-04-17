ALTER TABLE integration_adapters DROP COLUMN IF EXISTS config_encrypted;
ALTER TABLE webhook_subscriptions DROP COLUMN IF EXISTS secret_encrypted;
