-- Add ip_address column to assets
ALTER TABLE assets ADD COLUMN IF NOT EXISTS ip_address VARCHAR(50);
CREATE INDEX IF NOT EXISTS idx_assets_ip_address ON assets(tenant_id, ip_address);

-- Migrate existing IP data from attributes JSONB
UPDATE assets SET ip_address = attributes->>'ip_address'
WHERE attributes->>'ip_address' IS NOT NULL AND ip_address IS NULL;

-- Credentials table
CREATE TABLE IF NOT EXISTS credentials (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    name        VARCHAR(100) NOT NULL,
    type        VARCHAR(30) NOT NULL,
    params      BYTEA NOT NULL,
    created_by  UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, name)
);

-- Scan targets table
CREATE TABLE IF NOT EXISTS scan_targets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    name            VARCHAR(200) NOT NULL,
    cidrs           TEXT[] NOT NULL,
    collector_type  VARCHAR(30) NOT NULL,
    credential_id   UUID NOT NULL REFERENCES credentials(id),
    mode            VARCHAR(20) NOT NULL DEFAULT 'smart',
    location_id     UUID REFERENCES locations(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_scan_targets_tenant ON scan_targets(tenant_id);
CREATE INDEX IF NOT EXISTS idx_credentials_tenant ON credentials(tenant_id);
