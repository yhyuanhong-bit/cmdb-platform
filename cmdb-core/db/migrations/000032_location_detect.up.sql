-- switch_port_mapping: maps switch ports to rack locations
CREATE TABLE IF NOT EXISTS switch_port_mapping (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    switch_asset_id UUID NOT NULL REFERENCES assets(id),
    port_name VARCHAR(50) NOT NULL,
    connected_rack_id UUID REFERENCES racks(id),
    connected_u_position INT,
    description TEXT,
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(tenant_id, switch_asset_id, port_name)
);

CREATE INDEX IF NOT EXISTS idx_switch_port_rack ON switch_port_mapping(connected_rack_id);

-- asset_location_history: tracks all location changes
CREATE TABLE IF NOT EXISTS asset_location_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    asset_id UUID NOT NULL REFERENCES assets(id),
    from_rack_id UUID REFERENCES racks(id),
    to_rack_id UUID REFERENCES racks(id),
    detected_by VARCHAR(20) NOT NULL,
    work_order_id UUID REFERENCES work_orders(id),
    detected_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_location_history_asset ON asset_location_history(asset_id, detected_at DESC);
CREATE INDEX IF NOT EXISTS idx_location_history_tenant ON asset_location_history(tenant_id, detected_at DESC);

-- mac_address_cache: latest known MAC locations from SNMP scans
CREATE TABLE IF NOT EXISTS mac_address_cache (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    mac_address VARCHAR(17) NOT NULL,
    switch_asset_id UUID NOT NULL REFERENCES assets(id),
    port_name VARCHAR(50) NOT NULL,
    vlan_id INT,
    asset_id UUID REFERENCES assets(id),
    detected_rack_id UUID REFERENCES racks(id),
    first_seen TIMESTAMPTZ DEFAULT now(),
    last_seen TIMESTAMPTZ DEFAULT now(),
    UNIQUE(tenant_id, mac_address)
);

CREATE INDEX IF NOT EXISTS idx_mac_cache_asset ON mac_address_cache(asset_id);
CREATE INDEX IF NOT EXISTS idx_mac_cache_switch ON mac_address_cache(switch_asset_id, port_name);
