# Rack Management Business Logic Analysis Report

**Date:** 2026-04-08
**Scope:** 5 pages — RackManagement, RackDetailUnified, AddNewRack, DataCenter3D, FacilityMap
**Overall Completion:** 70%

---

## 1. Functional Status Summary

| Page | API Integration | Hardcoded Data | Completion |
|------|----------------|---------------|-----------|
| RackManagement | 95% (4 hooks) | 2 items (breadcrumb, rack layout) | 85% |
| RackDetailUnified | 80% (8 hooks) | 6 items (maintenance, env, uSlots, asset specs) | 65% |
| AddNewRack | 100% (5 hooks) | 1 item (form defaults) | 95% |
| DataCenter3D | 70% (3 hooks) | 5 items (alerts, stats, timestamp) | 60% |
| FacilityMap | 90% (1 hook) | 2 items (facility stats, temp calc) | 80% |

---

## 2. Critical Production Gaps (Must Fix)

### Gap 1: Asset-to-Rack Assignment (UI Missing)

**Problem:** No way to assign or move assets to a rack from the UI.

**Current state:**
- Backend API exists: `POST /racks/{id}/slots` (body: `{ asset_id, start_u, end_u, side }`)
- Frontend hook exists: `useCreateRackSlot()` in useTopology.ts
- **No UI calls this hook** — no "Add Asset" button, no slot assignment modal

**Impact:** Users cannot populate rack visualizations with real assets from the frontend. Must use API directly or seed data.

**Fix needed:**
- Add "Assign Asset" button in RackDetailUnified visualization tab
- Modal: select asset (dropdown), start_u, end_u, side (front/rear)
- Call `useCreateRackSlot()` mutation

---

### Gap 2: Environmental Metrics (All Hardcoded)

**Problem:** Temperature, humidity, power draw, airflow are static values.

**Hardcoded in RackDetailUnified (lines 29-34):**
```
Temperature: 23.1°C (min 19.4, max 26.8, threshold 30)
Humidity: 45% (min 38, max 52, threshold 60)
Power Draw: 32.4 kW (min 28.1, max 35.2, threshold 40)
Airflow: 1250 CFM (min 1100, max 1400, threshold 1500)
```

**Fix needed:**
- Use existing `useMetrics` hook to query rack's assets' metrics
- Aggregate: avg temperature, total power draw from metrics table
- Fall back to defaults if no data available

---

### Gap 3: Maintenance History (Disconnected)

**Problem:** 4 hardcoded maintenance records, not from API.

**Current state:**
- Phase 2 added `GET /racks/{id}/maintenance` endpoint
- Returns work orders linked to assets in the rack
- **Frontend does NOT call this endpoint** — uses hardcoded array

**Fix needed:**
- Import rack maintenance data from Phase 2 endpoint
- Replace hardcoded `maintenanceHistory` array with API call

---

### Gap 4: Network Connection Management (Display-Only)

**Problem:** Network connections table is read-only. No add/edit/delete.

**Current state:**
- Phase 3 added: `POST /racks/{id}/network-connections`, `DELETE /racks/{id}/network-connections/{connId}`
- Frontend hooks exist: `useCreateNetworkConnection()`, `useDeleteNetworkConnection()`
- **No UI buttons or forms call these hooks**

**Fix needed:**
- Add "Add Connection" button → modal (port, device/asset, speed, vlans, type)
- Add delete button per connection row
- Wire to existing hooks

---

### Gap 5: Breadcrumb Location (Hardcoded)

**Problem:** RackManagement shows hardcoded "IDC Alpha → Module 1" instead of actual location.

**Lines 100-104:** Static breadcrumb text, not from LocationContext.

**Fix needed:**
- Read `path` from `useLocationContext()`
- Build breadcrumb from `path.territory?.name → path.region?.name → ...`
- Simple fix (~15 min)

---

### Gap 6: Console Tab U-Slots (Hardcoded)

**Problem:** Console tab shows hardcoded equipment layout, not from API.

**`uSlots` (lines 48-60):** 11 hardcoded items (PDU, compute, network, etc.)

**Current state:**
- `useRackSlots()` hook already fetches real slot data
- Visualization tab DOES use API slots (with BIA colors)
- Console tab uses SEPARATE hardcoded `uSlots` array

**Fix needed:**
- Console tab should also use `useRackSlots()` data
- Or merge console and visualization tabs

---

## 3. Medium Priority Gaps

### Gap 7: DataCenter3D Alerts (Hardcoded)

3 hardcoded alert entries in Chinese. Should use `useAlerts()` filtered by location.

### Gap 8: DataCenter3D Stats (Hardcoded)

Power consumption, PUE, temperature, cooling efficiency are static. Should use energy/metrics endpoints.

### Gap 9: Selected Asset Detail (Hardcoded)

RackDetailUnified shows hardcoded specs (Intel Xeon, serial DR-DKY-22619) when an asset is selected. Should fetch from the selected asset's API data.

### Gap 10: Rack Status Validation

Status field is free-text editable. Should be a dropdown with valid values (active/maintenance/decommissioned).

---

## 4. Low Priority / Nice-to-Have

| Item | Status |
|------|--------|
| Bulk rack operations (3-dot menu) | Stub — alert('Coming Soon') |
| DataCenter3D zoom controls | Stub |
| FacilityMap drag-drop rack placement | Not implemented |
| Cross-rack comparison dashboard | Not implemented |
| Rack decommission workflow | Not implemented |

---

## 5. Cross-Page Navigation Status

| Path | Status | Notes |
|------|--------|-------|
| CampusOverview → Dashboard → RackManagement | ✅ | LocationContext flows correctly |
| RackManagement → RackDetail (click row) | ✅ | `navigate(/racks/${rack.id})` |
| RackManagement → AddNewRack (button) | ✅ | `navigate('/racks/add')` |
| RackDetail → AssetDetail (click asset) | ✅ | `navigate(/assets/${assetTag})` |
| FacilityMap → RackDetail (click rack) | ✅ | `navigate(/racks/${rackId})` |
| DataCenter3D → RackDetail (click 3D) | ✅ | `navigate(/racks/${rackId})` |
| RackDetail → RackManagement (back) | ✅ | Breadcrumb navigation |
| AddNewRack → RackManagement (cancel/save) | ✅ | `navigate('/racks')` |

---

## 6. Hardcoded Data Inventory

### Critical (should be API): 11 items

| File | Data | Lines | Should Use |
|------|------|-------|-----------|
| RackDetailUnified | maintenanceHistory (4 records) | 22-27 | GET /racks/{id}/maintenance |
| RackDetailUnified | environmentMetrics (4 metrics) | 29-34 | useMetrics aggregate |
| RackDetailUnified | uSlots console layout (11 items) | 48-60 | useRackSlots() |
| RackDetailUnified | selected asset specs | 338-343 | Selected asset API data |
| DataCenter3D | alerts (3 items) | 59-63 | useAlerts() |
| DataCenter3D | stats (power, PUE, temp, cooling) | 472-490 | Energy endpoints |
| DataCenter3D | last update timestamp | 467 | Dynamic |

### Medium (could improve): 8 items

| File | Data | Lines | Issue |
|------|------|-------|-------|
| RackManagement | breadcrumb "IDC Alpha" | 100-104 | Should use LocationContext |
| RackManagement | rackA01Layout | 10-26 | One-rack-only visualization |
| DataCenter3D | fallback UUID | 181 | Hardcoded Neihu campus |
| DataCenter3D | floor display "Floor 2" | 441 | Should be dynamic |
| FacilityMap | facility stats | 155-158 | Hardcoded area/zones |
| FacilityMap | temperature calculation | 36 | Pseudo-random from ID |
| RackDetailUnified | default selected slot | 455 | Should be null |
| AddNewRack | form defaults | 14-17 | Acceptable |

---

## 7. Recommended Fix Priority

### Phase A (Essential for production — ~8h)

| # | Fix | Effort | Impact |
|---|-----|--------|--------|
| 1 | Asset-to-Rack assignment modal | 3h | Critical — core rack management feature |
| 2 | Replace maintenance history with API | 1h | Phase 2 endpoint already exists |
| 3 | Network connection add/delete buttons | 2h | Phase 3 hooks already exist |
| 4 | Fix breadcrumb to use LocationContext | 0.5h | Bug fix |
| 5 | Console tab use useRackSlots() | 1h | Remove duplicate hardcoded data |
| 6 | Rack status dropdown (not free-text) | 0.5h | Validation fix |

### Phase B (Quality improvement — ~6h)

| # | Fix | Effort |
|---|-----|--------|
| 7 | Environmental metrics from useMetrics | 2h |
| 8 | DataCenter3D alerts from useAlerts | 1h |
| 9 | DataCenter3D stats from energy endpoints | 1.5h |
| 10 | Selected asset detail from API | 1.5h |

### Phase C (Polish — ~4h)

| # | Fix | Effort |
|---|-----|--------|
| 11 | FacilityMap dynamic stats | 1h |
| 12 | Bulk operations menu | 2h |
| 13 | DataCenter3D floor selector | 1h |
