# CMDB Platform - Real Data Integration Gap Analysis

**Date:** 2026-04-07
**Purpose:** Identify what each page needs to transition from mock/hardcoded data to real backend API integration

---

## Executive Summary

| Category | Count |
|----------|-------|
| Pages with hardcoded data | 33 / 46 |
| New API endpoints needed | 28 |
| New database tables needed | 9 |
| Existing endpoints needing enhancement | 8 |
| "Coming Soon" features to implement | 8 |

### Priority Classification

| Priority | Pages | Effort | Impact |
|----------|-------|--------|--------|
| P0 Critical | AssetDetail, Dashboard, RackDetail | High | Core user experience |
| P1 High | EnergyMonitor, AlertTopology, Inventory (2), MaintenanceTaskView | High | Operational visibility |
| P2 Medium | SystemHealth, MonitoringAlerts, SensorConfig, TaskDispatch, Audit | Medium | Enhanced functionality |
| P3 Low | PredictiveHub, UserProfile, ComponentUpgrade, Help (3) | Low-Medium | Future features |

---

## Page-by-Page Gap Analysis

---

### P0-01: AssetDetail (`/assets/:id`)

**Current State:** Asset CRUD works, but all telemetry/monitoring/financial/network data is hardcoded

#### Hardcoded Data Inventory

| Data | Current Source | Lines |
|------|---------------|-------|
| CPU/Memory telemetry (10 data points) | Hardcoded SVG path coordinates | 101-112 |
| Temperature trend (38.2C) | Static value | 393 |
| Hardware health (CPU 24%, RAM 89%, NVMe 12%) | Static values | 333-337 |
| Vibration (1.2 mm/s, 20 bars) | Static | 454 |
| Power draw (412W) | Static | 475 |
| Warranty status | "Unknown" | 14 |
| Uptime, MTBF | "-" defaults | 14-40 |
| Financial (cost, depreciation, book value) | All "-" | 32-35 |
| Network (IP, mgmt IP, VLAN, domain) | All "-" | 36-39 |
| Physical specs (CPU, memory, storage, OS) | All "-" | 22-26 |

#### Backend Status

| Endpoint | Exists? | Returns |
|----------|---------|---------|
| `GET /assets/{id}` | YES | name, type, status, vendor, model, serial, attributes, tags |
| `GET /monitoring/metrics?asset_id=&name=&range=` | YES | `[{time, name, value}]` from TimescaleDB |
| Warranty/financial API | NO | - |
| Network config API | NO | - |
| Uptime/MTBF calculation | NO | - |

#### Action Plan

**Step 1: Connect existing metrics API (no backend changes)**
```
Frontend change only:
- Import useMetrics hook
- Query: GET /monitoring/metrics?asset_id={id}&name=cpu_usage&time_range=24h
- Query: GET /monitoring/metrics?asset_id={id}&name=memory_usage&time_range=24h
- Query: GET /monitoring/metrics?asset_id={id}&name=temperature&time_range=24h
- Query: GET /monitoring/metrics?asset_id={id}&name=power_kw&time_range=24h
- Replace hardcoded SVG with dynamic chart from metric data points
```

**Step 2: Use attributes JSONB for extended fields (no schema changes)**
```
Store in assets.attributes:
- ip_address, mgmt_ip, vlan, domain (network)
- cpu_arch, memory_gb, storage_tb, os (specs)
- warranty_expiry, purchase_date, cost (financial)

Frontend: read from asset.attributes.* instead of hardcoded "-"
```

**Step 3: New endpoints needed (future)**

| Endpoint | Table | Purpose |
|----------|-------|---------|
| `GET /assets/{id}/metrics-summary` | metrics (aggregate) | Current CPU%, Memory%, Disk% |
| `GET /assets/{id}/uptime` | metrics (calculate) | Uptime % from status changes |

**Estimated effort:** Step 1: 2h frontend | Step 2: 1h frontend | Step 3: 4h backend

---

### P0-02: Dashboard (`/dashboard`)

**Current State:** Stats cards work, but occupancy, heatmap, financial, task progress are hardcoded

#### Hardcoded Data Inventory

| Data | Current Source |
|------|---------------|
| Rack occupancy (76%) | Hardcoded with TODO comment |
| Heatmap (6x12 grid) | Seeded pseudo-random values |
| Financial breakdown (In-use 82%, Broken 13%, Disposed 5%) | Static |
| Task progress (Mar-24 Audit at 76%) | Static |

#### Backend Status

| Endpoint | Exists? | Returns |
|----------|---------|---------|
| `GET /dashboard/stats` | YES | total_assets, total_racks (returns 0!), critical_alerts, active_orders |
| `GET /bia/stats` | YES | total, by_tier counts |
| Occupancy API | NO | - |
| Lifecycle stats API | NO | - |

#### Action Plan

**Step 1: Fix dashboard/stats to return real rack count**
```go
// cmdb-core: add CountRacks query
-- name: CountRacks :one
SELECT count(*) FROM racks WHERE tenant_id = $1;

// Update GetStats to include rack count
```

**Step 2: Add occupancy calculation**
```
New endpoint: GET /racks/stats
Returns: { total_racks, total_u, used_u, occupancy_pct }

Query:
SELECT count(*) as total_racks,
       sum(r.total_u) as total_u,
       count(rs.id) as used_slots
FROM racks r
LEFT JOIN rack_slots rs ON rs.rack_id = r.id
WHERE r.tenant_id = $1
```

**Step 3: Add lifecycle stats**
```
New endpoint: GET /assets/lifecycle-stats
Returns: { operational, maintenance, retired, disposed, inventoried }

Query: SELECT status, count(*) FROM assets WHERE tenant_id = $1 GROUP BY status
```

**Step 4: Connect heatmap to real rack data**
```
Frontend: fetch racks with slot counts, render grid based on actual rack occupancy
```

**Estimated effort:** 4h backend + 3h frontend

---

### P0-03: RackDetail (`/racks/:id`)

**Current State:** Rack/assets/slots from API, but alerts, network, maintenance, environment, activity all hardcoded

#### Hardcoded Data Inventory

| Data | Items | What's Needed |
|------|-------|---------------|
| Equipment list | 17 items | Already available via `GET /racks/{id}/slots` — frontend needs to use it |
| Alerts | 3 items | Filter alerts by rack's assets |
| Network connections | 6 items | New: rack_network_connections table |
| Maintenance history | 4 items | Join work_orders by rack's location |
| Environment metrics | 5 metrics | New: environment sensor data or metrics query |
| Recent activity | 5 items | New: activity feed endpoint |

#### Action Plan

**Step 1: Replace equipment list with API data (frontend only)**
```
Already have: GET /racks/{id}/slots returns asset details per U-slot
Frontend: remove hardcoded equipment[], use slots API data instead
```

**Step 2: Connect alerts to rack**
```
Frontend: GET /monitoring/alerts then filter by asset_ids in this rack
Or new endpoint: GET /racks/{id}/alerts
```

**Step 3: Connect maintenance history**
```
New endpoint: GET /racks/{id}/maintenance
Query: SELECT wo.* FROM work_orders wo
JOIN assets a ON wo.asset_id = a.id
WHERE a.rack_id = $1
ORDER BY wo.created_at DESC LIMIT 10
```

**Step 4: New tables needed**

| Table | Columns | Purpose |
|-------|---------|---------|
| `rack_network_connections` | id, rack_id, port, connected_device, speed, status, vlan | Network topology per rack |

**Step 5: Environment metrics via existing metrics table**
```
Query metrics for rack's assets:
SELECT avg(value) FROM metrics
WHERE asset_id IN (SELECT asset_id FROM rack_slots WHERE rack_id = $1)
AND name IN ('temperature', 'humidity', 'power_kw')
AND time > now() - interval '1 hour'
GROUP BY name
```

**Estimated effort:** 6h backend + 4h frontend

---

### P1-01: EnergyMonitor (`/monitoring/energy`)

**Current State:** Raw power/PUE metrics from API, everything else hardcoded

#### Hardcoded Data Inventory

| Data | Items |
|------|-------|
| Power trend (12 hourly points) | Process + lighting breakdown |
| Rack heatmap (16 racks with intensity) | Static values |
| Power events (4 events) | Title, location, severity |
| Bottom stats (IT 842kW, Cooling 312kW, UPS 43kW, Misc 51kW) | Static |
| BIA power donut (Tier1 840kW, Tier2 290kW, Tier3 110kW) | Static |
| Carbon footprint (2.4 MT) | Static |
| Peak demand (1.52 MW) | Static |

#### Action Plan

**New endpoints needed:**

| Endpoint | Purpose | Query Source |
|----------|---------|-------------|
| `GET /energy/trend?granularity=hourly&duration=24h` | Pre-aggregated power data | metrics table, time_bucket |
| `GET /energy/breakdown` | Power by category | metrics grouped by asset type |
| `GET /energy/summary` | PUE, carbon, peak | metrics aggregation |
| `GET /racks/power-heatmap` | Per-rack power draw | metrics grouped by rack |

**Database:** No new tables needed — all data can be derived from existing `metrics` hypertable with proper queries

**Estimated effort:** 8h backend + 4h frontend

---

### P1-02: AlertTopology (`/monitoring/topology`)

**Current State:** 90% hardcoded — 5 fixed nodes, 5 fixed edges, only alerts from API

#### Hardcoded Data Inventory

| Data | Items |
|------|-------|
| Topology nodes | 5 fixed nodes (DB, Web, CDN, Middleware, Cache) |
| Edges/connections | 5 fixed connections |
| Node positions (x, y) | Hardcoded coordinates |
| Node metrics (CPU%, mem%, disk%) | Static values per node |
| Root cause badge | Fixed on node-1 |

#### Action Plan

**Option A: Derive from BIA dependencies (minimal backend work)**
```
Already have: GET /bia/assessments/{id}/dependencies returns asset-to-assessment links
Could build: system → asset dependency graph from bia_dependencies table

Frontend:
1. Fetch BIA assessments + dependencies
2. Fetch assets for each dependency
3. Build node graph from asset relationships
4. Auto-layout with force-directed algorithm (e.g., d3-force)
5. Overlay current alerts on nodes
```

**Option B: New topology service (more work, better result)**

| Endpoint | Purpose |
|----------|---------|
| `GET /topology/graph?location_id=` | Returns nodes + edges for a location |
| `GET /topology/nodes/{id}/metrics` | Current CPU%, memory%, disk% for a node |

| New Table | Columns |
|-----------|---------|
| `asset_dependencies` | id, tenant_id, source_asset_id, target_asset_id, dependency_type (network/app/data), created_at |

**Recommendation:** Start with Option A (reuse BIA data), evolve to Option B

**Estimated effort:** Option A: 6h frontend | Option B: 12h backend + 6h frontend

---

### P1-03: HighSpeedInventory (`/inventory`)

**Current State:** Task list from API, but rack tiles, discrepancies, import errors all hardcoded

#### Action Plan

**Step 1: Replace rack tiles with real data**
```
Existing: GET /inventory/tasks/{id}/items returns items with status
New query: aggregate items by rack → { rack_name, total_items, scanned_items, status }

New endpoint: GET /inventory/tasks/{id}/racks-summary
Query:
SELECT r.name, count(ii.id) as total,
  count(ii.id) FILTER (WHERE ii.status = 'scanned') as scanned
FROM inventory_items ii
JOIN assets a ON ii.asset_id = a.id
JOIN racks r ON a.rack_id = r.id
WHERE ii.task_id = $1
GROUP BY r.id, r.name
```

**Step 2: Replace discrepancies with real data**
```
Existing: GET /inventory/tasks/{id}/items has status field
New endpoint: GET /inventory/tasks/{id}/discrepancies
Query: SELECT * FROM inventory_items WHERE task_id = $1 AND status = 'discrepancy'
Join with assets for name, location details
```

**Step 3: Import error tracking**
```
Existing: import_jobs table has error_details JSONB
Frontend: read from import_jobs.error_details after import
```

**Estimated effort:** 4h backend + 3h frontend

---

### P1-04: InventoryItemDetail (`/inventory/detail`)

**Current State:** Item data from API, but scan history and discrepancy notes hardcoded

#### New Tables Needed

| Table | Columns | Purpose |
|-------|---------|---------|
| `inventory_scan_history` | id, item_id, scanned_at, scanned_by, method (qr/rfid/manual), result (match/mismatch/location_update), note | Historical scan records |
| `inventory_notes` | id, item_id, author_id, severity (info/warning/critical), text, created_at | Discrepancy discussion |

#### New Endpoints

| Endpoint | Purpose |
|----------|---------|
| `GET /inventory/items/{id}/scan-history` | Returns historical scans |
| `GET /inventory/items/{id}/notes` | Returns discussion notes |
| `POST /inventory/items/{id}/notes` | Add a note |

**Estimated effort:** 4h backend + 2h frontend

---

### P1-05: MaintenanceTaskView (`/maintenance/task/:id`)

**Current State:** Work order + logs from API, but steps, associated assets, environment hardcoded

#### Hardcoded Data

| Data | Source |
|------|--------|
| Task steps (4 procedural items) | Static array |
| Associated assets (2 items with health %) | Static |
| Environmental context (temp, humidity, load) | Static |
| Comment submission | Non-functional (no endpoint) |

#### Action Plan

**Step 1: Use work_order description for procedures (no backend changes)**
```
Store structured steps in work_orders.description as JSON:
{ "steps": [{"text": "...", "status": "pending"}] }
Frontend: parse and render
```

**Step 2: Connect associated assets**
```
Existing: work_orders has asset_id (single asset)
Enhancement: add work_order_assets table for multi-asset work orders
Or: use work_orders.asset_id + query asset's rack neighbors
```

**Step 3: Environment from metrics API**
```
Frontend: query metrics for the work order's asset location
GET /monitoring/metrics?asset_id={order.asset_id}&name=temperature&time_range=1h
```

**Step 4: Comment endpoint**
```
New endpoint: POST /work-orders/{id}/comments
Table: work_order_comments (id, order_id, author_id, text, created_at)
Or: extend work_order_logs with comment type
```

**Estimated effort:** 4h backend + 3h frontend

---

### P2-01: SystemHealth (`/monitoring/health`)

**Current State:** System health + alerts from API, but trends and resource utilization hardcoded

#### Action Plan

**Step 1: Alert trend histogram**
```
New endpoint: GET /monitoring/alerts/trend?hours=24
Query:
SELECT date_trunc('hour', fired_at) as hour,
  count(*) FILTER (WHERE severity = 'critical') as critical,
  count(*) FILTER (WHERE severity = 'warning') as warning,
  count(*) FILTER (WHERE severity = 'info') as info
FROM alert_events WHERE tenant_id = $1
AND fired_at > now() - interval '24 hours'
GROUP BY hour ORDER BY hour
```

**Step 2: Resource utilization**
```
New endpoint: GET /monitoring/resources
Query metrics for facility-level aggregates:
- Storage: sum(value) WHERE name = 'disk_used_gb' / sum WHERE name = 'disk_total_gb'
- Power: avg(value) WHERE name = 'power_kw'
- Memory: avg(value) WHERE name = 'memory_usage'
```

**Estimated effort:** 3h backend + 2h frontend

---

### P2-02: MonitoringAlerts (`/monitoring`)

**Current State:** Alert list + ack/resolve from API, trend and AIOps hardcoded

#### Action Plan

**Step 1: Alert trend — same as SystemHealth Step 1**

**Step 2: AIOps recommendations (future)**
```
Could leverage existing prediction/RCA service:
GET /prediction/recommendations?alert_id={id}
Uses existing Dify integration for analysis
```

**Estimated effort:** 2h backend (shared with SystemHealth) + 2h frontend

---

### P2-03: SensorConfiguration (`/monitoring/sensors`)

**Current State:** Sensors derived from assets (not real sensor devices), rules from API

#### Action Plan

**Short-term: Keep deriving sensors from assets (acceptable)**
```
Frontend improvement:
- Fetch all assets, not just first 10
- Add proper type mapping (sensor types from asset sub_type)
- Show last metric timestamp as "last seen"
```

**Long-term: Dedicated sensor registry**

| Table | Columns |
|-------|---------|
| `sensors` | id, tenant_id, asset_id, name, type, mac_address, ip_address, polling_interval, status, last_heartbeat |

**Estimated effort:** Short-term: 2h frontend | Long-term: 6h backend + 3h frontend

---

### P2-04: TaskDispatch (`/maintenance/dispatch`)

**Current State:** Work orders from API, technicians and zones hardcoded

#### Action Plan

**Step 1: Technician list from users API**
```
Frontend: GET /users with role filter
Calculate load: count active work_orders per assignee_id
```

**Step 2: Zone concept**
```
Zones can map to locations (IDC/room level)
No new table needed — use existing location hierarchy
Frontend: group work_orders by location → show capacity per location
```

**Step 3: Assignment endpoint**
```
Existing: PUT /work-orders/{id} can set assignee_id
Frontend: connect assign button to update mutation
```

**Estimated effort:** 2h backend (add user role filter) + 4h frontend

---

### P2-05: AuditEventDetail (`/audit/detail`)

**Current State:** Event from API, but fallback template fills missing fields

#### Action Plan

**Enhancement: Enrich audit event response**
```go
// In GetAuditEvent handler, join with users table:
SELECT ae.*, u.display_name, u.email
FROM audit_events ae
LEFT JOIN users u ON ae.operator_id = u.id
WHERE ae.id = $1

// Add to response: operator_name, operator_email
// Frontend: remove FALLBACK_EVENT, use API data directly
```

**Estimated effort:** 1h backend + 1h frontend

---

### P3-01: PredictiveHub (`/predictive`)

**Current State:** Prediction/RCA endpoints exist but many dashboard elements hardcoded

#### Action Plan

| Need | Approach | Effort |
|------|----------|--------|
| Failure distribution | Aggregate prediction_results by type | 2h backend |
| RUL (Remaining Useful Life) | Add rul_days field to prediction_results | 1h backend |
| AI conversation | Integrate with existing Dify workflow | 4h backend |
| Alert insights | Aggregate from alerts + predictions | 2h backend |

**Estimated effort:** 9h backend + 4h frontend

---

### P3-02: UserProfile (`/system/profile`)

**Current State:** User CRUD works, sessions/2FA/notifications hardcoded

#### Action Plan

| Need | Table/Endpoint | Effort |
|------|---------------|--------|
| Session list | New `user_sessions` table + endpoint | 3h |
| 2FA status | Add `two_factor_enabled` to users table | 1h |
| Notification prefs | Store in users.attributes JSONB | 1h |
| Password change | `POST /auth/change-password` | 2h |

**Estimated effort:** 7h total

---

### P3-03: ComponentUpgradeRecommendations (`/assets/upgrades`)

**Current State:** 100% hardcoded — needs recommendation engine

#### Action Plan

**Phase 1: Rule-based recommendations**
```
New table: upgrade_rules
  - asset_type, metric_name, threshold, recommendation_text, priority

New endpoint: GET /assets/upgrade-recommendations
Logic:
  - For each asset, check metrics against rules
  - If CPU > 80% sustained → recommend CPU upgrade
  - If memory > 90% → recommend memory upgrade
  - If disk > 85% → recommend storage upgrade
```

**Phase 2: ML-based recommendations (future)**
```
Integrate with prediction service for intelligent recommendations
```

**Estimated effort:** Phase 1: 6h backend + 3h frontend

---

### P3-04: Help/Training Pages (VideoPlayer, VideoLibrary, TroubleshootingGuide)

**Current State:** 100% hardcoded — demo/training content

#### Action Plan

| Option | Approach | Effort |
|--------|----------|--------|
| A: Keep static | Acceptable for internal docs | 0h |
| B: CMS integration | New tables for content management | 12h |
| C: External CMS | Link to Confluence/Notion | 2h |

**Recommendation:** Option A (keep static) unless user base grows

---

## New Database Tables Summary

| # | Table | Needed By | Priority |
|---|-------|-----------|----------|
| 1 | `rack_network_connections` | RackDetail | P0 |
| 2 | `inventory_scan_history` | InventoryItemDetail | P1 |
| 3 | `inventory_notes` | InventoryItemDetail | P1 |
| 4 | `work_order_comments` | MaintenanceTaskView | P1 |
| 5 | `asset_dependencies` | AlertTopology | P1 |
| 6 | `user_sessions` | UserProfile | P3 |
| 7 | `upgrade_rules` | ComponentUpgrade | P3 |
| 8 | `sensors` | SensorConfig | P3 |
| 9 | `ai_conversations` | PredictiveHub | P3 |

## New API Endpoints Summary

### P0 (Critical — Core Experience)

| # | Endpoint | Method | Service | Purpose |
|---|----------|--------|---------|---------|
| 1 | `/racks/stats` | GET | topology | Total racks, occupancy %, used_u |
| 2 | `/assets/lifecycle-stats` | GET | asset | Count by status |
| 3 | `/assets/{id}/metrics-summary` | GET | monitoring | Current CPU/mem/disk % |
| 4 | `/racks/{id}/maintenance` | GET | maintenance | Work orders for rack's assets |
| 5 | `/racks/{id}/alerts` | GET | monitoring | Alerts for rack's assets |

### P1 (High — Operational Visibility)

| # | Endpoint | Method | Service | Purpose |
|---|----------|--------|---------|---------|
| 6 | `/monitoring/alerts/trend` | GET | monitoring | Alert counts by hour (24h) |
| 7 | `/monitoring/resources` | GET | monitoring | Storage/power/memory utilization |
| 8 | `/energy/trend` | GET | monitoring | Aggregated power data |
| 9 | `/energy/breakdown` | GET | monitoring | Power by category |
| 10 | `/energy/summary` | GET | monitoring | PUE, carbon, peak demand |
| 11 | `/racks/power-heatmap` | GET | topology | Per-rack power draw |
| 12 | `/inventory/tasks/{id}/racks-summary` | GET | inventory | Items grouped by rack |
| 13 | `/inventory/tasks/{id}/discrepancies` | GET | inventory | Discrepancy items |
| 14 | `/inventory/items/{id}/scan-history` | GET | inventory | Historical scans |
| 15 | `/inventory/items/{id}/notes` | GET/POST | inventory | Discussion notes |
| 16 | `/work-orders/{id}/comments` | GET/POST | maintenance | Work order comments |
| 17 | `/topology/graph` | GET | topology | Asset dependency graph |

### P2 (Medium — Enhanced Functionality)

| # | Endpoint | Method | Service | Purpose |
|---|----------|--------|---------|---------|
| 18 | `/racks/{id}/environment` | GET | monitoring | Temp/humidity/power metrics |
| 19 | `/racks/{id}/activity-feed` | GET | audit | Recent events for rack |
| 20 | `/racks/{id}/network-connections` | GET | topology | Network topology |
| 21 | `/users?role=technician` | GET | identity | Filter users by role |
| 22 | `/audit/events/{id}` (enhanced) | GET | audit | Enriched with operator details |

### P3 (Low — Future Features)

| # | Endpoint | Method | Service | Purpose |
|---|----------|--------|---------|---------|
| 23 | `/prediction/failure-analysis/{id}` | GET | prediction | Failure distribution |
| 24 | `/users/{id}/sessions` | GET | identity | Active sessions |
| 25 | `/auth/change-password` | POST | identity | Password change |
| 26 | `/assets/upgrade-recommendations` | GET | asset | Rule-based upgrades |
| 27 | `/sensors` | GET | monitoring | Sensor registry |
| 28 | `/sensors/discover` | POST | monitoring | Sensor auto-discovery |

## Existing Endpoints Needing Enhancement

| # | Endpoint | Enhancement | Priority |
|---|----------|-------------|----------|
| 1 | `GET /dashboard/stats` | Add real rack count (CountRacks query) | P0 |
| 2 | `GET /assets/{id}` | Populate attributes with network/specs/financial data | P0 |
| 3 | `GET /audit/events` | Join operator details (display_name, email) | P2 |
| 4 | `GET /prediction/results` | Add rul_days, predicted_failure_date | P3 |
| 5 | `GET /users/{id}` | Add two_factor_enabled field | P3 |
| 6 | `GET /system/health` | Add resource utilization (storage, power, memory) | P2 |
| 7 | `GET /inventory/tasks/{id}/items` | Include asset details (name, location, rack) | P1 |
| 8 | `GET /monitoring/metrics` | Add pre-aggregated modes (hourly/daily summary) | P1 |

## "Coming Soon" Features to Implement

| # | Page | Feature | Backend Need |
|---|------|---------|-------------|
| 1 | AssetManagement | Import Excel/CSV | Connect to ingestion-engine `/import/upload` |
| 2 | AssetManagement | Export CSV | New: `GET /assets/export?format=csv` |
| 3 | HighSpeedInventory | Scan QR | Mobile integration / camera API |
| 4 | MonitoringAlerts | Silence Management | New: `POST /monitoring/silence` with duration |
| 5 | SensorConfiguration | Discover Sensors | New: `POST /sensors/discover` |
| 6 | TaskDispatch | Auto-assign | Algorithm: match technician skills + load + zone |
| 7 | UserProfile | Password Change | New: `POST /auth/change-password` |
| 8 | ComponentUpgrade | Request Upgrade | New: create work order from recommendation |

---

## Implementation Roadmap

### Phase 1: Quick Wins (Frontend-Only, 0 Backend Changes)

| # | Page | Change | Effort |
|---|------|--------|--------|
| 1 | AssetDetail | Connect metrics API for CPU/memory/temp charts | 2h |
| 2 | AssetDetail | Read specs/network from asset.attributes | 1h |
| 3 | RackDetail | Use /racks/{id}/slots instead of hardcoded equipment | 1h |
| 4 | RackDetail | Filter /monitoring/alerts by rack's asset IDs | 1h |
| 5 | SensorConfig | Fetch all assets (not first 10) for sensor list | 0.5h |
| 6 | TaskDispatch | Use /users for technician list | 1h |
| 7 | AssetManagement | Connect Import to ingestion-engine upload API | 2h |

**Total: ~8.5h frontend work, high impact**

### Phase 2: Essential Backend Endpoints (P0+P1)

| # | Endpoint | Backend Effort | Frontend Effort |
|---|----------|---------------|-----------------|
| 1 | `/racks/stats` (occupancy) | 2h | 1h |
| 2 | `/assets/lifecycle-stats` | 1h | 1h |
| 3 | `/monitoring/alerts/trend` | 2h | 2h |
| 4 | `/racks/{id}/maintenance` | 2h | 1h |
| 5 | `/inventory/tasks/{id}/racks-summary` | 2h | 2h |
| 6 | `/inventory/tasks/{id}/discrepancies` | 1h | 1h |
| 7 | `/energy/breakdown` + `/energy/summary` | 4h | 3h |
| 8 | Dashboard stats fix (CountRacks) | 0.5h | 0.5h |

**Total: ~14.5h backend + ~11.5h frontend**

### Phase 3: New Tables + Endpoints (P1+P2)

| # | Work | Backend | Frontend |
|---|------|---------|----------|
| 1 | inventory_scan_history table + API | 4h | 2h |
| 2 | inventory_notes table + API | 3h | 2h |
| 3 | work_order_comments + API | 3h | 2h |
| 4 | rack_network_connections + API | 3h | 2h |
| 5 | rack environment metrics endpoint | 2h | 2h |
| 6 | activity feed endpoint | 3h | 2h |
| 7 | audit event enrichment | 1h | 1h |
| 8 | topology graph (from BIA deps) | 4h | 6h |

**Total: ~23h backend + ~19h frontend**

### Phase 4: Advanced Features (P3)

| # | Work | Effort |
|---|------|--------|
| 1 | Prediction enhancement (RUL, failure distribution) | 9h backend + 4h frontend |
| 2 | User sessions + 2FA + password change | 7h total |
| 3 | Upgrade recommendation engine | 9h total |
| 4 | Sensor registry | 9h total |

**Total: ~38h**

---

## Grand Total Estimation

| Phase | Backend | Frontend | Total |
|-------|---------|----------|-------|
| Phase 1 (Quick Wins) | 0h | 8.5h | 8.5h |
| Phase 2 (Essential APIs) | 14.5h | 11.5h | 26h |
| Phase 3 (New Tables) | 23h | 19h | 42h |
| Phase 4 (Advanced) | ~25h | ~13h | 38h |
| **Grand Total** | **62.5h** | **52h** | **114.5h** |
