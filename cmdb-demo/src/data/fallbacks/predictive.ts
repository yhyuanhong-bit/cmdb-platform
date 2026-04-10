/* ──────────────────────────────────────────────
   Fallback / seed data for PredictiveHub
   Extracted from src/pages/PredictiveHub.tsx
   ────────────────────────────────────────────── */

/* ── Interfaces ─────────────────────────────── */

export interface Alert {
  id: string
  asset: string
  issue: string
  urgency: 'HIGH' | 'MEDIUM' | 'LOW'
  failureWindow: string
}

export interface TimelineAsset {
  id: string
  name: string
  subtitle: string
  bars: { start: number; end: number; type: 'critical' | 'major' | 'minor' }[]
}

export interface RecRow {
  id: string
  asset: string
  failureMode: string
  urgency: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW'
  confidence: number
  action: string
}

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

export interface MaintenanceTask {
  asset: string
  failure: string
  probability: number
  urgency: 'CRITICAL' | 'HIGH' | 'MEDIUM'
}

/* ── Tab 2: Alerts ──────────────────────────── */

export const FALLBACK_ALERTS_DATA: (Omit<Alert, 'issue'> & { issueKey: string })[] = [
  { id: 'SRV-PROD-01', asset: 'SRV-PROD-01', issueKey: 'predictive_hub.issue_fan_bearing_wear', urgency: 'HIGH', failureWindow: '2026-05-15' },
  { id: 'DB-CLUSTER-04', asset: 'DB-CLUSTER-04', issueKey: 'predictive_hub.issue_ssd_write_endurance', urgency: 'MEDIUM', failureWindow: '2026-05-28' },
  { id: 'NET-CORE-SWITCH-B', asset: 'NET-CORE-SWITCH-B', issueKey: 'predictive_hub.issue_capacitor_thermal', urgency: 'HIGH', failureWindow: '2026-05-18' },
  { id: 'UPS-ZONE-04', asset: 'UPS-ZONE-04', issueKey: 'predictive_hub.issue_battery_voltage_drift', urgency: 'LOW', failureWindow: '2026-06-12' },
  { id: 'HYPER-V-NODE-12', asset: 'HYPER-V-NODE-12', issueKey: 'predictive_hub.issue_redundant_psu', urgency: 'HIGH', failureWindow: '2026-05-16' },
]

/* ── Tab 3: Insights ────────────────────────── */

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

/* ── Tab 4: Recommendations ─────────────────── */

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

/* ── Tab 5: Timeline ────────────────────────── */

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

/* ── Tab 6: Forecast ────────────────────────── */

export const FORECAST_TASKS: (Omit<MaintenanceTask, 'failure'> & { failureKey: string })[] = [
  { asset: 'Compute-Node-Alpha-09', failureKey: 'predictive_hub.forecast_thermal_degradation', probability: 91, urgency: 'CRITICAL' },
  { asset: 'UPS-Central-Bank-4', failureKey: 'predictive_hub.forecast_capacitor_exhaustion', probability: 64, urgency: 'HIGH' },
  { asset: 'Switch-Fabric-9022', failureKey: 'predictive_hub.forecast_optical_sfp', probability: 32, urgency: 'MEDIUM' },
]

export const SERVER_DATA = [12, 18, 22, 28, 35, 42, 50, 58, 68, 74, 80, 84]
export const UPS_DATA = [8, 10, 14, 16, 18, 22, 28, 34, 38, 40, 42, 44]
export const MONTHS = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC']
