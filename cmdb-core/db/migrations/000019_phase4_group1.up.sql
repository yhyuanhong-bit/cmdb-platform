CREATE TABLE upgrade_rules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    asset_type  VARCHAR(50) NOT NULL,
    category    VARCHAR(30) NOT NULL,
    metric_name VARCHAR(100) NOT NULL,
    threshold   NUMERIC(10,2) NOT NULL,
    duration_days INT NOT NULL DEFAULT 7,
    priority    VARCHAR(20) NOT NULL DEFAULT 'medium',
    recommendation TEXT NOT NULL,
    enabled     BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_upgrade_rules_tenant ON upgrade_rules(tenant_id);

INSERT INTO upgrade_rules (tenant_id, asset_type, category, metric_name, threshold, duration_days, priority, recommendation) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'server', 'cpu', 'cpu_usage', 80.00, 7, 'high', 'Upgrade CPU to next generation processor'),
    ('a0000000-0000-0000-0000-000000000001', 'server', 'memory', 'memory_usage', 85.00, 7, 'high', 'Increase memory capacity'),
    ('a0000000-0000-0000-0000-000000000001', 'server', 'storage', 'disk_usage', 85.00, 7, 'critical', 'Expand storage or migrate to larger drives'),
    ('a0000000-0000-0000-0000-000000000001', 'network', 'network', 'cpu_usage', 70.00, 7, 'medium', 'Upgrade network equipment firmware or hardware')
ON CONFLICT DO NOTHING;
