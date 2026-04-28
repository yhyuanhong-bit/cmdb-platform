# phase-3.10 frontend TODO survey (2026-04-28)

## Summary
- **8 TODOs** found with explicit `phase-3.10` or `TODO: backend` labels
- **19 additional Coming Soon stubs** found across 12 pages (functionally equivalent — unimplemented features waiting on backend or design)
- **1 maps to an existing endpoint that just needs a field added** (`/energy/summary` missing `peak_recorded_date`)
- **6 need new backend endpoints** (do not exist in `generated.go`)
- **1 needs a new dedicated endpoint** (workaround currently uses `PUT /maintenance/orders/{id}` which is state-restricted)
- **19 Coming Soon stubs** are split: some need new backend endpoints, most are pure frontend actions (filters, exports, revoke, AI)

---

## Per-TODO (phase-3.10 tagged)

### `cmdb-demo/src/pages/EnergyMonitor.tsx:350`
**TODO:** `surface the actual peak-recorded date from the /power/summary endpoint once the backend exposes it.`
**Page intent:** Power Load dashboard shows a "Peak Demand Record" stat card with the peak MW value; the card should also display the date when that peak was recorded.
**Backend status:** `GET /energy/summary` exists (impl_energy.go:94) and returns `peak_kw` — but the query only does `MAX(value)` with no `time` column returned. Backend needs to return `peak_recorded_at` alongside `peak_kw` in the same response.
**Priority:** P1 — the card already renders the MW value; the date label is visible but blank, which looks broken.
**Effort:** S — single SQL change (`SELECT value, time … ORDER BY value DESC LIMIT 1`), add field to response JSON, update TS type.

---

### `cmdb-demo/src/pages/EnergyMonitor.tsx:502`
**TODO:** `wire up GET /power/ups-autonomy when the backend endpoint is available. Previously hardcoded "42 min".`
**Page intent:** Power Load dashboard shows a UPS autonomy stat (how many minutes of backup power remain). Currently shows `—` and "Coming Soon".
**Backend status:** No `/power/ups-autonomy` route in `generated.go`. No UPS autonomy metrics in `impl_energy.go`. Would need to aggregate `power_kw` vs UPS capacity from asset metadata.
**Priority:** P1 — the stat card is visible on a primary dashboard page; showing `—` with "Coming Soon" is conspicuous.
**Effort:** M — new route, query UPS assets for battery capacity/current load, compute autonomy estimate.

---

### `cmdb-demo/src/pages/EnergyMonitor.tsx:607`
**TODO:** `wire up GET /power/racks/heatmap when the backend endpoint ships. Previously rendered a 12-cell fabricated grid.`
**Page intent:** Power Load dashboard shows a rack power heatmap (color-coded cells by kW consumption per rack). Currently renders an EmptyState placeholder.
**Backend status:** `GET /dashboard/rack-heatmap` exists (generated.go:10052, impl_dashboard.go:136) but is under `/dashboard/`, not `/power/racks/`. The frontend TODO specifies `/power/racks/heatmap`. The existing dashboard endpoint may return compatible data — needs verification of response shape before deciding whether to reuse or add a new route.
**Priority:** P1 — the heatmap section is a prominent visual block replaced by a grey empty state.
**Effort:** S–M — likely reuse existing `/dashboard/rack-heatmap`; needs frontend hook update + shape verification, possibly an alias route.

---

### `cmdb-demo/src/pages/EnergyMonitor.tsx:621`
**TODO:** `wire up GET /power/events when the backend event stream endpoint ships. Previously rendered a static list of fabricated severity-tagged rows.`
**Page intent:** Power Load dashboard shows a power events table (outages, load spikes, PDU alerts) with severity tags. Currently shows EmptyState.
**Backend status:** No `/power/events` route exists. `GET /audit/events` (generated.go:10031) exists but is general audit log — not power-specific. Would need a filtered view or a dedicated power-event endpoint.
**Priority:** P2 — useful operational visibility but the page is already functional without it; lower urgency than stat cards.
**Effort:** M — new route or query parameter on audit events to filter by power-related event types.

---

### `cmdb-demo/src/pages/AssetLifecycleTimeline.tsx:238`
**TODO:** `wire up GET /compliance/assets/{id} when the compliance scan endpoint ships. Previously rendered a static fabricated list (ISO 27001, Security Patching, Physical Audit).`
**Page intent:** Asset Lifecycle Timeline sidebar shows a "Compliance Summary" section listing which compliance frameworks the asset passes/fails. Currently shows EmptyState + a "Generate Audit Report" Coming Soon button.
**Backend status:** No `/compliance/assets/:id` route. The `assets` table has `asset_compliance`, `audit_compliance`, `data_compliance` boolean fields (generated.go:1520–1524) but no dedicated compliance endpoint.
**Priority:** P1 — the compliance section is a named sidebar panel on a detail page; EmptyState here feels intentionally incomplete to users.
**Effort:** L — needs new handler, data model for scan results per framework, and likely a background scan runner; boolean flags alone are insufficient for the "list with status" UI.

---

### `cmdb-demo/src/pages/predictive/TimelineTab.tsx:140`
**TODO:** `wire up GET /racks/{id}/occupancy once the backend exposes a per-rack U-slot occupancy endpoint. Previously rendered a fabricated 42-slot grid with hardcoded critical indices.`
**Page intent:** Predictive Hub → Timeline tab shows a rack U-slot occupancy visualizer (42-slot grid with filled/empty slots). Currently shows EmptyState.
**Backend status:** `GET /racks/:id/slots` exists (generated.go:10171) and returns per-slot assignments. However, the TODO specifically asks for `/racks/{id}/occupancy` — a summary view (used_u, free_u, per-slot status array). The existing slots endpoint may be sufficient for the UI if the frontend iterates slot data.
**Priority:** P1 — this is on the Predictive Hub, a key feature page; the grid visualizer is its primary visual.
**Effort:** S — frontend can likely consume existing `GET /racks/:id/slots` + `GET /racks/:id` (total_u) to derive occupancy without a new endpoint. Wire-up only.

---

### `cmdb-demo/src/pages/predictive/TimelineTab.tsx:161`
**TODO:** `wire up GET /metrics/environmental (temperature, humidity, grid stability) once the telemetry endpoint ships. Previously rendered hardcoded 23.4 C / 44% / 99.7% values.`
**Page intent:** Predictive Hub → Timeline tab shows an environment context panel with temperature, humidity, and grid stability metrics. Currently shows EmptyState.
**Backend status:** No `/metrics/environmental` route. `GET /monitoring/metrics` exists (generated.go:10135) — a generic metric query endpoint. Could be used with name filters (`temperature`, `humidity`) instead of a dedicated route.
**Priority:** P1 — visible empty state on a key feature page.
**Effort:** M — new dedicated route or frontend hook using existing `/monitoring/metrics?name=temperature,humidity,grid_stability`. If sensor data is already stored in the `metrics` table (likely, given sensor infrastructure), this is mostly frontend wire-up.

---

### `cmdb-demo/src/pages/TaskDispatch.tsx:223`
**TODO:** `Replace with a dedicated POST /maintenance/orders/{id}/assign endpoint that allows assignee changes on a wider state set (approved, in_progress).`
**Page intent:** Task Dispatch page lets dispatchers assign a technician to a work order. The current workaround uses `PUT /maintenance/orders/{id}` (UpdateWorkOrder) but that endpoint rejects assignment on orders in `approved` or `in_progress` state, so only `submitted`/`rejected` orders can be reassigned.
**Backend status:** No `POST /maintenance/orders/:id/assign` route. `PUT /maintenance/orders/:id` exists (generated.go:10108). Builder agent (Task #1) is actively building this endpoint.
**Priority:** P0 — this is a hard functional gap: dispatchers cannot reassign approved/in-progress orders even when operationally necessary.
**Effort:** M — new route, permission check, update `assignee_id` without state-machine restrictions (or with a separate state check appropriate for assignment).

---

## Coming Soon stubs (not tagged, but equivalent unimplemented features)

These are buttons/actions that fire `toast.info('Coming Soon')` instead of performing an action. Listed for completeness; not all require new backend endpoints.

| File | Line | Label | Backend needed? | Priority |
|------|------|-------|-----------------|----------|
| `AssetLifecycle.tsx` | 120 | Filters | No (pure frontend filter UI) | P2 |
| `AssetLifecycle.tsx` | 124 | Export Report | Needs `GET /assets/export` or similar | P2 |
| `AssetLifecycleTimeline.tsx` | 248 | Generate Audit Report | Needs `POST /compliance/assets/{id}/report` | P1 |
| `MonitoringAlerts.tsx` | 205 | Export (alerts) | Needs `GET /monitoring/alerts/export` | P2 |
| `MonitoringAlerts.tsx` | 213,353 | Silence alert | Needs `POST /monitoring/alerts/{id}/silence` | P1 |
| `AuditHistory.tsx` | 294 | Advanced Filters | No (pure frontend filter UI) | P2 |
| `DataCenter3D.tsx` | 416 | Deploy Agent | Needs new agent-deployment endpoint | P2 |
| `DataCenter3D.tsx` | 541,545 | Zoom +/- | No (pure frontend 3D viewer action) | P2 |
| `SystemSettings.tsx` | 480 | Regenerate QR | Needs `POST /system/qr/regenerate` | P2 |
| `EquipmentHealthOverview.tsx` | 689 | Export health report | Needs export endpoint | P2 |
| `TaskDispatch.tsx` | 500 | Auto-assign | Needs `POST /maintenance/orders/auto-assign` | P2 |
| `VideoPlayer.tsx` | 116 | Download SOP PDF | Needs file-storage/SOP endpoint | P2 |
| `UserProfile.tsx` | 166 | Reset 2FA Key | Needs `POST /users/{id}/2fa/reset` | P1 |
| `UserProfile.tsx` | 187 | Revoke Session | `GET /users/:id/sessions` exists; needs `DELETE /users/:id/sessions/:sessionId` | P1 |
| `WorkOrder.tsx` | 221 | History (per order) | `GET /maintenance/orders/:id/logs` already exists — just needs wire-up | P1 |
| `WorkOrder.tsx` | 379 | AI Review | Needs AI/LLM integration or stub endpoint | P2 |
| `predictive/ForecastTab.tsx` | 79 | Isolate Node | Needs `POST /topology/isolate/{id}` or similar | P2 |
| `bia/BIAOverview.tsx` | 251 | Export BIA | Needs `GET /bia/assessments/{id}/export` | P2 |
| `asset-detail/tabs/MaintenanceTab.tsx` | 35 | Filter Logs | No (pure frontend filter) | P2 |

---

## Recommended fix order

1. **`TaskDispatch.tsx:223`** — `POST /maintenance/orders/{id}/assign` (P0, blocks core dispatch workflow; Task #1 already in progress)
2. **`EnergyMonitor.tsx:350`** — Add `peak_recorded_at` to `/energy/summary` response (P1, S-effort, high ROI for small change)
3. **`TimelineTab.tsx:140`** — Wire existing `GET /racks/:id/slots` + `/racks/:id` for occupancy grid (P1, S-effort, no new endpoint needed)
4. **`WorkOrder.tsx:221`** — Wire `GET /maintenance/orders/:id/logs` for history view (P1, no new endpoint needed)
5. **`UserProfile.tsx:187`** — Add `DELETE /users/:id/sessions/:sessionId` for session revoke (P1, S-effort)
6. **`MonitoringAlerts.tsx:213,353`** — Add `POST /monitoring/alerts/{id}/silence` (P1, M-effort)
7. **`TimelineTab.tsx:161`** — Wire `/monitoring/metrics` with env name filters (P1, M-effort)
8. **`EnergyMonitor.tsx:502`** — New `GET /power/ups-autonomy` (P1, M-effort)
9. **`AssetLifecycleTimeline.tsx:238`** — New `GET /compliance/assets/{id}` (P1, L-effort)
10. **`EnergyMonitor.tsx:607`** — Evaluate reuse of `/dashboard/rack-heatmap` vs new route (P1, S-M)
11. **`EnergyMonitor.tsx:621`** — New `GET /power/events` (P2, M-effort)
12. Remaining P2 Coming Soon stubs as bandwidth allows

---

## Backend wishlist (new endpoints)

| Endpoint | Used by | Notes |
|----------|---------|-------|
| `POST /maintenance/orders/{id}/assign` | `TaskDispatch.tsx:223` | Assign on any state; Task #1 in progress |
| `GET /power/ups-autonomy` | `EnergyMonitor.tsx:502` | UPS battery autonomy estimate in minutes |
| `GET /power/racks/heatmap` | `EnergyMonitor.tsx:607` | May reuse `/dashboard/rack-heatmap` |
| `GET /power/events` | `EnergyMonitor.tsx:621` | Power-specific event stream |
| `GET /compliance/assets/{id}` | `AssetLifecycleTimeline.tsx:238`, `AssetLifecycleTimeline.tsx:248` | Per-asset compliance framework scan results |
| `GET /metrics/environmental` | `TimelineTab.tsx:161` | Temp/humidity/grid stability — likely `/monitoring/metrics?names=...` |
| `DELETE /users/{id}/sessions/{sessionId}` | `UserProfile.tsx:187` | Session revoke; list endpoint exists |
| `POST /monitoring/alerts/{id}/silence` | `MonitoringAlerts.tsx:213,353` | Alert silencing with duration |
| `POST /compliance/assets/{id}/report` | `AssetLifecycleTimeline.tsx:248` | Audit report generation |

**Fields to add to existing endpoints:**

| Endpoint | Missing field | Consumer |
|----------|--------------|----------|
| `GET /energy/summary` | `peak_recorded_at` (timestamp of when peak_kw was recorded) | `EnergyMonitor.tsx:350` |

**Endpoints that exist and just need frontend wire-up:**

| Endpoint | Consumer | Notes |
|----------|---------|-------|
| `GET /racks/:id/slots` + `GET /racks/:id` | `TimelineTab.tsx:140` | Derive occupancy from slot list + total_u |
| `GET /maintenance/orders/:id/logs` | `WorkOrder.tsx:221` | Already registered, not wired in UI |
