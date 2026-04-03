# Frontend Integration (Phase 5b) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Connect all 32 data-driven frontend pages to the real backend API by generating TypeScript types from openapi.yaml, migrating API module imports, creating 5 new hooks, adding shared QueryWrapper component, and replacing mock data with hook calls in every page.

**Architecture:** Generate TS types via openapi-typescript from the shared openapi.yaml. Replace inline interface definitions in 11 API modules with generated imports. Add 5 missing hooks (inventory, audit, dashboard, prediction, identity). Create a reusable QueryWrapper component for loading/error states. Migrate 32 pages: delete mock data, add hook calls, wrap renders with QueryWrapper. Update Vite proxy config and re-enable AuthGuard.

**Tech Stack:** openapi-typescript 7.x, React 19, React Query 5, Zustand 5, existing Tailwind/Gin/TypeScript stack

**Spec Reference:** `docs/superpowers/specs/2026-04-03-frontend-backend-integration-design.md` — Section 5

---

## File Structure

```
cmdb-demo/
├── src/generated/
│   └── api-types.ts                         # NEW: auto-generated from openapi.yaml
│
├── src/lib/api/
│   ├── client.ts                            # NO CHANGE
│   ├── types.ts                             # NO CHANGE (wrappers stay)
│   ├── assets.ts                            # MODIFY: import type from generated
│   ├── topology.ts                          # MODIFY
│   ├── maintenance.ts                       # MODIFY
│   ├── monitoring.ts                        # MODIFY
│   ├── inventory.ts                         # MODIFY
│   ├── audit.ts                             # MODIFY
│   ├── identity.ts                          # MODIFY
│   ├── prediction.ts                        # MODIFY
│   └── integration.ts                       # MODIFY
│
├── src/hooks/
│   ├── useAssets.ts                         # NO CHANGE
│   ├── useTopology.ts                       # NO CHANGE
│   ├── useMaintenance.ts                    # NO CHANGE
│   ├── useMonitoring.ts                     # NO CHANGE
│   ├── useAuth.ts                           # NO CHANGE
│   ├── usePermission.ts                     # NO CHANGE
│   ├── useInventory.ts                      # NEW
│   ├── useAudit.ts                          # NEW
│   ├── useDashboard.ts                      # NEW
│   ├── usePrediction.ts                     # NEW
│   └── useIdentity.ts                       # NEW
│
├── src/components/
│   └── QueryWrapper.tsx                     # NEW
│
├── src/pages/
│   ├── Dashboard.tsx                        # MODIFY (mock → hook)
│   ├── AssetManagementUnified.tsx           # MODIFY
│   ├── ... (30 more pages)                  # MODIFY
│   └── locations/
│       ├── GlobalOverview.tsx               # MODIFY
│       ├── RegionOverview.tsx               # MODIFY
│       ├── CityOverview.tsx                 # MODIFY
│       └── CampusOverview.tsx               # MODIFY
│
├── src/App.tsx                              # MODIFY (re-enable AuthGuard)
├── vite.config.ts                           # MODIFY (add proxy)
├── .env.development                         # NEW
└── package.json                             # MODIFY (add generate-api script)
```

---

## Task 1: Generate TS Types + Vite Config + .env

**Files:**
- Create: `cmdb-demo/src/generated/api-types.ts` (auto-generated)
- Create: `cmdb-demo/.env.development`
- Modify: `cmdb-demo/vite.config.ts`
- Modify: `cmdb-demo/package.json`

- [ ] **Step 1: Install openapi-typescript**

```bash
cd /cmdb-platform/cmdb-demo
npm install -D openapi-typescript
```

- [ ] **Step 2: Generate types**

```bash
mkdir -p src/generated
npx openapi-typescript ../api/openapi.yaml -o src/generated/api-types.ts
```

Expected: `src/generated/api-types.ts` created with TypeScript interfaces for all schemas.

- [ ] **Step 3: Verify generated types**

```bash
grep 'Asset' src/generated/api-types.ts | head -5
grep 'Location' src/generated/api-types.ts | head -5
```

Expected: Asset, Location, and other schema types present.

- [ ] **Step 4: Add generate-api script to package.json**

Read `package.json`, then add to `"scripts"`:

```json
"generate-api": "openapi-typescript ../api/openapi.yaml -o src/generated/api-types.ts"
```

- [ ] **Step 5: Create .env.development**

Create `cmdb-demo/.env.development`:

```
VITE_API_URL=/api/v1
```

- [ ] **Step 6: Add proxy to vite.config.ts**

Read `vite.config.ts`, then add `proxy` to the `server` config:

```typescript
server: {
  port: 5174,
  host: '0.0.0.0',
  proxy: {
    '/api/v1': {
      target: 'http://localhost:8080',
      changeOrigin: true,
    },
  },
},
```

- [ ] **Step 7: Verify TypeScript compiles**

```bash
npx tsc --noEmit 2>&1 | head -20
```

Note: There may be existing TS errors. The generated types file itself should not introduce new ones.

- [ ] **Step 8: Commit**

```bash
git add src/generated/ .env.development vite.config.ts package.json package-lock.json
git commit -m "feat: generate TS types from openapi.yaml + vite proxy + env config"
```

---

## Task 2: Migrate API Modules (Replace Inline Types with Generated)

**Files:**
- Modify: `cmdb-demo/src/lib/api/assets.ts`
- Modify: `cmdb-demo/src/lib/api/topology.ts`
- Modify: `cmdb-demo/src/lib/api/maintenance.ts`
- Modify: `cmdb-demo/src/lib/api/monitoring.ts`
- Modify: `cmdb-demo/src/lib/api/inventory.ts`
- Modify: `cmdb-demo/src/lib/api/audit.ts`
- Modify: `cmdb-demo/src/lib/api/identity.ts`
- Modify: `cmdb-demo/src/lib/api/prediction.ts`
- Modify: `cmdb-demo/src/lib/api/integration.ts`

- [ ] **Step 1: Read generated types to understand the import path and naming**

```bash
head -50 src/generated/api-types.ts
```

The generated file uses a specific export pattern. It may export types nested under `components['schemas']` or as top-level types depending on openapi-typescript version. Understand the exact pattern before modifying API modules.

If types are at `components['schemas']['Asset']`, you'll need:
```typescript
import type { components } from '@/generated/api-types'
type Asset = components['schemas']['Asset']
export type { Asset }
```

If types are exported directly:
```typescript
import type { Asset } from '@/generated/api-types'
export type { Asset }
```

- [ ] **Step 2: Migrate each API module**

For EACH of the 9 API module files (assets.ts, topology.ts, maintenance.ts, monitoring.ts, inventory.ts, audit.ts, identity.ts, prediction.ts, integration.ts):

1. Read the current file
2. Find the `export interface Xxx { ... }` block(s)
3. Replace them with imports from generated types + re-exports
4. Keep all `const xxxApi = { ... }` functions UNCHANGED
5. Keep the `apiClient` import UNCHANGED

Pattern for each file:
```typescript
// BEFORE:
export interface Asset {
  id: string
  // ... 15+ fields
}

// AFTER:
import type { components } from '@/generated/api-types'
export type Asset = components['schemas']['Asset']
```

For files with multiple interfaces (e.g., topology.ts has Location + Rack, monitoring.ts has AlertEvent + AlertRule + Incident):
```typescript
import type { components } from '@/generated/api-types'
export type Location = components['schemas']['Location']
export type Rack = components['schemas']['Rack']
```

IMPORTANT: The `@/` alias maps to `./src/` (configured in vite.config.ts tsconfig). If it doesn't work, use relative path `../../generated/api-types`.

- [ ] **Step 3: Update client.ts BASE_URL default**

In `src/lib/api/client.ts`, change the default BASE_URL:

```typescript
// BEFORE:
const BASE_URL = import.meta.env.VITE_API_URL || 'http://10.134.143.218:8080/api/v1'

// AFTER:
const BASE_URL = import.meta.env.VITE_API_URL || '/api/v1'
```

- [ ] **Step 4: Verify build**

```bash
npx tsc --noEmit 2>&1 | head -30
```

Fix any import errors.

- [ ] **Step 5: Commit**

```bash
git add src/lib/api/
git commit -m "feat: migrate 9 API modules to use generated types from openapi.yaml"
```

---

## Task 3: New Hooks (5 files) + QueryWrapper Component

**Files:**
- Create: `cmdb-demo/src/hooks/useInventory.ts`
- Create: `cmdb-demo/src/hooks/useAudit.ts`
- Create: `cmdb-demo/src/hooks/useDashboard.ts`
- Create: `cmdb-demo/src/hooks/usePrediction.ts`
- Create: `cmdb-demo/src/hooks/useIdentity.ts`
- Create: `cmdb-demo/src/components/QueryWrapper.tsx`

- [ ] **Step 1: Read existing hooks to match the pattern**

```bash
cat src/hooks/useAssets.ts
cat src/hooks/useMaintenance.ts
```

Follow the exact same pattern: useQuery with queryKey, useMutation with queryClient.invalidateQueries.

- [ ] **Step 2: Create useInventory.ts**

```typescript
import { useQuery } from '@tanstack/react-query'
import { inventoryApi } from '../lib/api/inventory'

export function useInventoryTasks(params?: Record<string, string>) {
  return useQuery({
    queryKey: ['inventoryTasks', params],
    queryFn: () => inventoryApi.list(params),
  })
}

export function useInventoryTask(id: string) {
  return useQuery({
    queryKey: ['inventoryTasks', id],
    queryFn: () => inventoryApi.getById(id),
    enabled: !!id,
  })
}

export function useInventoryItems(taskId: string) {
  return useQuery({
    queryKey: ['inventoryTasks', taskId, 'items'],
    queryFn: () => inventoryApi.listItems(taskId),
    enabled: !!taskId,
  })
}
```

- [ ] **Step 3: Create useAudit.ts**

```typescript
import { useQuery } from '@tanstack/react-query'
import { auditApi } from '../lib/api/audit'

export function useAuditEvents(params?: Record<string, string>) {
  return useQuery({
    queryKey: ['auditEvents', params],
    queryFn: () => auditApi.query(params),
  })
}
```

- [ ] **Step 4: Create useDashboard.ts**

```typescript
import { useQuery } from '@tanstack/react-query'
import { apiClient } from '../lib/api/client'
import type { ApiResponse } from '../lib/api/types'

interface DashboardStats {
  total_assets: number
  total_racks: number
  critical_alerts: number
  active_orders: number
}

export function useDashboardStats(params?: Record<string, string>) {
  return useQuery({
    queryKey: ['dashboardStats', params],
    queryFn: () => apiClient.get<ApiResponse<DashboardStats>>('/dashboard/stats', params),
  })
}
```

- [ ] **Step 5: Create usePrediction.ts**

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { predictionApi } from '../lib/api/prediction'

export function usePredictionModels() {
  return useQuery({
    queryKey: ['predictionModels'],
    queryFn: () => predictionApi.listModels(),
  })
}

export function usePredictionsByAsset(ciId: string) {
  return useQuery({
    queryKey: ['predictions', ciId],
    queryFn: () => predictionApi.listByCI(ciId),
    enabled: !!ciId,
  })
}

export function useCreateRCA() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: predictionApi.createRCA,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['predictions'] }),
  })
}

export function useVerifyRCA() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) => predictionApi.verifyRCA(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['predictions'] }),
  })
}
```

- [ ] **Step 6: Create useIdentity.ts**

```typescript
import { useQuery } from '@tanstack/react-query'
import { identityApi } from '../lib/api/identity'

export function useUsers(params?: Record<string, string>) {
  return useQuery({
    queryKey: ['users', params],
    queryFn: () => identityApi.listUsers(params),
  })
}

export function useUser(id: string) {
  return useQuery({
    queryKey: ['users', id],
    queryFn: () => identityApi.getUser(id),
    enabled: !!id,
  })
}

export function useRoles() {
  return useQuery({
    queryKey: ['roles'],
    queryFn: () => identityApi.listRoles(),
  })
}
```

- [ ] **Step 7: Create QueryWrapper.tsx**

```tsx
import { type ReactNode } from 'react'
import { type UseQueryResult } from '@tanstack/react-query'
import Icon from './Icon'

interface QueryWrapperProps<T> {
  query: UseQueryResult<T>
  children: (data: T) => ReactNode
  emptyMessage?: string
}

export default function QueryWrapper<T>({ query, children, emptyMessage }: QueryWrapperProps<T>) {
  if (query.isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-sky-400 border-t-transparent" />
      </div>
    )
  }

  if (query.error) {
    return (
      <div className="flex flex-col items-center justify-center py-20 gap-3">
        <Icon name="error" className="text-red-400 text-4xl" />
        <p className="text-red-300 text-sm">Failed to load data</p>
        <button
          onClick={() => query.refetch()}
          className="px-4 py-1.5 rounded bg-white/10 text-sm hover:bg-white/20 transition"
        >
          Retry
        </button>
      </div>
    )
  }

  if (!query.data) {
    return (
      <div className="flex flex-col items-center justify-center py-20 gap-2">
        <Icon name="inbox" className="text-white/30 text-4xl" />
        <p className="text-white/40 text-sm">{emptyMessage || 'No data'}</p>
      </div>
    )
  }

  return <>{children(query.data)}</>
}
```

- [ ] **Step 8: Commit**

```bash
git add src/hooks/ src/components/QueryWrapper.tsx
git commit -m "feat: add 5 new hooks (inventory, audit, dashboard, prediction, identity) + QueryWrapper"
```

---

## Task 4: Migrate Core Pages — Dashboard + Auth + Location (6 pages)

**Files:**
- Modify: `cmdb-demo/src/App.tsx`
- Modify: `cmdb-demo/src/pages/Dashboard.tsx`
- Modify: `cmdb-demo/src/pages/locations/GlobalOverview.tsx`
- Modify: `cmdb-demo/src/pages/locations/RegionOverview.tsx`
- Modify: `cmdb-demo/src/pages/locations/CityOverview.tsx`
- Modify: `cmdb-demo/src/pages/locations/CampusOverview.tsx`

For each page, the migration pattern is:
1. Add hook imports at top
2. Delete mock interfaces + mock data arrays
3. Replace `useState(mockData)` / `const data = MOCK_DATA` with hook call
4. Add loading/error handling (use QueryWrapper or inline isLoading/error checks)
5. Adapt field names if the mock used different names than the API (e.g., mock `serialNumber` vs API `serial_number`)

- [ ] **Step 1: Re-enable AuthGuard in App.tsx**

Read `src/App.tsx`. Find where MainLayout is used. The AuthGuard is likely commented out or removed. Wrap MainLayout:

```tsx
// Find: <Route element={<MainLayout />}>
// Change to: <Route element={<AuthGuard><MainLayout /></AuthGuard>}>
```

If AuthGuard import is missing, add: `import AuthGuard from './components/AuthGuard'`

NOTE: For development, you may want to keep AuthGuard disabled until the backend is actually running. Consider adding a dev bypass:
```tsx
const DevLayout = import.meta.env.DEV ? MainLayout : () => <AuthGuard><MainLayout /></AuthGuard>
```

Actually, SKIP this step for now — leave AuthGuard as-is. Focus on data migration first. Auth integration is a separate concern that requires the backend running.

- [ ] **Step 2: Migrate Dashboard.tsx**

Read the full file. Find:
- Mock data: `allIdcs`, `BIA_SEGMENTS`, `HEATMAP_DATA`, `CRITICAL_EVENTS`, hardcoded stats calculations
- These should be replaced with `useDashboardStats()`

The Dashboard aggregates data from multiple sources. For the initial migration:
- Replace the top-level stats (Total Assets, Racks, Alerts) with `useDashboardStats()`
- Keep secondary visual elements (heatmap, BIA pie chart) with hardcoded data for now — these need additional backend endpoints not yet available

```tsx
import { useDashboardStats } from '../hooks/useDashboard'

// Inside component:
const statsQuery = useDashboardStats()
const stats = statsQuery.data?.data

// Replace hardcoded values:
// totalAssets → stats?.total_assets ?? 0
// criticalAlerts → stats?.critical_alerts ?? 0
// activeOrders → stats?.active_orders ?? 0
```

- [ ] **Step 3: Migrate GlobalOverview.tsx**

This page uses `COUNTRIES` mock array. Replace with:
```tsx
import { useRootLocations } from '../../hooks/useTopology'

const locationsQuery = useRootLocations()
const countries = locationsQuery.data?.data ?? []
```

Adapt rendering: mock countries had fields like `nameCn`, `idcCount`, `totalAssets`. The API returns Location objects with `name`, `name_en`, `metadata` (which may contain stats). For fields not directly available from the Location API, use `useLocationStats(id)` per country, or simplify the display for now.

- [ ] **Step 4: Migrate RegionOverview, CityOverview, CampusOverview**

These pages use URL slug params (`useParams<{countrySlug}>`) and look up mock data by slug.

For each page, the pattern is:
```tsx
import { useRootLocations, useLocationChildren, useRacks } from '../../hooks/useTopology'

// Step 1: Find parent location by slug (from root locations or parent's children)
const { countrySlug } = useParams()
const rootQuery = useRootLocations()
const country = rootQuery.data?.data?.find(l => l.slug === countrySlug)

// Step 2: Fetch children using parent's UUID
const childrenQuery = useLocationChildren(country?.id ?? '')
const regions = childrenQuery.data?.data ?? []
```

For CampusOverview, also fetch racks:
```tsx
const racksQuery = useRacks(campus?.id ?? '')
```

Keep mock data as fallback during development if the slug lookup chain fails.

- [ ] **Step 5: Commit**

```bash
git add src/App.tsx src/pages/Dashboard.tsx src/pages/locations/
git commit -m "feat: migrate Dashboard + 4 location pages from mock to API hooks"
```

---

## Task 5: Migrate Asset Pages (7 pages)

**Files:**
- Modify: `cmdb-demo/src/pages/AssetManagementUnified.tsx`
- Modify: `cmdb-demo/src/pages/AssetDetailUnified.tsx`
- Modify: `cmdb-demo/src/pages/AssetLifecycle.tsx`
- Modify: `cmdb-demo/src/pages/AssetLifecycleTimeline.tsx`
- Modify: `cmdb-demo/src/pages/AutoDiscovery.tsx`
- Modify: `cmdb-demo/src/pages/ComponentUpgradeRecommendations.tsx`
- Modify: `cmdb-demo/src/pages/EquipmentHealthOverview.tsx`

- [ ] **Step 1: Migrate AssetManagementUnified.tsx**

This page ALREADY has `useAssets()` but falls back to mock. Remove the mock fallback:

```tsx
// BEFORE:
const { data: apiData } = useAssets()
const apiAssets = apiData?.data ?? null
const assets = apiAssets || mockAssets  // falls back to mock

// AFTER:
const { data: apiData, isLoading, error } = useAssets(filterParams)
const assets = apiData?.data ?? []
```

Delete the entire `mockAssets` array.

- [ ] **Step 2: Migrate remaining 6 asset pages**

For each page:
1. Import `useAssets` or `useAsset`
2. Delete mock arrays (mockAssets, EQUIPMENT, etc.)
3. Replace with hook call
4. Add loading state

AssetDetail: `useAsset(assetId)` — the asset ID comes from a query param or navigation state
AssetLifecycle: `useAssets()` — group by status in frontend
EquipmentHealth: `useAssets({type: 'server'})` — filter server type
AutoDiscovery: `useAssets()` — show discovered assets
ComponentUpgrades: `useAssets()` — show assets with upgrade potential

- [ ] **Step 3: Commit**

```bash
git add src/pages/AssetManagementUnified.tsx src/pages/AssetDetailUnified.tsx src/pages/AssetLifecycle.tsx src/pages/AssetLifecycleTimeline.tsx src/pages/AutoDiscovery.tsx src/pages/ComponentUpgradeRecommendations.tsx src/pages/EquipmentHealthOverview.tsx
git commit -m "feat: migrate 7 asset pages from mock to useAssets/useAsset hooks"
```

---

## Task 6: Migrate Rack Pages (5 pages)

**Files:**
- Modify: `cmdb-demo/src/pages/RackManagement.tsx`
- Modify: `cmdb-demo/src/pages/RackDetailUnified.tsx`
- Modify: `cmdb-demo/src/pages/DataCenter3D.tsx`
- Modify: `cmdb-demo/src/pages/AddNewRack.tsx`
- Modify: `cmdb-demo/src/pages/FacilityMap.tsx`

- [ ] **Step 1: Migrate all 5 rack pages**

Pattern for each:
```tsx
import { useRacks } from '../hooks/useTopology'

// RackManagement already imports useRacks but it's commented out.
// Uncomment and remove mock data.

const racksQuery = useRacks(locationId)
const racks = racksQuery.data?.data ?? []
```

For RackDetail: use both `useRack(rackId)` and `useRackAssets(rackId)`.
For AddNewRack: the form submission should call the topology API's createRack.

Read each page, identify mock data blocks, replace with hooks.

- [ ] **Step 2: Commit**

```bash
git add src/pages/RackManagement.tsx src/pages/RackDetailUnified.tsx src/pages/DataCenter3D.tsx src/pages/AddNewRack.tsx src/pages/FacilityMap.tsx
git commit -m "feat: migrate 5 rack pages from mock to useRacks/useRack hooks"
```

---

## Task 7: Migrate Maintenance Pages (5 pages)

**Files:**
- Modify: `cmdb-demo/src/pages/MaintenanceHub.tsx`
- Modify: `cmdb-demo/src/pages/MaintenanceTaskView.tsx`
- Modify: `cmdb-demo/src/pages/WorkOrder.tsx`
- Modify: `cmdb-demo/src/pages/AddMaintenanceTask.tsx`
- Modify: `cmdb-demo/src/pages/TaskDispatch.tsx`

- [ ] **Step 1: Migrate all 5 maintenance pages**

```tsx
import { useWorkOrders, useWorkOrder, useCreateWorkOrder, useTransitionWorkOrder } from '../hooks/useMaintenance'

// MaintenanceHub:
const ordersQuery = useWorkOrders()
const orders = ordersQuery.data?.data ?? []
// Delete mockTasks and mockRecords arrays

// WorkOrder:
const transitionMutation = useTransitionWorkOrder()
// Replace mock status toggle with: transitionMutation.mutate({ id, data: { status, comment } })

// AddMaintenanceTask:
const createMutation = useCreateWorkOrder()
// Replace form submission with: createMutation.mutate(formData)
```

- [ ] **Step 2: Commit**

```bash
git add src/pages/MaintenanceHub.tsx src/pages/MaintenanceTaskView.tsx src/pages/WorkOrder.tsx src/pages/AddMaintenanceTask.tsx src/pages/TaskDispatch.tsx
git commit -m "feat: migrate 5 maintenance pages from mock to useWorkOrders hooks"
```

---

## Task 8: Migrate Monitoring + Inventory + Audit + Prediction + System Pages (10 pages)

**Files:**
- Modify: `cmdb-demo/src/pages/MonitoringAlerts.tsx`
- Modify: `cmdb-demo/src/pages/SystemHealth.tsx`
- Modify: `cmdb-demo/src/pages/AlertTopologyAnalysis.tsx`
- Modify: `cmdb-demo/src/pages/HighSpeedInventory.tsx`
- Modify: `cmdb-demo/src/pages/InventoryItemDetail.tsx`
- Modify: `cmdb-demo/src/pages/AuditHistory.tsx`
- Modify: `cmdb-demo/src/pages/AuditEventDetail.tsx`
- Modify: `cmdb-demo/src/pages/PredictiveHub.tsx`
- Modify: `cmdb-demo/src/pages/RolesPermissions.tsx`
- Modify: `cmdb-demo/src/pages/UserProfile.tsx`

- [ ] **Step 1: Migrate monitoring pages (3)**

```tsx
// MonitoringAlerts.tsx:
import { useAlerts, useAcknowledgeAlert, useResolveAlert } from '../hooks/useMonitoring'
const alertsQuery = useAlerts(filterParams)
const alerts = alertsQuery.data?.data ?? []
// Delete ALERTS mock array + TREND_DATA
// Wire ack/resolve buttons to mutations

// SystemHealth.tsx:
import { useAlerts } from '../hooks/useMonitoring'
const criticalQuery = useAlerts({ severity: 'critical' })

// AlertTopologyAnalysis.tsx:
import { useAlerts } from '../hooks/useMonitoring'
const alertsQuery = useAlerts()
// Keep topology visualization logic, just feed it real alert data
```

- [ ] **Step 2: Migrate inventory pages (2)**

```tsx
// HighSpeedInventory.tsx:
import { useInventoryTasks } from '../hooks/useInventory'
const tasksQuery = useInventoryTasks()
// Delete RACKS, DISCREPANCIES, IMPORT_ERRORS mock arrays

// InventoryItemDetail.tsx:
import { useInventoryTask, useInventoryItems } from '../hooks/useInventory'
const taskQuery = useInventoryTask(taskId)
const itemsQuery = useInventoryItems(taskId)
```

- [ ] **Step 3: Migrate audit pages (2)**

```tsx
// AuditHistory.tsx:
import { useAuditEvents } from '../hooks/useAudit'
const eventsQuery = useAuditEvents(filterParams)
// Delete AUDIT_ENTRIES mock array

// AuditEventDetail.tsx:
import { useAuditEvents } from '../hooks/useAudit'
const eventsQuery = useAuditEvents({ target_id: targetId })
```

- [ ] **Step 4: Migrate prediction page**

```tsx
// PredictiveHub.tsx:
import { usePredictionModels, usePredictionsByAsset } from '../hooks/usePrediction'
const modelsQuery = usePredictionModels()
// Delete ASSETS, ALERTS_DATA, TIMELINE_ASSETS mock arrays
```

- [ ] **Step 5: Migrate system pages**

```tsx
// RolesPermissions.tsx:
import { useUsers, useRoles } from '../hooks/useIdentity'
const usersQuery = useUsers()
const rolesQuery = useRoles()

// UserProfile.tsx:
// Already uses authStore.user — just ensure it displays real data when logged in
```

- [ ] **Step 6: Commit**

```bash
git add src/pages/MonitoringAlerts.tsx src/pages/SystemHealth.tsx src/pages/AlertTopologyAnalysis.tsx src/pages/HighSpeedInventory.tsx src/pages/InventoryItemDetail.tsx src/pages/AuditHistory.tsx src/pages/AuditEventDetail.tsx src/pages/PredictiveHub.tsx src/pages/RolesPermissions.tsx src/pages/UserProfile.tsx
git commit -m "feat: migrate 10 remaining pages (monitoring, inventory, audit, prediction, system)"
```

---

## Pages NOT Modified (8 — keep as-is)

| Page | Reason |
|------|--------|
| Welcome.tsx | Static content, no API |
| Login.tsx | Already wired to authStore.login() |
| TroubleshootingGuide.tsx | Static content |
| VideoLibrary.tsx | Static content |
| VideoPlayer.tsx | Static content |
| SystemSettings.tsx | Frontend-only settings |
| EnergyMonitor.tsx | Keep mock (needs metrics ingestion) |
| SensorConfiguration.tsx | Keep mock (no backend endpoint) |

---

## Task Summary

| Task | Files | Scope |
|------|-------|-------|
| 1 | 4 | TS type generation + Vite proxy + env |
| 2 | 10 | API module type migration |
| 3 | 6 | 5 new hooks + QueryWrapper |
| 4 | 6 | Dashboard + 4 location pages |
| 5 | 7 | 7 asset pages |
| 6 | 5 | 5 rack pages |
| 7 | 5 | 5 maintenance pages |
| 8 | 10 | 10 remaining pages (monitoring, inventory, audit, prediction, system) |
| **Total** | **53 files** | **32 pages migrated** |

## Verification

After all 8 tasks, verify:

```bash
cd /cmdb-platform/cmdb-demo
npx tsc --noEmit          # TypeScript compiles
npm run build             # Vite builds
npm run dev               # Dev server starts
```

With backend running:
1. Open `http://localhost:5174`
2. Login with admin/admin123
3. Navigate to Dashboard → should show real stats
4. Navigate to Assets → should show seed data assets
5. Navigate to Locations → should show Taiwan hierarchy
6. Navigate to Monitoring → should show seed alerts
