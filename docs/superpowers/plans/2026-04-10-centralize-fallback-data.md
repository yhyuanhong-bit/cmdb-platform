# Centralize Hardcoded Fallback Data — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move all hardcoded/fallback data scattered across page components into a centralized `src/data/fallbacks/` directory, making mock data manageable without changing any runtime behavior.

**Architecture:** Each page's inline `FALLBACK_*` / `SEED_*` / mock constants are extracted into domain-specific files under `src/data/fallbacks/`. Pages switch from inline constants to named imports. Type definitions are co-located with their data. No logic changes — only data relocation.

**Tech Stack:** TypeScript, React (import-only changes)

---

## File Structure

### New files to create:
- `cmdb-demo/src/data/fallbacks/predictive.ts` — PredictiveHub fallback data (alerts, timeline assets, recommendations, heatmap, forecast, rack slots, chart data)
- `cmdb-demo/src/data/fallbacks/energy.ts` — EnergyMonitor fallback data (power trend, rack heatmap, power events, bottom stats defaults)
- `cmdb-demo/src/data/fallbacks/inventory.ts` — InventoryItemDetail fallback asset
- `cmdb-demo/src/data/fallbacks/bia.ts` — BIA seed rules, assessments, stats
- `cmdb-demo/src/data/fallbacks/lifecycle.ts` — AssetLifecycleTimeline stages, financials, compliance
- `cmdb-demo/src/data/fallbacks/alerts.ts` — AlertTopologyAnalysis fallback alerts
- `cmdb-demo/src/data/fallbacks/dispatch.ts` — TaskDispatch fallback zones
- `cmdb-demo/src/data/fallbacks/index.ts` — Barrel export (optional, for discoverability)

### Files to modify:
- `cmdb-demo/src/pages/PredictiveHub.tsx` — Remove ~150 lines of inline data, add imports
- `cmdb-demo/src/pages/EnergyMonitor.tsx` — Remove ~40 lines of inline data, add imports
- `cmdb-demo/src/pages/InventoryItemDetail.tsx` — Remove ~16 lines of inline data, add import
- `cmdb-demo/src/pages/bia/BIAOverview.tsx` — Remove ~25 lines of inline data, add imports
- `cmdb-demo/src/pages/AssetLifecycleTimeline.tsx` — Remove ~80 lines of inline data, add imports
- `cmdb-demo/src/pages/AlertTopologyAnalysis.tsx` — Remove inline fallback array, add import
- `cmdb-demo/src/pages/TaskDispatch.tsx` — Remove inline fallback zones, add import

---

### Task 1: Create predictive fallback data file

**Files:**
- Create: `cmdb-demo/src/data/fallbacks/predictive.ts`

- [ ] **Step 1: Create the fallback data file**

```ts
// cmdb-demo/src/data/fallbacks/predictive.ts
// Centralized fallback data for PredictiveHub page

export interface Alert {
  id: string
  asset: string
  issue: string
  urgency: 'HIGH' | 'MEDIUM' | 'LOW'
  failureWindow: string
}

export const FALLBACK_ALERTS_DATA: (Omit<Alert, 'issue'> & { issueKey: string })[] = [
  { id: 'SRV-PROD-01', asset: 'SRV-PROD-01', issueKey: 'predictive_hub.issue_fan_bearing_wear', urgency: 'HIGH', failureWindow: '2026-05-15' },
  { id: 'DB-CLUSTER-04', asset: 'DB-CLUSTER-04', issueKey: 'predictive_hub.issue_ssd_write_endurance', urgency: 'MEDIUM', failureWindow: '2026-05-28' },
  { id: 'NET-CORE-SWITCH-B', asset: 'NET-CORE-SWITCH-B', issueKey: 'predictive_hub.issue_capacitor_thermal', urgency: 'HIGH', failureWindow: '2026-05-18' },
  { id: 'UPS-ZONE-04', asset: 'UPS-ZONE-04', issueKey: 'predictive_hub.issue_battery_voltage_drift', urgency: 'LOW', failureWindow: '2026-06-12' },
  { id: 'HYPER-V-NODE-12', asset: 'HYPER-V-NODE-12', issueKey: 'predictive_hub.issue_redundant_psu', urgency: 'HIGH', failureWindow: '2026-05-16' },
]

export interface TimelineAsset {
  id: string
  name: string
  subtitle: string
  bars: { start: number; end: number; type: 'critical' | 'major' | 'minor' }[]
}

export const TIMELINE_ASSETS: (Omit<TimelineAsset, 'subtitle'> & { subtitleKey: string })[] = [
  { id: 'SRV-PROD-001', name: 'SRV-PROD-001', subtitleKey: 'predictive_hub.subtitle_core_production', bars: [{ start: 2, end: 8, type: 'critical' }, { start: 14, end: 18, type: 'minor' }] },
  { id: 'NET-BORD-RT-01', name: 'NET-BORD-RT-01', subtitleKey: 'predictive_hub.subtitle_border_router', bars: [{ start: 5, end: 12, type: 'major' }, { start: 20, end: 25, type: 'critical' }] },
  { id: 'UPS-BAT-04', name: 'UPS-BAT-04', subtitleKey: 'predictive_hub.subtitle_battery_backup', bars: [{ start: 1, end: 4, type: 'minor' }, { start: 10, end: 16, type: 'major' }, { start: 22, end: 26, type: 'critical' }] },
]

export const INSIGHT_RECOMMENDATIONS = [
  { titleKey: 'predictive_hub.rec_title_replace_fan', asset: 'SRV-PROD-001', priority: 'HIGH' as const, descriptionKey: 'predictive_hub.rec_desc_replace_fan' },
  { titleKey: 'predictive_hub.rec_title_recalibrate_power', asset: 'UPS-BAT-04', priority: 'MEDIUM' as const, descriptionKey: 'predictive_hub.rec_desc_recalibrate_power' },
  { titleKey: 'predictive_hub.rec_title_optimise_disk', asset: 'DB-NODE-A', priority: 'CRITICAL' as const, descriptionKey: 'predictive_hub.rec_desc_optimise_disk' },
  { titleKey: 'predictive_hub.rec_title_firmware_patch', asset: 'NET-BORD-RT-01', priority: 'CRITICAL' as const, descriptionKey: 'predictive_hub.rec_desc_firmware_patch' },
]

export interface RecRow {
  id: string
  asset: string
  failureMode: string
  urgency: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW'
  confidence: number
  action: string
}

export const REC_ROWS: (Omit<RecRow, 'failureMode' | 'action'> & { failureModeKey: string; actionKey: string })[] = [
  { id: 'SRV-PROD-001', asset: 'SRV-PROD-001', failureModeKey: 'predictive_hub.failure_fan_bearing', urgency: 'CRITICAL', confidence: 94, actionKey: 'predictive_hub.action_replace_fan_48h' },
  { id: 'UPS-BAT-04', asset: 'UPS-BAT-04', failureModeKey: 'predictive_hub.failure_battery_voltage', urgency: 'HIGH', confidence: 87, actionKey: 'predictive_hub.action_schedule_cell' },
  { id: 'DB-NODE-A', asset: 'DB-NODE-A', failureModeKey: 'predictive_hub.failure_ssd_endurance', urgency: 'CRITICAL', confidence: 91, actionKey: 'predictive_hub.action_rebalance_raid' },
  { id: 'NET-BORD-RT-01', asset: 'NET-BORD-RT-01', failureModeKey: 'predictive_hub.failure_optical_sfp', urgency: 'MEDIUM', confidence: 72, actionKey: 'predictive_hub.action_clean_optical' },
]

export const HEATMAP_REGIONS = [
  { name: 'DC-A1', risk: 'high' }, { name: 'DC-A2', risk: 'medium' }, { name: 'DC-A3', risk: 'low' },
  { name: 'DC-B1', risk: 'critical' }, { name: 'DC-B2', risk: 'low' }, { name: 'DC-B3', risk: 'medium' },
  { name: 'EDGE-01', risk: 'high' }, { name: 'EDGE-02', risk: 'low' }, { name: 'EDGE-03', risk: 'low' },
  { name: 'COLO-1', risk: 'medium' }, { name: 'COLO-2', risk: 'high' }, { name: 'COLO-3', risk: 'low' },
]

export interface TimelineEvent {
  time: string
  severity: 'CRITICAL' | 'POTENTIAL ISSUE' | 'SCHEDULED'
  asset: string
  description: string
  impact?: string
  recoveryCost?: string
  moduleCost?: string
  estCost?: string
  button: { label: string; variant: 'danger' | 'warning' | 'default' }
}

export const TIMELINE_EVENTS: (Omit<TimelineEvent, 'description' | 'impact' | 'button'> & { descriptionKey: string; impactKey?: string; button: { labelKey: string; variant: 'danger' | 'warning' | 'default' } })[] = [
  { time: '08:00 AM', severity: 'CRITICAL', asset: 'Core Switch A01', descriptionKey: 'predictive_hub.timeline_desc_fan_speed', impactKey: 'predictive_hub.timeline_impact_downtime', recoveryCost: '$3,260', button: { labelKey: 'predictive_hub.timeline_btn_execute_emergency', variant: 'danger' } },
  { time: '10:00 AM', severity: 'POTENTIAL ISSUE', asset: 'UPS Main Battery Bank', descriptionKey: 'predictive_hub.timeline_desc_ups_battery', moduleCost: '$4,800', button: { labelKey: 'predictive_hub.timeline_btn_dispatch_inspection', variant: 'warning' } },
  { time: '01:00 PM', severity: 'SCHEDULED', asset: 'HVAC Condenser Unit 04', descriptionKey: 'predictive_hub.timeline_desc_hvac', estCost: '$150', button: { labelKey: 'predictive_hub.timeline_btn_confirmed', variant: 'default' } },
]

export const RACK_SLOTS = Array.from({ length: 42 }, (_, i) => {
  const occupied = [1, 2, 3, 4, 8, 9, 10, 14, 15, 16, 17, 22, 23, 24, 28, 29, 30, 31, 35, 36, 37, 38, 39]
  const critical = [8, 9, 10]
  if (critical.includes(i)) return 'critical'
  if (occupied.includes(i)) return 'occupied'
  return 'empty'
})

export interface MaintenanceTask {
  asset: string
  failure: string
  probability: number
  urgency: 'CRITICAL' | 'HIGH' | 'MEDIUM'
}

export const FORECAST_TASKS: (Omit<MaintenanceTask, 'failure'> & { failureKey: string })[] = [
  { asset: 'Compute-Node-Alpha-09', failureKey: 'predictive_hub.forecast_thermal_degradation', probability: 91, urgency: 'CRITICAL' },
  { asset: 'UPS-Central-Bank-4', failureKey: 'predictive_hub.forecast_capacitor_exhaustion', probability: 64, urgency: 'HIGH' },
  { asset: 'Switch-Fabric-9022', failureKey: 'predictive_hub.forecast_optical_sfp', probability: 32, urgency: 'MEDIUM' },
]

export const SERVER_DATA = [12, 18, 22, 28, 35, 42, 50, 58, 68, 74, 80, 84]
export const UPS_DATA = [8, 10, 14, 16, 18, 22, 28, 34, 38, 40, 42, 44]
export const MONTHS = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC']
```

- [ ] **Step 2: Commit**

```bash
git add cmdb-demo/src/data/fallbacks/predictive.ts
git commit -m "refactor: extract PredictiveHub fallback data to centralized file"
```

---

### Task 2: Create energy fallback data file

**Files:**
- Create: `cmdb-demo/src/data/fallbacks/energy.ts`

- [ ] **Step 1: Create the fallback data file**

```ts
// cmdb-demo/src/data/fallbacks/energy.ts
// Centralized fallback data for EnergyMonitor page

export const FALLBACK_POWER_TREND = [
  { time: '00:00', process: 820, lighting: 180 },
  { time: '02:00', process: 780, lighting: 160 },
  { time: '04:00', process: 810, lighting: 150 },
  { time: '06:00', process: 920, lighting: 200 },
  { time: '08:00', process: 1080, lighting: 240 },
  { time: '10:00', process: 1180, lighting: 260 },
  { time: '12:00', process: 1240, lighting: 270 },
  { time: '14:00', process: 1200, lighting: 250 },
  { time: '16:00', process: 1150, lighting: 240 },
  { time: '18:00', process: 1050, lighting: 220 },
  { time: '20:00', process: 920, lighting: 190 },
  { time: '22:00', process: 860, lighting: 170 },
]

export const RACK_HEATMAP = [
  ['RACK-A01', 0.4], ['RACK-A02', 0.6], ['RACK-A03', 0.85], ['RACK-A04', 0.5],
  ['RACK-B01', 0.7], ['RACK-B02', 0.95], ['RACK-B03', 0.3], ['RACK-B04', 0.9],
  ['RACK-C01', 0.55], ['RACK-C02', 0.75], ['RACK-C03', 0.65], ['RACK-C04', 0.8],
  ['RACK-D01', 0.45], ['RACK-D02', 0.6], ['RACK-D03', 0.7], ['RACK-D04', 0.5],
] as [string, number][]

export const POWER_EVENTS = [
  { titleKey: 'power_load.event_power_fluctuation_warning', location: 'RACK-B04', descKey: 'power_load.event_pdu_a_suspected_trip', time: '14:22:08', severity: 'error' },
  { titleKey: 'power_load.event_power_overload_alert', location: 'UPS-MAIN-01', descKey: 'power_load.event_load_exceeds_85_threshold', time: '13:15:44', severity: 'warning' },
  { titleKey: 'power_load.event_outage_switchover_test', location: 'ATS-ROOM-A', descKey: 'power_load.event_auto_switchover_completed', time: '11:30:00', severity: 'info' },
  { titleKey: 'power_load.event_breaker_fault', location: 'RACK-A03', descKey: 'power_load.event_pdu_b_output_anomaly', time: '09:45:22', severity: 'error' },
]

export const FALLBACK_BOTTOM_STATS = (t: (key: string) => string) => [
  { label: t('facility_energy.it_equipment'), value: '842.1 kW', icon: 'memory', pct: 67.5 },
  { label: t('facility_energy.cooling'), value: '312.4 kW', icon: 'ac_unit', pct: 25.0 },
  { label: t('facility_energy.ups'), value: '42.8 kW', icon: 'battery_charging_full', pct: 3.4 },
  { label: t('facility_energy.misc'), value: '51.1 kW', icon: 'more_horiz', pct: 4.1 },
]

export const FALLBACK_CARBON_MT = 2.4
export const FALLBACK_PEAK_MW = '1.52'
export const FALLBACK_LOAD_PCT = 74
```

- [ ] **Step 2: Commit**

```bash
git add cmdb-demo/src/data/fallbacks/energy.ts
git commit -m "refactor: extract EnergyMonitor fallback data to centralized file"
```

---

### Task 3: Create inventory fallback data file

**Files:**
- Create: `cmdb-demo/src/data/fallbacks/inventory.ts`

- [ ] **Step 1: Create the fallback data file**

```ts
// cmdb-demo/src/data/fallbacks/inventory.ts
// Centralized fallback data for InventoryItemDetail page

export const FALLBACK_ASSET = {
  name: "NODE-4483-B3",
  tag: "IDC01-B3-S04-P1",
  serialNumber: "SN-2023-XK7R-4483",
  model: "Dell PowerEdge R750xs",
  manufacturer: "Dell Technologies",
  location: "IDC-01 / Room B / Rack B3 / Slot 04 / Position 1",
  status: "Powered Off",
  expectedStatus: "Active",
  owner: "Infrastructure Team",
  purchaseDate: "2023-06-15",
  warrantyExpiry: "2026-06-15",
  lastMaintenance: "2025-12-10",
  ipAddress: "10.128.3.44",
  macAddress: "A4:BF:01:23:45:67",
}
```

- [ ] **Step 2: Commit**

```bash
git add cmdb-demo/src/data/fallbacks/inventory.ts
git commit -m "refactor: extract InventoryItemDetail fallback data to centralized file"
```

---

### Task 4: Create BIA fallback data file

**Files:**
- Create: `cmdb-demo/src/data/fallbacks/bia.ts`

- [ ] **Step 1: Create the fallback data file**

```ts
// cmdb-demo/src/data/fallbacks/bia.ts
// Centralized fallback data for BIAOverview page

export const SEED_RULES = [
  { id: '1', tier_name: 'critical',  tier_level: 1, display_name: 'Tier 1 - CRITICAL',  description: 'Core payment systems, building monitoring - downtime causes major financial/safety impact', color: '#ff6b6b', icon: 'error', min_score: 85, max_score: 100, rto_threshold: 4, rpo_threshold: 15 },
  { id: '2', tier_name: 'important', tier_level: 2, display_name: 'Tier 2 - IMPORTANT', description: 'Core system groups (CRM, ERP) - downtime impacts business efficiency', color: '#ffa94d', icon: 'warning', min_score: 60, max_score: 84, rto_threshold: 12, rpo_threshold: 60 },
  { id: '3', tier_name: 'normal',    tier_level: 3, display_name: 'Tier 3 - NORMAL',    description: 'General business systems - downtime has workarounds available', color: '#9ecaff', icon: 'info', min_score: 30, max_score: 59, rto_threshold: 24, rpo_threshold: 240 },
  { id: '4', tier_name: 'minor',     tier_level: 4, display_name: 'Tier 4 - MINOR',     description: 'Test/sandbox environments - downtime has no business impact', color: '#8e9196', icon: 'expand_circle_down', min_score: 0, max_score: 29, rto_threshold: 72, rpo_threshold: null },
]

export const SEED_ASSESSMENTS = [
  { id: '1', system_name: 'Core Payment Gateway', system_code: 'SYS-PROD-PAY-001', owner: 'David Yun', bia_score: 98, tier: 'critical',  rto_hours: 4, rpo_minutes: 15,   data_compliance: true,  asset_compliance: true,  audit_compliance: true,  description: 'Processes all online payment transactions' },
  { id: '2', system_name: 'CRM Core',             system_code: 'SYS-PROD-CRM-001', owner: 'Lin Sheng', bia_score: 85, tier: 'important', rto_hours: 12, rpo_minutes: 120, data_compliance: true,  asset_compliance: true,  audit_compliance: false, description: 'Customer data and service history' },
  { id: '3', system_name: 'Admin Panel',           system_code: 'SYS-CORP-ADM-001', owner: 'Wang Zhi', bia_score: 62, tier: 'normal',    rto_hours: 24, rpo_minutes: 240, data_compliance: true,  asset_compliance: false, audit_compliance: false, description: 'Internal process management' },
  { id: '4', system_name: 'QA Sandbox',            system_code: 'SYS-TEST-QA-001',  owner: null,       bia_score: 15, tier: 'minor',     rto_hours: 72, rpo_minutes: null, data_compliance: false, asset_compliance: false, audit_compliance: false, description: 'QA testing environment' },
]

export const SEED_STATS = {
  total: 4,
  by_tier: { critical: 1, important: 1, normal: 1, minor: 1 },
  avg_compliance: 58,
  data_compliant: 3,
  asset_compliant: 2,
  audit_compliant: 1,
}
```

- [ ] **Step 2: Commit**

```bash
git add cmdb-demo/src/data/fallbacks/bia.ts
git commit -m "refactor: extract BIA fallback data to centralized file"
```

---

### Task 5: Create lifecycle fallback data file

**Files:**
- Create: `cmdb-demo/src/data/fallbacks/lifecycle.ts`

- [ ] **Step 1: Create the fallback data file**

```ts
// cmdb-demo/src/data/fallbacks/lifecycle.ts
// Centralized fallback data for AssetLifecycleTimeline page

export const ASSET_FALLBACK = {
  id: '-',
  status: 'UNKNOWN',
  lastSync: '-',
  serial: '-',
  primaryIp: '-',
  avgLatency: '-',
  uptime: '-',
}

export const TIMELINE_STAGES = [
  {
    id: 1,
    phaseKey: 'asset_lifecycle_timeline.phase_procurement',
    statusKey: 'asset_lifecycle_timeline.status_completed',
    statusColor: 'text-[#69db7c]',
    dotColor: 'bg-[#69db7c]',
    lineColor: 'bg-[#69db7c]/40',
    date: '2023.01.12',
    technician: 'M. Sterling',
    description: 'Approved vendor bid accepted. Hardware order placed with Dell Technologies under PO-2023-8842.',
    hasDetail: true,
  },
  {
    id: 2,
    phaseKey: 'asset_lifecycle_timeline.phase_installation_deployment',
    statusKey: 'asset_lifecycle_timeline.status_completed',
    statusColor: 'text-[#69db7c]',
    dotColor: 'bg-[#69db7c]',
    lineColor: 'bg-[#69db7c]/40',
    date: '2023.01.20',
    technician: 'K. Vance',
    description: 'Physical rack installation at IDC-NORTH-01. OS provisioning and network configuration completed.',
    hasDetail: true,
  },
  {
    id: 3,
    phaseKey: 'asset_lifecycle_timeline.phase_maintenance_cycle',
    statusKey: 'asset_lifecycle_timeline.status_ongoing',
    statusColor: 'text-[#ffa94d]',
    dotColor: 'bg-[#ffa94d]',
    lineColor: 'bg-[#ffa94d]/40',
    date: '2023.09.15',
    technician: 'J. Thorne',
    description: 'Scheduled firmware update v4.2.1 applied. Cooling unit inspection flagged for follow-up.',
    hasDetail: false,
    highlighted: true,
  },
  {
    id: 4,
    phaseKey: 'asset_lifecycle_timeline.phase_infrastructure_upgrade',
    statusKey: 'asset_lifecycle_timeline.status_planned',
    statusColor: 'text-on-surface-variant',
    dotColor: 'bg-on-surface-variant/50',
    lineColor: 'bg-on-surface-variant/20',
    date: 'Expected: Q1 2024',
    technician: 'SysOps Alpha',
    description: 'NVMe storage tier migration and memory expansion to 512GB. Budget allocated under CAPEX-2024-Q1.',
    hasDetail: false,
    costEstimate: '$14,200.00',
  },
  {
    id: 5,
    phaseKey: 'asset_lifecycle_timeline.phase_decommission_recycle',
    statusKey: 'asset_lifecycle_timeline.status_done',
    statusColor: 'text-on-surface-variant',
    dotColor: 'bg-on-surface-variant/50',
    lineColor: 'bg-transparent',
    date: '2027.01.12',
    technician: 'Standard 48-Mo Lifecycle',
    description: 'End-of-life scheduled per corporate asset rotation policy. Data wipe and certified recycling.',
    hasDetail: false,
  },
]

export const FINANCIALS = {
  acquisitionCost: '$248,500.00',
  depreciatedValue: '$192,420.00',
  maintenanceRoi: 12.4,
}

export const COMPLIANCE = [
  { label: 'ISO 27001 CERTIFICATION', icon: 'verified', status: 'pass', color: 'text-[#69db7c]' },
  { label: 'SECURITY PATCHING V4.2', icon: 'security', status: 'pass', color: 'text-[#69db7c]' },
  { label: 'PHYSICAL ACCESS AUDIT', icon: 'warning', status: 'warn', color: 'text-error' },
]
```

- [ ] **Step 2: Commit**

```bash
git add cmdb-demo/src/data/fallbacks/lifecycle.ts
git commit -m "refactor: extract AssetLifecycleTimeline fallback data to centralized file"
```

---

### Task 6: Create alerts and dispatch fallback data files

**Files:**
- Create: `cmdb-demo/src/data/fallbacks/alerts.ts`
- Create: `cmdb-demo/src/data/fallbacks/dispatch.ts`
- Create: `cmdb-demo/src/data/fallbacks/index.ts`

- [ ] **Step 1: Create alerts fallback**

```ts
// cmdb-demo/src/data/fallbacks/alerts.ts
// Centralized fallback data for AlertTopologyAnalysis page

export const FALLBACK_ALERTS = [
  { id: 'ALT-001', severity: 'CRITICAL' as const, assetName: 'DB-PROD-SQL-01', description: 'CPU utilization exceeded 85% threshold for over 15 minutes.', timestamp: '2026-03-28 09:14:22', nodeId: 'node-1' },
  { id: 'ALT-002', severity: 'WARNING' as const, assetName: 'APP-PORTAL-WEB-04', description: 'HTTP response time degraded to 4.2s (SLA threshold: 2s).', timestamp: '2026-03-28 09:15:41', nodeId: 'node-2' },
]
```

- [ ] **Step 2: Create dispatch fallback**

```ts
// cmdb-demo/src/data/fallbacks/dispatch.ts
// Centralized fallback data for TaskDispatch page

export const FALLBACK_ZONES = (colors: string[]) => [
  { label: "Zone A", pct: 0, color: colors[0] },
  { label: "Zone B", pct: 0, color: colors[1] },
  { label: "Zone C", pct: 0, color: colors[2] },
  { label: "Zone D", pct: 0, color: colors[3] },
]
```

- [ ] **Step 3: Create barrel export**

```ts
// cmdb-demo/src/data/fallbacks/index.ts
export * from './predictive'
export * from './energy'
export * from './inventory'
export * from './bia'
export * from './lifecycle'
export * from './alerts'
export * from './dispatch'
```

- [ ] **Step 4: Commit**

```bash
git add cmdb-demo/src/data/fallbacks/
git commit -m "refactor: add alerts, dispatch fallback files and barrel index"
```

---

### Task 7: Update PredictiveHub.tsx to use centralized imports

**Files:**
- Modify: `cmdb-demo/src/pages/PredictiveHub.tsx:52-208` — remove inline data, add import

- [ ] **Step 1: Add import and remove inline data**

At top of file, add:
```ts
import { FALLBACK_ALERTS_DATA, TIMELINE_ASSETS, INSIGHT_RECOMMENDATIONS, REC_ROWS, HEATMAP_REGIONS, TIMELINE_EVENTS, RACK_SLOTS, FORECAST_TASKS, SERVER_DATA, UPS_DATA, MONTHS } from '../data/fallbacks/predictive'
import type { Alert, TimelineAsset, RecRow, TimelineEvent, MaintenanceTask } from '../data/fallbacks/predictive'
```

Remove from the file:
- The `Alert` interface (lines ~54-60)
- `FALLBACK_ALERTS_DATA` constant (lines ~62-68)
- The `TimelineAsset` interface (lines ~74-79)
- `TIMELINE_ASSETS` constant (lines ~81-85)
- `INSIGHT_RECOMMENDATIONS` constant (lines ~89-94)
- The `RecRow` interface (lines ~106-113)
- `REC_ROWS` constant (lines ~115-120)
- `HEATMAP_REGIONS` constant (lines ~122-127)
- The `TimelineEvent` interface (lines ~140-150)
- `TIMELINE_EVENTS` constant (lines ~152-156)
- `RACK_SLOTS` constant (lines ~170-176)
- The `MaintenanceTask` interface (lines ~188-193)
- `FORECAST_TASKS` constant (lines ~195-199)
- `SERVER_DATA`, `UPS_DATA`, `MONTHS` constants (lines ~206-208)

Keep in the file (NOT moved — these are UI config, not fallback data):
- `TAB_DEFINITIONS`
- `GANTT_BAR_COLORS`
- `INSIGHT_PRIORITY_COLORS`
- `RISK_COLOR`
- `SEVERITY_CONFIG`
- `BUTTON_STYLES`
- `RACK_COLOR`
- `CHART_WIDTH`, `CHART_HEIGHT`, `CHART_PADDING`, `INNER_W`, `INNER_H`
- `toPath()`, `toAreaPath()` functions

- [ ] **Step 2: Verify build**

```bash
cd cmdb-demo && npx tsc --noEmit 2>&1 | head -20
```

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/src/pages/PredictiveHub.tsx
git commit -m "refactor: PredictiveHub imports fallback data from centralized file"
```

---

### Task 8: Update EnergyMonitor.tsx to use centralized imports

**Files:**
- Modify: `cmdb-demo/src/pages/EnergyMonitor.tsx:60-94` — remove inline data, add import

- [ ] **Step 1: Add import and remove inline data**

At top of file, add:
```ts
import { FALLBACK_POWER_TREND, RACK_HEATMAP, POWER_EVENTS, FALLBACK_BOTTOM_STATS, FALLBACK_CARBON_MT, FALLBACK_PEAK_MW, FALLBACK_LOAD_PCT } from '../data/fallbacks/energy'
```

Remove from the file:
- `FALLBACK_POWER_TREND` constant (lines ~61-74)
- `rackHeatmap` constant (lines ~76-81) — replace usage with `RACK_HEATMAP`
- `powerEvents` constant (lines ~83-88) — replace usage with `POWER_EVENTS`

Update inline fallback values to use imported constants:
- `carbonMT` fallback `2.4` → `FALLBACK_CARBON_MT`
- `peakMW` fallback `'1.52'` → `FALLBACK_PEAK_MW`
- Load percentage fallback `74` → `FALLBACK_LOAD_PCT`
- Bottom stats fallback array → `FALLBACK_BOTTOM_STATS(t)`

- [ ] **Step 2: Verify build**

```bash
cd cmdb-demo && npx tsc --noEmit 2>&1 | head -20
```

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/src/pages/EnergyMonitor.tsx
git commit -m "refactor: EnergyMonitor imports fallback data from centralized file"
```

---

### Task 9: Update InventoryItemDetail.tsx to use centralized imports

**Files:**
- Modify: `cmdb-demo/src/pages/InventoryItemDetail.tsx:6-25` — remove inline data, add import

- [ ] **Step 1: Add import and remove inline data**

At top of file, add:
```ts
import { FALLBACK_ASSET } from '../data/fallbacks/inventory'
```

Remove the inline `FALLBACK_ASSET` constant (lines ~10-25) and the comment block above it.

- [ ] **Step 2: Verify build**

```bash
cd cmdb-demo && npx tsc --noEmit 2>&1 | head -20
```

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/src/pages/InventoryItemDetail.tsx
git commit -m "refactor: InventoryItemDetail imports fallback data from centralized file"
```

---

### Task 10: Update BIAOverview.tsx to use centralized imports

**Files:**
- Modify: `cmdb-demo/src/pages/bia/BIAOverview.tsx:36-61` — remove inline data, add import

- [ ] **Step 1: Add import and remove inline data**

At top of file, add:
```ts
import { SEED_RULES, SEED_ASSESSMENTS, SEED_STATS } from '../../data/fallbacks/bia'
```

Remove the `SEED_RULES`, `SEED_ASSESSMENTS`, `SEED_STATS` constants and the comment block (lines ~36-61).

- [ ] **Step 2: Verify build**

```bash
cd cmdb-demo && npx tsc --noEmit 2>&1 | head -20
```

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/src/pages/bia/BIAOverview.tsx
git commit -m "refactor: BIAOverview imports fallback data from centralized file"
```

---

### Task 11: Update AssetLifecycleTimeline.tsx to use centralized imports

**Files:**
- Modify: `cmdb-demo/src/pages/AssetLifecycleTimeline.tsx:7-117` — remove inline data, add import

- [ ] **Step 1: Add import and remove inline data**

At top of file, add:
```ts
import { ASSET_FALLBACK, TIMELINE_STAGES, FINANCIALS, COMPLIANCE } from '../data/fallbacks/lifecycle'
```

Remove from the file:
- `assetFallback` constant (lines ~12-20) — all usages change to `ASSET_FALLBACK`
- `timelineStages` constant (lines ~22-90) — all usages change to `TIMELINE_STAGES`
- `financials` constant (lines ~92-96) — all usages change to `FINANCIALS`
- `compliance` constant (lines ~98-117) — all usages change to `COMPLIANCE`

Update all references in the component:
- `assetFallback` → `ASSET_FALLBACK`
- `timelineStages` → `TIMELINE_STAGES`
- `financials` → `FINANCIALS`
- `compliance` → `COMPLIANCE`

- [ ] **Step 2: Verify build**

```bash
cd cmdb-demo && npx tsc --noEmit 2>&1 | head -20
```

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/src/pages/AssetLifecycleTimeline.tsx
git commit -m "refactor: AssetLifecycleTimeline imports fallback data from centralized file"
```

---

### Task 12: Update AlertTopologyAnalysis.tsx to use centralized imports

**Files:**
- Modify: `cmdb-demo/src/pages/AlertTopologyAnalysis.tsx:210-214` — remove inline fallback, add import

- [ ] **Step 1: Add import and remove inline data**

At top of file, add:
```ts
import { FALLBACK_ALERTS } from '../data/fallbacks/alerts'
```

Replace the inline fallback array (lines ~210-214):
```ts
// Before:
    : [
        { id: 'ALT-001', severity: 'CRITICAL' as const, ... },
        { id: 'ALT-002', severity: 'WARNING' as const, ... },
      ];

// After:
    : FALLBACK_ALERTS;
```

- [ ] **Step 2: Verify build**

```bash
cd cmdb-demo && npx tsc --noEmit 2>&1 | head -20
```

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/src/pages/AlertTopologyAnalysis.tsx
git commit -m "refactor: AlertTopologyAnalysis imports fallback data from centralized file"
```

---

### Task 13: Update TaskDispatch.tsx to use centralized imports

**Files:**
- Modify: `cmdb-demo/src/pages/TaskDispatch.tsx:113-121` — remove inline fallback, add import

- [ ] **Step 1: Add import and remove inline data**

At top of file, add:
```ts
import { FALLBACK_ZONES } from '../data/fallbacks/dispatch'
```

Replace the inline fallback (lines ~114-121):
```ts
// Before:
    return [
      { label: "Zone A", pct: 0, color: ZONE_COLORS[0] },
      ...
    ];

// After:
    return FALLBACK_ZONES(ZONE_COLORS);
```

- [ ] **Step 2: Verify build**

```bash
cd cmdb-demo && npx tsc --noEmit 2>&1 | head -20
```

- [ ] **Step 3: Commit**

```bash
git add cmdb-demo/src/pages/TaskDispatch.tsx
git commit -m "refactor: TaskDispatch imports fallback zones from centralized file"
```

---

### Task 14: Final verification

- [ ] **Step 1: Full TypeScript check**

```bash
cd cmdb-demo && npx tsc --noEmit
```

- [ ] **Step 2: Run existing tests**

```bash
cd cmdb-demo && npx vitest run 2>&1 | tail -20
```

- [ ] **Step 3: Verify no remaining inline fallback data in pages**

```bash
grep -rn "FALLBACK_\|SEED_RULES\|SEED_ASSESSMENTS\|SEED_STATS\|assetFallback\|timelineStages\|financials\b\|compliance\b.*=.*\[" cmdb-demo/src/pages/ | grep -v "import\|from\|//"
```

Expected: No matches (all data now imported).
