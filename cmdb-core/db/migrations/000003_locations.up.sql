CREATE TABLE locations (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID         NOT NULL REFERENCES tenants(id),
    name       VARCHAR(255) NOT NULL,
    name_en    VARCHAR(255),
    slug       VARCHAR(100) NOT NULL,
    level      VARCHAR(20)  NOT NULL,
    parent_id  UUID         REFERENCES locations(id),
    path       LTREE,
    status     VARCHAR(20)  NOT NULL DEFAULT 'active',
    metadata   JSONB        NOT NULL DEFAULT '{}',
    sort_order INT          NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_locations_path ON locations USING GIST(path);
CREATE INDEX idx_locations_parent_id ON locations(parent_id);
CREATE INDEX idx_locations_tenant_id ON locations(tenant_id);
CREATE INDEX idx_locations_tenant_slug ON locations(tenant_id, slug);
