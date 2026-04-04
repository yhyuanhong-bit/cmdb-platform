import { memo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import StatCard from '../components/StatCard'
import StatusBadge from '../components/StatusBadge'
import { usePredictionModels, usePredictionsByAsset } from '../hooks/usePrediction'

/* ──────────────────────────────────────────────
   Shared helpers
   ────────────────────────────────────────────── */

function Icon({ name, className = '' }: { name: string; className?: string }) {
  return (
    <span className={`material-symbols-outlined ${className}`}>{name}</span>
  )
}

type TabKey = 'overview' | 'alerts' | 'insights' | 'recommendations' | 'timeline' | 'forecast'

const TAB_DEFINITIONS: { key: TabKey; labelZh: string; labelEn: string }[] = [
  { key: 'overview', labelZh: '總覽', labelEn: 'Overview' },
  { key: 'alerts', labelZh: '預測告警', labelEn: 'Alerts' },
  { key: 'insights', labelZh: '維護洞察', labelEn: 'Insights' },
  { key: 'recommendations', labelZh: '建議', labelEn: 'Recommendations' },
  { key: 'timeline', labelZh: '時間線', labelEn: 'Timeline' },
  { key: 'forecast', labelZh: '故障預測', labelEn: 'Forecast' },
]

/* ──────────────────────────────────────────────
   Tab 1: Overview data
   ────────────────────────────────────────────── */

const FAILURE_DIST = [
  { label: 'Mechanical', pct: 38, color: 'bg-error' },
  { label: 'Electrical', pct: 28, color: 'bg-tertiary' },
  { label: 'Thermal', pct: 20, color: 'bg-primary' },
  { label: 'Software', pct: 14, color: 'bg-secondary' },
]

// ASSETS fallback (used when API returns empty)
const FALLBACK_ASSETS = [
  { name: 'CORE-SW-A01', type: 'Core Switch', failureDate: '2026-04-15', rulDays: 12, rulMax: 90, severity: 'CRITICAL', severityColor: 'text-error', severityBg: 'bg-error-container' },
  { name: 'SRV-DB-04', type: 'Database Server', failureDate: '2026-05-02', rulDays: 28, rulMax: 90, severity: 'HIGH', severityColor: 'text-tertiary', severityBg: 'bg-tertiary-container' },
  { name: 'PUMP-MAIN-02', type: 'Cooling Pump', failureDate: '2026-06-18', rulDays: 74, rulMax: 90, severity: 'MEDIUM', severityColor: 'text-primary', severityBg: 'bg-primary-container' },
  { name: 'UPS-BATT-03', type: 'UPS Battery Bank', failureDate: '2026-07-01', rulDays: 88, rulMax: 90, severity: 'LOW', severityColor: 'text-secondary', severityBg: 'bg-secondary-container' },
]

const AI_MESSAGES = [
  { role: 'ai' as const, text: '根據最近的遙測數據分析，CORE-SW-A01 的光模塊衰減速率已超出正常閾值 2.3 倍。建議在未來 7 天內安排更換作業。', time: '14:18' },
  { role: 'ai' as const, text: 'SRV-DB-04 的 SMART 健康指標顯示 Reallocated Sector Count 持續上升。預測硬碟將在 28 天內達到臨界故障閾值，建議立即啟動數據遷移計劃。', time: '14:19' },
  { role: 'user' as const, text: 'PUMP-MAIN-02 的振動數據是否已納入模型？', time: '14:19' },
  { role: 'ai' as const, text: '已確認。PUMP-MAIN-02 的振動頻譜已於 03/27 更新至預測模型中。當前振動水平在容許範圍內，但呈緩慢上升趨勢。建議在 30 天內進行預防性軸承潤滑保養。', time: '14:20' },
]

/* ──────────────────────────────────────────────
   Tab 2: Alerts data
   ────────────────────────────────────────────── */

const ALERT_FILTER_TABS = [
  { key: 'ALL ASSETS', label: 'ALL ASSETS' },
  { key: 'DATACENTER-A', label: 'DATACENTER-A' },
  { key: 'DATACENTER-B', label: 'DATACENTER-B' },
  { key: 'EDGE-NODES', label: 'EDGE-NODES' },
] as const

interface Alert {
  id: string
  asset: string
  issue: string
  urgency: 'HIGH' | 'MEDIUM' | 'LOW'
  failureWindow: string
}

const FALLBACK_ALERTS_DATA: Alert[] = [
  { id: 'SRV-PROD-01', asset: 'SRV-PROD-01', issue: 'Fan Bearing Wear', urgency: 'HIGH', failureWindow: '2026-05-15' },
  { id: 'DB-CLUSTER-04', asset: 'DB-CLUSTER-04', issue: 'SSD Write Endurance Exhaustion', urgency: 'MEDIUM', failureWindow: '2026-05-28' },
  { id: 'NET-CORE-SWITCH-B', asset: 'NET-CORE-SWITCH-B', issue: 'Capacitor Thermal Degradation', urgency: 'HIGH', failureWindow: '2026-05-18' },
  { id: 'UPS-ZONE-04', asset: 'UPS-ZONE-04', issue: 'Battery Cell Voltage Drift', urgency: 'LOW', failureWindow: '2026-06-12' },
  { id: 'HYPER-V-NODE-12', asset: 'HYPER-V-NODE-12', issue: 'Redundant PSU Failure Path', urgency: 'HIGH', failureWindow: '2026-05-16' },
]

/* ──────────────────────────────────────────────
   Tab 3: Insights data
   ────────────────────────────────────────────── */

const INSIGHTS_STATS = [
  { label: 'Critical Maintenance', value: 14, status: 'UPCOMING', color: 'text-error', bgColor: 'bg-error-container' },
  { label: 'Major Maintenance', value: 28, status: 'PENDING', color: 'text-[#fbbf24]', bgColor: 'bg-[#92400e]' },
  { label: 'Minor Maintenance', value: 42, status: 'SCHEDULED', color: 'text-primary', bgColor: 'bg-[#1e3a5f]' },
]

interface TimelineAsset {
  id: string
  name: string
  subtitle: string
  bars: { start: number; end: number; type: 'critical' | 'major' | 'minor' }[]
}

const TIMELINE_ASSETS: TimelineAsset[] = [
  { id: 'SRV-PROD-001', name: 'SRV-PROD-001', subtitle: 'CORE PRODUCTION SERVER', bars: [{ start: 2, end: 8, type: 'critical' }, { start: 14, end: 18, type: 'minor' }] },
  { id: 'NET-BORD-RT-01', name: 'NET-BORD-RT-01', subtitle: 'BORDER ROUTER CLUSTER', bars: [{ start: 5, end: 12, type: 'major' }, { start: 20, end: 25, type: 'critical' }] },
  { id: 'UPS-BAT-04', name: 'UPS-BAT-04', subtitle: 'BATTERY BACKUP SYSTEM', bars: [{ start: 1, end: 4, type: 'minor' }, { start: 10, end: 16, type: 'major' }, { start: 22, end: 26, type: 'critical' }] },
]

const GANTT_BAR_COLORS = { critical: 'bg-error', major: 'bg-tertiary', minor: 'bg-primary' }

const INSIGHT_RECOMMENDATIONS = [
  { title: 'Replace Fan Module', asset: 'SRV-PROD-001', priority: 'HIGH' as const, description: 'Fan module RPM readings show 22% degradation over the past 72 hours. Predictive model forecasts complete bearing failure within 5 days under current thermal load.' },
  { title: 'Recalibrate Power Sensor', asset: 'UPS-BAT-04', priority: 'MEDIUM' as const, description: 'Voltage drift detected in battery cells 3 and 7. Sensor recalibration is required to maintain accurate state-of-charge reporting and prevent false alarms.' },
  { title: 'Optimise Disk Array', asset: 'DB-NODE-A', priority: 'CRITICAL' as const, description: 'Write endurance on SSDs in RAID array has reached 91.4%. Immediate rebalancing and proactive drive replacement recommended to avoid data integrity risk.' },
  { title: 'Firmware Patch Required', asset: 'NET-BORD-RT-01', priority: 'CRITICAL' as const, description: 'CVE-2024-3892 affects current firmware version. Patch resolves optical transceiver power management bug that may cause intermittent link failures under high load.' },
]

const INSIGHT_PRIORITY_COLORS: Record<string, string> = {
  CRITICAL: 'bg-error-container text-on-error-container',
  HIGH: 'bg-[#92400e] text-[#fbbf24]',
  MEDIUM: 'bg-[#1e3a5f] text-primary',
}

/* ──────────────────────────────────────────────
   Tab 4: Recommendations data
   ────────────────────────────────────────────── */

interface RecRow {
  id: string
  asset: string
  failureMode: string
  urgency: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW'
  confidence: number
  action: string
}

const REC_ROWS: RecRow[] = [
  { id: 'SRV-PROD-001', asset: 'SRV-PROD-001', failureMode: 'Fan Bearing Degradation', urgency: 'CRITICAL', confidence: 94, action: 'Replace fan module within 48h' },
  { id: 'UPS-BAT-04', asset: 'UPS-BAT-04', failureMode: 'Battery Cell Voltage Drift', urgency: 'HIGH', confidence: 87, action: 'Schedule cell replacement during next window' },
  { id: 'DB-NODE-A', asset: 'DB-NODE-A', failureMode: 'SSD Write Endurance Exhaustion', urgency: 'CRITICAL', confidence: 91, action: 'Rebalance RAID array; pre-stage replacement drives' },
  { id: 'NET-BORD-RT-01', asset: 'NET-BORD-RT-01', failureMode: 'Optical SFP Attenuation', urgency: 'MEDIUM', confidence: 72, action: 'Clean optical path; schedule SFP swap if degradation continues' },
]

const HEATMAP_REGIONS = [
  { name: 'DC-A1', risk: 'high' }, { name: 'DC-A2', risk: 'medium' }, { name: 'DC-A3', risk: 'low' },
  { name: 'DC-B1', risk: 'critical' }, { name: 'DC-B2', risk: 'low' }, { name: 'DC-B3', risk: 'medium' },
  { name: 'EDGE-01', risk: 'high' }, { name: 'EDGE-02', risk: 'low' }, { name: 'EDGE-03', risk: 'low' },
  { name: 'COLO-1', risk: 'medium' }, { name: 'COLO-2', risk: 'high' }, { name: 'COLO-3', risk: 'low' },
]

const RISK_COLOR: Record<string, string> = {
  critical: 'bg-error/80',
  high: 'bg-tertiary/60',
  medium: 'bg-[#92400e]/60',
  low: 'bg-primary/20',
}

/* ──────────────────────────────────────────────
   Tab 5: Timeline data
   ────────────────────────────────────────────── */

interface TimelineEvent {
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

const TIMELINE_EVENTS: TimelineEvent[] = [
  { time: '08:00 AM', severity: 'CRITICAL', asset: 'Core Switch A01', description: 'Fan speed degradation in PSU-2; potential bearing failure within 48h.', impact: 'Est. Downtime 2h 15m', recoveryCost: '$3,260', button: { label: 'Execute Emergency', variant: 'danger' } },
  { time: '10:00 AM', severity: 'POTENTIAL ISSUE', asset: 'UPS Main Battery Bank', description: 'Internal resistance anomaly detected across cells 4-7. Pattern consistent with early-stage electrolyte degradation.', moduleCost: '$4,800', button: { label: 'Dispatch Inspection', variant: 'warning' } },
  { time: '01:00 PM', severity: 'SCHEDULED', asset: 'HVAC Condenser Unit 04', description: 'Routine filter replacement and condenser coil inspection per quarterly maintenance cycle.', estCost: '$150', button: { label: 'Confirmed', variant: 'default' } },
]

const SEVERITY_CONFIG: Record<string, { dot: string; label: string; bg: string }> = {
  CRITICAL: { dot: 'bg-error', label: 'text-error', bg: 'bg-error-container' },
  'POTENTIAL ISSUE': { dot: 'bg-tertiary', label: 'text-tertiary', bg: 'bg-[#92400e]' },
  SCHEDULED: { dot: 'bg-primary', label: 'text-primary', bg: 'bg-[#1e3a5f]' },
}

const BUTTON_STYLES: Record<string, string> = {
  danger: 'bg-error-container text-on-error-container hover:bg-error/30',
  warning: 'bg-[#92400e] text-[#fbbf24] hover:bg-[#92400e]/80',
  default: 'bg-[#064e3b] text-[#34d399] hover:bg-[#064e3b]/80',
}

const RACK_SLOTS = Array.from({ length: 42 }, (_, i) => {
  const occupied = [1, 2, 3, 4, 8, 9, 10, 14, 15, 16, 17, 22, 23, 24, 28, 29, 30, 31, 35, 36, 37, 38, 39]
  const critical = [8, 9, 10]
  if (critical.includes(i)) return 'critical'
  if (occupied.includes(i)) return 'occupied'
  return 'empty'
})

const RACK_COLOR: Record<string, string> = {
  critical: 'bg-error/60',
  occupied: 'bg-primary/30',
  empty: 'bg-surface-container-low',
}

/* ──────────────────────────────────────────────
   Tab 6: Forecast data
   ────────────────────────────────────────────── */

interface MaintenanceTask {
  asset: string
  failure: string
  probability: number
  urgency: 'CRITICAL' | 'HIGH' | 'MEDIUM'
}

const FORECAST_TASKS: MaintenanceTask[] = [
  { asset: 'Compute-Node-Alpha-09', failure: 'Thermal Degradation (CPU 4)', probability: 91, urgency: 'CRITICAL' },
  { asset: 'UPS-Central-Bank-4', failure: 'Capacitor Exhaustion', probability: 64, urgency: 'HIGH' },
  { asset: 'Switch-Fabric-9022', failure: 'Optical SFP Attenuation', probability: 32, urgency: 'MEDIUM' },
]

const CHART_WIDTH = 720
const CHART_HEIGHT = 260
const CHART_PADDING = { top: 20, right: 20, bottom: 30, left: 45 }
const INNER_W = CHART_WIDTH - CHART_PADDING.left - CHART_PADDING.right
const INNER_H = CHART_HEIGHT - CHART_PADDING.top - CHART_PADDING.bottom
const SERVER_DATA = [12, 18, 22, 28, 35, 42, 50, 58, 68, 74, 80, 84]
const UPS_DATA = [8, 10, 14, 16, 18, 22, 28, 34, 38, 40, 42, 44]
const MONTHS = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC']

function toPath(data: number[]): string {
  return data
    .map((val, i) => {
      const x = CHART_PADDING.left + (i / (data.length - 1)) * INNER_W
      const y = CHART_PADDING.top + INNER_H - (val / 100) * INNER_H
      return `${i === 0 ? 'M' : 'L'}${x},${y}`
    })
    .join(' ')
}

function toAreaPath(data: number[]): string {
  const linePart = data
    .map((val, i) => {
      const x = CHART_PADDING.left + (i / (data.length - 1)) * INNER_W
      const y = CHART_PADDING.top + INNER_H - (val / 100) * INNER_H
      return `${i === 0 ? 'M' : 'L'}${x},${y}`
    })
    .join(' ')
  const lastX = CHART_PADDING.left + INNER_W
  const firstX = CHART_PADDING.left
  const baseY = CHART_PADDING.top + INNER_H
  return `${linePart} L${lastX},${baseY} L${firstX},${baseY} Z`
}

/* ──────────────────────────────────────────────
   Shared sub-components
   ────────────────────────────────────────────── */

function RulBar({ days, max }: { days: number; max: number }) {
  const pct = Math.round((days / max) * 100)
  let barColor = 'bg-error'
  if (days > 60) barColor = 'bg-primary'
  else if (days > 30) barColor = 'bg-tertiary'

  return (
    <div className="flex items-center gap-3 min-w-[180px]">
      <div className="flex-1 h-2.5 rounded-full bg-surface-container-low">
        <div
          className={`h-2.5 rounded-full ${barColor} transition-all duration-500`}
          style={{ width: `${pct}%` }}
        />
      </div>
      <span className="text-xs font-label text-on-surface-variant whitespace-nowrap w-16 text-right">
        {days} Days
      </span>
    </div>
  )
}

function ConfidenceBar({ value }: { value: number }) {
  const color = value >= 90 ? 'bg-error' : value >= 80 ? 'bg-tertiary' : value >= 60 ? 'bg-[#fbbf24]' : 'bg-primary'
  return (
    <div className="flex items-center gap-2">
      <div className="flex-1 h-2 bg-surface-container-low rounded-full overflow-hidden">
        <div className={`h-full rounded-full ${color}`} style={{ width: `${value}%` }} />
      </div>
      <span className="text-xs font-mono text-on-surface-variant w-10 text-right">{value}%</span>
    </div>
  )
}

/* ══════════════════════════════════════════════
   TAB CONTENT COMPONENTS
   ══════════════════════════════════════════════ */

/* ── Tab 1: Overview ─────────────────────────── */

function OverviewTab() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [currentPage, setCurrentPage] = useState(1)
  const [selectedAssetId] = useState('')

  const { data: modelsResponse, isLoading: modelsLoading } = usePredictionModels()
  const { data: predictionsResponse } = usePredictionsByAsset(selectedAssetId)
  const models = modelsResponse?.data ?? []
  const predictions = predictionsResponse?.data ?? []

  const SEVERITY_MAP: Record<string, { color: string; bg: string }> = {
    critical: { color: 'text-error', bg: 'bg-error-container' },
    high: { color: 'text-tertiary', bg: 'bg-tertiary-container' },
    medium: { color: 'text-primary', bg: 'bg-primary-container' },
    low: { color: 'text-secondary', bg: 'bg-secondary-container' },
  }

  // Map API predictions to ASSETS shape, fall back to static
  const ASSETS = predictions.length > 0
    ? predictions.map((p) => {
        const sevKey = (p.severity ?? 'medium').toLowerCase()
        const sevStyle = SEVERITY_MAP[sevKey] ?? SEVERITY_MAP.medium
        const daysUntilExpiry = p.expires_at ? Math.max(0, Math.round((new Date(p.expires_at).getTime() - Date.now()) / (1000 * 60 * 60 * 24))) : 45
        return {
          name: p.ci_id,
          type: p.prediction_type,
          failureDate: p.expires_at ? new Date(p.expires_at).toISOString().split('T')[0] : '—',
          rulDays: daysUntilExpiry,
          rulMax: 90,
          severity: (p.severity ?? 'MEDIUM').toUpperCase(),
          severityColor: sevStyle.color,
          severityBg: sevStyle.bg,
        }
      })
    : FALLBACK_ASSETS

  return (
    <div className="space-y-6">
      {/* Failure Distribution mini chart */}
      <div className="bg-surface-container rounded-xl p-5">
        <div className="flex items-center gap-2 mb-3">
          <div className="bg-surface-container-high rounded-lg p-2">
            <Icon name="pie_chart" className="text-primary text-xl" />
          </div>
          <span className="text-xs text-on-surface-variant font-label uppercase tracking-widest">
            {t('predictive.failure_distribution')}
          </span>
        </div>
        <div className="flex gap-1 h-3 rounded-full overflow-hidden mt-1">
          {FAILURE_DIST.map((d) => (
            <div key={d.label} className={`${d.color} transition-all`} style={{ width: `${d.pct}%` }} />
          ))}
        </div>
        <div className="flex flex-wrap items-center gap-x-6 gap-y-1 mt-3">
          {FAILURE_DIST.map((d) => (
            <div key={d.label} className="flex items-center gap-1.5">
              <div className={`w-2 h-2 rounded-full ${d.color}`} />
              <span className="text-[10px] text-on-surface-variant font-label">{d.label} {d.pct}%</span>
            </div>
          ))}
          <button
            onClick={() => navigate('/monitoring')}
            className="ml-auto flex items-center gap-1 text-xs text-primary font-label hover:underline"
          >
            查看監控
            <Icon name="arrow_forward" className="text-sm" />
          </button>
        </div>
      </div>

      {/* Assets table */}
      <div className="bg-surface-container rounded-xl">
        <div className="px-6 py-4 flex items-center justify-between">
          <div>
            <h2 className="font-headline text-lg font-bold text-on-surface">
              {t('predictive.assets_requiring_attention_zh')}
            </h2>
            <p className="text-xs text-on-surface-variant font-label tracking-widest uppercase mt-0.5">
              {t('predictive.assets_requiring_attention')}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <button className="bg-surface-container-high hover:bg-surface-container-highest px-3 py-1.5 rounded-lg text-xs font-label text-on-surface-variant flex items-center gap-1.5 transition-colors">
              <Icon name="filter_list" className="text-base" />
              {t('common.filter')}
            </button>
            <button className="bg-surface-container-high hover:bg-surface-container-highest px-3 py-1.5 rounded-lg text-xs font-label text-on-surface-variant flex items-center gap-1.5 transition-colors">
              <Icon name="download" className="text-base" />
              {t('common.export')}
            </button>
          </div>
        </div>

        <div className="grid grid-cols-[1.5fr_1fr_2fr_1fr_1fr] gap-4 px-6 py-3 bg-surface-container-low text-[10px] text-on-surface-variant font-label uppercase tracking-widest">
          <span>{t('predictive.table_asset_name')}</span>
          <span>{t('predictive.table_failure_date')}</span>
          <span>{t('predictive.table_rul_indicator')}</span>
          <span>{t('predictive.table_severity')}</span>
          <span className="text-right">{t('predictive.table_action')}</span>
        </div>

        {ASSETS.map((a) => (
          <div key={a.name} className="grid grid-cols-[1.5fr_1fr_2fr_1fr_1fr] gap-4 px-6 py-4 items-center hover:bg-surface-container-high transition-colors">
            <div>
              <p className="font-headline text-sm font-bold text-on-surface">{a.name}</p>
              <p className="text-[10px] text-on-surface-variant font-label mt-0.5">{a.type}</p>
            </div>
            <span className="text-sm text-on-surface tabular-nums">{a.failureDate}</span>
            <RulBar days={a.rulDays} max={a.rulMax} />
            <span className={`inline-flex items-center justify-center text-[10px] font-label font-bold tracking-widest px-3 py-1 rounded-lg ${a.severityBg} ${a.severityColor} w-fit`}>
              {a.severity}
            </span>
            <div className="text-right">
              <button className="text-xs text-primary font-label hover:underline flex items-center gap-1 ml-auto">
                {t('predictive.view_details_zh')}
                <Icon name="arrow_forward" className="text-sm" />
              </button>
            </div>
          </div>
        ))}

        <div className="px-6 py-4 flex items-center justify-between">
          <span className="text-xs text-on-surface-variant font-label">
            {t('predictive.showing_assets', { shown: 4, total: 42 })}
          </span>
          <div className="flex items-center gap-1">
            <button
              className="bg-surface-container-high hover:bg-surface-container-highest w-8 h-8 rounded-lg flex items-center justify-center transition-colors disabled:opacity-30"
              disabled={currentPage === 1}
              onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
            >
              <Icon name="chevron_left" className="text-base text-on-surface-variant" />
            </button>
            {[1, 2, 3].map((p) => (
              <button
                key={p}
                onClick={() => setCurrentPage(p)}
                className={`w-8 h-8 rounded-lg flex items-center justify-center text-xs font-label transition-colors ${
                  p === currentPage
                    ? 'bg-primary text-on-primary font-bold'
                    : 'bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest'
                }`}
              >
                {p}
              </button>
            ))}
            <span className="text-on-surface-variant text-xs px-1">...</span>
            <button
              className="bg-surface-container-high hover:bg-surface-container-highest w-8 h-8 rounded-lg flex items-center justify-center transition-colors"
              onClick={() => setCurrentPage((p) => p + 1)}
            >
              <Icon name="chevron_right" className="text-base text-on-surface-variant" />
            </button>
          </div>
        </div>
      </div>

      {/* AI Advisor panel */}
      <div className="bg-surface-container rounded-xl p-5">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Icon name="smart_toy" className="text-primary text-xl" />
            <h3 className="font-headline text-base font-bold text-on-surface">
              {t('predictive.ai_maintenance_advisor')}
            </h3>
          </div>
          <span className="text-[10px] text-on-surface-variant font-label tracking-widest uppercase">
            {t('predictive.ai_version')}
          </span>
        </div>
        <div className="bg-surface-container-low rounded-xl p-4 flex flex-col gap-3 max-h-[320px] overflow-y-auto">
          {AI_MESSAGES.map((msg, i) => (
            <div key={i} className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}>
              <div className={`max-w-[85%] rounded-xl px-4 py-3 ${msg.role === 'user' ? 'bg-surface-container-high text-on-surface' : 'bg-surface-container text-on-surface'}`}>
                {msg.role === 'ai' && (
                  <div className="flex items-center gap-1.5 mb-1.5">
                    <Icon name="smart_toy" className="text-primary text-sm" />
                    <span className="text-[10px] text-primary font-label font-bold tracking-widest uppercase">
                      {t('predictive.ai_advisor')}
                    </span>
                  </div>
                )}
                <p className="text-sm leading-relaxed">{msg.text}</p>
                <p className="text-[10px] text-on-surface-variant mt-2 text-right tabular-nums">{msg.time}</p>
              </div>
            </div>
          ))}
        </div>
        <div className="mt-3 flex items-center gap-2">
          <div className="flex-1 bg-surface-container-low rounded-xl px-4 py-2.5 flex items-center gap-2">
            <Icon name="chat" className="text-on-surface-variant text-lg" />
            <span className="text-sm text-on-surface-variant">{t('predictive.ai_input_placeholder')}</span>
          </div>
          <button className="bg-primary rounded-xl p-2.5 hover:opacity-90 transition-opacity">
            <Icon name="send" className="text-on-primary text-lg" />
          </button>
        </div>
      </div>
    </div>
  )
}

/* ── Tab 2: Alerts ───────────────────────────── */

function AlertsTab() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [activeFilter, setActiveFilter] = useState('ALL ASSETS')

  return (
    <div className="space-y-6">
      {/* Filter tabs + sort */}
      <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
        <div className="flex gap-1">
          {ALERT_FILTER_TABS.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setActiveFilter(tab.key)}
              className={`px-4 py-2 rounded-lg text-[0.6875rem] font-semibold tracking-wider uppercase transition-colors ${
                activeFilter === tab.key
                  ? 'bg-surface-container-high text-primary'
                  : 'bg-surface-container text-on-surface-variant hover:bg-surface-container-high'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>
        <div className="flex items-center gap-2 bg-surface-container px-4 py-2 rounded-lg">
          <Icon name="sort" className="text-on-surface-variant text-[18px]" />
          <span className="text-on-surface-variant text-[0.6875rem] font-semibold tracking-wider uppercase">
            {t('predictive_alerts.sort_by_urgency')}
          </span>
        </div>
      </div>

      {/* Alert table */}
      <div className="bg-surface-container rounded-xl overflow-hidden">
        <div className="grid grid-cols-[1fr_1.5fr_0.7fr_0.8fr_1fr] gap-4 px-6 py-3 bg-surface-container-high">
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('predictive_alerts.table_asset_identity')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('predictive_alerts.table_predicted_issue')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('predictive_alerts.table_urgency')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('predictive_alerts.table_failure_window')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase text-right">{t('predictive_alerts.table_actions')}</span>
        </div>

        {FALLBACK_ALERTS_DATA.map((alert, idx) => (
          <div
            key={alert.id}
            className={`grid grid-cols-[1fr_1.5fr_0.7fr_0.8fr_1fr] gap-4 px-6 py-4 items-center ${
              idx % 2 === 0 ? 'bg-surface-container' : 'bg-surface-container-low'
            }`}
          >
            <div className="flex items-center gap-3">
              <Icon name="dns" className="text-primary text-[20px]" />
              <span className="text-sm font-semibold text-on-surface font-headline">{alert.asset}</span>
            </div>
            <span className="text-sm text-on-surface-variant">{alert.issue}</span>
            <div>
              <StatusBadge status={alert.urgency} />
            </div>
            <span className="text-sm text-on-surface-variant font-mono">{alert.failureWindow}</span>
            <div className="flex justify-end">
              <button
                onClick={(e) => { e.stopPropagation(); navigate('/maintenance/add'); }}
                className="bg-surface-container-high hover:bg-surface-container-highest text-primary text-[0.6875rem] font-semibold tracking-wider uppercase px-4 py-2 rounded-lg transition-colors"
              >
                {t('predictive_alerts.btn_schedule_maintenance')}
              </button>
            </div>
          </div>
        ))}
      </div>

      {/* Telemetry stream */}
      <div className="flex justify-end">
        <div className="bg-surface-container rounded-xl p-5 w-full max-w-md">
          <div className="flex items-center gap-2 mb-3">
            <Icon name="stream" className="text-primary text-[18px]" />
            <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">
              {t('predictive_alerts.section_telemetry_stream')}
            </span>
            <span className="ml-auto w-2 h-2 rounded-full bg-[#34d399] animate-pulse" />
          </div>
          <div className="bg-surface-container-low rounded-lg p-3 font-mono text-[0.625rem] text-on-surface-variant space-y-1.5 max-h-32 overflow-y-auto">
            <div className="opacity-80">STREAM_IN &gt; node:SRV-PROD-01 | temp:72.4 C | fan_rpm:1820</div>
            <div className="opacity-80">STREAM_IN &gt; node:DB-CLUSTER-04 | ssd_wear:94.2% | iops:12400</div>
            <div className="opacity-80">STREAM_IN &gt; node:NET-CORE-SWITCH-B | cap_temp:68.1 C | pkt_loss:0.02%</div>
            <div className="opacity-80">STREAM_IN &gt; node:UPS-ZONE-04 | voltage:13.2V | cycles:842</div>
          </div>
        </div>
      </div>
    </div>
  )
}

/* ── Tab 3: Insights ─────────────────────────── */

function InsightsTab() {
  const { t } = useTranslation()
  const days = Array.from({ length: 7 }, (_, i) => `DAY ${String((i * 4) + 1).padStart(2, '0')}`)

  return (
    <div className="space-y-6">
      {/* Stats row */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {INSIGHTS_STATS.map((s) => (
          <div key={s.label} className="bg-surface-container-low rounded-lg p-5">
            <div className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase mb-2">
              {s.label}
            </div>
            <div className="flex items-end gap-3">
              <span className={`font-headline font-bold text-3xl ${s.color}`}>{s.value}</span>
              <span className={`${s.bgColor} ${s.color} text-[0.625rem] font-semibold tracking-wider uppercase px-2 py-0.5 rounded mb-1`}>
                {s.status}
              </span>
            </div>
          </div>
        ))}
      </div>

      {/* 30-Day Gantt Timeline */}
      <div className="bg-surface-container rounded-xl p-6">
        <div className="flex items-center gap-2 mb-1">
          <Icon name="timeline" className="text-primary text-[20px]" />
          <h2 className="font-headline text-sm font-bold tracking-wide text-on-surface uppercase">
            {t('predictive_insights.section_30day_timeline')}
          </h2>
        </div>
        <p className="text-on-surface-variant text-[0.6875rem] tracking-wide mb-6 ml-7">
          {t('predictive_insights.timeline_subtitle')}
        </p>

        <div className="flex gap-5 mb-5 ml-7">
          {[
            { label: 'Critical', color: 'bg-error' },
            { label: 'Major', color: 'bg-tertiary' },
            { label: 'Minor', color: 'bg-primary' },
          ].map((l) => (
            <div key={l.label} className="flex items-center gap-1.5">
              <span className={`w-3 h-3 rounded-sm ${l.color}`} />
              <span className="text-[0.625rem] text-on-surface-variant uppercase tracking-wider">{l.label}</span>
            </div>
          ))}
        </div>

        <div className="space-y-3">
          {TIMELINE_ASSETS.map((asset) => (
            <div key={asset.id} className="flex items-center gap-4">
              <div className="w-44 shrink-0">
                <div className="text-xs font-semibold text-on-surface font-headline">{asset.name}</div>
                <div className="text-[0.5625rem] text-on-surface-variant tracking-wider uppercase">{asset.subtitle}</div>
              </div>
              <div className="flex-1 relative h-8 bg-surface-container-low rounded">
                {asset.bars.map((bar, i) => {
                  const left = (bar.start / 30) * 100
                  const width = ((bar.end - bar.start) / 30) * 100
                  return (
                    <div
                      key={i}
                      className={`absolute top-1 bottom-1 rounded ${GANTT_BAR_COLORS[bar.type]} opacity-80`}
                      style={{ left: `${left}%`, width: `${width}%` }}
                    />
                  )
                })}
              </div>
            </div>
          ))}
        </div>

        <div className="flex ml-48 mt-2">
          {days.map((d) => (
            <span key={d} className="flex-1 text-[0.5625rem] text-on-surface-variant tracking-wider">{d}</span>
          ))}
        </div>
      </div>

      {/* Recommendation cards 2x2 */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Icon name="tips_and_updates" className="text-primary text-[20px]" />
          <h2 className="font-headline text-sm font-bold tracking-wide text-on-surface uppercase">
            {t('predictive_insights.section_proactive_recommendations')}
          </h2>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {INSIGHT_RECOMMENDATIONS.map((rec) => (
            <div key={rec.title} className="bg-surface-container rounded-xl p-5 flex flex-col gap-3">
              <div className="flex items-start justify-between gap-3">
                <h3 className="text-sm font-semibold text-on-surface font-headline">
                  {rec.title} &mdash; {rec.asset}
                </h3>
                <span className={`shrink-0 px-2.5 py-1 rounded text-[0.625rem] font-semibold tracking-wider uppercase ${INSIGHT_PRIORITY_COLORS[rec.priority]}`}>
                  {rec.priority}
                </span>
              </div>
              <p className="text-on-surface-variant text-xs leading-relaxed">{rec.description}</p>
              <div className="flex gap-2 mt-auto pt-1">
                <button className="bg-on-primary-container/20 text-on-primary-container text-[0.6875rem] font-semibold tracking-wider uppercase px-4 py-2 rounded-lg hover:bg-on-primary-container/30 transition-colors">
                  {t('predictive_insights.btn_repair_now')}
                </button>
                <button className="bg-surface-container-high text-on-surface-variant text-[0.6875rem] font-semibold tracking-wider uppercase px-4 py-2 rounded-lg hover:bg-surface-container-highest transition-colors">
                  {t('predictive_insights.btn_detailed_report')}
                </button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

/* ── Tab 4: Recommendations ──────────────────── */

function RecommendationsTab() {
  const { t } = useTranslation()

  return (
    <div className="space-y-6">
      {/* Stats row */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCard icon="warning" label={t('predictive_recommendations.stat_critical_assets_at_risk')} value={14} sub={t('predictive_recommendations.stat_critical_sub')} subColor="text-error" />
        <StatCard icon="schedule" label={t('predictive_recommendations.stat_downtime_saved')} value="128.5h" sub={t('predictive_recommendations.stat_downtime_sub')} subColor="text-[#34d399]" />
        <StatCard icon="verified" label={t('predictive_recommendations.stat_system_reliability')} value="99.98%" sub={t('predictive_recommendations.stat_reliability_sub')} subColor="text-[#34d399]" />
        <StatCard icon="query_stats" label={t('predictive_recommendations.stat_roi_diagnostics')} value="87%" sub={t('predictive_recommendations.stat_roi_sub')} subColor="text-primary" />
      </div>

      {/* Confidence table */}
      <div className="bg-surface-container rounded-xl overflow-hidden">
        <div className="grid grid-cols-[1fr_1.5fr_0.7fr_1fr_1.5fr] gap-4 px-6 py-3 bg-surface-container-high">
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('predictive_recommendations.table_asset_identity')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('predictive_recommendations.table_predicted_failure_mode')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('predictive_recommendations.table_urgency')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('predictive_recommendations.table_confidence')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('predictive_recommendations.table_recommended_action')}</span>
        </div>

        {REC_ROWS.map((row, idx) => (
          <div
            key={row.id}
            className={`grid grid-cols-[1fr_1.5fr_0.7fr_1fr_1.5fr] gap-4 px-6 py-4 items-center ${
              idx % 2 === 0 ? 'bg-surface-container' : 'bg-surface-container-low'
            }`}
          >
            <div className="flex items-center gap-3">
              <Icon name="dns" className="text-primary text-[20px]" />
              <span className="text-sm font-semibold text-on-surface font-headline">{row.asset}</span>
            </div>
            <span className="text-sm text-on-surface-variant">{row.failureMode}</span>
            <div>
              <StatusBadge status={row.urgency} />
            </div>
            <ConfidenceBar value={row.confidence} />
            <span className="text-xs text-on-surface-variant leading-relaxed">{row.action}</span>
          </div>
        ))}
      </div>

      {/* Bottom panels */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Regional Risk Heatmap */}
        <div className="bg-surface-container rounded-xl p-5">
          <div className="flex items-center gap-2 mb-4">
            <Icon name="map" className="text-primary text-[20px]" />
            <h2 className="font-headline text-sm font-bold tracking-wide text-on-surface uppercase">
              {t('predictive_recommendations.section_regional_risk_heatmap')}
            </h2>
          </div>
          <div className="grid grid-cols-3 gap-2">
            {HEATMAP_REGIONS.map((r) => (
              <div
                key={r.name}
                className={`${RISK_COLOR[r.risk]} rounded-lg p-3 flex flex-col items-center justify-center min-h-[60px]`}
              >
                <span className="text-[0.625rem] font-semibold text-on-surface tracking-wider uppercase">{r.name}</span>
                <span className="text-[0.5625rem] text-on-surface-variant uppercase tracking-wider mt-0.5">{r.risk}</span>
              </div>
            ))}
          </div>
          <div className="flex gap-4 mt-4">
            {['critical', 'high', 'medium', 'low'].map((level) => (
              <div key={level} className="flex items-center gap-1.5">
                <span className={`w-2.5 h-2.5 rounded-sm ${RISK_COLOR[level]}`} />
                <span className="text-[0.5625rem] text-on-surface-variant uppercase tracking-wider">{level}</span>
              </div>
            ))}
          </div>
        </div>

        {/* AI Model Health */}
        <div className="bg-surface-container rounded-xl p-5">
          <div className="flex items-center gap-2 mb-4">
            <Icon name="model_training" className="text-primary text-[20px]" />
            <h2 className="font-headline text-sm font-bold tracking-wide text-on-surface uppercase">
              {t('predictive_recommendations.section_ai_model_health')}
            </h2>
          </div>
          <div className="space-y-5">
            <div>
              <div className="flex items-center justify-between mb-2">
                <span className="text-[0.6875rem] text-on-surface-variant tracking-wider uppercase font-semibold">
                  {t('predictive_recommendations.label_prediction_accuracy')}
                </span>
                <span className="text-sm font-bold font-headline text-[#34d399]">94.2%</span>
              </div>
              <div className="h-2.5 bg-surface-container-low rounded-full overflow-hidden">
                <div className="h-full bg-[#34d399] rounded-full" style={{ width: '94.2%' }} />
              </div>
            </div>
            <div>
              <div className="flex items-center justify-between mb-2">
                <span className="text-[0.6875rem] text-on-surface-variant tracking-wider uppercase font-semibold">
                  {t('predictive_recommendations.label_data_ingestion_latency')}
                </span>
                <span className="text-sm font-bold font-headline text-primary">1.1ms</span>
              </div>
              <div className="h-2.5 bg-surface-container-low rounded-full overflow-hidden">
                <div className="h-full bg-primary rounded-full" style={{ width: '5%' }} />
              </div>
            </div>
            <div className="bg-surface-container-low rounded-lg p-4 space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-[0.625rem] text-on-surface-variant tracking-wider uppercase">{t('predictive_recommendations.label_model_version')}</span>
                <span className="text-xs text-on-surface font-mono">v3.8.1-stable</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-[0.625rem] text-on-surface-variant tracking-wider uppercase">{t('predictive_recommendations.label_last_retrained')}</span>
                <span className="text-xs text-on-surface font-mono">2026-03-25 03:00 UTC</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-[0.625rem] text-on-surface-variant tracking-wider uppercase">{t('predictive_recommendations.label_training_samples')}</span>
                <span className="text-xs text-on-surface font-mono">2,847,392</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-[0.625rem] text-on-surface-variant tracking-wider uppercase">{t('predictive_recommendations.label_status')}</span>
                <span className="flex items-center gap-1.5">
                  <span className="w-2 h-2 rounded-full bg-[#34d399]" />
                  <span className="text-xs text-[#34d399] font-semibold">{t('common.operational')}</span>
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

/* ── Tab 5: Timeline ─────────────────────────── */

function TimelineTab() {
  const { t } = useTranslation()
  const [filter, setFilter] = useState<'all' | 'critical' | 'scheduled'>('all')

  const filters = [
    { key: 'all' as const, label: t('predictive_timeline.filter_all_events') },
    { key: 'critical' as const, label: t('predictive_timeline.filter_critical_only') },
    { key: 'scheduled' as const, label: t('predictive_timeline.filter_scheduled') },
  ]

  const filteredEvents = TIMELINE_EVENTS.filter((e) => {
    if (filter === 'critical') return e.severity === 'CRITICAL'
    if (filter === 'scheduled') return e.severity === 'SCHEDULED'
    return true
  })

  return (
    <div className="space-y-6">
      {/* Filters */}
      <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
        <div className="flex gap-1">
          {filters.map((f) => (
            <button
              key={f.key}
              onClick={() => setFilter(f.key)}
              className={`px-4 py-2 rounded-lg text-[0.6875rem] font-semibold tracking-wider uppercase transition-colors ${
                filter === f.key
                  ? 'bg-surface-container-high text-primary'
                  : 'bg-surface-container text-on-surface-variant hover:bg-surface-container-high'
              }`}
            >
              {f.label}
            </button>
          ))}
        </div>
        <div className="flex items-center gap-2 bg-surface-container px-4 py-2 rounded-lg">
          <Icon name="calendar_month" className="text-on-surface-variant text-[18px]" />
          <span className="text-on-surface-variant text-[0.6875rem] font-semibold tracking-wider">
            2026-03-28 &mdash; 2026-03-28
          </span>
        </div>
      </div>

      {/* TODAY marker */}
      <div className="flex items-center gap-3">
        <div className="bg-primary px-3 py-1 rounded">
          <span className="text-[0.6875rem] font-bold tracking-wider text-on-primary uppercase">{t('predictive_timeline.label_today')}</span>
        </div>
        <div className="flex-1 h-px bg-primary/30" />
        <span className="text-[0.625rem] text-on-surface-variant font-mono">2026-03-28 UTC</span>
      </div>

      {/* Vertical timeline */}
      <div className="relative">
        <div className="absolute left-[72px] top-0 bottom-0 w-px bg-surface-container-highest" />
        <div className="space-y-6">
          {filteredEvents.map((event, idx) => {
            const config = SEVERITY_CONFIG[event.severity]
            return (
              <div key={idx} className="flex gap-4">
                <div className="w-16 shrink-0 pt-5">
                  <span className="text-xs font-mono text-on-surface-variant">{event.time}</span>
                </div>
                <div className="relative shrink-0 flex flex-col items-center pt-5">
                  <div className={`w-3.5 h-3.5 rounded-full ${config.dot} z-10`} />
                </div>
                <div className="flex-1 bg-surface-container rounded-xl p-5">
                  <div className="flex items-center gap-2 mb-2">
                    <span className={`${config.bg} ${config.label} text-[0.625rem] font-semibold tracking-wider uppercase px-2 py-0.5 rounded`}>
                      {event.severity}
                    </span>
                    <span className="text-sm font-semibold text-on-surface font-headline">{event.asset}</span>
                  </div>
                  <p className="text-on-surface-variant text-sm leading-relaxed mb-3">{event.description}</p>
                  <div className="flex flex-wrap gap-4 mb-4">
                    {event.impact && (
                      <div className="flex items-center gap-1.5">
                        <Icon name="timer" className="text-error text-[16px]" />
                        <span className="text-xs text-on-surface-variant">{event.impact}</span>
                      </div>
                    )}
                    {event.recoveryCost && (
                      <div className="flex items-center gap-1.5">
                        <Icon name="payments" className="text-tertiary text-[16px]" />
                        <span className="text-xs text-on-surface-variant">{t('predictive_timeline.label_recovery_cost')}: {event.recoveryCost}</span>
                      </div>
                    )}
                    {event.moduleCost && (
                      <div className="flex items-center gap-1.5">
                        <Icon name="payments" className="text-tertiary text-[16px]" />
                        <span className="text-xs text-on-surface-variant">{t('predictive_timeline.label_module_cost')}: {event.moduleCost}</span>
                      </div>
                    )}
                    {event.estCost && (
                      <div className="flex items-center gap-1.5">
                        <Icon name="payments" className="text-primary text-[16px]" />
                        <span className="text-xs text-on-surface-variant">{t('predictive_timeline.label_est_cost')}: {event.estCost}</span>
                      </div>
                    )}
                  </div>
                  <button className={`${BUTTON_STYLES[event.button.variant]} text-[0.6875rem] font-semibold tracking-wider uppercase px-4 py-2 rounded-lg transition-colors`}>
                    {event.button.label}
                  </button>
                </div>
              </div>
            )
          })}
        </div>
      </div>

      {/* Bottom panels */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Rack Occupancy Visualizer */}
        <div className="bg-surface-container rounded-xl p-5">
          <div className="flex items-center gap-2 mb-4">
            <Icon name="grid_view" className="text-primary text-[20px]" />
            <h2 className="font-headline text-sm font-bold tracking-wide text-on-surface uppercase">
              {t('predictive_timeline.section_rack_occupancy')}
            </h2>
          </div>
          <div className="grid grid-cols-6 gap-1.5 mb-4">
            {RACK_SLOTS.map((slot, i) => (
              <div
                key={i}
                className={`${RACK_COLOR[slot]} rounded h-5 flex items-center justify-center`}
                title={`U${i + 1}`}
              >
                <span className="text-[0.5rem] text-on-surface-variant/50 font-mono">{i + 1}</span>
              </div>
            ))}
          </div>
          <div className="flex gap-4">
            {[
              { label: t('predictive_timeline.legend_occupied'), color: 'bg-primary/30' },
              { label: t('predictive_timeline.legend_critical'), color: 'bg-error/60' },
              { label: t('predictive_timeline.legend_empty'), color: 'bg-surface-container-low' },
            ].map((l) => (
              <div key={l.label} className="flex items-center gap-1.5">
                <span className={`w-2.5 h-2.5 rounded-sm ${l.color}`} />
                <span className="text-[0.5625rem] text-on-surface-variant uppercase tracking-wider">{l.label}</span>
              </div>
            ))}
          </div>
        </div>

        {/* Environment Context */}
        <div className="bg-surface-container rounded-xl p-5">
          <div className="flex items-center gap-2 mb-4">
            <Icon name="thermostat" className="text-primary text-[20px]" />
            <h2 className="font-headline text-sm font-bold tracking-wide text-on-surface uppercase">
              {t('predictive_timeline.section_environment_context')}
            </h2>
          </div>
          <div className="space-y-4">
            {[
              { icon: 'device_thermostat', iconColor: 'text-tertiary', label: t('predictive_timeline.env_temperature'), value: '23.4 C', valueColor: 'text-on-surface', barColor: 'bg-tertiary/60', barWidth: '47%', min: '18 C', max: '32 C' },
              { icon: 'humidity_percentage', iconColor: 'text-primary', label: t('predictive_timeline.env_humidity'), value: '44%', valueColor: 'text-on-surface', barColor: 'bg-primary/60', barWidth: '44%', min: '20%', max: '80%' },
              { icon: 'bolt', iconColor: 'text-[#34d399]', label: t('predictive_timeline.env_grid_stability'), value: '99.7%', valueColor: 'text-[#34d399]', barColor: 'bg-[#34d399]/60', barWidth: '99.7%', min: '0%', max: '100%' },
            ].map((env) => (
              <div key={env.label} className="bg-surface-container-low rounded-lg p-4">
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-2">
                    <Icon name={env.icon} className={`${env.iconColor} text-[18px]`} />
                    <span className="text-[0.6875rem] text-on-surface-variant tracking-wider uppercase font-semibold">{env.label}</span>
                  </div>
                  <span className={`text-sm font-bold font-headline ${env.valueColor}`}>{env.value}</span>
                </div>
                <div className="h-2 bg-surface-container rounded-full overflow-hidden">
                  <div className={`h-full ${env.barColor} rounded-full`} style={{ width: env.barWidth }} />
                </div>
                <div className="flex justify-between mt-1">
                  <span className="text-[0.5rem] text-on-surface-variant">{env.min}</span>
                  <span className="text-[0.5rem] text-on-surface-variant">{env.max}</span>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}

/* ── Tab 6: Forecast ─────────────────────────── */

function ForecastTab() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  return (
    <div className="space-y-6">
      {/* Header row: Stats + Immediate Attention */}
      <div className="flex flex-col lg:flex-row gap-6">
        <div className="flex-1 grid grid-cols-2 gap-4">
          <div className="bg-surface-container-low rounded-lg p-5">
            <div className="flex items-center justify-between mb-2">
              <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('failure_forecast.stat_critical_threats')}</span>
              <Icon name="warning" className="text-error text-[18px]" />
            </div>
            <div className="font-headline font-bold text-3xl text-error">03</div>
            <span className="text-xs text-error">{t('failure_forecast.stat_active_threat_vectors')}</span>
          </div>
          <div className="bg-surface-container-low rounded-lg p-5">
            <div className="flex items-center justify-between mb-2">
              <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('failure_forecast.stat_risk_index')}</span>
              <Icon name="speed" className="text-tertiary text-[18px]" />
            </div>
            <div className="font-headline font-bold text-3xl text-tertiary">12.4%</div>
            <span className="text-xs text-tertiary">{t('failure_forecast.stat_composite_fleet_risk')}</span>
          </div>
        </div>

        {/* Immediate Attention warning box */}
        <div className="lg:w-96 bg-error-container rounded-xl p-5">
          <div className="flex items-center gap-2 mb-3">
            <Icon name="crisis_alert" className="text-on-error-container text-[20px]" />
            <span className="text-[0.6875rem] font-bold tracking-wider text-on-error-container uppercase">{t('failure_forecast.immediate_attention')}</span>
          </div>
          <p className="text-on-error-container text-sm leading-relaxed mb-2">
            <span className="font-semibold">Storage Node SN-V82</span> has entered a pre-failure state. SMART diagnostics report multiple reallocated sectors and rising read error rates. Predicted total drive failure within 18 hours.
          </p>
          <div className="flex items-center gap-3 mb-4">
            <div className="flex items-center gap-1.5">
              <Icon name="timer" className="text-on-error-container text-[16px]" />
              <span className="text-xs text-on-error-container">~18h to failure</span>
            </div>
            <div className="flex items-center gap-1.5">
              <Icon name="database" className="text-on-error-container text-[16px]" />
              <span className="text-xs text-on-error-container">4.2 TB at risk</span>
            </div>
          </div>
          <button className="bg-error text-on-error text-[0.6875rem] font-bold tracking-wider uppercase px-5 py-2.5 rounded-lg hover:bg-error/80 transition-colors w-full">
            {t('failure_forecast.btn_isolate_node')}
          </button>
        </div>
      </div>

      {/* SVG line chart */}
      <div className="bg-surface-container rounded-xl p-5">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Icon name="show_chart" className="text-primary text-[20px]" />
            <h2 className="font-headline text-sm font-bold tracking-wide text-on-surface uppercase">
              {t('failure_forecast.section_failure_rate_chart')}
            </h2>
          </div>
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-1.5">
              <span className="w-5 h-0.5 bg-error rounded" />
              <span className="text-[0.625rem] text-on-surface-variant tracking-wider uppercase">{t('failure_forecast.legend_server_nodes')}</span>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="w-5 h-0.5 bg-primary rounded" />
              <span className="text-[0.625rem] text-on-surface-variant tracking-wider uppercase">{t('failure_forecast.legend_ups_units')}</span>
            </div>
          </div>
        </div>

        <div className="w-full overflow-x-auto">
          <svg viewBox={`0 0 ${CHART_WIDTH} ${CHART_HEIGHT}`} className="w-full min-w-[500px]" preserveAspectRatio="xMidYMid meet">
            {[0, 25, 50, 75, 100].map((val) => {
              const y = CHART_PADDING.top + INNER_H - (val / 100) * INNER_H
              return (
                <g key={val}>
                  <line x1={CHART_PADDING.left} y1={y} x2={CHART_PADDING.left + INNER_W} y2={y} stroke="#2b363d" strokeWidth="1" />
                  <text x={CHART_PADDING.left - 8} y={y + 4} fill="#c4c6cc" fontSize="9" textAnchor="end" fontFamily="Inter">{val}%</text>
                </g>
              )
            })}
            {MONTHS.map((m, i) => {
              const x = CHART_PADDING.left + (i / (MONTHS.length - 1)) * INNER_W
              return <text key={m} x={x} y={CHART_HEIGHT - 5} fill="#c4c6cc" fontSize="8" textAnchor="middle" fontFamily="Inter">{m}</text>
            })}
            <path d={toAreaPath(SERVER_DATA)} fill="url(#serverGrad)" opacity="0.15" />
            <path d={toAreaPath(UPS_DATA)} fill="url(#upsGrad)" opacity="0.1" />
            <path d={toPath(SERVER_DATA)} fill="none" stroke="#ffb4ab" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
            <path d={toPath(UPS_DATA)} fill="none" stroke="#9ecaff" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
            {SERVER_DATA.map((val, i) => {
              const x = CHART_PADDING.left + (i / (SERVER_DATA.length - 1)) * INNER_W
              const y = CHART_PADDING.top + INNER_H - (val / 100) * INNER_H
              return <circle key={`s-${i}`} cx={x} cy={y} r="3" fill="#ffb4ab" />
            })}
            {UPS_DATA.map((val, i) => {
              const x = CHART_PADDING.left + (i / (UPS_DATA.length - 1)) * INNER_W
              const y = CHART_PADDING.top + INNER_H - (val / 100) * INNER_H
              return <circle key={`u-${i}`} cx={x} cy={y} r="3" fill="#9ecaff" />
            })}
            {(() => {
              const peakIdx = SERVER_DATA.length - 1
              const x = CHART_PADDING.left + (peakIdx / (SERVER_DATA.length - 1)) * INNER_W
              const y = CHART_PADDING.top + INNER_H - (SERVER_DATA[peakIdx] / 100) * INNER_H
              return (
                <g>
                  <line x1={x} y1={y - 8} x2={x} y2={y - 28} stroke="#ffb4ab" strokeWidth="1" strokeDasharray="3,2" />
                  <rect x={x - 70} y={y - 50} width="140" height="20" rx="4" fill="#93000a" />
                  <text x={x} y={y - 36} fill="#ffb4ab" fontSize="8" textAnchor="middle" fontFamily="Inter" fontWeight="600">
                    Peak-Failure: 84.2% CRITICAL
                  </text>
                </g>
              )
            })()}
            <defs>
              <linearGradient id="serverGrad" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="#ffb4ab" />
                <stop offset="100%" stopColor="#ffb4ab" stopOpacity="0" />
              </linearGradient>
              <linearGradient id="upsGrad" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="#9ecaff" />
                <stop offset="100%" stopColor="#9ecaff" stopOpacity="0" />
              </linearGradient>
            </defs>
          </svg>
        </div>
      </div>

      {/* Proactive Maintenance Tasks table */}
      <div className="bg-surface-container rounded-xl overflow-hidden">
        <div className="px-6 py-4 bg-surface-container-high">
          <div className="flex items-center gap-2">
            <Icon name="build" className="text-primary text-[20px]" />
            <h2 className="font-headline text-sm font-bold tracking-wide text-on-surface uppercase">
              {t('failure_forecast.section_proactive_tasks')}
            </h2>
          </div>
        </div>

        <div className="grid grid-cols-[1.2fr_1.5fr_0.8fr_0.7fr_0.7fr] gap-4 px-6 py-3 bg-surface-container-high">
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('failure_forecast.table_asset')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('failure_forecast.table_failure_mode')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('failure_forecast.table_probability')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('failure_forecast.table_urgency')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase text-right">{t('failure_forecast.table_action')}</span>
        </div>

        {FORECAST_TASKS.map((task, idx) => {
          const probColor = task.probability >= 80 ? 'bg-error' : task.probability >= 50 ? 'bg-tertiary' : 'bg-[#fbbf24]'
          return (
            <div
              key={task.asset}
              className={`grid grid-cols-[1.2fr_1.5fr_0.8fr_0.7fr_0.7fr] gap-4 px-6 py-4 items-center ${
                idx % 2 === 0 ? 'bg-surface-container' : 'bg-surface-container-low'
              }`}
            >
              <div className="flex items-center gap-3">
                <Icon name="dns" className="text-primary text-[20px]" />
                <span className="text-sm font-semibold text-on-surface font-headline">{task.asset}</span>
              </div>
              <span className="text-sm text-on-surface-variant">{task.failure}</span>
              <div className="flex items-center gap-2">
                <div className="flex-1 h-2 bg-surface-container-low rounded-full overflow-hidden">
                  <div className={`h-full rounded-full ${probColor}`} style={{ width: `${task.probability}%` }} />
                </div>
                <span className="text-xs font-mono text-on-surface-variant w-10 text-right">{task.probability}%</span>
              </div>
              <div>
                <StatusBadge status={task.urgency} />
              </div>
              <div className="flex justify-end">
                <button
                  onClick={(e) => { e.stopPropagation(); navigate('/maintenance/add'); }}
                  className="bg-on-primary-container/20 text-on-primary-container text-[0.6875rem] font-semibold tracking-wider uppercase px-3 py-2 rounded-lg hover:bg-on-primary-container/30 transition-colors whitespace-nowrap"
                >
                  {t('failure_forecast.btn_initiate_task')}
                </button>
              </div>
            </div>
          )
        })}
      </div>

      {/* System health footer */}
      <div className="flex justify-end">
        <div className="bg-surface-container rounded-xl px-5 py-3 flex items-center gap-3">
          <span className="w-2.5 h-2.5 rounded-full bg-[#34d399] animate-pulse" />
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('failure_forecast.system_health_label')}</span>
          <span className="text-sm font-bold font-headline text-[#34d399]">{t('failure_forecast.system_health_optimal')}</span>
        </div>
      </div>
    </div>
  )
}

/* ══════════════════════════════════════════════
   MAIN COMPONENT
   ══════════════════════════════════════════════ */

const PredictiveHub = memo(function PredictiveHub() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [activeTab, setActiveTab] = useState<TabKey>('overview')
  const { data: modelsResponse } = usePredictionModels()
  const models = modelsResponse?.data ?? []

  const renderTabContent = () => {
    switch (activeTab) {
      case 'overview':
        return <OverviewTab />
      case 'alerts':
        return <AlertsTab />
      case 'insights':
        return <InsightsTab />
      case 'recommendations':
        return <RecommendationsTab />
      case 'timeline':
        return <TimelineTab />
      case 'forecast':
        return <ForecastTab />
      default:
        return <OverviewTab />
    }
  }

  return (
    <div className="min-h-screen bg-surface p-6 font-body text-on-surface">
      {/* Breadcrumb */}
      <button
        onClick={() => navigate('/dashboard')}
        className="flex items-center gap-1 text-on-surface-variant text-sm mb-6 hover:text-primary transition-colors"
      >
        <Icon name="arrow_back" className="text-[18px]" />
        <span className="uppercase tracking-wider text-[0.6875rem] font-semibold">
          {t('common.back_to_dashboard')}
        </span>
      </button>

      {/* ── Shared Header (always visible) ──────── */}
      <div className="flex items-start justify-between mb-6">
        <div>
          <h1 className="font-headline text-3xl font-bold tracking-tight text-on-surface">
            {t('predictive.title_zh')}
          </h1>
          <p className="text-on-surface-variant text-sm mt-1 font-label tracking-widest uppercase">
            Predictive Maintenance Hub
          </p>
        </div>
        <div className="flex items-center gap-6">
          <button
            onClick={() => navigate('/maintenance')}
            className="bg-surface-container-high hover:bg-surface-container-highest px-4 py-2.5 rounded-lg flex items-center gap-2 text-sm font-semibold text-on-surface transition-colors"
          >
            <Icon name="build" className="text-primary text-xl" />
            維護管理
          </button>
          <div className="bg-surface-container-high px-4 py-2 rounded-lg flex items-center gap-2">
            <Icon name="model_training" className="text-primary text-xl" />
            <div>
              <p className="text-[10px] text-on-surface-variant font-label uppercase tracking-widest">
                {t('predictive.model_accuracy')}
              </p>
              <p className="text-primary font-headline text-xl font-bold leading-tight">
                {models.length > 0 ? `${models.filter(m => m.enabled).length}/${models.length}` : '98.4%'}
              </p>
            </div>
          </div>
          <div className="bg-surface-container-high px-4 py-2 rounded-lg flex items-center gap-2">
            <Icon name="schedule" className="text-on-surface-variant text-xl" />
            <div>
              <p className="text-[10px] text-on-surface-variant font-label uppercase tracking-widest">
                {t('predictive.last_update')}
              </p>
              <p className="text-on-surface font-headline text-xl font-bold tabular-nums leading-tight">
                14:20:05
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Stat cards row */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        {[
          { label: 'Total Assets at Risk', value: '42', icon: 'warning', delta: '5 since yesterday', deltaColor: 'text-tertiary' },
          { label: 'High Priority Tasks', value: '12', icon: 'priority_high', delta: '3 critical overdue', deltaColor: 'text-error' },
          { label: 'Downtime Saved', value: '158h', icon: 'timer', delta: 'Est. $420k saved', deltaColor: 'text-primary' },
        ].map((s) => (
          <div key={s.label} className="bg-surface-container rounded-xl p-5 flex flex-col gap-3">
            <div className="flex items-center gap-2">
              <div className="bg-surface-container-high rounded-lg p-2">
                <Icon name={s.icon} className="text-primary text-xl" />
              </div>
              <span className="text-xs text-on-surface-variant font-label uppercase tracking-widest">
                {s.label}
              </span>
            </div>
            <p className="font-headline text-4xl font-extrabold tracking-tight text-on-surface">
              {s.value}
            </p>
            <p className={`text-xs font-label ${s.deltaColor}`}>{s.delta}</p>
          </div>
        ))}
      </div>

      {/* ── Tab Navigation ──────────────────────── */}
      <div className="flex gap-1 mb-6 border-b border-surface-container-highest pb-0">
        {TAB_DEFINITIONS.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`px-5 py-3 rounded-t-lg text-[0.6875rem] font-semibold tracking-wider transition-colors relative ${
              activeTab === tab.key
                ? 'bg-surface-container text-primary'
                : 'text-on-surface-variant hover:bg-surface-container-high hover:text-on-surface'
            }`}
          >
            <span className="block">{tab.labelZh}</span>
            {activeTab === tab.key && (
              <span className="absolute bottom-0 left-0 right-0 h-0.5 bg-primary rounded-t" />
            )}
          </button>
        ))}
      </div>

      {/* ── Tab Content ─────────────────────────── */}
      {renderTabContent()}
    </div>
  )
})

export default PredictiveHub
