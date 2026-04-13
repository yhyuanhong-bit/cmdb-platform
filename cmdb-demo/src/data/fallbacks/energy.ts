/* ──────────────────────────────────────────────
   Fallback / seed data for EnergyMonitor
   Extracted from src/pages/EnergyMonitor.tsx
   ────────────────────────────────────────────── */

import type { TFunction } from 'i18next'

/* ── Power Load View ────────────────────────── */

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

export const RACK_HEATMAP: [string, number][] = [
  ['RACK-A01', 0.4], ['RACK-A02', 0.6], ['RACK-A03', 0.85], ['RACK-A04', 0.5],
  ['RACK-B01', 0.7], ['RACK-B02', 0.95], ['RACK-B03', 0.3], ['RACK-B04', 0.9],
  ['RACK-C01', 0.55], ['RACK-C02', 0.75], ['RACK-C03', 0.65], ['RACK-C04', 0.8],
  ['RACK-D01', 0.45], ['RACK-D02', 0.6], ['RACK-D03', 0.7], ['RACK-D04', 0.5],
]

export const POWER_EVENTS = [
  { titleKey: 'power_load.event_power_fluctuation_warning', location: 'RACK-B04', descKey: 'power_load.event_pdu_a_suspected_trip', time: '14:22:08', severity: 'error' },
  { titleKey: 'power_load.event_power_overload_alert', location: 'UPS-MAIN-01', descKey: 'power_load.event_load_exceeds_85_threshold', time: '13:15:44', severity: 'warning' },
  { titleKey: 'power_load.event_outage_switchover_test', location: 'ATS-ROOM-A', descKey: 'power_load.event_auto_switchover_completed', time: '11:30:00', severity: 'info' },
  { titleKey: 'power_load.event_breaker_fault', location: 'RACK-A03', descKey: 'power_load.event_pdu_b_output_anomaly', time: '09:45:22', severity: 'error' },
]

/* ── Facility View ──────────────────────────── */

export function FALLBACK_BOTTOM_STATS(t: TFunction): { label: string; value: string; icon: string; pct: number }[] {
  return [
    { label: t('facility_energy.it_equipment'), value: '842.1 kW', icon: 'memory', pct: 67.5 },
    { label: t('facility_energy.cooling'), value: '312.4 kW', icon: 'ac_unit', pct: 25.0 },
    { label: t('facility_energy.ups'), value: '42.8 kW', icon: 'battery_charging_full', pct: 3.4 },
    { label: t('facility_energy.misc'), value: '51.1 kW', icon: 'more_horiz', pct: 4.1 },
  ]
}

export const FALLBACK_CARBON_MT = 2.4
export const FALLBACK_PEAK_MW = '1.52'
export const FALLBACK_LOAD_PCT = 74
