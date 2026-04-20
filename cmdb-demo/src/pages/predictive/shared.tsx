import { useTranslation } from 'react-i18next'

/**
 * Shared helpers for Predictive Hub tab components.
 * Extracted verbatim from the original PredictiveHub.tsx during the phase 3.2
 * refactor. No behavior changes.
 */

export function Icon({ name, className = '' }: { name: string; className?: string }) {
  return (
    <span className={`material-symbols-outlined ${className}`}>{name}</span>
  )
}

export type TabKey =
  | 'overview'
  | 'alerts'
  | 'insights'
  | 'recommendations'
  | 'timeline'
  | 'forecast'

export const TAB_DEFINITIONS: { key: TabKey; labelKey: string }[] = [
  { key: 'overview', labelKey: 'predictive_hub.tab_overview' },
  { key: 'alerts', labelKey: 'predictive_hub.tab_alerts' },
  { key: 'insights', labelKey: 'predictive_hub.tab_insights' },
  { key: 'recommendations', labelKey: 'predictive_hub.tab_recommendations' },
  { key: 'timeline', labelKey: 'predictive_hub.tab_timeline' },
  { key: 'forecast', labelKey: 'predictive_hub.tab_forecast' },
]

/* ── Tab 1: Overview data ───────────────────── */

export type AdvisorMessage = { text: string; time: string }

/* ── Tab 2: Alerts data ─────────────────────── */

export const ALERT_FILTER_TABS = [
  { key: 'ALL ASSETS', labelKey: 'predictive_hub.filter_all_assets' },
  { key: 'DATACENTER-A', labelKey: 'predictive_hub.filter_datacenter_a' },
  { key: 'DATACENTER-B', labelKey: 'predictive_hub.filter_datacenter_b' },
  { key: 'EDGE-NODES', labelKey: 'predictive_hub.filter_edge_nodes' },
] as const

/* ── Tab 3: Insights data ───────────────────── */

export const GANTT_BAR_COLORS = {
  critical: 'bg-error',
  major: 'bg-tertiary',
  minor: 'bg-primary',
}

export const INSIGHT_PRIORITY_COLORS: Record<string, string> = {
  CRITICAL: 'bg-error-container text-on-error-container',
  HIGH: 'bg-[#92400e] text-[#fbbf24]',
  MEDIUM: 'bg-[#1e3a5f] text-primary',
}

/* ── Tab 4: Recommendations data ────────────── */

export const RISK_COLOR: Record<string, string> = {
  critical: 'bg-error/80',
  high: 'bg-tertiary/60',
  medium: 'bg-[#92400e]/60',
  low: 'bg-primary/20',
}

/* ── Tab 5: Timeline data ───────────────────── */

export const SEVERITY_CONFIG: Record<string, { dot: string; label: string; bg: string }> = {
  CRITICAL: { dot: 'bg-error', label: 'text-error', bg: 'bg-error-container' },
  'POTENTIAL ISSUE': { dot: 'bg-tertiary', label: 'text-tertiary', bg: 'bg-[#92400e]' },
  SCHEDULED: { dot: 'bg-primary', label: 'text-primary', bg: 'bg-[#1e3a5f]' },
}

export const BUTTON_STYLES: Record<string, string> = {
  danger: 'bg-error-container text-on-error-container hover:bg-error/30',
  warning: 'bg-[#92400e] text-[#fbbf24] hover:bg-[#92400e]/80',
  default: 'bg-[#064e3b] text-[#34d399] hover:bg-[#064e3b]/80',
}

/* ── Tab 6: Forecast data ───────────────────── */

export const CHART_WIDTH = 720
export const CHART_HEIGHT = 260
export const CHART_PADDING = { top: 20, right: 20, bottom: 30, left: 45 }
export const INNER_W = CHART_WIDTH - CHART_PADDING.left - CHART_PADDING.right
export const INNER_H = CHART_HEIGHT - CHART_PADDING.top - CHART_PADDING.bottom

export function toPath(data: number[]): string {
  return data
    .map((val, i) => {
      const x = CHART_PADDING.left + (i / (data.length - 1)) * INNER_W
      const y = CHART_PADDING.top + INNER_H - (val / 100) * INNER_H
      return `${i === 0 ? 'M' : 'L'}${x},${y}`
    })
    .join(' ')
}

export function toAreaPath(data: number[]): string {
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

/* ── Shared sub-components ──────────────────── */

export function RulBar({ days, max }: { days: number; max: number }) {
  const { t } = useTranslation()
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
        {t('predictive_hub.rul_days', { days })}
      </span>
    </div>
  )
}

export function ConfidenceBar({ value }: { value: number }) {
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

export function EmptyState({ message }: { message: string }) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-on-surface-variant">
      <span className="material-symbols-outlined text-4xl mb-2">info</span>
      <p className="text-sm">{message}</p>
    </div>
  )
}

export function LoadingSpinner() {
  return (
    <div className="flex items-center justify-center py-16">
      <span className="material-symbols-outlined text-4xl text-primary animate-spin">progress_activity</span>
    </div>
  )
}
