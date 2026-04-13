CREATE TABLE racks (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID         NOT NULL REFERENCES tenants(id),
    location_id       UUID         NOT NULL REFERENCES locations(id),
    name              VARCHAR(255) NOT NULL,
    row_label         VARCHAR(50),
    total_u           INT          NOT NULL DEFAULT 42,
    power_capacity_kw NUMERIC(8,2),
    status            VARCHAR(20)  NOT NULL DEFAULT 'active',
    tags              TEXT[],
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_racks_tenant_id ON racks(tenant_id);
CREATE INDEX idx_racks_location_id ON racks(location_id);

CREATE TABLE assets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL REFERENCES tenants(id),
    asset_tag       VARCHAR(100) NOT NULL UNIQUE,
    property_number VARCHAR(100),
    control_number  VARCHAR(100),
    name            VARCHAR(255) NOT NULL,
    type            VARCHAR(50)  NOT NULL,
    sub_type        VARCHAR(50),
    status          VARCHAR(30)  NOT NULL DEFAULT 'inventoried',
    bia_level       VARCHAR(20)  NOT NULL DEFAULT 'normal',
    location_id     UUID         REFERENCES locations(id),
    rack_id         UUID         REFERENCES racks(id),
    vendor          VARCHAR(255),
    model           VARCHAR(255),
    serial_number   VARCHAR(255),
    attributes      JSONB        NOT NULL DEFAULT '{}',
    tags            TEXT[],
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_assets_tenant_id ON assets(tenant_id);
CREATE INDEX idx_assets_location_id ON assets(location_id);
CREATE INDEX idx_assets_rack_id ON assets(rack_id);
CREATE INDEX idx_assets_serial_number ON assets(serial_number);
CREATE INDEX idx_assets_tenant_type_subtype ON assets(tenant_id, type, sub_type);
CREATE INDEX idx_assets_tenant_status ON assets(tenant_id, status);
CREATE INDEX idx_assets_tags ON assets USING GIN(tags);
CREATE INDEX idx_assets_attributes ON assets USING GIN(attributes);

CREATE TABLE rack_slots (
    id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rack_id  UUID        NOT NULL REFERENCES racks(id) ON DELETE CASCADE,
    asset_id UUID        NOT NULL REFERENCES assets(id),
    start_u  INT         NOT NULL,
    end_u    INT         NOT NULL,
    side     VARCHAR(5)  NOT NULL DEFAULT 'front',
    UNIQUE(rack_id, start_u, side),
    CHECK (end_u >= start_u)
);
