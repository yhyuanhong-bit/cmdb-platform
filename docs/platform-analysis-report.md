# CMDB Platform - Complete Analysis Report

**Date:** 2026-04-07
**Branch:** feat/cmdb-core-phase1
**Analyzed by:** Claude Opus 4.6

---

## 1. Platform Overview

**Enterprise-Grade CMDB + AIOps Unified Operations Platform**

A full-stack IT infrastructure management platform covering asset lifecycle, auto-discovery, monitoring, predictive AI, business impact analysis, and data quality governance. Designed for multi-tenant, geographically distributed data center operations.

### Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Frontend (React 19 + TypeScript + Vite)      :5175     │
│  46 pages, 20+ hooks, TanStack Query, Zustand, i18n    │
└──────────────┬──────────────────────┬───────────────────┘
               │ /api/v1             │ /api/v1/ingestion
┌──────────────▼──────────┐  ┌───────▼───────────────────┐
│  cmdb-core (Go/Gin)     │  │  ingestion-engine         │
│  93 API endpoints       │  │  (Python/FastAPI)          │
│  12 domain services     │  │  3 collectors (SNMP/SSH/   │
│  RBAC + JWT auth        │  │  IPMI), pipeline, Celery   │
│  WebSocket + MCP        │  │  credential encryption     │
│             :8080       │  │             :8000          │
└──────┬──────┬───────────┘  └──────┬──────┬──────────────┘
       │      │                     │      │
┌──────▼──────▼─────────────────────▼──────▼──────────────┐
│  Infrastructure                                          │
│  PostgreSQL+TimescaleDB :5432  |  Redis :6379            │
│  NATS JetStream :4222          |  Celery Workers         │
│  Prometheus :9090  |  Grafana :3000  |  OTEL Collector   │
└──────────────────────────────────────────────────────────┘
```

### Tech Stack

| Layer | Technology |
|-------|-----------|
| Frontend | React 19, TypeScript 6, Vite 8, Tailwind CSS 4, TanStack Query, Zustand, i18next |
| Backend API | Go 1.22+, Gin, oapi-codegen, sqlc |
| Ingestion Engine | Python 3.12, FastAPI, Celery, asyncpg |
| Database | PostgreSQL 17 + TimescaleDB (time-series hypertable) |
| Cache/Broker | Redis 7 (JWT tokens, Celery broker) |
| Event Bus | NATS JetStream (7-day retention, file storage) |
| Observability | Prometheus, Grafana, OpenTelemetry, Promtail |
| AI/MCP | Dify integration, Model Context Protocol (7 tools) |

---

## 2. Database Schema (39 Tables)

### Core Tables

| # | Table | Purpose |
|---|-------|---------|
| 1 | `tenants` | Multi-tenant organizations |
| 2 | `users` | User accounts (bcrypt password) |
| 3 | `departments` | Organizational structure |
| 4 | `roles` | RBAC roles (JSONB permissions) |
| 5 | `user_roles` | User-role assignments |
| 6 | `locations` | Physical hierarchy (LTREE path) |
| 7 | `racks` | Rack units (42U, power capacity) |
| 8 | `rack_slots` | U-position assignments (front/rear) |
| 9 | `assets` | Configuration items (JSONB attributes, TEXT[] tags, ip_address) |

### Operations Tables

| # | Table | Purpose |
|---|-------|---------|
| 10 | `work_orders` | Maintenance jobs (state machine) |
| 11 | `work_order_logs` | State transition audit trail |
| 12 | `alert_rules` | Monitoring threshold definitions |
| 13 | `alert_events` | Active/resolved alerts |
| 14 | `incidents` | Incident records |
| 15 | `inventory_tasks` | Physical inventory tasks |
| 16 | `inventory_items` | Scan items (expected vs actual) |
| 17 | `audit_events` | Immutable change log (JSONB diff) |

### Time-Series & Predictions

| # | Table | Purpose |
|---|-------|---------|
| 18 | `metrics` | TimescaleDB hypertable (7-day raw, 180-day 5min, 730-day 1hr) |
| 19 | `metrics_5min` | Continuous aggregate (avg/max/min) |
| 20 | `metrics_1hour` | Continuous aggregate |
| 21 | `prediction_models` | AI model registry (Dify/OpenAI) |
| 22 | `prediction_results` | Failure predictions |
| 23 | `rca_analyses` | Root cause analysis results |

### Integration & Webhooks

| # | Table | Purpose |
|---|-------|---------|
| 24 | `integration_adapters` | External system connectors |
| 25 | `webhook_subscriptions` | Event subscriptions (BIA filter support) |
| 26 | `webhook_deliveries` | Delivery log with response codes |

### BIA (Business Impact Analysis)

| # | Table | Purpose |
|---|-------|---------|
| 27 | `bia_assessments` | System assessments (score, tier, RTO/RPO) |
| 28 | `bia_scoring_rules` | Tier definitions (critical/important/normal/minor) |
| 29 | `bia_dependencies` | System-to-asset links (runs_on/depends_on) |

### Data Quality

| # | Table | Purpose |
|---|-------|---------|
| 30 | `quality_rules` | Quality rule definitions (4 dimensions) |
| 31 | `quality_scores` | Per-asset quality scores |

### Discovery & Ingestion

| # | Table | Purpose |
|---|-------|---------|
| 32 | `discovered_assets` | Discovery staging area |
| 33 | `credentials` | Encrypted scan credentials (AES-256-GCM) |
| 34 | `scan_targets` | Discovery target configurations |
| 35 | `asset_field_authorities` | Field-level source priority matrix |
| 36 | `import_conflicts` | Merge conflicts (pending resolution) |
| 37 | `discovery_tasks` | Discovery run tracking |
| 38 | `discovery_candidates` | Raw discovery results |
| 39 | `import_jobs` | Bulk import job tracking |

---

## 3. Backend API (cmdb-core — 93 Endpoints)

### Authentication (3)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/auth/login` | JWT + refresh token |
| POST | `/auth/refresh` | Token rotation |
| GET | `/auth/current-user` | Current user info |

### Assets (5)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/assets` | List (paginated, filterable) |
| POST | `/assets` | Create |
| GET | `/assets/{id}` | Get details |
| PUT | `/assets/{id}` | Update |
| DELETE | `/assets/{id}` | Delete |

### Locations (8)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET/POST | `/locations` | List/Create |
| GET/PUT/DELETE | `/locations/{id}` | CRUD |
| GET | `/locations/{id}/ancestors` | Path to root |
| GET | `/locations/{id}/children` | Direct children |
| GET | `/locations/{id}/descendants` | Full subtree |

### Racks & Slots (8)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST/GET/PUT/DELETE | `/racks` | Rack CRUD |
| GET | `/racks/{id}/assets` | Assets in rack |
| GET | `/racks/{id}/slots` | Slot occupancy |
| POST/DELETE | `/racks/{id}/slots` | Slot management |

### Maintenance (6)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET/POST | `/work-orders` | List/Create |
| GET/PUT | `/work-orders/{id}` | Get/Update |
| POST | `/work-orders/{id}/transition` | State machine |
| GET | `/work-orders/{id}/logs` | Transition log |

### Monitoring & Alerts (10)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/alerts` | List alerts |
| POST | `/alerts/{id}/acknowledge` | Acknowledge |
| POST | `/alerts/{id}/resolve` | Resolve |
| GET/POST/PUT | `/alert-rules` | Rule CRUD |
| GET | `/metrics` | TimescaleDB query |
| GET/POST/PUT | `/incidents` | Incident CRUD |

### Inventory (7)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET/POST | `/inventory-tasks` | Task CRUD |
| GET | `/inventory-tasks/{id}/items` | Items list |
| POST | `/inventory-tasks/{id}/scan` | Scan item |
| PUT | `/inventory-tasks/{id}/complete` | Complete task |
| GET | `/inventory-tasks/{id}/summary` | Progress stats |

### BIA (8)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET/POST | `/bia-assessments` | Assessment CRUD |
| GET/PUT/DELETE | `/bia-assessments/{id}` | Single assessment |
| GET/POST | `/bia-assessments/{id}/dependencies` | Dependencies |
| GET | `/bia-stats` | Compliance summary |
| GET | `/bia-impact/{id}` | Impact analysis |

### Quality (6)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET/POST | `/quality-rules` | Rule CRUD |
| POST | `/quality-scan` | Run quality scan |
| GET | `/quality-dashboard` | Aggregate metrics |
| GET | `/quality-worst-assets` | Lowest scores |
| GET | `/assets/{id}/quality-history` | Asset trend |

### Discovery (5)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/discovery/pending` | Staging list |
| POST | `/discovery/ingest` | Ingest from collector |
| POST | `/discovery/{id}/approve` | Approve |
| POST | `/discovery/{id}/ignore` | Ignore |
| GET | `/discovery/stats` | 24h statistics |

### Other (27)
- Dashboard stats, user management, roles RBAC, audit events, prediction/RCA, system health, adapters, webhooks

---

## 4. Backend API (ingestion-engine — 18 Endpoints)

### Credentials (4)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/credentials?tenant_id=` | List (masked) |
| POST | `/credentials` | Create (encrypted) |
| PUT | `/credentials/{id}` | Update |
| DELETE | `/credentials/{id}` | Delete (FK check) |

### Scan Targets (4)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/scan-targets?tenant_id=` | List with credential name |
| POST | `/scan-targets` | Create |
| PUT | `/scan-targets/{id}` | Update |
| DELETE | `/scan-targets/{id}` | Delete |

### Discovery (3)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/discovery/scan` | Trigger scan |
| GET | `/discovery/tasks?tenant_id=` | Task list |
| GET | `/discovery/tasks/{id}` | Task detail |

### Import (4)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/import/upload` | Upload Excel/CSV |
| GET | `/import/{id}/preview` | Preview data |
| POST | `/import/{id}/confirm` | Confirm + dispatch |
| GET | `/import/{id}/progress` | Poll progress |

### Collectors (3)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/collectors` | List registered |
| POST | `/collectors/{name}/start\|stop` | Lifecycle |
| POST | `/collectors/{name}/test` | Test connection |

---

## 5. Frontend (46 Pages, 20+ Hooks)

### Page Groups

| Group | Pages | Key Features |
|-------|-------|-------------|
| **Location Hierarchy** | GlobalOverview, RegionOverview, CityOverview, CampusOverview | LTREE-based drill-down, metadata cards |
| **Dashboard** | Dashboard | Stats cards, BIA donut, heatmap, critical events |
| **Assets** | AssetManagement, AssetDetail, AssetLifecycle | Card/table views, 4-tab detail, timeline |
| **Discovery** | AutoDiscovery (2 tabs: Review + Scan Management) | Approve/ignore, scan targets, task history |
| **Racks** | RackManagement, RackDetail, DataCenter3D, FacilityMap | U-slot visualization, 3D view |
| **Inventory** | HighSpeedInventory, InventoryItemDetail | QR scanning, discrepancy tracking |
| **Monitoring** | MonitoringAlerts, SystemHealth, SensorConfig, EnergyMonitor | Alert lifecycle, health donut, thresholds |
| **Maintenance** | MaintenanceHub, WorkOrder, TaskDispatch | State machine, scheduling |
| **Predictive AI** | PredictiveHub (6 sub-tabs) | RCA, failure prediction, AI chat |
| **BIA** | BIAOverview, SystemGrading, RtoRpoMatrices, ScoringRules, DependencyMap | Tier scoring, compliance matrices |
| **Quality** | QualityDashboard | 4-dimension scoring, rules management |
| **System** | SystemSettings (4 tabs), RolesPermissions, UserProfile | Credentials, integrations, RBAC |
| **Audit** | AuditHistory, AuditEventDetail | Change log with diff viewer |
| **Help** | TroubleshootingGuide, VideoLibrary, VideoPlayer | SOP videos, chapters |

### Design System

| Element | Value |
|---------|-------|
| Theme | Dark (Material Design 3 inspired) |
| Primary | `#9ecaff` (light blue) |
| Surface | `#0a151a` (dark navy) |
| Error | `#ffb4ab` (salmon) |
| Headline font | Manrope (bold) |
| Body font | Inter |
| Icons | Google Material Symbols Outlined |
| i18n | English, Simplified Chinese, Traditional Chinese |

---

## 6. Key Business Flows

### 6.1 Asset Discovery Flow

```
Scan Target (CIDR + Credential + Mode)
    → POST /discovery/scan
    → Celery task dispatched
    → Collector (SNMP/SSH/IPMI) scans network
    → RawAssetData collected per IP
    → Mode routing:
        auto   → pipeline → assets table (direct)
        review → POST /discovery/ingest → discovered_assets (staging)
        smart  → deduplicate check:
                   matched → pipeline (update existing)
                   new     → staging (manual review)
    → Frontend: AutoDiscovery page → approve/ignore
    → Approved → asset created/updated
    → NATS events published
```

### 6.2 Data Import Flow

```
Excel/CSV upload → parse → preview (stats + errors)
    → user confirms → Celery task dispatched
    → normalize → deduplicate → validate → authority check
        → auto-merge (high-priority source) → asset updated
        → conflict (low-priority source) → import_conflicts created
    → import_jobs updated with stats
    → NATS import.completed event
```

### 6.3 Authority & Conflict Resolution

```
Field Authority Matrix:
  serial_number: IPMI(100) > SNMP(80) > manual(50)
  vendor:        IPMI(100) > manual(50)
  name:          manual(100)
  status:        manual(100)

Incoming data → check source priority per field:
  priority >= max existing → auto-merge (update asset)
  priority < max existing  → create conflict (manual review)
  
Conflict resolution:
  approve → use incoming value, update asset
  reject  → keep current value, close conflict
```

### 6.4 BIA Impact Propagation

```
BIA Assessment (system_name, tier, RTO/RPO)
    → bia_dependencies (assessment_id, asset_id, dependency_type)
    → PropagateBIALevel(): MAX(tier) across assessments
    → Update asset.bia_level = max_tier
    → Webhook filter: only notify for critical/important BIA levels
```

### 6.5 Quality Scoring

```
POST /quality/scan
    → For each asset:
        → Evaluate quality_rules (completeness, accuracy, timeliness, consistency)
        → total_score = 0.4×completeness + 0.3×accuracy + 0.1×timeliness + 0.2×consistency
        → Insert quality_scores record
    → Dashboard shows aggregate + worst assets
```

### 6.6 Alert → Incident → RCA

```
Metric breaches alert_rule condition
    → alert_event created (status=firing)
    → Optional: auto-create incident
    → User acknowledges → status=acknowledged
    → User triggers RCA → AI provider analyzes
    → rca_analyses record (reasoning, conclusion_asset_id, confidence)
    → User verifies → human_verified=true
    → Alert resolves → status=resolved
```

---

## 7. Event Bus (NATS JetStream)

### Event Subjects

| Domain | Subjects |
|--------|----------|
| Asset | asset.created, asset.updated, asset.status_changed, asset.deleted |
| Location | location.created, location.updated, location.deleted |
| Rack | rack.created, rack.updated, rack.deleted, rack.occupancy_changed |
| Maintenance | maintenance.order_created, order_updated, order_transitioned |
| Inventory | inventory.task_created, task_completed, item_scanned |
| Alert | alert.fired, alert.resolved |
| Import | import.completed, import.conflict_created |
| Prediction | prediction.created |
| Audit | audit.recorded |

### Stream Configuration
- Name: CMDB
- Retention: 7 days (MaxAge policy)
- Storage: FileStorage (persistent)
- Subject format: `{subject}.{tenant_id}`

---

## 8. Security

| Feature | Implementation |
|---------|---------------|
| Authentication | JWT HMAC-SHA256, 15-min access token, 7-day refresh token |
| Token storage | Redis (refresh tokens), in-memory (access tokens) |
| Token rotation | Old refresh token deleted on new issue |
| RBAC | JSONB permissions in roles table |
| Credential encryption | AES-256-GCM (BYTEA in PostgreSQL) |
| Password hashing | bcrypt |
| Multi-tenancy | tenant_id in JWT claims, row-level filtering |
| Webhook signing | HMAC-SHA256 (optional per subscription) |
| Audit trail | Immutable audit_events with JSONB diff |

---

## 9. Deployment Modes

| Mode | Description | Config |
|------|-------------|--------|
| Cloud | Multi-tenant, shared infrastructure | `DEPLOY_MODE=cloud` |
| Edge | Single-tenant, local operation | `DEPLOY_MODE=edge` + `TENANT_ID=uuid` |

### Infrastructure Requirements

| Service | Required | Purpose |
|---------|----------|---------|
| PostgreSQL 17 + TimescaleDB | Yes | Primary datastore + time-series |
| Redis 7 | Yes | Cache, JWT tokens, Celery broker |
| NATS 2.10 | Optional | Event bus (degrades gracefully) |
| Prometheus | Optional | Metrics scraping |
| Grafana | Optional | Dashboard visualization |
| OTEL Collector | Optional | Distributed tracing |

---

## 10. Seed Data Summary

| Entity | Count | Examples |
|--------|-------|---------|
| Tenants | 1 | Taipei Campus (tw) |
| Users | 3 | admin, sarah.jenkins, mike.chen |
| Roles | 3 | super-admin, ops-admin, viewer |
| Locations | 24 | 1 country → 2 regions → 3 cities → 4 IDCs → 6 rooms |
| Racks | 10 | RACK-A01 through RACK-Q02 (42U each) |
| Assets | 20 | 6 servers, 4 network, 3 storage, 2 power |
| Alert Rules | 4 | CPU, memory, temperature, disk thresholds |
| Alert Events | 8 | Mixed firing/acknowledged/resolved |
| Work Orders | 6 | Various statuses |
| BIA Assessments | 4 | Payment Gateway (critical) → QA Sandbox (minor) |
| BIA Dependencies | 8 | System-to-asset links |
| Quality Rules | 5 | Completeness + accuracy + consistency |
| Discovered Assets | 5 | SNMP/IPMI sources, mixed statuses |
| Metrics | 236 | 24h × 3 assets × 4 metric types |
| Field Authorities | 10 | IPMI(100) > SNMP(80) > manual(50) |

---

## 11. File Structure

```
/cmdb-platform/
├── cmdb-core/                          # Go backend (API + domain logic)
│   ├── cmd/server/main.go              # Entry point
│   ├── internal/
│   │   ├── api/impl.go                 # 93 API handlers (2,727 lines)
│   │   ├── api/generated.go            # OpenAPI generated types (3,781 lines)
│   │   ├── domain/                     # 12 domain services
│   │   │   ├── asset/service.go
│   │   │   ├── identity/service.go + auth_service.go
│   │   │   ├── topology/service.go
│   │   │   ├── maintenance/service.go
│   │   │   ├── monitoring/service.go
│   │   │   ├── inventory/service.go
│   │   │   ├── audit/service.go
│   │   │   ├── dashboard/service.go
│   │   │   ├── prediction/service.go
│   │   │   ├── integration/service.go
│   │   │   ├── bia/service.go
│   │   │   ├── quality/service.go
│   │   │   └── discovery/service.go
│   │   ├── middleware/                  # Auth, CORS, tracing
│   │   ├── eventbus/                   # NATS JetStream (20 subjects)
│   │   ├── platform/                   # Database, cache, telemetry
│   │   ├── dbgen/                      # sqlc generated queries
│   │   ├── websocket/                  # Real-time updates
│   │   ├── ai/                         # LLM integration
│   │   └── mcp/                        # Model Context Protocol (7 tools)
│   ├── db/migrations/                  # 17 migration files
│   ├── db/queries/                     # SQL query definitions
│   ├── db/seed/seed.sql                # Rich seed dataset
│   └── deploy/                         # Docker Compose, Prometheus, Grafana
│
├── ingestion-engine/                   # Python data processing
│   ├── app/
│   │   ├── main.py                     # FastAPI app (6 routers)
│   │   ├── collectors/                 # SNMP, SSH, IPMI collectors
│   │   ├── credentials/               # AES-256-GCM encryption
│   │   ├── pipeline/                   # normalize → dedup → validate → authority
│   │   ├── routes/                     # 18 API endpoints
│   │   ├── tasks/                      # Celery: import + discovery
│   │   └── importers/                  # Excel/CSV parsing
│   └── tests/                          # 64 unit tests
│
├── cmdb-demo/                          # React frontend
│   ├── src/
│   │   ├── pages/                      # 46 page components
│   │   ├── components/                 # 18+ shared components
│   │   ├── hooks/                      # 20+ React Query hooks
│   │   ├── lib/api/                    # API clients (13 modules)
│   │   ├── stores/                     # Zustand auth store
│   │   ├── i18n/                       # 3 languages (en, zh-CN, zh-TW)
│   │   └── layouts/                    # MainLayout with sidebar nav
│   └── vite.config.ts                  # Dual proxy (8080 + 8000)
│
└── docs/                               # Specs + plans
    └── platform-analysis-report.md     # This file
```

---

## 12. Platform Maturity Assessment

| Dimension | Score | Notes |
|-----------|-------|-------|
| **Backend completeness** | 9/10 | 93 endpoints, 12 domain services, comprehensive |
| **Frontend completeness** | 8/10 | 46 pages, all features have UI, some pages are demo-grade |
| **Database design** | 9/10 | 39 tables, proper FKs, LTREE, TimescaleDB, JSONB |
| **API consistency** | 8/10 | OpenAPI spec, consistent patterns, some endpoints lack pagination |
| **Authentication** | 8/10 | JWT + refresh rotation, Redis-backed, missing OAuth/SSO |
| **Multi-tenancy** | 7/10 | Row-level filtering everywhere, no PG RLS yet |
| **Event architecture** | 8/10 | NATS JetStream, 20 subjects, webhook dispatch |
| **Discovery/Ingestion** | 8/10 | 3 collectors, pipeline, authority system, mode routing |
| **BIA** | 8/10 | Tier scoring, RTO/RPO, dependency propagation |
| **Quality** | 7/10 | 4-dimension scoring, rules engine, needs more rules |
| **Monitoring** | 7/10 | Alert lifecycle, TimescaleDB metrics, needs more rules |
| **AI/Predictive** | 6/10 | RCA via Dify, MCP tools, needs more ML models |
| **Testing** | 5/10 | 64 Python tests, no Go tests, no frontend tests |
| **Documentation** | 6/10 | Design specs exist, needs API docs, user guides |
| **Security** | 7/10 | Encryption, JWT, RBAC, needs audit hardening |
| **Observability** | 8/10 | OTEL, Prometheus, Grafana, structured logging |

**Overall: 7.5/10** — Production-ready for small-to-medium enterprise deployment with attention needed on testing, documentation, and security hardening.
