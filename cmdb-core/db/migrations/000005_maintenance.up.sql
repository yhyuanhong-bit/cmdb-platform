CREATE TABLE work_orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL REFERENCES tenants(id),
    code            VARCHAR(100) NOT NULL UNIQUE,
    title           VARCHAR(255) NOT NULL,
    type            VARCHAR(50)  NOT NULL,
    status          VARCHAR(30)  NOT NULL DEFAULT 'draft',
    priority        VARCHAR(20)  NOT NULL DEFAULT 'medium',
    location_id     UUID         REFERENCES locations(id),
    asset_id        UUID         REFERENCES assets(id),
    requestor_id    UUID         REFERENCES users(id),
    assignee_id     UUID         REFERENCES users(id),
    description     TEXT,
    reason          TEXT,
    prediction_id   UUID,
    scheduled_start TIMESTAMPTZ,
    scheduled_end   TIMESTAMPTZ,
    actual_start    TIMESTAMPTZ,
    actual_end      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_work_orders_tenant_id ON work_orders(tenant_id);
CREATE INDEX idx_work_orders_tenant_status ON work_orders(tenant_id, status);
CREATE INDEX idx_work_orders_asset_id ON work_orders(asset_id);

CREATE TABLE work_order_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID        NOT NULL REFERENCES work_orders(id) ON DELETE CASCADE,
    action      VARCHAR(50) NOT NULL,
    from_status VARCHAR(30),
    to_status   VARCHAR(30),
    operator_id UUID        REFERENCES users(id),
    comment     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
