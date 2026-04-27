import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import Icon from '../components/Icon'
import { useEnergyPue } from '../hooks/useEnergyPhase2'
import { useAllLocations } from '../hooks/useTopology'
import type { EnergyLocationPue } from '../lib/api/energyBilling'

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

const todayISO = () => new Date().toISOString().slice(0, 10)
const daysAgoISO = (n: number) => {
  const d = new Date()
  d.setDate(d.getDate() - n)
  return d.toISOString().slice(0, 10)
}

function fmt(n: string | number, digits = 2): string {
  const v = Number(n)
  if (!isFinite(v)) return '—'
  return v.toLocaleString(undefined, {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })
}

/** Industry rule of thumb: PUE ≤ 1.3 is excellent, 1.3-1.6 is good,
 *  1.6-2.0 is average, > 2.0 means cooling/UPS is eating most of the
 *  power budget. The colours here mirror that. */
function pueColor(pue: number | null): string {
  if (pue == null) return 'text-on-surface-variant'
  if (pue <= 1.3) return 'text-emerald-400'
  if (pue <= 1.6) return 'text-blue-400'
  if (pue <= 2.0) return 'text-amber-400'
  return 'text-red-400'
}

/* ------------------------------------------------------------------ */
/*  Inline SVG chart — keeps the bundle clean of yet another chart lib */
/* ------------------------------------------------------------------ */

interface ChartPoint {
  day: string
  pue: number | null
}

function PueLineChart({ points, height = 200 }: { points: ChartPoint[]; height?: number }) {
  const { t } = useTranslation()
  const width = 720
  const padding = { top: 16, right: 16, bottom: 28, left: 36 }
  const inner = {
    w: width - padding.left - padding.right,
    h: height - padding.top - padding.bottom,
  }

  const valid = points.filter((p) => p.pue != null) as { day: string; pue: number }[]
  if (valid.length === 0) {
    return (
      <div className="flex items-center justify-center h-[200px] text-on-surface-variant text-sm italic">
        {t('energy_pue.chart_no_data')}
      </div>
    )
  }

  const minPue = Math.min(1.0, ...valid.map((p) => p.pue))
  const maxPue = Math.max(2.5, ...valid.map((p) => p.pue))
  const span = maxPue - minPue || 1

  const xFor = (i: number) => padding.left + (i / Math.max(valid.length - 1, 1)) * inner.w
  const yFor = (v: number) => padding.top + (1 - (v - minPue) / span) * inner.h

  const path = valid
    .map((p, i) => `${i === 0 ? 'M' : 'L'}${xFor(i).toFixed(1)} ${yFor(p.pue).toFixed(1)}`)
    .join(' ')

  // Y-axis gridlines at PUE = 1.0, 1.5, 2.0, 2.5
  const gridYs = [1.0, 1.5, 2.0, 2.5].filter((y) => y >= minPue && y <= maxPue)

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="w-full h-auto" role="img" aria-label="PUE over time">
      {/* Gridlines + labels */}
      {gridYs.map((y) => (
        <g key={y}>
          <line
            x1={padding.left}
            x2={padding.left + inner.w}
            y1={yFor(y)}
            y2={yFor(y)}
            stroke="#202b32"
            strokeWidth="1"
            strokeDasharray={y === 1.0 ? '0' : '2 2'}
          />
          <text x={4} y={yFor(y) + 3} fill="#8e9196" fontSize="10" fontFamily="Inter">
            {y.toFixed(1)}
          </text>
        </g>
      ))}
      {/* PUE line */}
      <path d={path} stroke="#9ecaff" strokeWidth="2" fill="none" strokeLinecap="round" strokeLinejoin="round" />
      {/* Points */}
      {valid.map((p, i) => (
        <circle
          key={p.day}
          cx={xFor(i)}
          cy={yFor(p.pue)}
          r={3}
          fill={
            p.pue <= 1.3 ? '#34d399' :
            p.pue <= 1.6 ? '#9ecaff' :
            p.pue <= 2.0 ? '#fbbf24' : '#f87171'
          }
        />
      ))}
      {/* X-axis dates — show endpoints + middle if room */}
      {valid.length > 0 && (
        <>
          <text x={xFor(0)} y={height - 8} fill="#8e9196" fontSize="10" fontFamily="Inter" textAnchor="start">
            {valid[0].day}
          </text>
          <text x={xFor(valid.length - 1)} y={height - 8} fill="#8e9196" fontSize="10" fontFamily="Inter" textAnchor="end">
            {valid[valid.length - 1].day}
          </text>
        </>
      )}
    </svg>
  )
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function EnergyPue() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  const [dayFrom, setDayFrom] = useState(daysAgoISO(30))
  const [dayTo, setDayTo] = useState(todayISO())
  const [locationFilter, setLocationFilter] = useState<string>('all')

  const locationsQ = useAllLocations()
  const locations = locationsQ.data?.data ?? []
  const locationName = (id?: string | null) => {
    if (!id) return t('energy_pue.unknown_location')
    return locations.find((l) => l.id === id)?.name ?? id.slice(0, 8) + '…'
  }

  const pueQ = useEnergyPue(
    dayFrom,
    dayTo,
    locationFilter === 'all' ? undefined : locationFilter,
  )
  const rows: EnergyLocationPue[] = pueQ.data?.data ?? []

  // Group rows by location for the per-location card grid + chart.
  const byLocation = useMemo(() => {
    const m = new Map<string, EnergyLocationPue[]>()
    for (const r of rows) {
      const arr = m.get(r.location_id) ?? []
      arr.push(r)
      m.set(r.location_id, arr)
    }
    // Sort each location's series ascending by day for the chart.
    for (const arr of m.values()) {
      arr.sort((a, b) => a.day.localeCompare(b.day))
    }
    return m
  }, [rows])

  // The chart shows the latest selected location, OR an aggregate if
  // the user hasn't picked a specific location and there are >= 2.
  const chartLocId = locationFilter !== 'all'
    ? locationFilter
    : Array.from(byLocation.keys())[0]
  const chartSeries = chartLocId
    ? (byLocation.get(chartLocId) ?? []).map<ChartPoint>((r) => ({
        day: r.day,
        pue: r.pue != null ? Number(r.pue) : null,
      }))
    : []

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      <header className="px-8 pt-6 pb-4">
        <nav className="flex items-center gap-1.5 text-xs text-on-surface-variant mb-3">
          <span className="hover:text-primary cursor-pointer" onClick={() => navigate('/monitoring/energy/bill')}>
            {t('energy_pue.breadcrumb_energy')}
          </span>
          <Icon name="chevron_right" className="text-[14px] text-on-surface-variant" />
          <span className="text-primary">{t('energy_pue.title')}</span>
        </nav>

        <div className="flex items-start justify-between flex-wrap gap-4">
          <div>
            <h1 className="font-headline font-bold text-2xl text-on-surface">{t('energy_pue.title')}</h1>
            <p className="text-sm text-on-surface-variant mt-1">{t('energy_pue.subtitle')}</p>
          </div>
          <button
            onClick={() => navigate('/monitoring/energy/anomalies')}
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-surface-container-high text-on-surface text-sm font-semibold hover:bg-surface-container-highest transition-colors"
          >
            <Icon name="warning" className="text-[18px]" />
            {t('energy_pue.btn_view_anomalies')}
          </button>
        </div>
      </header>

      {/* Filters */}
      <section className="px-8 pb-4">
        <div className="bg-surface-container rounded-lg p-5">
          <div className="flex flex-wrap items-end gap-4">
            <div>
              <label className="block text-xs text-on-surface-variant mb-1">{t('energy_pue.field_day_from')}</label>
              <input
                type="date"
                value={dayFrom}
                onChange={(e) => setDayFrom(e.target.value)}
                className="bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none"
              />
            </div>
            <div>
              <label className="block text-xs text-on-surface-variant mb-1">{t('energy_pue.field_day_to')}</label>
              <input
                type="date"
                value={dayTo}
                onChange={(e) => setDayTo(e.target.value)}
                className="bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none"
              />
            </div>
            <div>
              <label className="block text-xs text-on-surface-variant mb-1">{t('energy_pue.field_location')}</label>
              <select
                value={locationFilter}
                onChange={(e) => setLocationFilter(e.target.value)}
                className="bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none"
              >
                <option value="all">{t('energy_pue.all_locations')}</option>
                {locations.map((l) => (
                  <option key={l.id} value={l.id}>
                    {l.name} ({l.level})
                  </option>
                ))}
              </select>
            </div>
            <div className="flex flex-col gap-1 ml-auto">
              <span className="text-xs text-on-surface-variant">{t('energy_pue.quick_select')}</span>
              <div className="flex gap-1">
                {[
                  { key: 'last_7', days: 7 },
                  { key: 'last_30', days: 30 },
                  { key: 'last_90', days: 90 },
                ].map(({ key, days }) => (
                  <button
                    key={key}
                    onClick={() => {
                      setDayFrom(daysAgoISO(days))
                      setDayTo(todayISO())
                    }}
                    className="px-3 py-1.5 rounded-md text-xs bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest transition-colors"
                  >
                    {t(`energy_pue.${key}`)}
                  </button>
                ))}
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Chart */}
      <section className="px-8 pb-4">
        <div className="bg-surface-container rounded-lg p-5">
          <div className="flex items-center justify-between mb-3">
            <h2 className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant">
              {chartLocId
                ? t('energy_pue.chart_title_location', { name: locationName(chartLocId) })
                : t('energy_pue.chart_title_no_location')}
            </h2>
            <div className="flex items-center gap-3 text-[0.6875rem] text-on-surface-variant">
              <span><span className="inline-block w-2 h-2 rounded-full bg-emerald-400 mr-1" />{t('energy_pue.legend_excellent')}</span>
              <span><span className="inline-block w-2 h-2 rounded-full bg-blue-400 mr-1" />{t('energy_pue.legend_good')}</span>
              <span><span className="inline-block w-2 h-2 rounded-full bg-amber-400 mr-1" />{t('energy_pue.legend_average')}</span>
              <span><span className="inline-block w-2 h-2 rounded-full bg-red-400 mr-1" />{t('energy_pue.legend_poor')}</span>
            </div>
          </div>
          {pueQ.isLoading ? (
            <div className="h-[200px] flex justify-center items-center">
              <div className="animate-spin rounded-full h-6 w-6 border-2 border-sky-400 border-t-transparent" />
            </div>
          ) : pueQ.error ? (
            <div className="rounded-lg bg-red-900/20 p-4 text-red-300 text-sm">
              {t('energy_pue.load_failed')}{' '}
              <button onClick={() => pueQ.refetch()} className="underline">{t('common.retry')}</button>
            </div>
          ) : (
            <PueLineChart points={chartSeries} />
          )}
        </div>
      </section>

      {/* Per-location latest card grid */}
      <section className="px-8 pb-8">
        <h2 className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant mb-3">
          {t('energy_pue.section_locations')}
        </h2>
        {byLocation.size === 0 ? (
          <div className="bg-surface-container rounded-lg p-10 text-center text-on-surface-variant text-sm">
            {t('energy_pue.empty_state')}
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {Array.from(byLocation.entries()).map(([locId, series]) => {
              // Most recent row for this location.
              const latest = series[series.length - 1]
              const pue = latest.pue != null ? Number(latest.pue) : null
              return (
                <div key={locId} className="bg-surface-container rounded-lg p-5">
                  <div className="flex items-baseline justify-between mb-2">
                    <h3 className="font-headline text-base font-bold text-on-surface truncate">
                      {latest.location_name ?? locationName(locId)}
                    </h3>
                    {latest.location_level && (
                      <span className="text-[0.6875rem] uppercase text-on-surface-variant">
                        {latest.location_level}
                      </span>
                    )}
                  </div>
                  <div className="flex items-baseline gap-2 mb-3">
                    <span className={`font-headline text-3xl font-bold ${pueColor(pue)}`}>
                      {pue != null ? fmt(pue, 2) : '—'}
                    </span>
                    <span className="text-xs text-on-surface-variant">PUE</span>
                  </div>
                  <p className="text-xs text-on-surface-variant mb-2">
                    {t('energy_pue.as_of')} {latest.day}
                  </p>
                  <div className="grid grid-cols-2 gap-2 text-xs">
                    <div>
                      <p className="text-on-surface-variant">{t('energy_pue.it_kwh')}</p>
                      <p className="font-mono text-on-surface">
                        {fmt(latest.it_kwh, 2)}{' '}
                        <span className="text-on-surface-variant">({latest.it_asset_count})</span>
                      </p>
                    </div>
                    <div>
                      <p className="text-on-surface-variant">{t('energy_pue.non_it_kwh')}</p>
                      <p className="font-mono text-on-surface">
                        {fmt(latest.non_it_kwh, 2)}{' '}
                        <span className="text-on-surface-variant">({latest.non_it_asset_count})</span>
                      </p>
                    </div>
                  </div>
                  <button
                    onClick={() => setLocationFilter(locId)}
                    className="mt-3 w-full text-xs text-primary hover:underline text-left"
                  >
                    {t('energy_pue.btn_focus_chart')}
                  </button>
                </div>
              )
            })}
          </div>
        )}
      </section>
    </div>
  )
}
