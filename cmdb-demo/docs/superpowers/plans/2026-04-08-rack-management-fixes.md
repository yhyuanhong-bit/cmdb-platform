# Rack Management Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 10 production gaps in the rack management pages — connect hardcoded data to real APIs, add missing UI for asset assignment and network connection management, fix breadcrumb and rack status controls.

**Architecture:** Frontend-only changes (all backend endpoints already exist from Phase 2/3). Replace hardcoded arrays with existing API hooks. Add modals for asset-to-rack slot assignment and network connection CRUD.

**Tech Stack:** React/TypeScript, existing TanStack Query hooks (useRackSlots, useCreateRackSlot, useRackNetworkConnections, useCreateNetworkConnection, useDeleteNetworkConnection, useMetrics, useAlerts, useActivityFeed).

**Analysis:** `docs/rack-management-analysis.md`

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `cmdb-demo/src/components/AssignAssetToRackModal.tsx` | Modal for assigning asset to rack U-slot |
| `cmdb-demo/src/components/AddNetworkConnectionModal.tsx` | Modal for adding rack network connection |

### Modified Files

| File | Changes |
|------|---------|
| `cmdb-demo/src/pages/RackDetailUnified.tsx` | Replace 6 hardcoded data sources, add assign/network modals, fix console tab |
| `cmdb-demo/src/pages/RackManagement.tsx` | Fix breadcrumb to use LocationContext |
| `cmdb-demo/src/pages/DataCenter3D.tsx` | Replace hardcoded alerts + stats with API |
| `cmdb-demo/src/i18n/locales/en.json` | Add rack management i18n keys |
| `cmdb-demo/src/i18n/locales/zh-CN.json` | Same |
| `cmdb-demo/src/i18n/locales/zh-TW.json` | Same |

---

## Phase A: Essential Production Fixes (~8h)

### Task 1: Fix RackManagement Breadcrumb (0.5h)

**Files:**
- Modify: `cmdb-demo/src/pages/RackManagement.tsx`

- [ ] **Step 1: Read the file, find hardcoded breadcrumb**

Find lines ~100-104 where breadcrumb shows hardcoded "IDC Alpha → Module 1".

- [ ] **Step 2: Replace with LocationContext**

The page already imports `useLocationContext`. Use the `path` to build the breadcrumb:
```tsx
const breadcrumbParts = []
if (path.territory) breadcrumbParts.push(path.territory.name)
if (path.region) breadcrumbParts.push(path.region.name)
if (path.city) breadcrumbParts.push(path.city.name)
if (path.campus) breadcrumbParts.push(path.campus.name)
if (path.idc) breadcrumbParts.push(path.idc.name)
const breadcrumbText = breadcrumbParts.length > 0 ? breadcrumbParts.join(' → ') : t('racks.breadcrumb_rack_management')
```

Replace the hardcoded breadcrumb JSX with dynamic text.

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/src/pages/RackManagement.tsx
git commit -m "fix: use LocationContext for RackManagement breadcrumb instead of hardcoded text"
```

---

### Task 2: Replace Maintenance History with API (1h)

**Files:**
- Modify: `cmdb-demo/src/pages/RackDetailUnified.tsx`

- [ ] **Step 1: Read the file, find hardcoded maintenanceHistory**

Find `maintenanceHistory` array (~line 22-27) with 4 hardcoded records.

- [ ] **Step 2: Import rack maintenance hook**

Phase 2 added `GET /racks/{id}/maintenance` via custom endpoint. Check if a hook exists, or use inline `useQuery`:

```tsx
import { apiClient } from '../lib/api/client'

// Inside component:
const { data: maintData } = useQuery({
  queryKey: ['rackMaintenance', rackId],
  queryFn: () => apiClient.get(`/racks/${rackId}/maintenance`),
  enabled: !!rackId,
})
const maintenanceHistory = ((maintData as any)?.maintenance ?? []).map((wo: any) => ({
  date: wo.scheduled_start ? new Date(wo.scheduled_start).toLocaleDateString() : new Date(wo.created_at).toLocaleDateString(),
  type: wo.type ?? 'inspection',
  description: wo.title,
  engineer: wo.assignee_name ?? '-',
  status: wo.status,
}))
```

- [ ] **Step 3: Delete the hardcoded array, use API data**

- [ ] **Step 4: Commit**

```bash
git add cmdb-demo/src/pages/RackDetailUnified.tsx
git commit -m "feat: replace hardcoded maintenance history with API data in RackDetail"
```

---

### Task 3: Asset-to-Rack Assignment Modal (3h)

**Files:**
- Create: `cmdb-demo/src/components/AssignAssetToRackModal.tsx`
- Modify: `cmdb-demo/src/pages/RackDetailUnified.tsx`

- [ ] **Step 1: Create the modal**

Create `cmdb-demo/src/components/AssignAssetToRackModal.tsx`:

Props: `{ open, onClose, rackId, totalU }`

Fields:
- Asset (dropdown — fetch from `useAssets()`, filter to those without rack assignment)
- Start U (number input, 1 to totalU)
- End U (number input, >= Start U)
- Side (dropdown: front / rear)

On submit: call `useCreateRackSlot()` with `{ rack_id: rackId, asset_id, start_u, end_u, side }`

Use existing modal styling pattern (bg-[#1a1f2e], bg-[#0d1117] inputs).

Add i18n keys: `rack_detail.assign_asset_title`, `rack_detail.assign_asset`, `rack_detail.field_asset`, `rack_detail.field_start_u`, `rack_detail.field_end_u`, `rack_detail.field_side`, `rack_detail.side_front`, `rack_detail.side_rear`

- [ ] **Step 2: Add "Assign Asset" button in RackDetailUnified**

In the VisualizationTab, add a button next to the existing equipment list:
```tsx
<button onClick={() => setShowAssignModal(true)}
  className="flex items-center gap-2 px-4 py-2 rounded-lg bg-sky-600 text-white text-sm font-semibold">
  <Icon name="add" className="text-[18px]" /> {t('rack_detail.assign_asset')}
</button>
```

Add state: `const [showAssignModal, setShowAssignModal] = useState(false)`

Render modal: `<AssignAssetToRackModal open={showAssignModal} onClose={() => setShowAssignModal(false)} rackId={rackId} totalU={rack.total_u} />`

- [ ] **Step 3: Add i18n keys**

Add to `rack_detail` namespace in all 3 locale files.

- [ ] **Step 4: Commit**

```bash
git add cmdb-demo/src/components/AssignAssetToRackModal.tsx cmdb-demo/src/pages/RackDetailUnified.tsx cmdb-demo/src/i18n/locales/*.json
git commit -m "feat: add asset-to-rack assignment modal with U-slot selection"
```

---

### Task 4: Network Connection Add/Delete UI (2h)

**Files:**
- Create: `cmdb-demo/src/components/AddNetworkConnectionModal.tsx`
- Modify: `cmdb-demo/src/pages/RackDetailUnified.tsx`

- [ ] **Step 1: Create the modal**

Create `cmdb-demo/src/components/AddNetworkConnectionModal.tsx`:

Props: `{ open, onClose, rackId }`

Fields:
- Port (text input, e.g. "Eth1/1")
- Connection type (dropdown: internal asset / external device)
- If internal: asset dropdown (from `useAssets()`)
- If external: device name (text input)
- Speed (dropdown: 1GbE / 10GbE / 25GbE / 100GbE)
- Status (dropdown: UP / DOWN)
- VLANs (text input, comma-separated numbers)
- Connection Type (dropdown: network / power / management)

On submit: call `useCreateNetworkConnection()` with the data. Convert VLANs string to `number[]`.

- [ ] **Step 2: Add buttons in RackDetailUnified NetworkTab**

Find the NetworkTab component. Add:
- "Add Connection" button at the top
- Delete button (trash icon) per connection row
- Wire delete to `useDeleteNetworkConnection()`

- [ ] **Step 3: Add i18n keys**

Add to `rack_detail` namespace: `add_connection`, `delete_connection`, `confirm_delete_connection`, `field_port`, `field_connection_type`, `field_speed`, `field_vlans`, etc.

- [ ] **Step 4: Commit**

```bash
git add cmdb-demo/src/components/AddNetworkConnectionModal.tsx cmdb-demo/src/pages/RackDetailUnified.tsx cmdb-demo/src/i18n/locales/*.json
git commit -m "feat: add network connection create/delete UI in RackDetail"
```

---

### Task 5: Console Tab Use Real Slot Data (1h)

**Files:**
- Modify: `cmdb-demo/src/pages/RackDetailUnified.tsx`

- [ ] **Step 1: Find ConsoleTab and hardcoded uSlots**

Find `uSlots` array (~lines 48-60) and the ConsoleTab component that uses it.

- [ ] **Step 2: Replace with useRackSlots data**

The main component already calls `useRackSlots(rackId)`. Pass this data as a prop to ConsoleTab:

```tsx
// Map rackSlots to the uSlots format
const consoleSlots = (rackSlots ?? []).map((slot: any) => ({
  startU: slot.start_u,
  endU: slot.end_u,
  label: slot.asset_name || slot.asset_tag || `U${slot.start_u}-${slot.end_u}`,
  type: slot.asset_type || 'compute',
}))
```

Pass `consoleSlots` to ConsoleTab instead of hardcoded `uSlots`.

- [ ] **Step 3: Delete hardcoded uSlots array**

- [ ] **Step 4: Commit**

```bash
git add cmdb-demo/src/pages/RackDetailUnified.tsx
git commit -m "fix: ConsoleTab uses real slot data instead of hardcoded uSlots"
```

---

### Task 6: Rack Status Dropdown (0.5h)

**Files:**
- Modify: `cmdb-demo/src/pages/RackDetailUnified.tsx`

- [ ] **Step 1: Find the rack edit inline panel**

Find where rack name/status/total_u are edited (inline edit or button).

- [ ] **Step 2: Change status from text input to dropdown**

```tsx
<select value={editData.status} onChange={e => setEditData(p => ({ ...p, status: e.target.value }))}
  className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm">
  <option value="active">{t('rack_detail.status_active')}</option>
  <option value="maintenance">{t('rack_detail.status_maintenance')}</option>
  <option value="decommissioned">{t('rack_detail.status_decommissioned')}</option>
  <option value="planned">{t('rack_detail.status_planned')}</option>
</select>
```

- [ ] **Step 3: Add i18n keys for status values**

- [ ] **Step 4: Commit**

```bash
git add cmdb-demo/src/pages/RackDetailUnified.tsx cmdb-demo/src/i18n/locales/*.json
git commit -m "fix: rack status edit uses dropdown instead of free-text input"
```

---

## Phase B: Quality Improvements (~6h)

### Task 7: Environmental Metrics from API (2h)

**Files:**
- Modify: `cmdb-demo/src/pages/RackDetailUnified.tsx`

- [ ] **Step 1: Find hardcoded environmentMetrics**

Find the object (~lines 29-34) with temperature, humidity, powerDraw, airflow.

- [ ] **Step 2: Query metrics API for rack's assets**

```tsx
// Get asset IDs from rack
const assetIds = (rackAssets ?? []).map((a: any) => a.id)

// Query latest metrics (temperature, power)
const { data: tempData } = useMetrics({
  asset_id: assetIds[0], // aggregate from first asset as proxy
  name: 'temperature',
  time_range: '1h'
})
const latestTemp = (tempData as any)?.data?.[0]?.value ?? 23.0

// Build environmentMetrics from API data
const environmentMetrics = {
  temperature: { current: latestTemp, min: latestTemp - 3, max: latestTemp + 3, threshold: 30, unit: '°C' },
  humidity: { current: 45, min: 38, max: 52, threshold: 60, unit: '%' }, // still placeholder until sensor API
  powerDraw: { current: rack?.power_current_kw ?? 0, min: 0, max: rack?.power_capacity_kw ?? 40, threshold: rack?.power_capacity_kw ?? 40, unit: 'kW' },
  airflow: { current: 1250, min: 1100, max: 1400, threshold: 1500, unit: 'CFM' }, // placeholder
}
```

- [ ] **Step 3: Delete hardcoded environmentMetrics, use computed values**

- [ ] **Step 4: Commit**

```bash
git add cmdb-demo/src/pages/RackDetailUnified.tsx
git commit -m "feat: derive rack environmental metrics from API (temperature + power)"
```

---

### Task 8: DataCenter3D Alerts from API (1h)

**Files:**
- Modify: `cmdb-demo/src/pages/DataCenter3D.tsx`

- [ ] **Step 1: Find hardcoded alerts array**

Find 3 hardcoded Chinese alert entries (~lines 59-63).

- [ ] **Step 2: Replace with useAlerts hook**

```tsx
import { useAlerts } from '../hooks/useMonitoring'

const { data: alertsData } = useAlerts({ severity: 'critical' })
const alerts = ((alertsData as any)?.data ?? []).slice(0, 5).map((a: any) => ({
  level: a.severity === 'critical' ? 'CRITICAL' : 'WARNING',
  text: a.message || `Alert on ${a.ci_id?.slice(0,8)}`,
  color: a.severity === 'critical' ? 'text-error' : 'text-tertiary',
}))
```

- [ ] **Step 3: Delete hardcoded array**

- [ ] **Step 4: Commit**

```bash
git add cmdb-demo/src/pages/DataCenter3D.tsx
git commit -m "feat: replace hardcoded 3D alerts with real alert API data"
```

---

### Task 9: DataCenter3D Stats from Energy API (1.5h)

**Files:**
- Modify: `cmdb-demo/src/pages/DataCenter3D.tsx`

- [ ] **Step 1: Find hardcoded stats**

Find the stats grid (~lines 472-490) with power, PUE, temperature, cooling.

- [ ] **Step 2: Query energy endpoints**

```tsx
const { data: energySummary } = useQuery({
  queryKey: ['energySummary'],
  queryFn: () => apiClient.get('/energy/summary'),
})
const summary = energySummary as any

const stats = [
  { label: t('datacenter_3d.stat_power_consumption'), value: `${summary?.total_kw?.toFixed(1) ?? '-'} kW`, icon: 'bolt' },
  { label: t('datacenter_3d.stat_pue'), value: summary?.pue?.toFixed(2) ?? '-', icon: 'speed' },
  { label: t('datacenter_3d.stat_avg_temperature'), value: `${latestTemp ?? '-'}°C`, icon: 'thermostat' },
  { label: t('datacenter_3d.stat_cooling_efficiency'), value: '94.2%', icon: 'ac_unit' }, // still placeholder
]
```

- [ ] **Step 3: Replace hardcoded stats, delete static timestamp**

- [ ] **Step 4: Commit**

```bash
git add cmdb-demo/src/pages/DataCenter3D.tsx
git commit -m "feat: replace hardcoded 3D stats with energy API data"
```

---

### Task 10: Selected Asset Detail from API (1.5h)

**Files:**
- Modify: `cmdb-demo/src/pages/RackDetailUnified.tsx`

- [ ] **Step 1: Find hardcoded selected asset specs**

Find the asset detail panel (~lines 338-343) with hardcoded Intel Xeon, serial, IP.

- [ ] **Step 2: Use real asset data from rackAssets**

When user selects an asset in the visualization, look up the full asset data from `rackAssets`:

```tsx
const selectedAssetData = rackAssets?.find((a: any) =>
  a.id === selectedAssetId || a.asset_tag === selectedAssetTag
)

// Use selectedAssetData.vendor, model, serial_number, ip_address, attributes
```

- [ ] **Step 3: Replace hardcoded specs with dynamic data**

- [ ] **Step 4: Commit**

```bash
git add cmdb-demo/src/pages/RackDetailUnified.tsx
git commit -m "feat: show real asset data in rack detail selection panel"
```

---

## Task 11: Build Verification

- [ ] **Step 1: TypeScript check**

```bash
cd /cmdb-platform/cmdb-demo && npx tsc --noEmit 2>&1 | grep -E "RackDetail|RackManagement|DataCenter3D|AssignAsset|AddNetworkConn" | head -10
```

- [ ] **Step 2: Restart and verify**

```bash
kill $(lsof -t -i:5175) 2>/dev/null; sleep 1
cd /cmdb-platform/cmdb-demo && nohup npx vite > /tmp/cmdb-frontend.log 2>&1 &
```
