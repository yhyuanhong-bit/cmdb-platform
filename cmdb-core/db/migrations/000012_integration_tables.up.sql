CREATE TABLE IF NOT EXISTS integration_adapters (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    name        VARCHAR(100) NOT NULL,
    type        VARCHAR(30) NOT NULL,
    direction   VARCHAR(20) NOT NULL,
    endpoint    VARCHAR(500),
    config      JSONB DEFAULT '{}',
    enabled     BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS webhook_subscriptions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    name        VARCHAR(100) NOT NULL,
    url         VARCHAR(500) NOT NULL,
    secret      VARCHAR(200),
    events      TEXT[] NOT NULL,
    enabled     BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID NOT NULL REFERENCES webhook_subscriptions(id),
    event_type      VARCHAR(50) NOT NULL,
    payload         JSONB NOT NULL,
    status_code     INT,
    response_body   TEXT,
    delivered_at    TIMESTAMPTZ DEFAULT now()
);

INSERT INTO integration_adapters (tenant_id, name, type, direction, endpoint, enabled) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'Prometheus Metrics', 'rest', 'inbound', 'http://prometheus:9090/api/v1', true),
    ('a0000000-0000-0000-0000-000000000001', 'SNMP Poller', 'snmp', 'inbound', '192.0.2.0/24', false),
    ('a0000000-0000-0000-0000-000000000001', 'ServiceNow ITSM', 'rest', 'bidirectional', 'https://instance.service-now.com/api', false)
ON CONFLICT DO NOTHING;

INSERT INTO webhook_subscriptions (tenant_id, name, url, events, enabled) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'Slack Alerts', 'https://hooks.slack.com/services/xxx', '{alert.fired,alert.resolved}', true),
    ('a0000000-0000-0000-0000-000000000001', 'Teams Notifications', 'https://outlook.office.com/webhook/xxx', '{maintenance.order_created}', false)
ON CONFLICT DO NOTHING;
