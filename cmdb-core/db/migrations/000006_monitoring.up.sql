CREATE TABLE alert_rules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID         NOT NULL REFERENCES tenants(id),
    name        VARCHAR(255) NOT NULL,
    metric_name VARCHAR(100) NOT NULL,
    condition   JSONB        NOT NULL DEFAULT '{}',
    severity    VARCHAR(20)  NOT NULL,
    enabled     BOOLEAN      NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE alert_events (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID           NOT NULL REFERENCES tenants(id),
    rule_id       UUID           REFERENCES alert_rules(id),
    asset_id      UUID           REFERENCES assets(id),
    status        VARCHAR(20)    NOT NULL DEFAULT 'firing',
    severity      VARCHAR(20)    NOT NULL,
    message       TEXT,
    trigger_value NUMERIC(12,4),
    fired_at      TIMESTAMPTZ    NOT NULL DEFAULT now(),
    acked_at      TIMESTAMPTZ,
    resolved_at   TIMESTAMPTZ
);

CREATE INDEX idx_alert_events_tenant_status ON alert_events(tenant_id, status);
CREATE INDEX idx_alert_events_asset_id ON alert_events(asset_id);
CREATE INDEX idx_alert_events_tenant_fired ON alert_events(tenant_id, fired_at DESC);

CREATE TABLE incidents (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL REFERENCES tenants(id),
    title       VARCHAR(255) NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'open',
    severity    VARCHAR(20) NOT NULL,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ
);
