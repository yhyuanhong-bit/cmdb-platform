-- BIA Assessments
CREATE TABLE bia_assessments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL REFERENCES tenants(id),
    system_name     VARCHAR(255) NOT NULL,
    system_code     VARCHAR(100) NOT NULL,
    owner           VARCHAR(255),
    bia_score       INT          NOT NULL DEFAULT 0,
    tier            VARCHAR(20)  NOT NULL DEFAULT 'normal',
    rto_hours       NUMERIC(10,2),
    rpo_minutes     NUMERIC(10,2),
    mtpd_hours      NUMERIC(10,2),
    data_compliance BOOLEAN      DEFAULT false,
    asset_compliance BOOLEAN     DEFAULT false,
    audit_compliance BOOLEAN     DEFAULT false,
    description     TEXT,
    last_assessed   TIMESTAMPTZ  DEFAULT now(),
    assessed_by     UUID         REFERENCES users(id),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_bia_assessments_tenant ON bia_assessments(tenant_id);
CREATE INDEX idx_bia_assessments_tier ON bia_assessments(tenant_id, tier);

-- BIA Scoring Rules
CREATE TABLE bia_scoring_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL REFERENCES tenants(id),
    tier_name       VARCHAR(20)  NOT NULL,
    tier_level      INT          NOT NULL,
    display_name    VARCHAR(100) NOT NULL,
    min_score       INT          NOT NULL,
    max_score       INT          NOT NULL,
    rto_threshold   NUMERIC(10,2),
    rpo_threshold   NUMERIC(10,2),
    description     TEXT,
    color           VARCHAR(20),
    icon            VARCHAR(50),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- BIA Dependencies
CREATE TABLE bia_dependencies (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL REFERENCES tenants(id),
    assessment_id   UUID         NOT NULL REFERENCES bia_assessments(id) ON DELETE CASCADE,
    asset_id        UUID         NOT NULL REFERENCES assets(id),
    dependency_type VARCHAR(50)  NOT NULL DEFAULT 'runs_on',
    criticality     VARCHAR(20)  DEFAULT 'high',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE(assessment_id, asset_id)
);
CREATE INDEX idx_bia_deps_assessment ON bia_dependencies(assessment_id);
CREATE INDEX idx_bia_deps_asset ON bia_dependencies(asset_id);
