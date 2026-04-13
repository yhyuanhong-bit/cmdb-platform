CREATE TABLE audit_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL REFERENCES tenants(id),
    action      VARCHAR(50) NOT NULL,
    module      VARCHAR(30),
    target_type VARCHAR(30),
    target_id   UUID,
    operator_id UUID        REFERENCES users(id),
    diff        JSONB,
    source      VARCHAR(20) NOT NULL DEFAULT 'web',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_events_tenant_created ON audit_events(tenant_id, created_at DESC);
CREATE INDEX idx_audit_events_target ON audit_events(target_type, target_id);
CREATE INDEX idx_audit_events_operator ON audit_events(operator_id);
