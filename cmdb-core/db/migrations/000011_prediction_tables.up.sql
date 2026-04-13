CREATE TABLE IF NOT EXISTS prediction_models (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL,
    type        VARCHAR(30) NOT NULL,
    provider    VARCHAR(30) NOT NULL,
    config      JSONB NOT NULL DEFAULT '{}',
    enabled     BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS prediction_results (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id),
    model_id            UUID NOT NULL REFERENCES prediction_models(id),
    asset_id            UUID NOT NULL REFERENCES assets(id),
    prediction_type     VARCHAR(30) NOT NULL,
    result              JSONB NOT NULL,
    severity            VARCHAR(20),
    recommended_action  TEXT,
    expires_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_prediction_results_asset ON prediction_results (asset_id);
CREATE INDEX IF NOT EXISTS idx_prediction_results_tenant ON prediction_results (tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS rca_analyses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    incident_id     UUID NOT NULL REFERENCES incidents(id),
    model_id        UUID REFERENCES prediction_models(id),
    reasoning       JSONB NOT NULL,
    conclusion_asset_id UUID REFERENCES assets(id),
    confidence      NUMERIC(3,2),
    human_verified  BOOLEAN DEFAULT false,
    verified_by     UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ DEFAULT now()
);

INSERT INTO prediction_models (id, name, type, provider, config, enabled) VALUES
    ('20000000-0000-0000-0000-000000000001', 'Default RCA', 'rca', 'dify',
     '{"base_url": "http://dify:3000", "api_key": "change-me", "workflow_id": "rca-v1"}', false)
ON CONFLICT DO NOTHING;
