# CMDB Platform Backend Technical Stack & Architecture Design

> **Date**: 2026-04-03
> **Status**: Approved
> **Scope**: Backend architecture, tech stack, data import engine, AI integration, multi-tenant isolation, federated deployment

---

## Table of Contents

1. [Decision Summary](#1-decision-summary)
2. [Overall Architecture & Tech Stack](#2-overall-architecture--tech-stack)
3. [Multi-Tenant Isolation & Data Model](#3-multi-tenant-isolation--data-model)
4. [Data Ingestion Engine](#4-data-ingestion-engine)
5. [Go Core Module Structure & AI Interface Layer](#5-go-core-module-structure--ai-interface-layer)
6. [Deployment Architecture & Cross-IDC Federation](#6-deployment-architecture--cross-idc-federation)
7. [Observability & Security](#7-observability--security)
8. [Resource Estimation](#8-resource-estimation)
9. [Implementation Phases](#9-implementation-phases)

---

## 1. Decision Summary

| Decision Point | Choice |
|----------------|--------|
| Architecture | **Go + Python Dual-Engine** (Modular Monolith + Ingestion Engine) |
| Scale | Large multi-tenant, 10+ IDCs, 100K+ assets |
| Data Sources | All: IPMI/SNMP/vCenter/K8s/Excel/ITSM/Monitoring systems |
| Conflict Resolution | Authoritative source auto-merge + conflict queue for manual approval |
| Tenant Isolation | Campus-level primary isolation + department-level permission control |
| Time-Series Storage | Tiered: raw 30d -> 5min aggregate 6mo -> 1hr aggregate 2yr |
| AI Integration | **Pluggable**: MCP Server + LLM Adapter + Model Registry + Webhook, platform operators choose freely |

---

## 2. Overall Architecture & Tech Stack

### 2.1 System Layer Architecture

```
                        +----------------------------+
                        |     Frontend (React 19)     |
                        |     Existing cmdb-demo      |
                        +--------------+-------------+
                                       | HTTPS
                        +--------------v-------------+
                        |      API Gateway (Go)       |
                        |  - JWT Auth + Token Refresh  |
                        |  - Tenant Identification     |
                        |  - Rate Limiting             |
                        |  - Request ID Injection      |
                        |  - CORS / Compression        |
                        +---+--------------------+----+
                  REST/WS   |                    | gRPC
         +------------------v----+    +----------v-----------+
         |   cmdb-core (Go)      |    | ingestion-engine     |
         |                       |    | (Python)             |
         | +-------------------+ |    |                      |
         | | Asset Module      | |    | - Collector Manager  |
         | | Topology Module   | |    | - Protocol Adapters  |
         | | Maintenance Mod   | |    | - Excel/CSV Parser   |
         | | Monitoring Mod    | |    | - Conflict Resolver  |
         | | Inventory Mod     | |    | - Scheduler (APSch)  |
         | | Audit Module      | |    | - Transform Engine   |
         | | Identity Module   | |    |                      |
         | +-------------------+ |    +----------+-----------+
         |                       |               |
         | +-------------------+ |               |
         | | MCP Server        | |               |
         | | WebSocket Hub     | |               |
         | | Webhook Dispatch  | |               |
         | +-------------------+ |               |
         +---+------+-----------+               |
             |      |                            |
    +--------v------v----------------------------v----+
    |           NATS JetStream (Event Bus)             |
    |  Subjects: asset.*, alert.*, import.*,           |
    |           audit.*, maintenance.*                  |
    +---+----------------+-------------------+---------+
        |                |                   |
+-------v-------+ +-----v--------+ +--------v------+
| PostgreSQL 17 | | TimescaleDB  | |    Redis 7.x  |
|               | | (PG Extension)| |               |
| - Business    | | - Time-series| | - Session     |
| - Audit logs  | | - PUE/Power  | | - Cache       |
| - Tenant RLS  | | - Alert hist | | - Lock        |
| - Location    | | - Downsampled| | - Pub/Sub     |
|   ltree       | | - Continuous | | - Queue       |
|               | |   Aggregates | |               |
+---------------+ +--------------+ +---------------+
```

### 2.2 Complete Tech Stack

| Layer | Technology | Version | Purpose |
|-------|-----------|---------|---------|
| **Go Core** | Go | 1.23+ | Primary language |
| | Gin | 1.10+ | HTTP framework |
| | sqlc | 1.27+ | Type-safe SQL -> Go code generation |
| | golang-migrate | 4.x | Database migrations |
| | Wire | 0.6+ | Compile-time dependency injection |
| | zap | 1.27+ | Structured logging |
| | otel-go | 1.x | OpenTelemetry tracing |
| | mcp-go | latest | MCP Server SDK |
| | nats.go | 1.x | NATS client |
| | go-redis | 9.x | Redis client |
| | casbin | 2.x | RBAC + multi-tenant permission engine |
| **Python Engine** | Python | 3.12+ | Ingestion engine language |
| | FastAPI | 0.115+ | Management API (task status/config) |
| | Celery | 5.4+ | Distributed task queue |
| | APScheduler | 3.10+ | Scheduled collection |
| | pysnmp | 7.x | SNMP v2c/v3 |
| | python-redfish | 3.x | Redfish/IPMI |
| | pyvmomi | 8.x | vCenter SDK |
| | kubernetes | 31.x | K8s API client |
| | openpyxl | 3.1+ | Excel read/write |
| | nats-py | 2.x | NATS client |
| | grpcio | 1.68+ | gRPC client/server |
| **Storage** | PostgreSQL | 17 | Primary business DB (ltree for hierarchy) |
| | TimescaleDB | 2.17+ | Time-series data (PG extension, same instance) |
| | Redis | 7.4+ | Cache / distributed lock / queue |
| | NATS JetStream | 2.10+ | Event bus / cross-IDC sync |
| **Infrastructure** | Docker | 27+ | Containerization |
| | K8s / Docker Compose | — | Orchestration (K8s for large, Compose for medium) |
| | Nginx | 1.27+ | Reverse proxy / frontend static |
| | Protobuf | 3.x | gRPC interface definitions |
| **Observability** | OpenTelemetry | — | Traces + Metrics |
| | Grafana + Loki | — | Log aggregation + dashboards |
| | Jaeger | — | Distributed tracing UI |

### 2.3 Key Technology Choices Rationale

| Choice | Rationale |
|--------|-----------|
| **sqlc over GORM** | 100K+ asset queries need precise SQL control; sqlc generates type-safe code with zero reflection overhead |
| **PostgreSQL ltree** | Location tree hierarchy (Country->Region->City->Campus->IDC) with native ancestor/descendant queries, 10x faster than recursive CTEs |
| **TimescaleDB over standalone InfluxDB** | Same PG instance extension; business data and time-series can JOIN; only one DB system to manage |
| **NATS JetStream over Kafka** | Lightweight, native multi-IDC Leaf Node federation, simple deployment, sufficient for 100K event throughput |
| **Casbin** | Mature multi-tenant RBAC engine supporting campus-level + department-level dual isolation policies |

---

## 3. Multi-Tenant Isolation & Data Model

### 3.1 Tenant Isolation Strategy

Two-level isolation using **Row-Level Security (RLS)** + **application-layer dual-dimension filtering**. No database sharding — keeps operations simple.

```
+-------------------------------------------------------+
|                 Tenant Isolation Model                  |
|                                                        |
|  Level 1: Campus (IDC) -- Primary Isolation            |
|  +-----------+  +-----------+  +-----------+           |
|  | Taipei    |  | Shanghai  |  | Tokyo     |           |
|  | tenant:tw |  | tenant:sh |  | tenant:jp |           |
|  +-----+-----+  +-----+-----+  +-----+-----+          |
|        |              |              |                  |
|  Level 2: Department -- Permission Control              |
|  +-----v-----+  +-----v-----+  +-----v-----+          |
|  | Ops Dept  |  | Net Dept  |  | Facility  |           |
|  | dept:ops  |  | dept:net  |  | dept:fac  |           |
|  | See: all  |  | See: net  |  | See: rack |           |
|  | assets    |  | devices   |  | + power   |           |
|  +-----------+  +-----------+  +-----------+           |
+-------------------------------------------------------+
```

**Implementation:**

```sql
-- Every business table carries tenant_id
-- PostgreSQL RLS enforces isolation at database level

ALTER TABLE assets ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON assets
  USING (tenant_id = current_setting('app.current_tenant')::uuid);

-- Go middleware sets context at request start:
-- SET LOCAL app.current_tenant = '<tenant_id>';
-- SET LOCAL app.current_dept = '<dept_id>';
```

**Casbin Permission Model (RBAC with domains):**

```ini
# model.conf
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _, _    # user, role, domain(tenant)

[matchers]
m = g(r.sub, p.sub, r.dom) && r.dom == p.dom && keyMatch2(r.obj, p.obj) && r.act == p.act
```

```csv
# Policy examples
# Taipei campus ops admin -- full asset access
p, role:ops-admin, tenant:tw, /api/v1/assets/*, *
p, role:ops-admin, tenant:tw, /api/v1/maintenance/*, *

# Taipei campus network dept -- read-only network devices
p, role:net-viewer, tenant:tw, /api/v1/assets/*, GET
# Application layer adds: sub_type IN ('switch','router','firewall')

# Cross-campus super admin
p, role:super-admin, *, /api/v1/*, *
```

### 3.2 Core Database Schema

```sql
-- ============================================
-- Foundation: Tenants & Identity
-- ============================================

CREATE TABLE tenants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL,
    slug        VARCHAR(50) UNIQUE NOT NULL,    -- 'tw', 'sh', 'jp'
    status      VARCHAR(20) DEFAULT 'active',
    settings    JSONB DEFAULT '{}',             -- tenant-level config
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE departments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    name        VARCHAR(100) NOT NULL,
    slug        VARCHAR(50) NOT NULL,
    permissions JSONB DEFAULT '{}',             -- visible asset types/scope
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE users (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL REFERENCES tenants(id),
    dept_id      UUID REFERENCES departments(id),
    username     VARCHAR(50) UNIQUE NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    email        VARCHAR(200),
    phone        VARCHAR(30),
    password_hash VARCHAR(200) NOT NULL,
    status       VARCHAR(20) DEFAULT 'active',  -- active | disabled
    source       VARCHAR(20) DEFAULT 'local',   -- local | ldap | sso
    created_at   TIMESTAMPTZ DEFAULT now(),
    updated_at   TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID REFERENCES tenants(id),    -- NULL = global role
    name        VARCHAR(50) NOT NULL,
    description TEXT,
    permissions JSONB NOT NULL DEFAULT '{}',
    is_system   BOOLEAN DEFAULT false,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

-- ============================================
-- Topology: Location Hierarchy Tree
-- ============================================

CREATE EXTENSION IF NOT EXISTS ltree;

CREATE TABLE locations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    name        VARCHAR(100) NOT NULL,
    name_en     VARCHAR(100),
    slug        VARCHAR(50) NOT NULL,
    level       VARCHAR(20) NOT NULL,           -- country | region | city | campus | idc
    parent_id   UUID REFERENCES locations(id),
    path        LTREE NOT NULL,                 -- 'tw.north.taipei.neihu'
    status      VARCHAR(20) DEFAULT 'active',
    metadata    JSONB DEFAULT '{}',             -- PUE, coordinates, area, etc.
    sort_order  INT DEFAULT 0,
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_locations_path ON locations USING GIST (path);
CREATE INDEX idx_locations_parent ON locations (parent_id);
CREATE INDEX idx_locations_tenant ON locations (tenant_id);

-- ============================================
-- Core: Asset (CI)
-- ============================================

CREATE TABLE assets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    asset_tag       VARCHAR(50) UNIQUE NOT NULL,   -- SRV-PROD-001
    property_number VARCHAR(50),
    control_number  VARCHAR(50),
    name            VARCHAR(200) NOT NULL,
    type            VARCHAR(50) NOT NULL,           -- server | network | storage | power
    sub_type        VARCHAR(50),                    -- rack_server | switch | ups
    status          VARCHAR(30) NOT NULL DEFAULT 'inventoried',
    -- inventoried -> deployed -> operational -> maintenance -> decommissioned
    bia_level       VARCHAR(20) DEFAULT 'normal',   -- critical | important | normal | minor
    location_id     UUID REFERENCES locations(id),
    rack_id         UUID,                           -- FK added after racks table
    vendor          VARCHAR(100),
    model           VARCHAR(100),
    serial_number   VARCHAR(100),
    attributes      JSONB DEFAULT '{}',             -- flexible extension fields
    tags            TEXT[] DEFAULT '{}',
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_assets_tenant ON assets (tenant_id);
CREATE INDEX idx_assets_location ON assets (location_id);
CREATE INDEX idx_assets_rack ON assets (rack_id);
CREATE INDEX idx_assets_serial ON assets (serial_number);
CREATE INDEX idx_assets_type ON assets (tenant_id, type, sub_type);
CREATE INDEX idx_assets_status ON assets (tenant_id, status);
CREATE INDEX idx_assets_tags ON assets USING GIN (tags);
CREATE INDEX idx_assets_attrs ON assets USING GIN (attributes);

-- Asset field authority source configuration
CREATE TABLE asset_field_authorities (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    field_name  VARCHAR(50) NOT NULL,           -- 'serial_number', 'vendor', 'model'
    source_type VARCHAR(30) NOT NULL,           -- 'ipmi', 'snmp', 'manual', 'vcenter'
    priority    INT NOT NULL DEFAULT 0,         -- higher = more authoritative
    UNIQUE (tenant_id, field_name, source_type)
);

-- ============================================
-- Racks & Slots
-- ============================================

CREATE TABLE racks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    location_id     UUID NOT NULL REFERENCES locations(id),
    name            VARCHAR(50) NOT NULL,
    row_label       VARCHAR(10),
    total_u         INT NOT NULL DEFAULT 42,
    power_capacity_kw NUMERIC(8,2),
    status          VARCHAR(20) DEFAULT 'active',
    tags            TEXT[] DEFAULT '{}',
    created_at      TIMESTAMPTZ DEFAULT now()
);

ALTER TABLE assets ADD CONSTRAINT fk_assets_rack
    FOREIGN KEY (rack_id) REFERENCES racks(id);

CREATE TABLE rack_slots (
    id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rack_id  UUID NOT NULL REFERENCES racks(id) ON DELETE CASCADE,
    asset_id UUID NOT NULL REFERENCES assets(id),
    start_u  INT NOT NULL,
    end_u    INT NOT NULL,
    side     VARCHAR(5) DEFAULT 'front',        -- front | back
    UNIQUE (rack_id, start_u, side),
    CHECK (end_u >= start_u)
);

-- ============================================
-- Maintenance Work Orders
-- ============================================

CREATE TABLE work_orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    code            VARCHAR(30) UNIQUE NOT NULL,    -- WO-2026-0001
    title           VARCHAR(200) NOT NULL,
    type            VARCHAR(30) NOT NULL,           -- repair | inspection | replacement | upgrade
    status          VARCHAR(30) NOT NULL DEFAULT 'draft',
    -- draft -> pending -> approved -> in_progress -> completed -> closed
    -- draft -> pending -> rejected
    priority        VARCHAR(20) DEFAULT 'medium',
    location_id     UUID REFERENCES locations(id),
    asset_id        UUID REFERENCES assets(id),
    requestor_id    UUID REFERENCES users(id),
    assignee_id     UUID REFERENCES users(id),
    description     TEXT,
    reason          TEXT,
    prediction_id   UUID,                           -- linked AI prediction (optional)
    scheduled_start TIMESTAMPTZ,
    scheduled_end   TIMESTAMPTZ,
    actual_start    TIMESTAMPTZ,
    actual_end      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE work_order_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID NOT NULL REFERENCES work_orders(id) ON DELETE CASCADE,
    action      VARCHAR(30) NOT NULL,               -- created | transitioned | assigned | commented
    from_status VARCHAR(30),
    to_status   VARCHAR(30),
    operator_id UUID REFERENCES users(id),
    comment     TEXT,
    created_at  TIMESTAMPTZ DEFAULT now()
);

-- ============================================
-- Monitoring & Alerts
-- ============================================

CREATE TABLE alert_rules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    name        VARCHAR(100) NOT NULL,
    metric_name VARCHAR(100) NOT NULL,
    condition   JSONB NOT NULL,                     -- {"op": ">", "threshold": 85}
    severity    VARCHAR(20) NOT NULL,               -- critical | warning | info
    enabled     BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE alert_events (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL REFERENCES tenants(id),
    rule_id       UUID REFERENCES alert_rules(id),
    asset_id      UUID REFERENCES assets(id),       -- direct FK, no indirect ci_id lookup
    status        VARCHAR(20) NOT NULL DEFAULT 'firing',  -- firing | acknowledged | resolved
    severity      VARCHAR(20) NOT NULL,
    message       TEXT NOT NULL,
    trigger_value NUMERIC(12,4),
    fired_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    acked_at      TIMESTAMPTZ,
    resolved_at   TIMESTAMPTZ
);

CREATE INDEX idx_alerts_tenant_status ON alert_events (tenant_id, status);
CREATE INDEX idx_alerts_asset ON alert_events (asset_id);

CREATE TABLE incidents (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    title       VARCHAR(200) NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'open',
    severity    VARCHAR(20) NOT NULL,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ
);

-- Time-series metrics (TimescaleDB hypertable)
CREATE TABLE metrics (
    time      TIMESTAMPTZ NOT NULL,
    asset_id  UUID NOT NULL,
    tenant_id UUID NOT NULL,
    name      VARCHAR(100) NOT NULL,               -- cpu_usage, power_kw, temperature
    value     DOUBLE PRECISION NOT NULL,
    labels    JSONB DEFAULT '{}'
);

SELECT create_hypertable('metrics', 'time');

-- Tiered retention policies
-- Raw data: 30 days
SELECT add_retention_policy('metrics', INTERVAL '30 days');

-- Mid-term: 5-minute aggregates, 6 months
CREATE MATERIALIZED VIEW metrics_5min
WITH (timescaledb.continuous) AS
SELECT time_bucket('5 minutes', time) AS bucket,
       asset_id, tenant_id, name,
       avg(value) AS avg_val,
       max(value) AS max_val,
       min(value) AS min_val
FROM metrics
GROUP BY bucket, asset_id, tenant_id, name;

SELECT add_retention_policy('metrics_5min', INTERVAL '180 days');

-- Long-term: 1-hour aggregates, 2 years
CREATE MATERIALIZED VIEW metrics_1hour
WITH (timescaledb.continuous) AS
SELECT time_bucket('1 hour', time) AS bucket,
       asset_id, tenant_id, name,
       avg(value) AS avg_val,
       max(value) AS max_val,
       min(value) AS min_val
FROM metrics
GROUP BY bucket, asset_id, tenant_id, name;

SELECT add_retention_policy('metrics_1hour', INTERVAL '730 days');

-- ============================================
-- Inventory
-- ============================================

CREATE TABLE inventory_tasks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    code            VARCHAR(30) UNIQUE NOT NULL,
    name            VARCHAR(200) NOT NULL,
    scope_location_id UUID NOT NULL REFERENCES locations(id),
    status          VARCHAR(20) DEFAULT 'planned',  -- planned | in_progress | completed | cancelled
    method          VARCHAR(20) NOT NULL,           -- barcode | rfid | manual
    planned_date    DATE NOT NULL,
    completed_date  DATE,
    assigned_to     UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE inventory_items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id     UUID NOT NULL REFERENCES inventory_tasks(id) ON DELETE CASCADE,
    asset_id    UUID REFERENCES assets(id),
    rack_id     UUID REFERENCES racks(id),
    expected    JSONB NOT NULL,                     -- expected state from system
    actual      JSONB,                             -- actual scan result
    status      VARCHAR(20) DEFAULT 'pending',     -- pending | scanned | discrepancy | resolved
    scanned_at  TIMESTAMPTZ,
    scanned_by  UUID REFERENCES users(id)
);

-- ============================================
-- Audit
-- ============================================

CREATE TABLE audit_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    action      VARCHAR(50) NOT NULL,               -- asset.created, rack.updated, order.transitioned
    module      VARCHAR(30) NOT NULL,               -- asset, rack, maintenance, identity
    target_type VARCHAR(30) NOT NULL,               -- asset, rack, work_order, user
    target_id   UUID NOT NULL,
    operator_id UUID NOT NULL REFERENCES users(id),
    diff        JSONB DEFAULT '{}',                 -- {"field": {"old": x, "new": y}}
    source      VARCHAR(20) DEFAULT 'web',          -- web | api | import | system
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_audit_tenant ON audit_events (tenant_id, created_at DESC);
CREATE INDEX idx_audit_target ON audit_events (target_type, target_id);

-- ============================================
-- AI Prediction
-- ============================================

CREATE TABLE prediction_models (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL,
    type        VARCHAR(30) NOT NULL,               -- failure_prediction | anomaly_detection | capacity_planning
    provider    VARCHAR(30) NOT NULL,               -- builtin | dify | openai | custom
    config      JSONB NOT NULL DEFAULT '{}',
    enabled     BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE prediction_results (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id),
    model_id            UUID NOT NULL REFERENCES prediction_models(id),
    asset_id            UUID NOT NULL REFERENCES assets(id),
    prediction_type     VARCHAR(30) NOT NULL,
    result              JSONB NOT NULL,
    severity            VARCHAR(20),
    recommended_action  TEXT,
    expires_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE rca_analyses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    incident_id     UUID NOT NULL REFERENCES incidents(id),
    model_id        UUID REFERENCES prediction_models(id),
    reasoning       JSONB NOT NULL,
    conclusion_asset_id UUID REFERENCES assets(id),
    confidence      NUMERIC(3,2),                   -- 0.00 ~ 1.00
    human_verified  BOOLEAN DEFAULT false,
    verified_by     UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- ============================================
-- Integration
-- ============================================

CREATE TABLE integration_adapters (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    name        VARCHAR(100) NOT NULL,
    type        VARCHAR(30) NOT NULL,               -- rest | grpc | snmp | webhook
    direction   VARCHAR(10) NOT NULL,               -- inbound | outbound | bidirectional
    endpoint    VARCHAR(500),
    config      JSONB DEFAULT '{}',
    enabled     BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE webhook_subscriptions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    name        VARCHAR(100) NOT NULL,
    url         VARCHAR(500) NOT NULL,
    secret      VARCHAR(200),
    events      TEXT[] NOT NULL,                     -- '{asset.created, alert.fired}'
    enabled     BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE webhook_deliveries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID NOT NULL REFERENCES webhook_subscriptions(id),
    event_type      VARCHAR(50) NOT NULL,
    payload         JSONB NOT NULL,
    status_code     INT,
    response_body   TEXT,
    delivered_at    TIMESTAMPTZ DEFAULT now()
);

-- ============================================
-- Data Import: Conflict Approval Queue
-- ============================================

CREATE TABLE import_conflicts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    asset_id        UUID NOT NULL REFERENCES assets(id),
    source_type     VARCHAR(30) NOT NULL,           -- ipmi | snmp | excel | vcenter
    field_name      VARCHAR(50) NOT NULL,
    current_value   TEXT,
    incoming_value  TEXT,
    status          VARCHAR(20) DEFAULT 'pending',  -- pending | approved | rejected | auto_resolved
    resolved_by     UUID REFERENCES users(id),
    resolved_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_conflicts_pending ON import_conflicts (tenant_id, status)
    WHERE status = 'pending';

-- ============================================
-- Data Import: Discovery & Import Jobs
-- ============================================

CREATE TABLE discovery_tasks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    type            VARCHAR(30) NOT NULL,           -- network_scan | vcenter_sync | itsm_import
    status          VARCHAR(20) DEFAULT 'running',  -- running | pending_review | completed | failed
    config          JSONB NOT NULL,                 -- scan scope, parameters
    stats           JSONB DEFAULT '{}',             -- {total: 120, matched: 105, new: 15}
    triggered_by    UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ DEFAULT now(),
    completed_at    TIMESTAMPTZ
);

CREATE TABLE discovery_candidates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id         UUID NOT NULL REFERENCES discovery_tasks(id) ON DELETE CASCADE,
    raw_data        JSONB NOT NULL,                 -- scanned raw data
    matched_asset_id UUID REFERENCES assets(id),    -- matched existing asset (NULL = new)
    status          VARCHAR(20) DEFAULT 'pending',  -- pending | approved | rejected | merged
    reviewed_by     UUID REFERENCES users(id),
    reviewed_at     TIMESTAMPTZ
);

CREATE TABLE import_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    type            VARCHAR(20) NOT NULL,           -- excel | csv | api_batch
    filename        VARCHAR(200),
    status          VARCHAR(20) DEFAULT 'parsing',  -- parsing | previewing | confirmed | processing | completed | failed
    total_rows      INT,
    processed_rows  INT DEFAULT 0,
    stats           JSONB DEFAULT '{}',             -- {created: 10, updated: 5, conflicts: 2, errors: 1}
    error_details   JSONB DEFAULT '[]',             -- [{row: 5, field: "serial", error: "..."}]
    uploaded_by     UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ DEFAULT now(),
    completed_at    TIMESTAMPTZ
);
```

### 3.3 Key Data Model Decisions

| Decision | Approach | Rationale |
|----------|----------|-----------|
| Primary Key | UUID v7 | Ordered, distributed-safe, B-tree friendly |
| Tenant Isolation | `tenant_id` + RLS | Single-DB multi-tenant, simple ops, RLS prevents app-layer leaks |
| Location Tree | ltree | Ancestor/descendant queries O(1), 10x faster than recursive CTEs |
| Flexible Fields | JSONB `attributes` | Different asset types have different fields; GIN index supports queries |
| Alert Linkage | `alert_events.asset_id` direct FK | Solves frontend serialNumber -> asset_id indirect lookup |
| Conflict Handling | `import_conflicts` + `asset_field_authorities` | Authority source auto-merge + non-authority conflicts enter approval queue |
| Time-Series Tiering | TimescaleDB continuous aggregates | Raw 30d -> 5min agg 6mo -> 1hr agg 2yr |

---

## 4. Data Ingestion Engine

### 4.1 Three Import Modes Overview

```
+-------------------------------------------------------------+
|              Ingestion Engine (Python)                        |
|                                                              |
|  +--------------------------------------------------------+ |
|  |            Collector Manager (Scheduling Core)          | |
|  |  - Register/Start/Stop Collectors                       | |
|  |  - Cron + Event-Driven dual scheduling                  | |
|  |  - Concurrency control + Retry strategy                 | |
|  +------+----------------+----------------+---------------+  |
|         |                |                |                  |
|  +------v------+  +------v------+  +------v------+          |
|  |  Automatic  |  | Semi-Auto   |  |  Manual     |          |
|  |  Collectors |  | Collectors  |  |  Importers  |          |
|  |             |  |             |  |             |          |
|  | IPMI/Redfish|  | Auto-Discov |  | Excel/CSV   |          |
|  | SNMP Poller |  | vCenter Sync|  | Web Form    |          |
|  | K8s Watcher |  | ITSM Bridge |  | API Upload  |          |
|  | Prom Bridge |  | Scan+Confirm|  | Template DL |          |
|  +------+------+  +------+------+  +------+------+          |
|         |                |                |                  |
|  +------v----------------v----------------v--------------+   |
|  |              Transform Pipeline                       |   |
|  |                                                       |   |
|  |  Raw Data -> Normalize -> Deduplicate -> Validate     |   |
|  |  -> Authority Check -> Merge or Queue Conflict        |   |
|  +----------------------------+--------------------------+   |
|                               |                              |
|  +----------------------------v--------------------------+   |
|  |              Conflict Resolver                        |   |
|  |                                                       |   |
|  |  Authoritative field -> Auto-merge, write audit_event |   |
|  |  Non-authoritative   -> Enter import_conflicts queue  |   |
|  |  Manual approval     -> Merge and record decision     |   |
|  +----------------------------+--------------------------+   |
|                               |                              |
|                    NATS publish                               |
|              asset.updated / import.completed                 |
+--------------------------------------------------------------+
```

### 4.2 Automatic Mode (Unattended Collectors)

No human intervention. Cron-scheduled or event-driven. Runs continuously.

**Collector Interface:**

```python
class CollectorProtocol(Protocol):
    """Every data source implements this interface"""

    name: str                           # 'ipmi', 'snmp', 'vcenter', 'k8s', 'prometheus'
    collect_type: str                   # 'full_sync' | 'incremental' | 'event_driven'

    async def collect(self, target: CollectTarget) -> list[RawAssetData]:
        """Collect raw data"""
        ...

    async def test_connection(self, target: CollectTarget) -> ConnectionResult:
        """Test connectivity"""
        ...

    def supported_fields(self) -> list[FieldMapping]:
        """Declare which fields this collector provides"""
        ...
```

**Collector Responsibilities:**

| Collector | Method | Frequency | Data |
|-----------|--------|-----------|------|
| **IPMI / Redfish** | HTTP REST to BMC | Every 5 min | Serial, temp, power, model, firmware |
| **SNMP Poller** | SNMPv2c/v3 Walk | Every 10 min | Interface status, traffic, MAC/ARP, model |
| **K8s Watcher** | Watch API (event-driven) | Real-time | Node/Pod status, resources, labels |
| **Prometheus Bridge** | PromQL HTTP API | Every 1 min | Metrics -> TimescaleDB metrics table |

**Per-Tenant Schedule Config (stored in integration_adapters.config):**

```yaml
schedules:
  - collector: ipmi
    targets:
      - endpoint: "10.134.143.0/24"
        credentials_ref: "vault://ipmi-creds-tw"
    cron: "*/5 * * * *"
    timeout_seconds: 120
    concurrency: 20

  - collector: snmp
    targets:
      - endpoint: "10.134.144.0/24"
        community: "vault://snmp-community-tw"
        version: "v2c"
    cron: "*/10 * * * *"
    timeout_seconds: 60
    concurrency: 50

  - collector: k8s
    targets:
      - kubeconfig_ref: "vault://k8s-config-tw"
        namespaces: ["production", "staging"]
    mode: "watch"
```

### 4.3 Semi-Automatic Mode

System discovers/syncs proactively, but humans confirm key steps.

**Example Flow: vCenter VM Sync**

1. Scheduled scan of vCenter (automatic) -> discovers 50 VMs, 8 are new
2. Auto-match against existing assets (automatic) -> 42 matched, update CPU/Memory
3. 8 new VMs enter "pending confirmation" queue (manual) -> admin confirms department, BIA level, rack location
4. After confirmation, auto-create Asset records + audit logs

**Example Flow: Auto Discovery**

1. User specifies IP range, triggers scan -> `POST /api/v1/discovery/scan`
2. System runs ICMP + Port Scan + SNMP Probe (automatic) -> discovers 120 active IPs
3. Cross-reference with existing assets (automatic) -> 105 registered, 15 unknown
4. 15 unknown devices enter "pending claim" queue (manual) -> admin confirms whether to manage

### 4.4 Manual Mode

User-driven data entry through UI.

**Excel/CSV Batch Import Flow:**

1. Download template: `GET /import/templates/{type}`
2. Upload filled file: `POST /import/upload`
3. System validates -> shows preview + error row highlights
4. User confirms: `POST /import/{id}/confirm`
5. Backend async processing -> progress updates via WebSocket

**Other Manual Methods:**

- Web Form single entry -> standard `POST /assets` CRUD
- API batch write -> `POST /import/batch` (max 1000 items/request, supports `dry_run`)

### 4.5 Transform Pipeline (Unified)

All data from any mode flows through the same pipeline:

```
Raw Data
    |
    v
+-----------+    +------------+    +------------+
| Normalize |----> Deduplicate|----> Validate   |
|           |    |            |    |            |
| - Field   |    | - serial   |    | - Required |
|   mapping |    |   dedup    |    | - Format   |
| - Type    |    | - asset_tag|    | - Referenc |
|   convert |    |   match    |    |   integrity|
| - Unit    |    |            |    | - Business |
|   unify   |    |            |    |   rules    |
+-----------+    +------------+    +-----+------+
                                        |
                                        v
                               +--------+---------+
                               | Authority Check  |
                               |                  |
                               | Query asset_field|
                               | _authorities     |
                               |                  |
                               | Is this field    |
                               | authoritative    |
                               | from this source?|
                               +----+--------+----+
                                    |        |
                               YES  |        | NO
                                    v        v
                             +--------+  +------------+
                             | Auto   |  | Same value?|
                             | merge  |  |            |
                             | + audit|  | YES->skip  |
                             | event  |  | NO ->queue |
                             +--------+  |   conflict |
                                         +------------+
```

### 4.6 Ingestion Engine Management API (FastAPI)

```
GET    /ingestion/collectors              # List all collectors and status
POST   /ingestion/collectors/{name}/start # Start collector
POST   /ingestion/collectors/{name}/stop  # Stop collector
POST   /ingestion/collectors/{name}/test  # Test connectivity

GET    /ingestion/schedules               # List schedule configs
PUT    /ingestion/schedules/{id}          # Update schedule

POST   /ingestion/discovery/scan          # Trigger semi-auto scan
GET    /ingestion/discovery/{id}          # View scan results
POST   /ingestion/discovery/{id}/approve  # Batch approve candidates

POST   /ingestion/import/upload           # Upload Excel/CSV
GET    /ingestion/import/{id}/preview     # Preview parsed results
POST   /ingestion/import/{id}/confirm     # Confirm and execute import
GET    /ingestion/import/{id}/progress    # Query import progress
GET    /ingestion/import/templates/{type} # Download import template

GET    /ingestion/conflicts               # Pending conflict list
POST   /ingestion/conflicts/{id}/resolve  # Resolve single conflict
POST   /ingestion/conflicts/batch-resolve # Batch resolve
```

### 4.7 Three Modes Comparison

| Dimension | Automatic | Semi-Automatic | Manual |
|-----------|-----------|---------------|--------|
| **Trigger** | Cron / Event Watch | User triggers scan, system executes | User upload/fill |
| **Human Involvement** | Conflict approval only | New device confirm + conflict approval | Full manual, system validates |
| **Data Sources** | IPMI, SNMP, K8s, Prometheus | vCenter, ITSM, Network Scan | Excel, CSV, Web Form, API |
| **Frequency** | Minute-level continuous | On-demand (daily/weekly) | On-demand |
| **Use Case** | Hardware status, metrics, real-time alerts | New device onboarding, system migration | Initial import, manual ledger corrections |
| **Error Handling** | Auto-retry + alert notification | Failed items marked for review | Error rows highlighted, user corrects and re-uploads |

---

## 5. Go Core Module Structure & AI Interface Layer

### 5.1 Go Project Structure (Modular Monolith)

```
cmdb-core/
+-- cmd/
|   +-- server/
|       +-- main.go                     # Entry: start HTTP + gRPC + MCP
|
+-- internal/
|   +-- config/
|   |   +-- config.go                   # Env vars + YAML config parsing
|   |
|   +-- middleware/
|   |   +-- auth.go                     # JWT parse + token refresh
|   |   +-- tenant.go                   # SET LOCAL app.current_tenant
|   |   +-- rbac.go                     # Casbin permission check
|   |   +-- audit.go                    # Auto audit log recording
|   |   +-- ratelimit.go               # Tenant-level rate limiting
|   |   +-- requestid.go              # Request ID injection
|   |   +-- recovery.go               # Panic recovery + error formatting
|   |
|   +-- domain/                         # ===== Business Modules =====
|   |   |
|   |   +-- asset/
|   |   |   +-- model.go               # Asset struct + state machine
|   |   |   +-- repository.go          # Interface definition
|   |   |   +-- repository_pg.go       # PostgreSQL impl (sqlc generated)
|   |   |   +-- service.go             # Business logic
|   |   |   +-- handler.go             # HTTP handler (Gin)
|   |   |   +-- events.go              # Domain event definitions
|   |   |
|   |   +-- topology/
|   |   |   +-- model.go               # Location + Rack + RackSlot
|   |   |   +-- repository.go
|   |   |   +-- repository_pg.go       # ltree queries
|   |   |   +-- service.go             # Hierarchy traversal + stats aggregation
|   |   |   +-- handler.go
|   |   |
|   |   +-- maintenance/
|   |   |   +-- model.go               # WorkOrder + state machine
|   |   |   +-- repository.go
|   |   |   +-- repository_pg.go
|   |   |   +-- service.go             # State transitions + log recording
|   |   |   +-- handler.go
|   |   |   +-- statemachine.go        # draft->pending->approved->in_progress->completed
|   |   |
|   |   +-- monitoring/
|   |   |   +-- model.go               # AlertRule + AlertEvent + Incident
|   |   |   +-- repository.go
|   |   |   +-- repository_pg.go
|   |   |   +-- metrics_repo.go        # TimescaleDB read/write
|   |   |   +-- service.go
|   |   |   +-- handler.go
|   |   |
|   |   +-- inventory/
|   |   |   +-- model.go               # InventoryTask + InventoryItem + Discrepancy
|   |   |   +-- repository.go
|   |   |   +-- repository_pg.go
|   |   |   +-- service.go             # Scan flow + diff comparison
|   |   |   +-- handler.go
|   |   |
|   |   +-- audit/
|   |   |   +-- model.go               # AuditEvent
|   |   |   +-- repository.go
|   |   |   +-- repository_pg.go
|   |   |   +-- service.go
|   |   |   +-- handler.go
|   |   |   +-- collector.go           # Receives events from middleware, async batch write
|   |   |
|   |   +-- identity/
|   |   |   +-- model.go               # User + Role + Tenant + Department
|   |   |   +-- repository.go
|   |   |   +-- repository_pg.go
|   |   |   +-- service.go             # User mgmt + role assignment
|   |   |   +-- handler.go
|   |   |   +-- auth_service.go        # Login + JWT issue + Refresh
|   |   |
|   |   +-- prediction/
|   |   |   +-- model.go               # PredictionModel + Result + RCA
|   |   |   +-- repository.go
|   |   |   +-- repository_pg.go
|   |   |   +-- service.go             # AI provider routing
|   |   |   +-- handler.go
|   |   |
|   |   +-- integration/
|   |       +-- model.go               # Adapter + Webhook + Delivery
|   |       +-- repository.go
|   |       +-- repository_pg.go
|   |       +-- service.go
|   |       +-- handler.go
|   |       +-- webhook_dispatcher.go  # Async delivery + retry
|   |
|   +-- eventbus/                       # ===== Event Bus =====
|   |   +-- bus.go                      # Interface: Publish / Subscribe
|   |   +-- nats.go                     # NATS JetStream implementation
|   |   +-- subjects.go                # Event subject constants
|   |
|   +-- platform/                       # ===== Infrastructure =====
|   |   +-- database/
|   |   |   +-- postgres.go            # PG connection pool + RLS setup
|   |   |   +-- migrate.go            # golang-migrate integration
|   |   +-- cache/
|   |   |   +-- redis.go              # Redis client + common ops wrapper
|   |   +-- telemetry/
|   |       +-- tracing.go            # OpenTelemetry tracer
|   |       +-- metrics.go            # Prometheus metrics
|   |       +-- logging.go            # zap logger config
|   |
|   +-- mcp/                            # ===== MCP Server =====
|   |   +-- server.go                  # MCP Server main entry
|   |   +-- tools.go                   # Tool definitions & registration
|   |   +-- resources.go              # Resource exposure
|   |
|   +-- ai/                            # ===== AI Adapter Layer =====
|       +-- provider.go                # Unified interface
|       +-- dify.go                    # Dify Adapter
|       +-- openai.go                  # OpenAI / Claude Adapter
|       +-- custom.go                  # Custom model HTTP Adapter
|       +-- registry.go               # Provider registry
|
+-- api/
|   +-- proto/                          # gRPC interface definitions (Go<->Python)
|   |   +-- ingestion.proto
|   |   +-- common.proto
|   +-- openapi/
|       +-- spec.yaml                  # OpenAPI 3.1 doc (auto-generated)
|
+-- db/
|   +-- migrations/                    # SQL migration files
|   |   +-- 000001_init_tenants.up.sql
|   |   +-- 000002_init_locations.up.sql
|   |   +-- 000003_init_assets.up.sql
|   |   +-- ...
|   +-- queries/                       # sqlc query definitions
|       +-- assets.sql
|       +-- locations.sql
|       +-- racks.sql
|       +-- ...
|
+-- sqlc.yaml
+-- wire.go                            # Wire DI bindings
+-- Dockerfile
+-- Makefile
+-- go.mod
```

### 5.2 Inter-Module Communication Rules

Modules do **not** call each other directly. Two communication methods:

**Synchronous queries (read-only):** Via Repository interfaces.
- AssetService needs Location info -> injects `topology.LocationRepository` (interface)
- No dependency on `topology.Service` or concrete implementation

**Asynchronous events (write-triggered side effects):** Via EventBus.
- Asset status change -> publish `asset.status_changed`
- Monitoring subscribes: update alert status
- Audit subscribes: record audit log
- Integration subscribes: trigger webhook
- Prediction subscribes: trigger re-prediction

**Event Subscription Matrix:**

| Event | Audit | Monitoring | Maintenance | Integration | Prediction |
|-------|-------|------------|-------------|-------------|------------|
| `asset.created` | Log | -- | -- | Webhook | -- |
| `asset.status_changed` | Log | Update alerts | Auto-close order | Webhook | Re-predict |
| `alert.fired` | Log | -- | Auto-create order (opt) | Webhook | Trigger RCA |
| `maintenance.order_transitioned` | Log | -- | -- | Webhook | -- |
| `import.conflict_created` | -- | -- | -- | Notify admin | -- |
| `prediction.created` | Log | -- | Suggest order | Webhook | -- |

### 5.3 MCP Server Design

Enables any AI Agent (Dify / Claude / custom) to query CMDB data.

**MCP Tools (callable by AI agents):**

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `search_assets` | Search assets by type, status, location, tags | type, status, location, query, limit |
| `get_asset_detail` | Full asset info with alerts, maintenance, predictions | asset_id, include_alerts, include_maintenance |
| `query_alerts` | Query alert events by severity, status, time range | severity, status, asset_id, time_range |
| `get_topology` | Location topology with asset distribution stats | location_path, depth, include_stats |
| `query_metrics` | Time-series metrics (CPU, temp, power, PUE) | asset_id, metric_name, time_range, aggregation |
| `query_work_orders` | Maintenance work orders by status/priority | status, priority, asset_id |
| `trigger_rca` | Trigger root cause analysis for an incident | incident_id, context |

**MCP Resources (static knowledge):**

| URI | Description |
|-----|-------------|
| `cmdb://schema/asset-types` | All asset type/subtype definitions and field descriptions |
| `cmdb://schema/severity-levels` | Alert severity levels and SLA requirements |
| `cmdb://topology/tree` | Complete location hierarchy structure |

### 5.4 AI Adapter Layer (Pluggable)

**Unified Provider Interface:**

```go
type AIProvider interface {
    Name() string
    Type() string   // "llm" | "ml_model" | "workflow"
    PredictFailure(ctx context.Context, req PredictionRequest) (*PredictionResult, error)
    AnalyzeRootCause(ctx context.Context, req RCARequest) (*RCAResult, error)
    HealthCheck(ctx context.Context) error
}
```

**Available Adapters:**

| Adapter | Provider Type | Use Case |
|---------|--------------|----------|
| `DifyProvider` | workflow | AI workflow orchestration; Dify calls back CMDB MCP Server for context |
| `LLMProvider` | llm | Direct LLM API (Claude / OpenAI / local models); OpenAI-compatible endpoint |
| `CustomModelProvider` | ml_model | Self-built inference service (Flask/FastAPI); time-series prediction models |

**Provider Registry:** Dynamically loads from `prediction_models` DB table at startup. Platform operators configure models via UI, system auto-registers the corresponding adapter.

**AI Integration Flow:**

```
Platform operator configures in prediction_models table:
  - Model: "Server Failure Prediction" / Provider: dify / Config: {...}
  - Model: "Network RCA" / Provider: claude / Config: {...}
  - Model: "Capacity Planning" / Provider: custom / Config: {...}
          |
          v
  AI Provider Registry (Go) -- dynamically loads providers
          |
     +----+----+----+
     v         v         v
  Dify      LLM API    Custom
  Workflow   (Claude/   ML Model
  Engine     OpenAI)    (FastAPI)
     |
     | MCP Client (Dify callback)
     v
  cmdb-core MCP Server
  (search_assets, query_alerts, query_metrics, ...)
```

### 5.5 gRPC Interface (Go <-> Python)

```protobuf
syntax = "proto3";
package cmdb.ingestion;

service IngestionService {
    // Python -> Go: Submit collected data
    rpc SubmitCollectedData (SubmitRequest) returns (SubmitResponse);

    // Python -> Go: Batch submit (high throughput)
    rpc SubmitBatch (stream SubmitRequest) returns (SubmitResponse);

    // Go -> Python: Trigger discovery scan
    rpc TriggerDiscovery (DiscoveryRequest) returns (DiscoveryResponse);

    // Go -> Python: Query collector status
    rpc GetCollectorStatus (CollectorStatusRequest) returns (CollectorStatusResponse);

    // Go -> Python: Start/stop collector
    rpc ControlCollector (ControlRequest) returns (ControlResponse);
}

message SubmitRequest {
    string tenant_id = 1;
    string source_type = 2;
    string collector_id = 3;
    repeated RawAssetData items = 4;
}

message RawAssetData {
    string unique_key = 1;
    map<string, string> fields = 2;
    string attributes_json = 3;
    int64 collected_at = 4;
}

message SubmitResponse {
    int32 accepted = 1;
    int32 conflicts = 2;
    int32 errors = 3;
    repeated string error_details = 4;
}
```

### 5.6 WebSocket Real-Time Push

Frontend pages (Dashboard, Monitoring) receive real-time updates via WebSocket.

**Connection:** `ws://host/api/v1/ws?token=<jwt>`

**Push Events:**

| Event | Target Page | Content |
|-------|-------------|---------|
| `alert.fired` | MonitoringAlerts | New alert details |
| `asset.status_changed` | Dashboard | Updated stats |
| `import.progress` | Import UI | Progress percentage |
| `discovery.found` | Auto Discovery | New device notification |
| `conflict.created` | Conflict Queue | Pending approval count |

Server filters events by `tenant_id` from JWT claims.

---

## 6. Deployment Architecture & Cross-IDC Federation

### 6.1 Deployment Topology

```
+------------------------------------------------------------------+
|                    Central Control Plane                           |
|                    (Cloud or HQ IDC)                              |
|                                                                   |
|  +----------+ +----------+ +----------+ +--------------------+   |
|  |cmdb-core | |cmdb-core | |ingestion | |   NATS Server      |   |
|  | (Go) x3  | | MCP Srv  | | (Py) x2  | |   (Hub Mode)       |   |
|  | replicas | |          | | replicas | |                    |   |
|  +----+-----+ +----+-----+ +----+-----+ +--------+-----------+   |
|       |             |            |                 |              |
|  +----v-------------v------------v------+  +-------v----------+  |
|  |    PostgreSQL 17 (Primary)           |  | Redis Sentinel   |  |
|  |    + TimescaleDB                     |  | (3-node)         |  |
|  |    + Streaming Replica x2            |  |                  |  |
|  +--------------------------------------+  +------------------+  |
|                                                                   |
|  +--------------------------------------------------------------+|
|  | Grafana + Loki + Jaeger + Prometheus (Observability)          ||
|  +--------------------------------------------------------------+|
+-----------------------------+------------------------------------+
                              |
                NATS Leaf Node (TLS encrypted)
                Cross-IDC event sync
                              |
         +--------------------+--------------------+
         |                    |                    |
         v                    v                    v
+-----------------+ +-----------------+ +-----------------+
|  Edge Node      | |  Edge Node      | |  Edge Node      |
|  Taipei IDC     | |  Shanghai IDC   | |  Tokyo IDC      |
|                 | |                 | |                 |
| +-------------+ | | +-------------+ | | +-------------+ |
| |cmdb-core x1 | | | |cmdb-core x1 | | | |cmdb-core x1 | |
| |(Go, read    | | | |(Go, read    | | | |(Go, read    | |
| | + local     | | | | + local     | | | | + local     | |
| | write buf)  | | | | write buf)  | | | | write buf)  | |
| +-------------+ | | +-------------+ | | +-------------+ |
| |ingestion x1 | | | |ingestion x1 | | | |ingestion x1 | |
| |(Python,     | | | |(Python,     | | | |(Python,     | |
| | local coll) | | | | local coll) | | | | local coll) | |
| +-------------+ | | +-------------+ | | +-------------+ |
| |NATS Leaf    | | | |NATS Leaf    | | | |NATS Leaf    | |
| |Node         | | | |Node         | | | |Node         | |
| +-------------+ | | +-------------+ | | +-------------+ |
| |PG Read      | | | |PG Read      | | | |PG Read      | |
| |Replica (opt)| | | |Replica (opt)| | | |Replica (opt)| |
| +-------------+ | | +-------------+ | | +-------------+ |
| |Redis x1     | | | |Redis x1     | | | |Redis x1     | |
| |(local cache)| | | |(local cache)| | | |(local cache)| |
| +-------------+ | | +-------------+ | | +-------------+ |
+-----------------+ +-----------------+ +-----------------+
```

### 6.2 Federation Sync Mechanism

**Upstream (Edge -> Central):**

- Collected data: ingestion publishes to NATS -> Leaf Node forwards to Hub -> Central cmdb-core consumes and writes to primary DB
- Metrics: batch compressed upload every 30s -> Central TimescaleDB
- Alerts: immediately upstream after local evaluation -> Central manages alert lifecycle

**Downstream (Central -> Edge):**

- Config changes: alert rules, collection schedules, permission policies -> publish to NATS -> Edge subscribes and updates local config
- Work order assignments: Central creates order -> notify Edge personnel
- Read replica sync (optional): PG Streaming Replica to Edge -> Edge queries without crossing WAN

**Disconnect Resilience:**

- Collected data buffered in NATS JetStream local queue (up to 7 days)
- Edge cmdb-core switches to local read mode
- Write operations enter local Write-Ahead Log
- On reconnection, automatically replay WAL to Central

### 6.3 NATS Subject Design

```
# Subject naming: {domain}.{action}.{tenant_id}

# Upstream (Edge -> Central)
ingestion.submit.tw-idc01
ingestion.metrics.tw-idc01
alert.fired.tw-idc01

# Downstream (Central -> Edge)
config.updated.tw-idc01
maintenance.assigned.tw-idc01

# Global broadcast
config.updated._all
system.announcement._all
```

### 6.4 Container Deployment (Docker Compose)

```yaml
services:
  cmdb-core:
    build: ./cmdb-core
    environment:
      - DATABASE_URL=postgres://cmdb:${DB_PASS}@postgres:5432/cmdb?sslmode=require
      - REDIS_URL=redis://redis:6379/0
      - NATS_URL=nats://nats:4222
      - JWT_SECRET=${JWT_SECRET}
      - DEPLOY_MODE=${DEPLOY_MODE}        # central | edge
      - TENANT_ID=${TENANT_ID}            # required for edge mode
      - MCP_ENABLED=true
      - MCP_PORT=3001
      - OTEL_ENDPOINT=http://otel-collector:4317
    ports:
      - "8080:8080"                       # REST API
      - "3001:3001"                       # MCP Server
      - "9090:9090"                       # gRPC (internal)
    deploy:
      replicas: ${CORE_REPLICAS:-2}
      resources:
        limits: { cpus: "2", memory: "512M" }

  ingestion-engine:
    build: ./ingestion-engine
    environment:
      - GRPC_TARGET=cmdb-core:9090
      - NATS_URL=nats://nats:4222
      - CELERY_BROKER_URL=redis://redis:6379/2
      - DEPLOY_MODE=${DEPLOY_MODE}
    ports:
      - "8081:8081"                       # FastAPI management API
    deploy:
      replicas: ${INGESTION_REPLICAS:-1}

  ingestion-worker:
    image: cmdb/ingestion:latest
    command: celery -A app.celery worker -l info -c 4
    deploy:
      replicas: ${WORKER_REPLICAS:-2}

  ingestion-beat:
    image: cmdb/ingestion:latest
    command: celery -A app.celery beat -l info
    deploy:
      replicas: 1

  postgres:
    image: timescale/timescaledb:latest-pg17
    command:
      - postgres
      - -c shared_buffers=1GB
      - -c effective_cache_size=3GB
      - -c work_mem=16MB
      - -c max_connections=200
      - -c wal_level=replica

  redis:
    image: redis:7.4-alpine
    command: redis-server --maxmemory 256mb --maxmemory-policy allkeys-lru

  nats:
    image: nats:2.10-alpine
    command: ["--jetstream", "--store_dir=/data", "--config=/etc/nats/nats.conf"]
    ports:
      - "4222:4222"                       # Client
      - "7422:7422"                       # Leaf Node

  otel-collector:
    image: otel/opentelemetry-collector-contrib:latest

  nginx:
    image: nginx:1.27-alpine
    ports:
      - "443:443"
      - "80:80"
```

### 6.5 K8s Deployment (Large Production)

Central Plane:
- cmdb-core: 3 replicas, HPA 3-10, targetCPU 70%
- ingestion-worker: 4 replicas, HPA 2-20, targetCPU 80%
- PostgreSQL: 3 instances (CloudNativePG/Patroni), 500Gi, daily backup

Edge Node (lightweight):
- cmdb-core: 1 replica, 1 CPU / 256Mi
- ingestion-engine: 1 replica, 2 CPU / 512Mi

---

## 7. Observability & Security

### 7.1 Three Pillars of Observability

**Traces (Distributed Tracing):**
- Go: otel-go SDK -> OTel Collector -> Jaeger
- Python: otel-python SDK
- Coverage: HTTP requests, gRPC calls, DB queries, NATS publish, Redis ops, external AI API

**Metrics (Monitoring):**
- Go: prometheus/client_golang -> OTel Collector -> Prometheus -> Grafana

Key Metrics:

| Metric | Type | Purpose |
|--------|------|---------|
| `api_request_duration_seconds` | histogram | P50/P95/P99 latency |
| `api_request_total` | counter | By status, method, path |
| `db_query_duration_seconds` | histogram | DB performance |
| `ingestion_items_processed_total` | counter | By source type |
| `ingestion_conflicts_total` | counter | Conflict tracking |
| `nats_messages_published_total` | counter | Event bus throughput |
| `active_websocket_connections` | gauge | WebSocket load |
| `ai_provider_request_duration_seconds` | histogram | AI call latency |
| `tenant_asset_count` | gauge | Per-tenant asset count |

**Logs (Aggregation):**
- Go: zap -> stdout -> Promtail -> Loki -> Grafana
- Python: structlog -> stdout
- Unified JSON format with `request_id`, `tenant_id`, `trace_id`, `module`, `duration_ms`

### 7.2 Grafana Dashboard Presets

| Dashboard | Panels | Alert Rules |
|-----------|--------|-------------|
| **API Overview** | QPS, latency P50/P95/P99, error rate, per-tenant distribution | P99 > 500ms |
| **Ingestion Pipeline** | Items/min, per-source throughput, conflicts, failure rate | Failure rate > 5% |
| **Database** | Connections, query latency, table sizes, WAL size, Replica Lag | Replica Lag > 30s |
| **NATS** | Message throughput, queue depth, per-subject traffic, Leaf Node latency | Queue depth > 10000 |
| **Per-Tenant** | Asset count, API calls, import status, conflict queue length | -- |
| **Edge Health** | Connection status, sync latency, local cache hit rate | Edge disconnect > 5min |

### 7.3 Security Design

**Layer 1 - Transport:**
- External: TLS 1.3 (Nginx SSL termination)
- Internal: NATS TLS + gRPC TLS (cross-IDC must encrypt)
- Edge <-> Central: mTLS mutual authentication

**Layer 2 - Authentication:**
- JWT RS256 (asymmetric keys; Edge only needs public key for verification)
- Access Token: 15 minutes
- Refresh Token: 7 days, single-use + rotation
- SSO/LDAP integration ready (identity module `source` field)

**Layer 3 - Authorization:**
- Casbin RBAC (campus + department dual-dimension)
- PostgreSQL RLS (database-layer safety net)
- API Rate Limiting (tenant-level + user-level)

**Layer 4 - Data:**
- Passwords: bcrypt (cost=12)
- Sensitive config (IPMI passwords, API keys): Vault reference or encrypted storage
- Audit logs: append-only, no modify, no delete
- PG backups: AES-256 encrypted

**Layer 5 - Network:**
- Edge Node exposes only port 443
- gRPC / NATS / DB ports internal-network only
- MCP Server restricted to specified IP ranges or mTLS clients

---

## 8. Resource Estimation

| Deployment Scale | Central Plane | Single Edge Node | Total (10 IDCs) |
|-----------------|---------------|-----------------|-----------------|
| **CPU** | 16 cores | 4 cores | 56 cores |
| **Memory** | 24 GB | 4 GB | 64 GB |
| **Storage** | 500 GB SSD (PG) + 200 GB (NATS) | 50 GB SSD | ~1.2 TB |
| **Components** | core x3 + ingestion x2 + worker x4 + beat x1 | core x1 + ingestion x1 + worker x1 | -- |
| **Est. Assets** | -- | ~10K per IDC | ~100K total |
| **API Throughput** | ~5,000 req/s | ~500 req/s (local) | -- |
| **Metrics Ingest** | ~50,000 points/s | ~5,000 points/s | -- |

---

## 9. Implementation Phases

### Phase 1: Core Skeleton (P0 - Unlock basic browsing)

1. **Auth**: login / refresh / me (3 endpoints)
2. **Topology**: Location CRUD + hierarchy traversal + stats (7 endpoints)
3. **Assets**: list + getById + serial number search (3 endpoints)
4. **Racks**: getById + listByLocation + listAssets (3 endpoints)
5. **Dashboard**: aggregate stats API (1 endpoint)
6. **Monitoring**: listAlerts + ack + resolve (3 endpoints)
7. **Maintenance**: list + getById + create + transition (4 endpoints)
8. **Audit**: query (1 endpoint)
9. **Inventory**: list + getById + listItems (3 endpoints)

**Total: 28 endpoints covering all pages' basic data display and core linkage paths.**

### Phase 2: Feature Completion (P1 - Unlock interactions)

- Complete Assets CRUD, lifecycle events, health overview
- Maintenance assign, logs
- Monitoring metrics query, health summary, topology
- Inventory scan/complete/summary
- Identity users/roles listing
- Prediction results query
- Excel/CSV import pipeline
- WebSocket real-time push

### Phase 3: Advanced Features (P2 - Unlock AI & integration)

- AI provider registry + MCP Server
- Dify / LLM / Custom model adapters
- Prediction models, RCA
- Integration adapters, webhooks
- Auto discovery, sensor config
- SNMP/IPMI/vCenter collectors
- Cross-IDC federation deployment

### Phase 4: Scale & Harden (P3 - Production readiness)

- K8s Helm chart for production deployment
- Edge Node deployment automation
- Full observability stack
- Security hardening (mTLS, Vault integration)
- Load testing and performance tuning
- Disaster recovery procedures
