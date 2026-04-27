import { useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import Icon from '../components/Icon'
import { usePredictiveRefresh } from '../hooks/usePredictiveRefresh'
import type { PredictiveRefresh, PredictiveRefreshKind } from '../lib/api/predictiveRefresh'

/* ------------------------------------------------------------------ */
/*  Capex backlog: groups open recommendations by target_date month    */
/*  so capex planners see when refreshes pile up.                      */
/* ------------------------------------------------------------------ */

interface MonthBucket {
  month: string // YYYY-MM
  total: number
  byKind: Record<PredictiveRefreshKind, number>
  topAssets: PredictiveRefresh[] // up to 3 highest-score for the tooltip
}

const kindOrder: PredictiveRefreshKind[] = [
  'warranty_expired',
  'warranty_expiring',
  'eol_passed',
  'eol_approaching',
  'aged_out',
]

const kindFill: Record<PredictiveRefreshKind, string> = {
  warranty_expired:  '#f87171',
  warranty_expiring: '#fbbf24',
  eol_passed:        '#dc2626',
  eol_approaching:   '#fb923c',
  aged_out:          '#a78bfa',
}

function bucketByMonth(rows: PredictiveRefresh[]): MonthBucket[] {
  const map = new Map<string, MonthBucket>()
  for (const r of rows) {
    if (!r.target_date) continue
    const month = r.target_date.slice(0, 7)
    let b = map.get(month)
    if (!b) {
      b = {
        month,
        total: 0,
        byKind: {
          warranty_expiring: 0,
          warranty_expired:  0,
          eol_approaching:   0,
          eol_passed:        0,
          aged_out:          0,
        },
        topAssets: [],
      }
      map.set(month, b)
    }
    b.total += 1
    b.byKind[r.kind] = (b.byKind[r.kind] ?? 0) + 1
    b.topAssets.push(r)
  }
  // Sort topAssets by score descending and trim.
  for (const b of map.values()) {
    b.topAssets.sort((a, b2) => Number(b2.risk_score) - Number(a.risk_score))
    b.topAssets = b.topAssets.slice(0, 3)
  }
  return Array.from(map.values()).sort((a, b) => a.month.localeCompare(b.month))
}

/* ------------------------------------------------------------------ */
/*  Inline SVG bar chart                                               */
/* ------------------------------------------------------------------ */

function CapexChart({
  buckets,
  onPick,
}: {
  buckets: MonthBucket[]
  onPick: (month: string) => void
}) {
  const { t } = useTranslation()
  const width = 800
  const height = 260
  const padding = { top: 16, right: 16, bottom: 36, left: 36 }
  const inner = {
    w: width - padding.left - padding.right,
    h: height - padding.top - padding.bottom,
  }

  if (buckets.length === 0) {
    return (
      <div className="flex items-center justify-center h-[200px] text-on-surface-variant text-sm italic">
        {t('predictive_capex.chart_empty')}
      </div>
    )
  }

  const max = Math.max(...buckets.map((b) => b.total), 1)
  const barWidth = inner.w / buckets.length
  const barInner = barWidth * 0.7
  const barGap = barWidth * 0.15

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="w-full h-auto" role="img" aria-label="Capex backlog by month">
      {/* Y gridlines */}
      {[0, 0.25, 0.5, 0.75, 1].map((frac) => {
        const y = padding.top + (1 - frac) * inner.h
        return (
          <g key={frac}>
            <line
              x1={padding.left}
              x2={padding.left + inner.w}
              y1={y}
              y2={y}
              stroke="#202b32"
              strokeWidth="1"
              strokeDasharray={frac === 0 ? '0' : '2 2'}
            />
            <text x={4} y={y + 3} fill="#8e9196" fontSize="10" fontFamily="Inter">
              {Math.round(max * frac)}
            </text>
          </g>
        )
      })}

      {/* Stacked bars */}
      {buckets.map((b, i) => {
        const x = padding.left + i * barWidth + barGap
        let cursor = padding.top + inner.h
        return (
          <g key={b.month} role="button" onClick={() => onPick(b.month)} className="cursor-pointer">
            {/* Hit-target rect that covers the slot */}
            <rect
              x={x - barGap / 2}
              y={padding.top}
              width={barWidth}
              height={inner.h}
              fill="transparent"
            />
            {kindOrder.map((kind) => {
              const count = b.byKind[kind] ?? 0
              if (count === 0) return null
              const segH = (count / max) * inner.h
              cursor -= segH
              return (
                <rect
                  key={kind}
                  x={x}
                  y={cursor}
                  width={barInner}
                  height={segH}
                  fill={kindFill[kind]}
                  opacity={0.85}
                >
                  <title>{`${b.month} · ${kind}: ${count}`}</title>
                </rect>
              )
            })}
            {/* Total label above the bar */}
            <text
              x={x + barInner / 2}
              y={padding.top + inner.h - (b.total / max) * inner.h - 4}
              fill="#e6e8eb"
              fontSize="10"
              fontFamily="Inter"
              textAnchor="middle"
            >
              {b.total}
            </text>
            {/* X axis tick */}
            <text
              x={x + barInner / 2}
              y={height - 12}
              fill="#8e9196"
              fontSize="10"
              fontFamily="Inter"
              textAnchor="middle"
            >
              {b.month}
            </text>
          </g>
        )
      })}
    </svg>
  )
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function PredictiveCapex() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  // Pull all open recommendations (default page size 200 from the
  // hook). For larger backlogs the future server filter would
  // enumerate per-month counts directly; today this is fine for the
  // expected scale.
  const listQ = usePredictiveRefresh({ status: 'open', page_size: 1000 })
  const rows: PredictiveRefresh[] = listQ.data?.data ?? []
  const buckets = useMemo(() => bucketByMonth(rows), [rows])

  // Aggregate counters for the summary strip.
  const totals = useMemo(() => {
    const byKind = {
      warranty_expiring: 0,
      warranty_expired:  0,
      eol_approaching:   0,
      eol_passed:        0,
      aged_out:          0,
    } as Record<PredictiveRefreshKind, number>
    for (const r of rows) byKind[r.kind] += 1
    return byKind
  }, [rows])

  const onPickMonth = (month: string) => {
    navigate(`/predictive/refresh?status=open&month=${month}`)
  }

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      <header className="px-8 pt-6 pb-4">
        <nav className="flex items-center gap-1.5 text-xs text-on-surface-variant mb-3">
          <span className="hover:text-primary cursor-pointer" onClick={() => navigate('/predictive')}>
            {t('predictive_capex.breadcrumb_predictive')}
          </span>
          <Icon name="chevron_right" className="text-[14px] text-on-surface-variant" />
          <span className="text-primary">{t('predictive_capex.title')}</span>
        </nav>

        <div className="flex items-start justify-between flex-wrap gap-4">
          <div>
            <h1 className="font-headline font-bold text-2xl text-on-surface">
              {t('predictive_capex.title')}
            </h1>
            <p className="text-sm text-on-surface-variant mt-1">{t('predictive_capex.subtitle')}</p>
          </div>
          <button
            onClick={() => navigate('/predictive/refresh')}
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-surface-container-high text-on-surface text-sm font-semibold hover:bg-surface-container-highest transition-colors"
          >
            <Icon name="list_alt" className="text-[18px]" />
            {t('predictive_capex.btn_view_queue')}
          </button>
        </div>
      </header>

      {/* Summary strip */}
      <section className="px-8 pb-4">
        <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
          {kindOrder.map((kind) => (
            <div key={kind} className="bg-surface-container rounded-lg p-4">
              <div className="flex items-center gap-2 mb-1">
                <span className="inline-block w-2 h-2 rounded-full" style={{ background: kindFill[kind] }} />
                <span className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant truncate">
                  {kind}
                </span>
              </div>
              <p className="font-headline text-2xl font-bold text-on-surface">{totals[kind]}</p>
            </div>
          ))}
        </div>
      </section>

      {/* Chart */}
      <section className="px-8 pb-4">
        <div className="bg-surface-container rounded-lg p-5">
          <div className="flex items-center justify-between mb-3">
            <h2 className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant">
              {t('predictive_capex.section_chart')}
            </h2>
            <span className="text-[0.6875rem] text-on-surface-variant italic">
              {t('predictive_capex.click_hint')}
            </span>
          </div>
          {listQ.isLoading ? (
            <div className="h-[260px] flex justify-center items-center">
              <div className="animate-spin rounded-full h-6 w-6 border-2 border-sky-400 border-t-transparent" />
            </div>
          ) : listQ.error ? (
            <div className="rounded-lg bg-red-900/20 p-4 text-red-300 text-sm">
              {t('predictive_capex.load_failed')}{' '}
              <button onClick={() => listQ.refetch()} className="underline">{t('common.retry')}</button>
            </div>
          ) : (
            <CapexChart buckets={buckets} onPick={onPickMonth} />
          )}
        </div>
      </section>

      {/* Per-month roll-up table — denser than the chart, useful for
          screen readers and data export. */}
      <section className="px-8 pb-8">
        <div className="bg-surface-container rounded-lg overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-surface-container-high text-on-surface-variant text-[0.6875rem] uppercase tracking-wider">
                <th className="px-4 py-3 text-left font-semibold">{t('predictive_capex.col_month')}</th>
                <th className="px-4 py-3 text-right font-semibold">{t('predictive_capex.col_total')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('predictive_capex.col_breakdown')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('predictive_capex.col_top')}</th>
              </tr>
            </thead>
            <tbody>
              {buckets.length === 0 && !listQ.isLoading && (
                <tr><td colSpan={4} className="py-10 text-center text-on-surface-variant text-sm">
                  {t('predictive_capex.empty_state')}
                </td></tr>
              )}
              {buckets.map((b) => (
                <tr key={b.month} className="border-t border-surface-container-high">
                  <td className="px-4 py-3 font-mono">
                    <button
                      onClick={() => onPickMonth(b.month)}
                      className="text-primary hover:underline text-left"
                    >
                      {b.month}
                    </button>
                  </td>
                  <td className="px-4 py-3 text-right font-mono font-semibold">{b.total}</td>
                  <td className="px-4 py-3">
                    <div className="flex flex-wrap items-center gap-1.5">
                      {kindOrder.map((kind) => {
                        const c = b.byKind[kind] ?? 0
                        if (c === 0) return null
                        return (
                          <span key={kind} className="inline-flex items-center gap-1 text-[0.6875rem]">
                            <span
                              className="inline-block w-2 h-2 rounded-full"
                              style={{ background: kindFill[kind] }}
                            />
                            <span className="text-on-surface-variant">{kind}</span>
                            <span className="font-mono">{c}</span>
                          </span>
                        )
                      })}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <ul className="text-xs text-on-surface-variant">
                      {b.topAssets.map((a) => (
                        <li key={a.asset_id} className="truncate">
                          <button
                            onClick={() => navigate(`/assets/${a.asset_id}`)}
                            className="text-primary hover:underline"
                          >
                            {a.asset_name ?? a.asset_id.slice(0, 8) + '…'}
                          </button>
                          <span className="font-mono text-on-surface-variant"> · {Number(a.risk_score).toFixed(0)}</span>
                        </li>
                      ))}
                    </ul>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  )
}
