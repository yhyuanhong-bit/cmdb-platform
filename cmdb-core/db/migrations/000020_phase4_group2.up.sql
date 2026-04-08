CREATE TABLE user_sessions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ip_address     VARCHAR(50),
    user_agent     TEXT,
    device_type    VARCHAR(30),
    browser        VARCHAR(50),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_active_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expired_at     TIMESTAMPTZ,
    is_current     BOOLEAN NOT NULL DEFAULT false
);
CREATE INDEX idx_user_sessions_user ON user_sessions(user_id, created_at DESC);

ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_ip VARCHAR(50);

CREATE TABLE sensors (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID NOT NULL REFERENCES tenants(id),
    asset_id         UUID REFERENCES assets(id) ON DELETE SET NULL,
    name             VARCHAR(200) NOT NULL,
    type             VARCHAR(50) NOT NULL,
    location         VARCHAR(200),
    polling_interval INT NOT NULL DEFAULT 30,
    enabled          BOOLEAN NOT NULL DEFAULT true,
    status           VARCHAR(20) NOT NULL DEFAULT 'offline',
    last_heartbeat   TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sensors_tenant ON sensors(tenant_id);
CREATE INDEX idx_sensors_asset ON sensors(asset_id);
