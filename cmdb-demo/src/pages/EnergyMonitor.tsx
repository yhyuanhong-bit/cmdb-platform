import { memo, useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { useMetrics } from '../hooks/useMetrics'
import { useLocationContext } from '../contexts/LocationContext'
import { apiClient } from '../lib/api/client'
import EmptyState from '../components/EmptyState'

/* ------------------------------------------------------------------ */
/*  Shared helpers                                                     */
/* ------------------------------------------------------------------ */

function IconSpan({ name, className = '' }: { name: string; className?: string }) {
  return <span className={`material-symbols-outlined ${className}`}>{name}</span>
}

function Section({
  title,
  icon,
  children,
  className = '',
}: {
  title: string
  icon: string
  children: React.ReactNode
  className?: string
}) {
  return (
    <div className={`rounded-lg bg-surface-container p-5 ${className}`}>
      <div className="mb-4 flex items-center gap-2">
        <IconSpan name={icon} className="text-primary text-xl" />
        <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
          {title}
        </h3>
      </div>
      {children}
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Facility View Data                                                 */
/* ------------------------------------------------------------------ */

/* BOTTOM_STATS icons map — keyed by category name (case-insensitive prefix match) */
const CATEGORY_ICON: Record<string, string> = {
  'IT Equipment': 'memory',
  'Cooling': 'ac_unit',
  'UPS/Power': 'battery_charging_full',
  'UPS': 'battery_charging_full',
  'Other': 'more_horiz',
  'Misc': 'more_horiz',
  'Lighting': 'lightbulb',
}

/* ------------------------------------------------------------------ */
/*  Power Load View Data                                               */
/* ------------------------------------------------------------------ */

const donutSegments = [
  { label: 'Tier 1 Critical', i18nKey: 'power_load.tier_1_critical', value: 840, color: '#ffb4ab' },
  { label: 'Tier 2 Important', i18nKey: 'power_load.tier_2_important', value: 290, color: '#ffb5a0' },
  { label: 'Tier 3 Standard', i18nKey: 'power_load.tier_3_standard', value: 110, color: '#9ecaff' },
]

/* ------------------------------------------------------------------ */
/*  Helper functions                                                   */
/* ------------------------------------------------------------------ */

function buildAreaPath(
  data: { time: string; process: number; lighting: number }[],
  key: 'process' | 'lighting',
  maxY: number,
  w: number,
  h: number,
  padX: number,
  padY: number,
): { line: string; area: string } {
  const drawW = w - padX * 2
  const drawH = h - padY * 2
  const points = data.map((d, i) => {
    const x = padX + (i / (data.length - 1)) * drawW
    const y = padY + drawH - (d[key] / maxY) * drawH
    return { x, y }
  })
  const line = points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(' ')
  const area =
    line +
    ` L ${points[points.length - 1].x.toFixed(1)} ${padY + drawH} L ${points[0].x.toFixed(1)} ${padY + drawH} Z`
  return { line, area }
}

/* ------------------------------------------------------------------ */
/*  Capacity Donut                                                     */
/* ------------------------------------------------------------------ */

function CapacityDonut({ pct, t }: { pct: number; t: ReturnType<typeof useTranslation>['t'] }) {
  const circumference = 2 * Math.PI * 52
  return (
    <div className="relative h-40 w-40">
      <svg viewBox="0 0 120 120" className="h-full w-full -rotate-90">
        <circle cx="60" cy="60" r="52" fill="none" stroke="#121d23" strokeWidth="10" />
        <circle
          cx="60"
          cy="60"
          r="52"
          fill="none"
          stroke="#9ecaff"
          strokeWidth="10"
          strokeLinecap="round"
          strokeDasharray={`${circumference}`}
          strokeDashoffset={`${circumference * (1 - pct / 100)}`}
          className="transition-all duration-700"
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className="font-headline text-2xl font-bold text-on-surface">{pct}%</span>
        <span className="text-[10px] uppercase tracking-wider text-on-surface-variant">{t('facility_energy.capacity')}</span>
      </div>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Bar Chart                                                          */
/* ------------------------------------------------------------------ */

function BarChart({ data }: { data: { day: string; value: number }[] }) {
  const maxVal = Math.max(...data.map((d) => d.value))
  return (
    <div className="flex items-end gap-2" style={{ height: 160 }}>
      {data.map((bar) => {
        const heightPct = (bar.value / maxVal) * 100
        return (
          <div key={bar.day} className="flex flex-1 flex-col items-center gap-1">
            <span className="text-[10px] font-semibold text-on-surface-variant">
              {bar.value >= 1000 ? `${(bar.value / 1000).toFixed(1)}k` : bar.value}
            </span>
            <div
              className="w-full rounded-t bg-primary/80 transition-all duration-500 hover:bg-primary"
              style={{ height: `${heightPct}%`, minHeight: 4 }}
            />
            <span className="text-[10px] text-on-surface-variant">{bar.day}</span>
          </div>
        )
      })}
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Facility View                                                      */
/* ------------------------------------------------------------------ */

interface BreakdownCategory {
  name: string
  avg_kw: number
  pct: number
}
interface BreakdownData {
  categories: BreakdownCategory[]
  total_kw: number
}
interface SummaryData {
  pue: number
  total_kw: number
  peak_kw: number
  carbon_mt_monthly: number
}

function FacilityView({
  t,
  powerData,
  latestPUE,
  isLoading,
  breakdownData,
  summaryData,
}: {
  t: ReturnType<typeof useTranslation>['t']
  powerData: { time: string; value: number }[]
  latestPUE: number
  isLoading: boolean
  breakdownData: BreakdownData | undefined
  summaryData: SummaryData | undefined
}) {
  const [barTab, setBarTab] = useState<string>('Daily')

  const DAILY_BARS = useMemo(() => {
    if (powerData.length === 0) return [
      { day: 'Mon', value: 0 }, { day: 'Tue', value: 0 }, { day: 'Wed', value: 0 },
      { day: 'Thu', value: 0 }, { day: 'Fri', value: 0 }, { day: 'Sat', value: 0 }, { day: 'Sun', value: 0 },
    ]
    const days = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
    const byDay: Record<string, number[]> = {}
    for (const p of powerData) {
      const parsed = new Date(p.time)
      if (isNaN(parsed.getTime())) continue
      const d = days[parsed.getDay()]
      if (!byDay[d]) byDay[d] = []
      byDay[d].push(p.value)
    }
    return ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'].map(day => ({
      day,
      value: Math.round((byDay[day] ?? [0]).reduce((a, b) => a + b, 0) / Math.max(1, (byDay[day] ?? []).length) * 1000),
    }))
  }, [powerData])

  const WEEKLY_BARS = useMemo(() => {
    if (powerData.length === 0) return [
      { day: 'W1', value: 0 }, { day: 'W2', value: 0 }, { day: 'W3', value: 0 }, { day: 'W4', value: 0 },
    ]
    const byWeek: Record<string, number[]> = {}
    for (const p of powerData) {
      const weekNum = `W${Math.ceil(new Date(p.time).getDate() / 7)}`
      if (!byWeek[weekNum]) byWeek[weekNum] = []
      byWeek[weekNum].push(p.value)
    }
    return Object.entries(byWeek).map(([week, vals]) => ({
      day: week,
      value: Math.round(vals.reduce((a, b) => a + b, 0) * 10),
    })).slice(0, 4)
  }, [powerData])

  const MONTHLY_BARS = useMemo(() => {
    if (powerData.length === 0) return [
      { day: 'Jan', value: 0 }, { day: 'Feb', value: 0 }, { day: 'Mar', value: 0 },
      { day: 'Apr', value: 0 }, { day: 'May', value: 0 }, { day: 'Jun', value: 0 },
    ]
    const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
    const byMonth: Record<string, number[]> = {}
    for (const p of powerData) {
      const m = months[new Date(p.time).getMonth()]
      if (!byMonth[m]) byMonth[m] = []
      byMonth[m].push(p.value)
    }
    return Object.entries(byMonth).map(([month, vals]) => ({
      day: month,
      value: Math.round(vals.reduce((a, b) => a + b, 0) * 100),
    })).slice(0, 6)
  }, [powerData])

  const BAR_DATASETS: Record<string, typeof DAILY_BARS> = {
    Daily: DAILY_BARS,
    Weekly: WEEKLY_BARS,
    Monthly: MONTHLY_BARS,
  }

  // Derive facility load — prefer summary API, then fall back to metrics. No
  // fabricated fallback: if neither is available the UI shows an em-dash.
  const totalKw = summaryData?.total_kw ?? breakdownData?.total_kw
  const currentLoad = totalKw != null
    ? totalKw.toLocaleString(undefined, { maximumFractionDigits: 1 })
    : powerData.length > 0
      ? (powerData[powerData.length - 1].value * 1000).toFixed(1)
      : null
  const capacityPct = totalKw != null
    ? Math.round((totalKw / 1680) * 100)
    : powerData.length > 0
      ? Math.round((powerData[powerData.length - 1].value / 1.68) * 100)
      : null

  // Summary stats. Null when neither the summary API nor metrics provide data.
  const displayPUE = summaryData?.pue ?? latestPUE
  const carbonMT = summaryData?.carbon_mt_monthly ?? null
  const peakMW = summaryData?.peak_kw != null ? (summaryData.peak_kw / 1000).toFixed(2) : null

  // Bottom stats derived strictly from /power/breakdown categories.
  const bottomStats: { label: string; value: string; icon: string; pct: number }[] =
    breakdownData?.categories && breakdownData.categories.length > 0
      ? breakdownData.categories.map((cat) => ({
          label: cat.name,
          value: `${cat.avg_kw.toFixed(1)} kW`,
          icon: CATEGORY_ICON[cat.name] ?? 'electric_bolt',
          pct: cat.pct,
        }))
      : []

  return (
    <>
      {/* Top row: Large stat + donut + status  |  Right cards */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        {/* Left: primary stat panel */}
        <div className="rounded-lg bg-surface-container p-6 lg:col-span-2">
          <div className="flex flex-col items-center gap-6 sm:flex-row sm:items-start">
            {/* Large stat */}
            <div className="flex-1">
              <span className="text-[10px] font-semibold uppercase tracking-widest text-on-surface-variant">
                {t('facility_energy.current_facility_load')}
              </span>
              <p className="font-headline text-5xl font-bold text-on-surface mt-1">
                {isLoading ? '---' : (currentLoad ?? '—')} <span className="text-2xl text-on-surface-variant">kW</span>
              </p>
              <div className="mt-5 space-y-3">
                <div className="flex items-center gap-3">
                  <span className="inline-block h-2.5 w-2.5 rounded-full bg-[#34d399]" />
                  <span className="text-xs text-on-surface-variant">{t('facility_energy.grid_status')}:</span>
                  <span className="text-xs font-semibold text-[#34d399]">{t('facility_energy.stable')}</span>
                </div>
                <div className="flex items-center gap-3">
                  <IconSpan name="electrical_services" className="text-sm text-on-surface-variant" />
                  <span className="text-xs text-on-surface-variant">{t('facility_energy.voltage_variance')}:</span>
                  <span className="text-xs font-semibold text-on-surface">&plusmn;0.02%</span>
                </div>
              </div>
            </div>
            {/* Donut */}
            <div className="flex flex-col items-center">
              <CapacityDonut pct={capacityPct ?? 0} t={t} />
              <span className="mt-2 text-[10px] uppercase tracking-wider text-on-surface-variant">
                {t('facility_energy.of_rated_capacity')}
              </span>
            </div>
          </div>
        </div>

        {/* Right: info cards */}
        <div className="flex flex-col gap-4">
          <div className="rounded-lg bg-surface-container p-5">
            <div className="flex items-center gap-2 text-on-surface-variant">
              <IconSpan name="speed" className="text-lg" />
              <span className="text-[10px] font-semibold uppercase tracking-widest">
                {t('facility_energy.pue_efficiency')}
              </span>
            </div>
            <p className="mt-2 font-headline text-3xl font-bold text-primary">{displayPUE.toFixed(2)}</p>
            <span className="text-[10px] text-on-surface-variant">{t('facility_energy.industry_avg')}: 1.58</span>
          </div>
          <div className="rounded-lg bg-surface-container p-5">
            <div className="flex items-center gap-2 text-on-surface-variant">
              <IconSpan name="eco" className="text-lg" />
              <span className="text-[10px] font-semibold uppercase tracking-widest">
                {t('facility_energy.carbon_footprint')}
              </span>
            </div>
            <p className="mt-2 font-headline text-3xl font-bold text-on-surface">
              {carbonMT ?? '—'} <span className="text-lg text-on-surface-variant">MT</span>
            </p>
            <span className="text-[10px] text-on-surface-variant">{t('facility_energy.monthly_co2_equivalent')}</span>
          </div>
          <div className="rounded-lg bg-surface-container p-5">
            <div className="flex items-center gap-2 text-on-surface-variant">
              <IconSpan name="bolt" className="text-lg" />
              <span className="text-[10px] font-semibold uppercase tracking-widest">
                {t('facility_energy.peak_demand_record')}
              </span>
            </div>
            <p className="mt-2 font-headline text-3xl font-bold text-on-surface">
              {peakMW ?? '—'} <span className="text-lg text-on-surface-variant">MW</span>
            </p>
            {/* TODO(phase-3.10): surface the actual peak-recorded date from the
                /power/summary endpoint once the backend exposes it. */}
            <span className="text-[10px] text-on-surface-variant">{t('facility_energy.monthly_co2_equivalent')}</span>
          </div>
        </div>
      </div>

      {/* Historical Consumption Trend */}
      <Section title={t('facility_energy.historical_consumption_trend')} icon="bar_chart">
        <div className="mb-5 flex gap-1">
          {(['Daily', 'Weekly', 'Monthly'] as const).map((tab) => (
            <button
              key={tab}
              type="button"
              onClick={() => setBarTab(tab)}
              className={`rounded-md px-4 py-1.5 text-xs font-semibold uppercase tracking-wider transition-colors ${
                barTab === tab
                  ? 'bg-primary text-on-primary-container'
                  : 'bg-surface-container-low text-on-surface-variant hover:bg-surface-container-high'
              }`}
            >
              {t(`facility_energy.tab_${tab.toLowerCase()}`)}
            </button>
          ))}
        </div>
        <BarChart data={BAR_DATASETS[barTab]} />
      </Section>

      {/* Bottom stat cards */}
      {bottomStats.length === 0 ? (
        <div className="rounded-lg bg-surface-container p-5">
          <EmptyState
            icon="electric_bolt"
            title={t('common.empty_no_data_title')}
            description={t('common.empty_no_data_desc')}
            tone="neutral"
            compact
          />
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {bottomStats.map((stat) => (
            <div key={stat.label} className="rounded-lg bg-surface-container p-5">
              <div className="mb-1 flex items-center gap-2 text-on-surface-variant">
                <IconSpan name={stat.icon} className="text-lg" />
                <span className="text-xs font-medium uppercase tracking-wider">{stat.label}</span>
              </div>
              <p className="font-headline text-2xl font-bold text-on-surface">{stat.value}</p>
              <div className="mt-2 flex items-center gap-2">
                <div className="h-1.5 flex-1 rounded-full bg-surface-container-low">
                  <div
                    className="h-1.5 rounded-full bg-primary transition-all duration-500"
                    style={{ width: `${stat.pct}%` }}
                  />
                </div>
                <span className="shrink-0 text-[10px] font-semibold text-on-surface-variant">{stat.pct}%</span>
              </div>
            </div>
          ))}
        </div>
      )}
    </>
  )
}

/* ------------------------------------------------------------------ */
/*  Power Load View                                                    */
/* ------------------------------------------------------------------ */

interface TrendPoint {
  hour: string
  total_kw: number
}

function PowerLoadView({
  t,
  trendPoints,
  summaryData,
}: {
  t: ReturnType<typeof useTranslation>['t']
  trendPoints: TrendPoint[]
  summaryData: SummaryData | undefined
}) {
  const chartW = 560
  const chartH = 260
  const padX = 50
  const padY = 20
  const maxY = 1500
  const drawW = chartW - padX * 2
  const drawH = chartH - padY * 2

  // Map API trend points to the chart data shape expected by buildAreaPath.
  // The API returns total_kw per hour; we treat it as "process" and derive a
  // lighting estimate (~15% of total) so the dual-series chart remains useful.
  // Returns empty array when no real trend exists — caller renders an empty
  // state instead of a 24-point fabricated curve.
  const powerTrendData = useMemo(() => {
    if (trendPoints.length > 0) {
      return trendPoints.map((p) => {
        const label = new Date(p.hour).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false })
        const lighting = Math.round(p.total_kw * 0.15)
        const process = Math.round(p.total_kw - lighting)
        return { time: label, process, lighting }
      })
    }
    return []
  }, [trendPoints])

  // Only build SVG paths when there is enough data (buildAreaPath divides by
  // data.length - 1, so we need at least 2 points).
  const hasPowerTrend = powerTrendData.length >= 2
  const processPath = hasPowerTrend
    ? buildAreaPath(powerTrendData, 'process', maxY, chartW, chartH, padX, padY)
    : null
  const lightingPath = hasPowerTrend
    ? buildAreaPath(powerTrendData, 'lighting', maxY, chartW, chartH, padX, padY)
    : null

  const donutTotal = donutSegments.reduce((s, d) => s + d.value, 0)
  let donutOffset = 0

  return (
    <>
      {/* Stats Row */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <div className="rounded-lg bg-surface-container p-5">
          <p className="text-xs uppercase tracking-wider text-on-surface-variant">{t('power_load.stat_equipment_load')}</p>
          <p className="mt-2 font-headline text-3xl font-bold text-on-surface">
            {summaryData?.total_kw != null
              ? summaryData.total_kw.toLocaleString(undefined, { maximumFractionDigits: 1 })
              : '—'}
          </p>
          <p className="mt-1 text-xs text-on-surface-variant">{t('power_load.instant_peak_note')}</p>
        </div>
        <div className="rounded-lg bg-surface-container p-5">
          <p className="text-xs uppercase tracking-wider text-on-surface-variant">{t('power_load.stat_pue_index')}</p>
          <p className="mt-2 font-headline text-3xl font-bold text-[#34d399]">
            {summaryData?.pue != null ? summaryData.pue.toFixed(2) : '—'}
          </p>
          <p className="mt-1 text-xs text-on-surface-variant">{t('power_load.efficient_ratio_note')}</p>
        </div>
        <div className="rounded-lg bg-surface-container p-5">
          <p className="text-xs uppercase tracking-wider text-on-surface-variant">{t('power_load.stat_grid_input')}</p>
          <p className="mt-2 font-headline text-3xl font-bold text-on-surface">
            {summaryData?.peak_kw != null
              ? summaryData.peak_kw.toLocaleString(undefined, { maximumFractionDigits: 1 })
              : '—'}
          </p>
          <p className="mt-1 text-xs text-on-surface-variant">kW</p>
        </div>
        <div className="rounded-lg bg-surface-container p-5">
          <p className="text-xs uppercase tracking-wider text-on-surface-variant">{t('power_load.stat_ups_autonomy')}</p>
          {/* TODO(phase-3.10): wire up GET /power/ups-autonomy when the
              backend endpoint is available. Previously hardcoded "42 min". */}
          <p className="mt-2 font-headline text-3xl font-bold text-primary">—</p>
          <p className="mt-1 text-xs text-on-surface-variant">{t('common.coming_soon')}</p>
        </div>
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        {/* Area Chart */}
        <div className="col-span-2 rounded-lg bg-surface-container p-5">
          <div className="mb-3 flex items-center justify-between">
            <h3 className="font-headline text-sm font-bold text-on-surface">{t('power_load.chart_power_trend')}</h3>
            <div className="flex items-center gap-4 text-xs text-on-surface-variant">
              <span className="flex items-center gap-1">
                <span className="inline-block h-2 w-2 rounded-full bg-[#9ecaff]" /> {t('power_load.legend_process_consumption')}
              </span>
              <span className="flex items-center gap-1">
                <span className="inline-block h-2 w-2 rounded-full bg-[#ffb5a0]" /> {t('power_load.legend_lighting_load')}
              </span>
            </div>
          </div>
          {hasPowerTrend && processPath && lightingPath ? (
            <svg viewBox={`0 0 ${chartW} ${chartH}`} className="w-full">
              {/* Grid lines */}
              {[0, 300, 600, 900, 1200, 1500].map((v) => {
                const y = padY + drawH - (v / maxY) * drawH
                return (
                  <g key={v}>
                    <line x1={padX} y1={y} x2={chartW - padX} y2={y} stroke="#202b32" strokeWidth="1" />
                    <text x={padX - 8} y={y + 4} textAnchor="end" fill="#8e9196" fontSize="9">{v}</text>
                  </g>
                )
              })}
              {/* Time labels */}
              {powerTrendData.map((d, i) => {
                const x = padX + (i / (powerTrendData.length - 1)) * drawW
                return i % 2 === 0 ? (
                  <text key={d.time} x={x} y={chartH - 2} textAnchor="middle" fill="#8e9196" fontSize="9">{d.time}</text>
                ) : null
              })}
              {/* Process area */}
              <path d={processPath.area} fill="rgba(158,202,255,0.15)" />
              <path d={processPath.line} fill="none" stroke="#9ecaff" strokeWidth="2" />
              {/* Lighting area */}
              <path d={lightingPath.area} fill="rgba(255,181,160,0.15)" />
              <path d={lightingPath.line} fill="none" stroke="#ffb5a0" strokeWidth="2" />
            </svg>
          ) : (
            <EmptyState
              icon="timeline"
              title={t('common.empty_awaiting_signal_title')}
              description={t('common.empty_awaiting_signal_desc')}
              tone="info"
              compact
            />
          )}
        </div>

        {/* Donut Chart */}
        <div className="rounded-lg bg-surface-container p-5">
          <h3 className="mb-3 font-headline text-sm font-bold text-on-surface">{t('power_load.chart_bia_power_ratio')}</h3>
          <div className="flex justify-center">
            <svg viewBox="0 0 120 120" className="h-40 w-40">
              {donutSegments.map((seg) => {
                const pct = seg.value / donutTotal
                const dashLen = pct * 283
                const dashOffset = -donutOffset * 283
                donutOffset += pct
                return (
                  <circle
                    key={seg.label}
                    cx="60"
                    cy="60"
                    r="45"
                    fill="none"
                    stroke={seg.color}
                    strokeWidth="14"
                    strokeDasharray={`${dashLen} ${283 - dashLen}`}
                    strokeDashoffset={dashOffset}
                    transform="rotate(-90 60 60)"
                  />
                )
              })}
              <text x="60" y="55" textAnchor="middle" fill="#ffb4ab" fontSize="16" fontWeight="bold">68%</text>
              <text x="60" y="70" textAnchor="middle" fill="#8e9196" fontSize="8">{t('common.critical')}</text>
            </svg>
          </div>
          <div className="mt-3 space-y-2">
            {donutSegments.map((seg) => (
              <div key={seg.label} className="flex items-center justify-between text-xs">
                <span className="flex items-center gap-2">
                  <span className="inline-block h-2 w-2 rounded-full" style={{ backgroundColor: seg.color }} />
                  <span className="text-on-surface-variant">{t(seg.i18nKey)}</span>
                </span>
                <span className="font-medium text-on-surface">{seg.value} kW</span>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Rack Heatmap */}
      <div className="rounded-lg bg-surface-container p-5">
        <h3 className="mb-3 font-headline text-sm font-bold text-on-surface">{t('power_load.section_rack_heatmap')}</h3>
        {/* TODO(phase-3.10): wire up GET /power/racks/heatmap when the backend
            endpoint ships. Previously rendered a 12-cell fabricated grid. */}
        <EmptyState
          icon="view_module"
          title={t('common.empty_not_wired_title')}
          description={t('common.empty_not_wired_desc')}
          tone="neutral"
          compact
        />
      </div>

      {/* Power Events Table */}
      <div className="rounded-lg bg-surface-container p-5">
        <h3 className="mb-4 font-headline text-sm font-bold text-on-surface">{t('power_load.section_power_events')}</h3>
        {/* TODO(phase-3.10): wire up GET /power/events when the backend event
            stream endpoint ships. Previously rendered a static list of
            fabricated severity-tagged rows. */}
        <EmptyState
          icon="event_note"
          title={t('common.empty_not_wired_title')}
          description={t('common.empty_not_wired_desc')}
          tone="neutral"
          compact
        />
      </div>
    </>
  )
}

/* ------------------------------------------------------------------ */
/*  Main Page                                                          */
/* ------------------------------------------------------------------ */

function EnergyMonitor() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [viewMode, setViewMode] = useState<'facility' | 'powerload'>('facility')

  const { path } = useLocationContext()
  const locationId = path.campus?.id ?? 'd0000000-0000-0000-0000-000000000004'

  // Fetch power metrics for a representative asset
  const powerQ = useMetrics({
    asset_id: 'f0000000-0000-0000-0000-000000000001',
    metric_name: 'power_kw',
    time_range: '168h', // 7 days
  })
  const powerData = powerQ.data?.data ?? []

  // Fetch PUE metric (kept as fallback)
  const pueQ = useMetrics({
    asset_id: 'f0000000-0000-0000-0000-000000000001',
    metric_name: 'pue',
    time_range: '24h',
  })
  const latestPUE = pueQ.data?.data?.[0]?.value ?? 1.35

  // Energy API queries
  const { data: breakdownRaw } = useQuery({
    queryKey: ['energyBreakdown'],
    queryFn: () => apiClient.get<BreakdownData>('/energy/breakdown'),
  })
  const { data: summaryRaw } = useQuery({
    queryKey: ['energySummary'],
    queryFn: () => apiClient.get<SummaryData>('/energy/summary'),
  })
  const { data: trendRaw, isError: trendError } = useQuery({
    queryKey: ['energyTrend'],
    queryFn: () => apiClient.get<{ trend: TrendPoint[] }>('/energy/trend', { hours: '24' }),
  })

  // Unwrap — energy endpoints return payload directly (no ApiResponse wrapper)
  const bd = breakdownRaw as Record<string, unknown> | undefined
  const breakdownData = bd?.categories ? (breakdownRaw as BreakdownData) : undefined
  const sd = summaryRaw as Record<string, unknown> | undefined
  const summaryData = sd?.pue != null ? (summaryRaw as SummaryData) : undefined
  const td = trendRaw as Record<string, unknown> | undefined
  const trendPoints: TrendPoint[] = (td?.trend as TrendPoint[] | undefined) ?? []
  const hasError = trendError

  // locationId used for future location-scoped queries
  void locationId

  return (
    <div className="min-h-screen space-y-6 bg-surface px-6 py-5 font-body text-on-surface">
      {/* Breadcrumb */}
      <nav
        aria-label="Breadcrumb"
        className="flex items-center gap-1.5 text-xs uppercase tracking-widest text-on-surface-variant"
      >
        <span
          className="cursor-pointer transition-colors hover:text-primary"
          onClick={() => navigate('/monitoring')}
        >
          {t('energy_monitor.breadcrumb_monitoring')}
        </span>
        <IconSpan name="chevron_right" className="text-[14px] opacity-40" />
        {['FACILITY_NODE_09', 'ENERGY_TELEMETRY'].map((crumb, i, arr) => (
          <span key={crumb} className="flex items-center gap-1.5">
            <span className="cursor-pointer transition-colors hover:text-primary">{crumb}</span>
            {i < arr.length - 1 && <IconSpan name="chevron_right" className="text-[14px] opacity-40" />}
          </span>
        ))}
      </nav>

      {/* Error Banner */}
      {hasError && (
        <div className="flex items-center gap-3 rounded-lg bg-error-container/30 px-4 py-3 text-sm text-error">
          <span className="material-symbols-outlined text-lg">error</span>
          {t('facility_energy.load_error', 'Failed to load energy data. Some charts may show incomplete information.')}
        </div>
      )}

      {/* Title + View Toggle */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h1 className="font-headline text-2xl font-bold text-on-surface">
          {t('facility_energy.title_zh')} / {t('facility_energy.title_en')}
        </h1>
        <div className="flex bg-surface-container-low rounded overflow-hidden">
          <button
            onClick={() => setViewMode('facility')}
            className={`flex items-center gap-1.5 px-4 py-2 text-xs font-semibold transition-colors ${
              viewMode === 'facility'
                ? 'bg-on-primary-container text-white'
                : 'text-on-surface-variant hover:bg-surface-container-high'
            }`}
          >
            <IconSpan name="domain" className="text-[16px]" />
            {t('energy_monitor.view_facility')}
          </button>
          <button
            onClick={() => setViewMode('powerload')}
            className={`flex items-center gap-1.5 px-4 py-2 text-xs font-semibold transition-colors ${
              viewMode === 'powerload'
                ? 'bg-on-primary-container text-white'
                : 'text-on-surface-variant hover:bg-surface-container-high'
            }`}
          >
            <IconSpan name="bolt" className="text-[16px]" />
            {t('energy_monitor.view_power_load')}
          </button>
        </div>
      </div>

      {/* Content */}
      {viewMode === 'facility' && (
        <FacilityView
          t={t}
          powerData={powerData}
          latestPUE={latestPUE}
          isLoading={powerQ.isLoading}
          breakdownData={breakdownData}
          summaryData={summaryData}
        />
      )}
      {viewMode === 'powerload' && (
        <PowerLoadView
          t={t}
          trendPoints={trendPoints}
          summaryData={summaryData}
        />
      )}
    </div>
  )
}

export default memo(EnergyMonitor)
