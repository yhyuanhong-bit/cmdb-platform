-- Dual-write rollout: new *_encrypted columns coexist with existing plaintext for one release; readers prefer encrypted and fall back.
ALTER TABLE integration_adapters ADD COLUMN IF NOT EXISTS config_encrypted BYTEA;
ALTER TABLE webhook_subscriptions ADD COLUMN IF NOT EXISTS secret_encrypted BYTEA;
