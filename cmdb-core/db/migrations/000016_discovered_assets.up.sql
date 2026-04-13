CREATE TABLE discovered_assets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL REFERENCES tenants(id),
    source          VARCHAR(50)  NOT NULL,
    external_id     VARCHAR(255),
    hostname        VARCHAR(255),
    ip_address      VARCHAR(50),
    raw_data        JSONB        NOT NULL DEFAULT '{}',
    status          VARCHAR(20)  NOT NULL DEFAULT 'pending',
    matched_asset_id UUID        REFERENCES assets(id),
    diff_details    JSONB,
    discovered_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    reviewed_by     UUID         REFERENCES users(id),
    reviewed_at     TIMESTAMPTZ
);
CREATE INDEX idx_discovered_assets_tenant ON discovered_assets(tenant_id);
CREATE INDEX idx_discovered_assets_status ON discovered_assets(tenant_id, status);
