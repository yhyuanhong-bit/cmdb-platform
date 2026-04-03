# Frontend-Backend Integration Design Spec

> **Date**: 2026-04-03
> **Status**: Approved
> **Scope**: OpenAPI Spec First toolchain, code generation, 36-page frontend integration with Go backend

---

## 1. Decision Summary

| Decision Point | Choice |
|----------------|--------|
| Integration scope | All 36 pages, one-shot |
| Type consistency | **OpenAPI Spec First + dual code generation** |
| openapi.yaml generation | AI-assisted (from existing frontend interfaces + backend handlers) |
| Frontend client style | **Generate types only**, keep existing apiClient + hooks |
| Approach for type mismatches | Single source of truth: openapi.yaml generates both Go structs and TS types |

---

## 2. Architecture: OpenAPI Toolchain

### 2.1 Single Source of Truth

```
cmdb-platform/
├── api/
│   └── openapi.yaml                    ← single source of truth (~2000 lines)
│
├── cmdb-core/                           ← Go backend
│   ├── internal/api/
│   │   ├── generated.go                 ← oapi-codegen: ServerInterface + types
│   │   ├── impl.go                      ← hand-written: implements ServerInterface
│   │   └── convert.go                   ← hand-written: dbgen → API type conversion
│   └── Makefile                         ← make generate-api
│
├── cmdb-demo/                           ← React frontend
│   ├── src/generated/
│   │   └── api-types.ts                 ← openapi-typescript: all TS types
│   ├── src/lib/api/
│   │   ├── client.ts                    ← keep as-is (JWT, refresh, error handling)
│   │   ├── assets.ts                    ← modify: import types from generated/
│   │   └── ...                          ← same for all 11 modules
│   └── package.json                     ← npm run generate-api
│
└── Makefile                             ← make generate (runs both)
```

### 2.2 Generation Flow

```
openapi.yaml
     │
     ├──→ oapi-codegen ──→ cmdb-core/internal/api/generated.go
     │    Generates:
     │    - ServerInterface (method per endpoint)
     │    - Request/Response Go structs (JSON tags match yaml)
     │    - Gin route binding function
     │    - Parameter parsing + validation
     │
     └──→ openapi-typescript ──→ cmdb-demo/src/generated/api-types.ts
          Generates:
          - All schema types as TypeScript interfaces
          - Path types (which endpoint accepts/returns what)
          - Enum union types (AssetType, Status, Severity, etc.)
```

### 2.3 One Command

```makefile
# Root Makefile
generate:
	oapi-codegen -package api -generate types,gin api/openapi.yaml > cmdb-core/internal/api/generated.go
	cd cmdb-demo && npx openapi-typescript ../api/openapi.yaml -o src/generated/api-types.ts
```

### 2.4 Tool Versions

| Tool | Version | Purpose |
|------|---------|---------|
| oapi-codegen | v2.4+ | Go server interface + types from OpenAPI |
| openapi-typescript | 7.x | TypeScript types from OpenAPI |
| openapi.yaml | 3.1 | API contract |

---

## 3. openapi.yaml Structure

### 3.1 Top-Level

```yaml
openapi: "3.1.0"
info:
  title: CMDB Platform API
  version: "1.0.0"
servers:
  - url: /api/v1
security:
  - BearerAuth: []
```

### 3.2 Shared Schemas (components/schemas)

**Response wrappers:**
- `Meta` — `{ request_id: string }`
- `Pagination` — `{ page, page_size, total, total_pages: integer }`
- `ErrorResponse` — `{ error: { code, message: string }, meta: Meta }`

**Business types (12 schemas):**

| Schema | Key Fields | Nullable Fields |
|--------|-----------|-----------------|
| `Asset` | id, asset_tag, name, type, status, bia_level, created_at, updated_at | sub_type, location_id, rack_id, vendor, model, serial_number, property_number, control_number |
| `Location` | id, name, name_en, slug, level, path, status, sort_order, created_at | parent_id, metadata |
| `Rack` | id, location_id, name, total_u, status, created_at | row_label, power_capacity_kw, used_u, tags |
| `WorkOrder` | id, code, title, type, status, priority, created_at | location_id, asset_id, assignee_id, requestor_id, description, reason, scheduled_start/end, actual_start/end |
| `WorkOrderLog` | id, order_id, action, created_at | from_status, to_status, operator_id, comment |
| `AlertEvent` | id, status, severity, message, fired_at | rule_id, asset_id, trigger_value, acked_at, resolved_at |
| `InventoryTask` | id, code, name, scope_location_id, status, method, planned_date, created_at | completed_date, assigned_to |
| `InventoryItem` | id, task_id, expected, status | asset_id, rack_id, actual, scanned_at, scanned_by |
| `AuditEvent` | id, action, module, target_type, target_id, operator_id, created_at | diff, source |
| `User` | id, username, display_name, status, source, created_at | email, phone |
| `Role` | id, name, permissions, is_system | tenant_id, description |
| `PredictionModel` | id, name, type, provider, enabled | config |
| `PredictionResult` | id, model_id, asset_id, prediction_type, result, created_at | severity, recommended_action, expires_at |
| `RCAAnalysis` | id, incident_id, reasoning, confidence, human_verified | model_id, conclusion_asset_id, verified_by |
| `LocationStats` | total_assets, total_racks, critical_alerts | avg_occupancy |
| `DashboardStats` | total_assets, total_racks, critical_alerts, active_orders | — |
| `TokenPair` | access_token, refresh_token, expires_in | — |
| `CurrentUser` | id, username, display_name, email, permissions | — |

**Enums:**

| Enum | Values |
|------|--------|
| `AssetType` | server, network, storage, power |
| `AssetStatus` | inventoried, deployed, operational, maintenance, decommissioned |
| `BIALevel` | critical, important, normal, minor |
| `AlertSeverity` | critical, warning, info |
| `AlertStatus` | firing, acknowledged, resolved |
| `OrderStatus` | draft, pending, approved, rejected, in_progress, completed, closed |
| `LocationLevel` | country, region, city, campus, idc |

### 3.3 All Paths (33 endpoints)

#### Auth (3)

| Method | Path | Request | Response |
|--------|------|---------|----------|
| POST | /auth/login | `{ username, password }` | `{ data: TokenPair, meta }` |
| POST | /auth/refresh | `{ refresh_token }` | `{ data: TokenPair, meta }` |
| GET | /auth/me | — | `{ data: CurrentUser, meta }` |

#### Topology (8)

| Method | Path | Query Params | Response |
|--------|------|-------------|----------|
| GET | /locations | `slug?`, `level?` | `{ data: Location[], meta }` |
| GET | /locations/{id} | — | `{ data: Location, meta }` |
| GET | /locations/{id}/children | — | `{ data: Location[], meta }` |
| GET | /locations/{id}/ancestors | — | `{ data: Location[], meta }` |
| GET | /locations/{id}/stats | — | `{ data: LocationStats, meta }` |
| GET | /locations/{id}/racks | — | `{ data: Rack[], meta }` |
| GET | /racks/{id} | — | `{ data: Rack, meta }` |
| GET | /racks/{id}/assets | — | `{ data: Asset[], meta }` |

**Note:** `GET /locations` supports optional `slug` + `level` query params for slug-based lookup (required by frontend location routing which uses slugs, not UUIDs).

#### Assets (3)

| Method | Path | Query Params | Response |
|--------|------|-------------|----------|
| GET | /assets | `page?, page_size?, type?, status?, location_id?, rack_id?, serial_number?` | `{ data: Asset[], pagination, meta }` |
| GET | /assets/{id} | — | `{ data: Asset, meta }` |
| POST | /assets | `{ asset_tag, name, type, ... }` | `{ data: Asset, meta }` |

#### Maintenance (4)

| Method | Path | Query Params / Body | Response |
|--------|------|-------------------|----------|
| GET | /maintenance/orders | `page?, page_size?, status?, asset_id?` | `{ data: WorkOrder[], pagination, meta }` |
| GET | /maintenance/orders/{id} | — | `{ data: WorkOrder, meta }` |
| POST | /maintenance/orders | `{ title, type, priority, ... }` | `{ data: WorkOrder, meta }` |
| POST | /maintenance/orders/{id}/transition | `{ status, comment }` | `{ data: WorkOrder, meta }` |

#### Monitoring (3)

| Method | Path | Query Params | Response |
|--------|------|-------------|----------|
| GET | /monitoring/alerts | `page?, page_size?, status?, severity?, asset_id?` | `{ data: AlertEvent[], pagination, meta }` |
| POST | /monitoring/alerts/{id}/ack | — | `{ data: AlertEvent, meta }` |
| POST | /monitoring/alerts/{id}/resolve | — | `{ data: AlertEvent, meta }` |

#### Inventory (3)

| Method | Path | Query Params | Response |
|--------|------|-------------|----------|
| GET | /inventory/tasks | `page?, page_size?` | `{ data: InventoryTask[], pagination, meta }` |
| GET | /inventory/tasks/{id} | — | `{ data: InventoryTask, meta }` |
| GET | /inventory/tasks/{id}/items | — | `{ data: InventoryItem[], meta }` |

#### Audit (1)

| Method | Path | Query Params | Response |
|--------|------|-------------|----------|
| GET | /audit/events | `page?, page_size?, module?, target_type?, target_id?` | `{ data: AuditEvent[], pagination, meta }` |

#### Dashboard (1)

| Method | Path | Query Params | Response |
|--------|------|-------------|----------|
| GET | /dashboard/stats | `idc_id?` | `{ data: DashboardStats, meta }` |

#### Identity (3)

| Method | Path | Query Params | Response |
|--------|------|-------------|----------|
| GET | /users | `page?, page_size?` | `{ data: User[], pagination, meta }` |
| GET | /users/{id} | — | `{ data: User, meta }` |
| GET | /roles | — | `{ data: Role[], meta }` |

#### Prediction (4)

| Method | Path | Body | Response |
|--------|------|------|----------|
| GET | /prediction/models | — | `{ data: PredictionModel[], meta }` |
| GET | /prediction/results/ci/{ciId} | — | `{ data: PredictionResult[], meta }` |
| POST | /prediction/rca | `{ incident_id, model_name?, context? }` | `{ data: RCAAnalysis, meta }` |
| POST | /prediction/rca/{id}/verify | `{ verified_by }` | `{ data: RCAAnalysis, meta }` |

---

## 4. Backend Changes (Go)

### 4.1 New Files

| File | Purpose | Lines (est.) |
|------|---------|-------------|
| `internal/api/generated.go` | Auto-generated by oapi-codegen: ServerInterface + all API types | ~800 (auto) |
| `internal/api/impl.go` | Implements ServerInterface, delegates to domain services | ~600 |
| `internal/api/convert.go` | dbgen types → API types conversion functions | ~300 |

### 4.2 convert.go — Type Conversion Layer

Converts internal `dbgen.*` types (with `pgtype.UUID`, `pgtype.Text`, etc.) to generated API types (with `string`, `*string`, etc.).

12 conversion functions + 3 helpers:

```
toAPIAsset(dbgen.Asset) → api.Asset
toAPILocation(dbgen.Location) → api.Location
toAPIRack(dbgen.Rack) → api.Rack
toAPIWorkOrder(dbgen.WorkOrder) → api.WorkOrder
toAPIWorkOrderLog(dbgen.WorkOrderLog) → api.WorkOrderLog
toAPIAlertEvent(dbgen.AlertEvent) → api.AlertEvent
toAPIInventoryTask(dbgen.InventoryTask) → api.InventoryTask
toAPIInventoryItem(dbgen.InventoryItem) → api.InventoryItem
toAPIAuditEvent(dbgen.AuditEvent) → api.AuditEvent
toAPIUser(dbgen.User) → api.User
toAPIRole(dbgen.Role) → api.Role
toAPIPredictionResult(dbgen.PredictionResult) → api.PredictionResult

pguuidToPtr(pgtype.UUID) → *string       // Valid → "uuid-string", !Valid → nil
pgtextToPtr(pgtype.Text) → *string       // Valid → "string", !Valid → nil
pgtsToPtr(pgtype.Timestamptz) → *string  // Valid → RFC3339 string, !Valid → nil
```

### 4.3 impl.go — ServerInterface Implementation

Single struct that implements every method in the generated `ServerInterface`:

```go
type APIServer struct {
    assetSvc       *asset.Service
    topologySvc    *topology.Service
    maintenanceSvc *maintenance.Service
    monitoringSvc  *monitoring.Service
    inventorySvc   *inventory.Service
    auditSvc       *audit.Service
    dashboardSvc   *dashboard.Service
    identitySvc    *identity.Service
    authSvc        *identity.AuthService
    predictionSvc  *prediction.Service
}
```

Each method follows the same pattern:
1. Extract tenant_id from gin context (set by auth middleware)
2. Parse request params (generated code handles this)
3. Call domain service
4. Convert dbgen result → API type via convert.go
5. Return using response helpers

### 4.4 Existing Handler Removal

The 9 per-module `handler.go` files become dead code once `impl.go` replaces them. They should be removed to avoid confusion:

| Remove | Reason |
|--------|--------|
| `domain/asset/handler.go` | Replaced by impl.go ListAssets/GetAsset |
| `domain/topology/handler.go` | Replaced by impl.go location/rack methods |
| `domain/maintenance/handler.go` | Replaced by impl.go work order methods |
| `domain/monitoring/handler.go` | Replaced by impl.go alert methods |
| `domain/inventory/handler.go` | Replaced by impl.go inventory methods |
| `domain/audit/handler.go` | Replaced by impl.go audit methods |
| `domain/dashboard/handler.go` | Replaced by impl.go dashboard stats |
| `domain/identity/handler.go` | Replaced by impl.go auth/user/role methods |
| `domain/prediction/handler.go` | Replaced by impl.go prediction methods |

Domain services, models, and business logic files are **not touched**.

### 4.5 main.go Update

Replace per-module handler creation + registration with:

```go
apiServer := api.NewAPIServer(/* inject all services */)
api.RegisterHandlers(r, apiServer)  // generated function, registers all routes
```

### 4.6 Location Slug Lookup

Add to existing `db/queries/locations.sql`:

```sql
-- name: GetLocationBySlug :one
SELECT * FROM locations
WHERE tenant_id = $1 AND slug = $2 AND level = $3;
```

Add to topology service: `GetBySlug(ctx, tenantID, slug, level)`.

The `GET /locations` endpoint gains optional `slug` + `level` query params. When provided, returns matching locations instead of root list.

---

## 5. Frontend Changes (React)

### 5.1 Generated Types Setup

```bash
# Install
cd cmdb-demo
npm install -D openapi-typescript

# Generate
npx openapi-typescript ../api/openapi.yaml -o src/generated/api-types.ts

# Add to package.json scripts
"generate-api": "openapi-typescript ../api/openapi.yaml -o src/generated/api-types.ts"
```

### 5.2 API Module Migration (11 files)

Each file in `src/lib/api/*.ts` changes from:

```typescript
// BEFORE: type defined inline
export interface Asset {
  id: string
  asset_tag: string
  // ... 15 fields
}

export const assetApi = {
  list: (params?) => apiClient.get<ApiListResponse<Asset>>('/assets', params),
}
```

To:

```typescript
// AFTER: type imported from generated
import type { Asset } from '@/generated/api-types'

export type { Asset }  // re-export for backward compat

export const assetApi = {
  list: (params?) => apiClient.get<ApiListResponse<Asset>>('/assets', params),
}
```

**What stays the same:** apiClient, all API functions, ApiResponse/ApiListResponse wrappers.
**What changes:** interface definitions removed, replaced by generated import.

### 5.3 New Hooks (5 files)

| File | Hooks | Pattern |
|------|-------|---------|
| `hooks/useInventory.ts` | useInventoryTasks, useInventoryTask, useInventoryItems | useQuery + queryKey |
| `hooks/useAudit.ts` | useAuditEvents | useQuery with filter params |
| `hooks/useDashboard.ts` | useDashboardStats | useQuery, staleTime 30s |
| `hooks/usePrediction.ts` | usePredictionModels, usePredictionsByAsset, useCreateRCA, useVerifyRCA | useQuery + useMutation |
| `hooks/useIdentity.ts` | useUsers, useUser, useRoles | useQuery |

All follow the existing pattern in `useAssets.ts` / `useMaintenance.ts`.

### 5.4 Shared Component: QueryWrapper

```tsx
// src/components/QueryWrapper.tsx
interface QueryWrapperProps<T> {
  query: UseQueryResult<T>
  children: (data: T) => ReactNode
  emptyMessage?: string
}
```

Handles loading spinner, error banner with retry, and empty state. Used by all 32 data pages.

### 5.5 Page Migration (32 pages)

Every page follows the same 4-step migration:

1. **Replace imports** — type source changes to generated
2. **Delete mock data** — remove const MOCK_*, mockAssets, ALERTS, etc.
3. **Add hook call** — `const { data, isLoading, error } = useXxx(params)`
4. **Add state handling** — wrap render in QueryWrapper or manual if/else

Rendering logic (JSX) stays largely unchanged since the generated types match the mock data shapes.

### 5.6 Location System Migration

**Problem:** Frontend routes use slugs (`/locations/:countrySlug`), API uses UUIDs.

**Solution:** Two-step resolution in each location page:

```
URL slug → GET /locations?slug=tw&level=country → Location object (has UUID)
        → GET /locations/{uuid}/children → child locations
```

**LocationContext changes:**
- Store full Location objects (with both slug and id) instead of just names
- Navigation: user clicks card → context stores Location → next page reads id from context
- Direct URL access (bookmark): page calls slug-lookup API first, then proceeds

### 5.7 Vite Configuration

```typescript
// vite.config.ts additions
server: {
  proxy: {
    '/api/v1': {
      target: 'http://localhost:8080',
      changeOrigin: true,
    },
  },
}
```

```bash
# .env.development
VITE_API_URL=/api/v1

# .env.development.local (optional, for direct backend access)
VITE_API_URL=http://localhost:8080/api/v1
```

### 5.8 Auth Flow Activation

Current state: Login page exists, authStore has full login/refresh logic, but `AuthGuard` is bypassed in demo mode.

Integration:
1. Re-enable `AuthGuard` wrapper around `MainLayout` in App.tsx
2. authStore.login() already calls `POST /auth/login` — just needs backend running
3. Token refresh already handled in apiClient — works automatically
4. Sidebar user card already reads from authStore.user — auto-populated after login

---

## 6. Page-to-Endpoint Complete Mapping

### Pages that need API integration (32)

| # | Page | Route | Hook(s) | API Endpoint(s) |
|---|------|-------|---------|-----------------|
| 1 | Login | /login | authStore.login | POST /auth/login |
| 2 | Dashboard | /dashboard | useDashboardStats | GET /dashboard/stats |
| 3 | GlobalOverview | /locations | useRootLocations | GET /locations |
| 4 | RegionOverview | /locations/:country | useLocationBySlug, useLocationChildren | GET /locations?slug, GET /locations/{id}/children |
| 5 | CityOverview | /locations/:country/:region | useLocationBySlug, useLocationChildren | GET /locations?slug, GET /locations/{id}/children |
| 6 | CampusOverview | /locations/:country/:region/:city | useLocationBySlug, useLocationChildren, useRacks | GET /locations?slug, GET /locations/{id}/children, GET /locations/{id}/racks |
| 7 | AssetManagement | /assets | useAssets | GET /assets |
| 8 | AssetDetail | /assets/detail | useAsset | GET /assets/{id} |
| 9 | AssetLifecycle | /assets/lifecycle | useAssets | GET /assets (group by status) |
| 10 | AssetLifecycleTimeline | /assets/lifecycle/timeline | useAssets | GET /assets |
| 11 | AutoDiscovery | /assets/discovery | useAssets | GET /assets |
| 12 | ComponentUpgrades | /assets/upgrades | useAssets | GET /assets |
| 13 | EquipmentHealth | /assets/equipment-health | useAssets | GET /assets?type=server |
| 14 | RackManagement | /racks | useRacks | GET /locations/{id}/racks |
| 15 | RackDetail | /racks/detail | useRack, useRackAssets | GET /racks/{id}, GET /racks/{id}/assets |
| 16 | DataCenter3D | /racks/3d | useRacks | GET /locations/{id}/racks |
| 17 | AddNewRack | /racks/add | useCreateRack (new mutation) | POST /racks |
| 18 | FacilityMap | /racks/facility-map | useRacks | GET /locations/{id}/racks |
| 19 | HighSpeedInventory | /inventory | useInventoryTasks | GET /inventory/tasks |
| 20 | InventoryItemDetail | /inventory/detail | useInventoryTask, useInventoryItems | GET /inventory/tasks/{id}, GET /inventory/tasks/{id}/items |
| 21 | MonitoringAlerts | /monitoring | useAlerts, useAcknowledgeAlert, useResolveAlert | GET /monitoring/alerts, POST .../ack, POST .../resolve |
| 22 | SystemHealth | /monitoring/health | useAlerts | GET /monitoring/alerts?severity=critical |
| 23 | AlertTopology | /monitoring/topology | useAlerts | GET /monitoring/alerts |
| 24 | MaintenanceHub | /maintenance | useWorkOrders | GET /maintenance/orders |
| 25 | MaintenanceTaskView | /maintenance/task | useWorkOrder | GET /maintenance/orders/{id} |
| 26 | WorkOrder | /maintenance/workorder | useWorkOrders, useTransitionWorkOrder | GET /maintenance/orders, POST .../transition |
| 27 | AddMaintenanceTask | /maintenance/add | useCreateWorkOrder | POST /maintenance/orders |
| 28 | TaskDispatch | /maintenance/dispatch | useWorkOrders | GET /maintenance/orders |
| 29 | PredictiveHub | /predictive | usePredictionModels, usePredictionsByAsset | GET /prediction/models, GET /prediction/results/ci/{id} |
| 30 | AuditHistory | /audit | useAuditEvents | GET /audit/events |
| 31 | AuditEventDetail | /audit/detail | useAuditEvents | GET /audit/events?target_id={id} |
| 32 | RolesPermissions | /system | useUsers, useRoles | GET /users, GET /roles |

### Pages with no API needed (4 + 2 partial)

| Page | Route | Reason |
|------|-------|--------|
| Welcome | /welcome | Static onboarding |
| TroubleshootingGuide | /help/troubleshooting | Static content |
| VideoLibrary | /help/videos | Static content |
| VideoPlayer | /help/videos/player | Static content |
| EnergyMonitor | /monitoring/energy | Keep mock (needs metrics ingestion first) |
| SensorConfiguration | /monitoring/sensors | Keep mock (no backend endpoint yet) |
| SystemSettings | /system/settings | Keep mock (pure frontend settings) |
| UserProfile | /system/profile | Uses GET /auth/me (already in authStore) |

---

## 7. Change Summary

| Area | Files Changed | Lines Est. |
|------|--------------|-----------|
| **openapi.yaml** (new) | 1 | ~2000 (AI generated) |
| **Go generated.go** (auto) | 1 | ~800 (auto) |
| **Go impl.go** (new) | 1 | ~600 |
| **Go convert.go** (new) | 1 | ~300 |
| **Go handler.go removal** | 9 deleted | -900 |
| **Go main.go update** | 1 modified | ~20 |
| **Go location slug query** | 1 modified | ~10 |
| **TS generated types** (auto) | 1 | ~500 (auto) |
| **TS API modules** | 11 modified | ~110 (import changes) |
| **TS new hooks** | 5 new | ~150 |
| **TS QueryWrapper** | 1 new | ~50 |
| **TS page migration** | 32 modified | ~640 (avg 20 lines/page) |
| **TS location context** | 1 modified | ~50 |
| **TS vite config** | 1 modified | ~10 |
| **TS App.tsx** (AuthGuard) | 1 modified | ~5 |
| **Root Makefile** | 1 modified | ~5 |
| **Total** | ~68 files | ~5250 lines (hand-written ~2650, generated ~2600) |
