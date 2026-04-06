CREATE TABLE quality_rules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL REFERENCES tenants(id),
    ci_type         VARCHAR(50),
    dimension       VARCHAR(20)  NOT NULL,
    field_name      VARCHAR(50)  NOT NULL,
    rule_type       VARCHAR(20)  NOT NULL,
    rule_config     JSONB        DEFAULT '{}',
    weight          INT          DEFAULT 10,
    enabled         BOOLEAN      DEFAULT true,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE quality_scores (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL REFERENCES tenants(id),
    asset_id        UUID         NOT NULL REFERENCES assets(id),
    completeness    NUMERIC(5,2) DEFAULT 0,
    accuracy        NUMERIC(5,2) DEFAULT 0,
    timeliness      NUMERIC(5,2) DEFAULT 0,
    consistency     NUMERIC(5,2) DEFAULT 0,
    total_score     NUMERIC(5,2) DEFAULT 0,
    issue_details   JSONB        DEFAULT '[]',
    scan_date       TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_quality_scores_asset ON quality_scores(asset_id);
CREATE INDEX idx_quality_scores_date ON quality_scores(scan_date DESC);
