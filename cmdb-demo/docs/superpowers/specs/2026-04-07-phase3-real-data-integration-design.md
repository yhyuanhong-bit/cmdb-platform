# Phase 3: Real Data Integration — New Tables + Endpoints

**Date:** 2026-04-07
**Scope:** 5 new DB tables, 15 new API endpoints, 6 frontend pages connected to real data
**Depends on:** Phase 1 (frontend quick wins) + Phase 2 (essential backend endpoints)

---

## 1. Overview

Phase 3 eliminates remaining hardcoded/mock data in 6 frontend pages by adding 5 new database tables, 15 Go API endpoints, a cross-domain activity feed, and frontend integration with force-directed topology layout.

```
New DB Tables (5)
  ├── inventory_scan_history     → InventoryItemDetail page
  ├── inventory_notes            → InventoryItemDetail page
  ├── work_order_comments        → MaintenanceTaskView page
  ├── asset_dependencies         → AlertTopologyAnalysis page
  └── rack_network_connections   → RackDetailUnified page

New Endpoints (15)
  ├── Inventory (4): scan-history GET/POST, notes GET/POST
  ├── Maintenance (2): comments GET/POST
  ├── Topology (4): dependencies GET/POST/DELETE, graph GET
  ├── Rack (3): network-connections GET/POST/DELETE
  └── Activity + Audit (2): activity-feed GET, audit event detail GET

Frontend Changes (6 pages)
  ├── InventoryItemDetail    → scan history + notes from API
  ├── MaintenanceTaskView    → comments from API
  ├── RackDetailUnified      → network + activity from API
  ├── RackManagement         → activity feed from API
  ├── AlertTopologyAnalysis  → dynamic nodes/edges + force layout
  └── AuditEventDetail       → enriched audit event from API
```

---

## 2. Database Schema

### Migration: `000018_phase3_tables.up.sql`

```sql
-- 1. Inventory scan history
CREATE TABLE inventory_scan_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id     UUID NOT NULL REFERENCES inventory_items(id) ON DELETE CASCADE,
    scanned_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    scanned_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    method      VARCHAR(20) NOT NULL,   -- qr / rfid / manual / barcode
    result      VARCHAR(20) NOT NULL,   -- match / mismatch / location_update
    note        TEXT
);
CREATE INDEX idx_scan_history_item_time ON inventory_scan_history(item_id, scanned_at DESC);

-- 2. Inventory notes
CREATE TABLE inventory_notes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id     UUID NOT NULL REFERENCES inventory_items(id) ON DELETE CASCADE,
    author_id   UUID REFERENCES users(id) ON DELETE SET NULL,
    severity    VARCHAR(20) NOT NULL DEFAULT 'info',  -- info / warning / critical
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
    dependency_type VARCHAR(50) NOT NULL DEFAULT 'depends_on',  -- network / application / data / power
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
    status             VARCHAR(20) DEFAULT 'UP',   -- UP / DOWN
    vlans              INTEGER[],
    connection_type    VARCHAR(50) DEFAULT 'network',  -- network / power / management
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (NOT (connected_asset_id IS NOT NULL AND external_device IS NOT NULL))
);
CREATE INDEX idx_rack_net_conn_rack ON rack_network_connections(rack_id);
CREATE INDEX idx_rack_net_conn_tenant ON rack_network_connections(tenant_id);
CREATE INDEX idx_rack_net_conn_asset ON rack_network_connections(connected_asset_id);
CREATE INDEX idx_rack_net_conn_vlans ON rack_network_connections USING GIN(vlans);
```

### Down migration: `000018_phase3_tables.down.sql`

```sql
DROP TABLE IF EXISTS rack_network_connections;
DROP TABLE IF EXISTS asset_dependencies;
DROP TABLE IF EXISTS work_order_comments;
DROP TABLE IF EXISTS inventory_notes;
DROP TABLE IF EXISTS inventory_scan_history;
```

### Design Decisions

| Decision | Rationale |
|----------|-----------|
| Tables 1-3 have no `tenant_id` | Consistent with `work_order_logs` pattern — tenant isolation via FK chain (item → task → tenant, order → tenant) |
| Tables 4-5 have `tenant_id` | Top-level entities referencing assets across racks — need explicit tenant scoping |
| `target_asset_id ON DELETE RESTRICT` | Prevent silent deletion of depended-upon assets in topology |
| `rack_network_connections` CHECK constraint | Allows unconnected ports (both NULL) while preventing both FK + string set |
| `vlans INTEGER[]` with GIN index | Queryable array instead of TEXT blob |
| `scanned_by/author_id ON DELETE SET NULL` | Preserve history when users are deactivated |

---

## 3. API Endpoints

### 3.1 Inventory Scan History + Notes (4 endpoints)

All nested under existing inventory task/item path to match Gin param naming (`:id` for task, `:itemId` for item).

**GET `/inventory/tasks/:id/items/:itemId/scan-history`**
```json
{
  "scan_history": [
    {
      "id": "uuid",
      "scanned_at": "2026-04-07T10:32:00Z",
      "scanned_by": "uuid",
      "operator_name": "admin",
      "method": "qr",
      "result": "match",
      "note": "Verified in rack position"
    }
  ]
}
```

**POST `/inventory/tasks/:id/items/:itemId/scan-history`**
```json
// Request
{ "method": "qr", "result": "match", "note": "Optional note" }
// Response: created scan record
```

**GET `/inventory/tasks/:id/items/:itemId/notes`**
```json
{
  "notes": [
    {
      "id": "uuid",
      "author_name": "admin",
      "severity": "warning",
      "text": "Serial number mismatch needs investigation",
      "created_at": "2026-04-07T10:35:00Z"
    }
  ]
}
```

**POST `/inventory/tasks/:id/items/:itemId/notes`**
```json
// Request
{ "severity": "warning", "text": "Note content" }
// Response: created note record
```

Implementation: JOIN `users` to get `operator_name` / `author_name` in GET responses. Use `c.GetString("user_id")` for `scanned_by` / `author_id` in POST.

### 3.2 Work Order Comments (2 endpoints)

**GET `/maintenance/orders/:id/comments`**
```json
{
  "comments": [
    {
      "id": "uuid",
      "author_name": "Sarah Jenkins",
      "text": "Replaced faulty DIMM in slot B2",
      "created_at": "2026-04-07T14:20:00Z"
    }
  ]
}
```

**POST `/maintenance/orders/:id/comments`**
```json
// Request
{ "text": "Comment content" }
// Response: created comment record
```

### 3.3 Asset Topology Dependencies (4 endpoints)

**GET `/topology/dependencies?asset_id=uuid`**
```json
{
  "dependencies": [
    {
      "id": "uuid",
      "source_asset_id": "uuid",
      "source_asset_name": "Core-SW-01",
      "target_asset_id": "uuid",
      "target_asset_name": "SRV-PROD-001",
      "dependency_type": "network",
      "description": "Primary uplink"
    }
  ]
}
```
Query: returns all dependencies where `source_asset_id = $1 OR target_asset_id = $1`, JOIN assets for names.

**POST `/topology/dependencies`**
```json
// Request
{
  "tenant_id": "uuid",
  "source_asset_id": "uuid",
  "target_asset_id": "uuid",
  "dependency_type": "network",
  "description": "Primary uplink"
}
```

**DELETE `/topology/dependencies/:id`**
Returns 204 on success.

**GET `/topology/graph?location_id=uuid`**
```json
{
  "nodes": [
    {
      "id": "asset-uuid",
      "name": "SRV-PROD-001",
      "type": "server",
      "sub_type": "rack",
      "status": "operational",
      "bia_level": "critical",
      "ip_address": "192.168.1.10",
      "rack_name": "RACK-A01",
      "metrics": null
    }
  ],
  "edges": [
    {
      "id": "dep-uuid",
      "source": "asset-uuid-1",
      "target": "asset-uuid-2",
      "type": "network",
      "description": "Core uplink"
    }
  ]
}
```

Query logic:
1. Get all assets in location (via `assets.location_id` or descendants)
2. Get all `asset_dependencies` where `source_asset_id` or `target_asset_id` is in the asset set
3. `metrics` field is optional — query latest from `metrics` table if available, otherwise `null`
4. No `x, y` coordinates — frontend calculates layout via force-directed algorithm

### 3.4 Rack Network Connections (3 endpoints)

**GET `/racks/:id/network-connections`**
```json
{
  "connections": [
    {
      "id": "uuid",
      "source_port": "Eth1/1",
      "connected_asset_id": "uuid",
      "connected_asset_name": "Core-SW-01",
      "external_device": null,
      "speed": "100GbE",
      "status": "UP",
      "vlans": [100, 200, 300],
      "connection_type": "network"
    }
  ]
}
```
Query: JOIN assets for `connected_asset_name` when `connected_asset_id` is not null.

**POST `/racks/:id/network-connections`**
```json
// Request (internal asset)
{
  "source_port": "Eth1/1",
  "connected_asset_id": "uuid",
  "speed": "100GbE",
  "status": "UP",
  "vlans": [100, 200, 300],
  "connection_type": "network"
}
// Request (external device)
{
  "source_port": "MGMT",
  "external_device": "OOB-SW-01",
  "speed": "1GbE",
  "status": "UP",
  "vlans": [999],
  "connection_type": "management"
}
```
Validation: enforce CHECK constraint — exactly one of `connected_asset_id` or `external_device`.

**DELETE `/racks/:id/network-connections/:connectionId`**
Returns 204 on success.

### 3.5 Activity Feed (1 endpoint)

**GET `/activity-feed?target_type=rack&target_id=uuid`**

Supports `target_type`: `rack`, `asset`, `location`

```json
{
  "events": [
    {
      "event_type": "audit",
      "action": "asset.updated",
      "description": "asset.updated: SRV-PROD-001 status changed",
      "timestamp": "2026-04-07T10:32:00Z",
      "severity": "info",
      "operator": "admin"
    },
    {
      "event_type": "alert",
      "action": "alert.firing",
      "description": "Temperature exceeded threshold: 38.5C",
      "timestamp": "2026-04-07T09:15:00Z",
      "severity": "warning",
      "operator": null
    }
  ]
}
```

UNION query with target_type-aware WHERE clauses:

```sql
(SELECT 'audit' as event_type, ae.action,
        ae.module || '.' || ae.action as description,
        ae.created_at as timestamp, 'info' as severity,
        u.display_name as operator
 FROM audit_events ae
 LEFT JOIN users u ON ae.operator_id = u.id
 WHERE ae.target_type = $1 AND ae.target_id = $2
   AND ae.tenant_id = $3)
UNION ALL
(SELECT 'alert', 'alert.' || al.status,
        al.message, al.fired_at, al.severity, NULL
 FROM alert_events al
 WHERE al.tenant_id = $3
   AND (
     ($1 = 'rack' AND al.asset_id IN (
       SELECT rs.asset_id FROM rack_slots rs WHERE rs.rack_id = $2))
     OR ($1 = 'asset' AND al.asset_id = $2)
     OR ($1 = 'location' AND al.asset_id IN (
       SELECT rs.asset_id FROM rack_slots rs
       JOIN racks r ON rs.rack_id = r.id WHERE r.location_id = $2))
   ))
UNION ALL
(SELECT 'maintenance', wol.action,
        COALESCE(wol.comment, wol.from_status || ' -> ' || wol.to_status),
        wol.created_at, 'info', NULL
 FROM work_order_logs wol
 JOIN work_orders wo ON wol.order_id = wo.id
 WHERE wo.tenant_id = $3
   AND (
     ($1 = 'rack' AND wo.asset_id IN (
       SELECT rs.asset_id FROM rack_slots rs WHERE rs.rack_id = $2))
     OR ($1 = 'asset' AND wo.asset_id = $2)
     OR ($1 = 'location' AND wo.asset_id IN (
       SELECT rs.asset_id FROM rack_slots rs
       JOIN racks r ON rs.rack_id = r.id WHERE r.location_id = $2))
   ))
ORDER BY timestamp DESC LIMIT 20
```

### 3.6 Audit Event Detail (1 endpoint, enhanced)

**GET `/audit/events/:id`**

Enhancement over existing list endpoint — returns single event with operator details:

```json
{
  "event": {
    "id": "uuid",
    "action": "update",
    "module": "asset",
    "target_type": "Asset",
    "target_id": "uuid",
    "operator_id": "uuid",
    "operator_name": "admin",
    "operator_email": "admin@example.com",
    "diff": { "status": { "old": "inventoried", "new": "operational" } },
    "source": "web",
    "created_at": "2026-04-07T10:30:00Z"
  }
}
```

Query: `SELECT ae.*, u.display_name, u.email FROM audit_events ae LEFT JOIN users u ON ae.operator_id = u.id WHERE ae.id = $1`

---

## 4. Frontend Changes

### 4.1 New Files

| File | Purpose |
|------|---------|
| `src/lib/api/activity.ts` | Activity feed API client |
| `src/hooks/useActivityFeed.ts` | `useActivityFeed(targetType, targetId)` hook |
| `src/hooks/useForceLayout.ts` | Force-directed graph layout algorithm |

### 4.2 Extended Files

**`src/lib/api/topology.ts`** — add:
- `listDependencies(assetId)` → `GET /topology/dependencies?asset_id=`
- `createDependency(data)` → `POST /topology/dependencies`
- `deleteDependency(id)` → `DELETE /topology/dependencies/:id`
- `getTopologyGraph(locationId)` → `GET /topology/graph?location_id=`
- `listNetworkConnections(rackId)` → `GET /racks/:id/network-connections`
- `createNetworkConnection(rackId, data)` → `POST /racks/:id/network-connections`
- `deleteNetworkConnection(rackId, connId)` → `DELETE /racks/:id/network-connections/:connId`

**`src/hooks/useTopology.ts`** — add:
- `useTopologyGraph(locationId)`
- `useAssetDependencies(assetId)`
- `useCreateDependency()`
- `useDeleteDependency()`
- `useRackNetworkConnections(rackId)`
- `useCreateNetworkConnection()`
- `useDeleteNetworkConnection()`

**`src/hooks/useInventory.ts`** — add:
- `useItemScanHistory(taskId, itemId)`
- `useItemNotes(taskId, itemId)`
- `useCreateItemScanRecord()`
- `useCreateItemNote()`

**`src/hooks/useMaintenance.ts`** — add:
- `useWorkOrderComments(orderId)`
- `useCreateWorkOrderComment()`

**`src/hooks/useAudit.ts`** — add:
- `useAuditEvent(eventId)` — single event fetch

### 4.3 Page Changes

**InventoryItemDetail** (`src/pages/InventoryItemDetail.tsx`)
- Remove `SCAN_HISTORY` hardcoded array → use `useItemScanHistory(taskId, itemId)`
- Remove `DISCREPANCY_NOTES` hardcoded array → use `useItemNotes(taskId, itemId)`
- Wire "Add Note" submit button → `useCreateItemNote().mutate()`
- Data shape: API returns `operator_name`/`author_name` (JOIN), frontend uses directly

**MaintenanceTaskView** (`src/pages/MaintenanceTaskView.tsx`)
- Wire comment submit button (currently `alert('Coming Soon')`) → `useCreateWorkOrderComment().mutate()`
- Add comment list below textarea → `useWorkOrderComments(orderId)`
- Keep existing `useWorkOrderLogs` for timeline (separate from comments)

**RackDetailUnified** (`src/pages/RackDetailUnified.tsx`)
- Remove `networkConnections` hardcoded array → use `useRackNetworkConnections(rackId)`
- Remove `recentActivity` hardcoded array → use `useActivityFeed('rack', rackId)`
- Keep `environmentMetrics` — query via existing `useMetrics` hook (aggregate rack assets' metrics)
- Add "Add Connection" button → modal with port/device/speed/vlan fields

**RackManagement** (`src/pages/RackManagement.tsx`)
- Remove `recentEvents` hardcoded array → use `useActivityFeed('location', locationId)`

**AlertTopologyAnalysis** (`src/pages/AlertTopologyAnalysis.tsx`)
- Remove `NODES` hardcoded array (5 nodes with x,y) → use `useTopologyGraph(locationId)`
- Remove `EDGES` hardcoded array (5 edges) → same API returns edges
- Add `useForceLayout(nodes, edges)` hook for automatic node positioning
- Keep existing alert overlay from `useAlerts()` (already dynamic)
- Add "Add Dependency" button → modal → `useCreateDependency().mutate()`
- Add "Delete Dependency" on edges → `useDeleteDependency().mutate()`

**AuditEventDetail** (`src/pages/AuditEventDetail.tsx`)
- Replace `useAuditEvents(params)[0]` workaround → `useAuditEvent(eventId)`
- Remove `FALLBACK_EVENT` and `FALLBACK_DIFF` constants
- Use enriched response (operator_name, operator_email) directly

### 4.4 Force-Directed Layout Hook

`src/hooks/useForceLayout.ts`:

```ts
interface LayoutNode { id: string; x: number; y: number; [key: string]: any }
interface LayoutEdge { source: string; target: string; [key: string]: any }

function useForceLayout(
  nodes: any[],
  edges: any[],
  width: number,
  height: number
): LayoutNode[] {
  return useMemo(() => {
    if (!nodes.length) return []

    // Initialize positions in circle
    const positioned = nodes.map((n, i) => ({
      ...n,
      x: width / 2 + Math.cos((2 * Math.PI * i) / nodes.length) * Math.min(width, height) * 0.35,
      y: height / 2 + Math.sin((2 * Math.PI * i) / nodes.length) * Math.min(width, height) * 0.35,
    }))

    // Run 100 iterations of force simulation
    for (let iter = 0; iter < 100; iter++) {
      // Repulsion between all node pairs
      for (let i = 0; i < positioned.length; i++) {
        for (let j = i + 1; j < positioned.length; j++) {
          const dx = positioned[j].x - positioned[i].x
          const dy = positioned[j].y - positioned[i].y
          const dist = Math.max(Math.sqrt(dx * dx + dy * dy), 1)
          const force = 5000 / (dist * dist)
          const fx = (dx / dist) * force
          const fy = (dy / dist) * force
          positioned[i].x -= fx
          positioned[i].y -= fy
          positioned[j].x += fx
          positioned[j].y += fy
        }
      }

      // Attraction along edges
      for (const edge of edges) {
        const s = positioned.find(n => n.id === edge.source)
        const t = positioned.find(n => n.id === edge.target)
        if (!s || !t) continue
        const dx = t.x - s.x
        const dy = t.y - s.y
        const dist = Math.sqrt(dx * dx + dy * dy)
        const force = (dist - 200) * 0.01
        const fx = (dx / Math.max(dist, 1)) * force
        const fy = (dy / Math.max(dist, 1)) * force
        s.x += fx; s.y += fy
        t.x -= fx; t.y -= fy
      }

      // Center gravity
      for (const n of positioned) {
        n.x += (width / 2 - n.x) * 0.01
        n.y += (height / 2 - n.y) * 0.01
      }
    }

    // Clamp to bounds
    for (const n of positioned) {
      n.x = Math.max(60, Math.min(width - 60, n.x))
      n.y = Math.max(60, Math.min(height - 60, n.y))
    }

    return positioned
  }, [nodes, edges, width, height])
}
```

---

## 5. Go Backend File Structure

### New Files

| File | Content |
|------|---------|
| `cmdb-core/db/migrations/000018_phase3_tables.up.sql` | 5 new tables |
| `cmdb-core/db/migrations/000018_phase3_tables.down.sql` | Rollback |
| `cmdb-core/internal/api/phase3_inventory_endpoints.go` | Scan history + notes handlers (4) |
| `cmdb-core/internal/api/phase3_maintenance_endpoints.go` | Comment handlers (2) |
| `cmdb-core/internal/api/phase3_topology_endpoints.go` | Dependencies + graph handlers (4) |
| `cmdb-core/internal/api/phase3_rack_endpoints.go` | Network connection handlers (3) |
| `cmdb-core/internal/api/phase3_activity_endpoints.go` | Activity feed + audit detail handlers (2) |

### Modified Files

| File | Changes |
|------|---------|
| `cmdb-core/cmd/server/main.go` | Register 15 new routes on `v1` group |

### Route Registration

```go
// Phase 3 routes
v1.GET("/inventory/tasks/:id/items/:itemId/scan-history", apiServer.GetItemScanHistory)
v1.POST("/inventory/tasks/:id/items/:itemId/scan-history", apiServer.CreateItemScanRecord)
v1.GET("/inventory/tasks/:id/items/:itemId/notes", apiServer.GetItemNotes)
v1.POST("/inventory/tasks/:id/items/:itemId/notes", apiServer.CreateItemNote)
v1.GET("/maintenance/orders/:id/comments", apiServer.GetWorkOrderComments)
v1.POST("/maintenance/orders/:id/comments", apiServer.CreateWorkOrderComment)
v1.GET("/topology/dependencies", apiServer.GetAssetDependencies)
v1.POST("/topology/dependencies", apiServer.CreateAssetDependency)
v1.DELETE("/topology/dependencies/:id", apiServer.DeleteAssetDependency)
v1.GET("/topology/graph", apiServer.GetTopologyGraph)
v1.GET("/racks/:id/network-connections", apiServer.GetRackNetworkConnections)
v1.POST("/racks/:id/network-connections", apiServer.CreateRackNetworkConnection)
v1.DELETE("/racks/:id/network-connections/:connectionId", apiServer.DeleteRackNetworkConnection)
v1.GET("/activity-feed", apiServer.GetActivityFeed)
v1.GET("/audit/events/:id", apiServer.GetAuditEventDetail)
```

---

## 6. Frontend File Structure

### New Files

| File | Content |
|------|---------|
| `src/lib/api/activity.ts` | `getActivityFeed(params)` |
| `src/hooks/useActivityFeed.ts` | `useActivityFeed(targetType, targetId)` |
| `src/hooks/useForceLayout.ts` | Force-directed layout algorithm |

### Modified Files

| File | Changes |
|------|---------|
| `src/lib/api/topology.ts` | Add dependencies, graph, network-connections endpoints (7 methods) |
| `src/hooks/useTopology.ts` | Add 7 new hooks |
| `src/hooks/useInventory.ts` | Add 4 new hooks (scan-history, notes) |
| `src/hooks/useMaintenance.ts` | Add 2 new hooks (comments) |
| `src/hooks/useAudit.ts` | Add `useAuditEvent(id)` hook |
| `src/pages/InventoryItemDetail.tsx` | Replace SCAN_HISTORY + DISCREPANCY_NOTES with API |
| `src/pages/MaintenanceTaskView.tsx` | Wire comment submit + list |
| `src/pages/RackDetailUnified.tsx` | Replace networkConnections + recentActivity |
| `src/pages/RackManagement.tsx` | Replace recentEvents |
| `src/pages/AlertTopologyAnalysis.tsx` | Replace NODES/EDGES + force layout |
| `src/pages/AuditEventDetail.tsx` | Replace FALLBACK_EVENT + FALLBACK_DIFF |
