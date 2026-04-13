CREATE TABLE inventory_tasks (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID         NOT NULL REFERENCES tenants(id),
    code              VARCHAR(100) NOT NULL UNIQUE,
    name              VARCHAR(255) NOT NULL,
    scope_location_id UUID         REFERENCES locations(id),
    status            VARCHAR(20)  NOT NULL DEFAULT 'planned',
    method            VARCHAR(50),
    planned_date      DATE,
    completed_date    DATE,
    assigned_to       UUID         REFERENCES users(id),
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_inventory_tasks_tenant_id ON inventory_tasks(tenant_id);

CREATE TABLE inventory_items (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id    UUID        NOT NULL REFERENCES inventory_tasks(id) ON DELETE CASCADE,
    asset_id   UUID        REFERENCES assets(id),
    rack_id    UUID        REFERENCES racks(id),
    expected   JSONB,
    actual     JSONB,
    status     VARCHAR(20) NOT NULL DEFAULT 'pending',
    scanned_at TIMESTAMPTZ,
    scanned_by UUID        REFERENCES users(id)
);
