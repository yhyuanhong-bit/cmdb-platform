-- Asset Field Authorities: determines which source wins for each field
CREATE TABLE IF NOT EXISTS asset_field_authorities (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    field_name  VARCHAR(50) NOT NULL,
    source_type VARCHAR(30) NOT NULL,
    priority    INT NOT NULL,
    UNIQUE (tenant_id, field_name, source_type)
);

-- Import Conflicts: tracks field-level conflicts between sources
CREATE TABLE IF NOT EXISTS import_conflicts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    asset_id        UUID NOT NULL REFERENCES assets(id),
    source_type     VARCHAR(30) NOT NULL,
    field_name      VARCHAR(50) NOT NULL,
    current_value   TEXT,
    incoming_value  TEXT,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    resolved_by     UUID REFERENCES users(id),
    resolved_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_import_conflicts_pending
    ON import_conflicts (tenant_id, status)
    WHERE status = 'pending';

-- Discovery Tasks: tracks collector/discovery runs
CREATE TABLE IF NOT EXISTS discovery_tasks (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL REFERENCES tenants(id),
    type          VARCHAR(30) NOT NULL,
    status        VARCHAR(20) NOT NULL DEFAULT 'running',
    config        JSONB,
    stats         JSONB NOT NULL DEFAULT '{}',
    triggered_by  UUID REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at  TIMESTAMPTZ
);

-- Discovery Candidates: raw results from discovery runs
CREATE TABLE IF NOT EXISTS discovery_candidates (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id          UUID NOT NULL REFERENCES discovery_tasks(id) ON DELETE CASCADE,
    raw_data         JSONB NOT NULL,
    matched_asset_id UUID REFERENCES assets(id),
    status           VARCHAR(20) NOT NULL DEFAULT 'pending',
    reviewed_by      UUID REFERENCES users(id),
    reviewed_at      TIMESTAMPTZ
);

-- Import Jobs: tracks file import operations
CREATE TABLE IF NOT EXISTS import_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    type            VARCHAR(20) NOT NULL,
    filename        VARCHAR(200) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'parsing',
    total_rows      INT,
    processed_rows  INT NOT NULL DEFAULT 0,
    stats           JSONB NOT NULL DEFAULT '{}',
    error_details   JSONB NOT NULL DEFAULT '[]',
    uploaded_by     UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_import_jobs_tenant_status
    ON import_jobs (tenant_id, status);

-- Seed default field authorities for tw tenant
INSERT INTO asset_field_authorities (tenant_id, field_name, source_type, priority) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'serial_number', 'ipmi',   100),
    ('a0000000-0000-0000-0000-000000000001', 'serial_number', 'snmp',   80),
    ('a0000000-0000-0000-0000-000000000001', 'serial_number', 'manual', 50),
    ('a0000000-0000-0000-0000-000000000001', 'vendor',        'ipmi',   100),
    ('a0000000-0000-0000-0000-000000000001', 'vendor',        'manual', 50),
    ('a0000000-0000-0000-0000-000000000001', 'model',         'ipmi',   100),
    ('a0000000-0000-0000-0000-000000000001', 'model',         'manual', 50),
    ('a0000000-0000-0000-0000-000000000001', 'name',          'manual', 100),
    ('a0000000-0000-0000-0000-000000000001', 'status',        'manual', 100),
    ('a0000000-0000-0000-0000-000000000001', 'bia_level',     'manual', 100)
ON CONFLICT DO NOTHING;
