-- 1. Inventory scan history
CREATE TABLE inventory_scan_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id     UUID NOT NULL REFERENCES inventory_items(id) ON DELETE CASCADE,
    scanned_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    scanned_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    method      VARCHAR(20) NOT NULL,
    result      VARCHAR(20) NOT NULL,
    note        TEXT
);
CREATE INDEX idx_scan_history_item_time ON inventory_scan_history(item_id, scanned_at DESC);

-- 2. Inventory notes
CREATE TABLE inventory_notes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id     UUID NOT NULL REFERENCES inventory_items(id) ON DELETE CASCADE,
    author_id   UUID REFERENCES users(id) ON DELETE SET NULL,
    severity    VARCHAR(20) NOT NULL DEFAULT 'info',
    text        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_inventory_notes_item ON inventory_notes(item_id);

-- 3. Work order comments
CREATE TABLE work_order_comments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID NOT NULL REFERENCES work_orders(id) ON DELETE CASCADE,
    author_id   UUID REFERENCES users(id) ON DELETE SET NULL,
    text        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_wo_comments_order ON work_order_comments(order_id);

-- 4. Asset dependencies (topology)
CREATE TABLE asset_dependencies (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    source_asset_id UUID NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    target_asset_id UUID NOT NULL REFERENCES assets(id) ON DELETE RESTRICT,
    dependency_type VARCHAR(50) NOT NULL DEFAULT 'depends_on',
    description     TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(source_asset_id, target_asset_id, dependency_type),
    CHECK (source_asset_id != target_asset_id)
);
CREATE INDEX idx_asset_deps_tenant ON asset_dependencies(tenant_id);
CREATE INDEX idx_asset_deps_target ON asset_dependencies(target_asset_id);

-- 5. Rack network connections
CREATE TABLE rack_network_connections (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id          UUID NOT NULL REFERENCES tenants(id),
    rack_id            UUID NOT NULL REFERENCES racks(id) ON DELETE CASCADE,
    source_port        VARCHAR(50) NOT NULL,
    connected_asset_id UUID REFERENCES assets(id),
    external_device    VARCHAR(255),
    speed              VARCHAR(20),
    status             VARCHAR(20) DEFAULT 'UP',
    vlans              INTEGER[],
    connection_type    VARCHAR(50) DEFAULT 'network',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (NOT (connected_asset_id IS NOT NULL AND external_device IS NOT NULL))
);
CREATE INDEX idx_rack_net_conn_rack ON rack_network_connections(rack_id);
CREATE INDEX idx_rack_net_conn_tenant ON rack_network_connections(tenant_id);
CREATE INDEX idx_rack_net_conn_asset ON rack_network_connections(connected_asset_id);
CREATE INDEX idx_rack_net_conn_vlans ON rack_network_connections USING GIN(vlans);
