import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useMetrics } from '../../../hooks/useMetrics'
import { useAuditEvents } from '../../../hooks/useAudit'

const TIME_RANGE_MAP: Record<string, string> = {
  '24H': '24h',
  '1W': '7d',
  '30D': '30d',
}

const EM_DASH = '—'

type MetricPoint = { time: string; name: string; value: number }

function avg(points: MetricPoint[]): number | null {
  if (points.length === 0) return null
  const sum = points.reduce((acc, p) => acc + p.value, 0)
  return sum / points.length
}

function peak(points: MetricPoint[]): number | null {
  if (points.length === 0) return null
  return Math.max(...points.map((p) => p.value))
}

function formatPct(val: number | null): string {
  if (val == null) return EM_DASH
  return `${val.toFixed(1)}%`
}

function formatNumber(val: number | null, unit: string, digits = 1): string {
  if (val == null) return EM_DASH
  return `${val.toFixed(digits)} ${unit}`
}

function buildLinePath(
  pts: MetricPoint[],
  width: number,
  height: number,
  yMax: number,
): string {
  if (pts.length < 2) return ''
  const padX = 50
  const padY = 20
  const drawW = width - padX * 2
  const drawH = height - padY * 2
  return pts
    .map((p, i) => {
      const x = padX + (i / (pts.length - 1)) * drawW
      const y = padY + drawH - (Math.min(p.value, yMax) / yMax) * drawH
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
    })
    .join(' ')
}

function formatTickTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

export default function UsageTab({ assetId }: { assetId?: string }) {
  const { t } = useTranslation()
  const [activeRange, setActiveRange] = useState<string>('24H')
  const timeRange = TIME_RANGE_MAP[activeRange] ?? '24h'

  const cpuQuery = useMetrics({ asset_id: assetId, metric_name: 'cpu_usage', time_range: timeRange })
  const memQuery = useMetrics({ asset_id: assetId, metric_name: 'memory_usage', time_range: timeRange })
  const tempQuery = useMetrics({ asset_id: assetId, metric_name: 'temperature', time_range: timeRange })
  const powerQuery = useMetrics({ asset_id: assetId, metric_name: 'power_kw', time_range: timeRange })

  // Backend returns DESC order. Reverse to ASC for chart left-to-right plotting.
  const cpuPoints: MetricPoint[] = useMemo(
    () => [...(cpuQuery.data?.data ?? [])].reverse(),
    [cpuQuery.data],
  )
  const memPoints: MetricPoint[] = useMemo(
    () => [...(memQuery.data?.data ?? [])].reverse(),
    [memQuery.data],
  )
  const tempPoints: MetricPoint[] = tempQuery.data?.data ?? []
  const powerPoints: MetricPoint[] = powerQuery.data?.data ?? []

  // Operational events sourced from audit trail filtered to this asset.
  const auditQuery = useAuditEvents(
    assetId
      ? { target_type: 'asset', target_id: assetId, page_size: '5' }
      : undefined,
  )
  const operationalEvents = (auditQuery.data?.data ?? []).slice(0, 5)

  // Stats: average + peak from CPU; uptime is unmodeled in backend so render dash.
  const cpuAvg = avg(cpuPoints)
  const cpuPeak = peak(cpuPoints)

  const chartW = 720
  const chartH = 280
  const padX = 50
  const padY = 20
  const drawW = chartW - padX * 2
  const drawH = chartH - padY * 2

  const isChartLoading = cpuQuery.isLoading || memQuery.isLoading
  const hasChartData = cpuPoints.length >= 2 || memPoints.length >= 2

  // Memory chart scale: cap at observed peak rounded up, fall back to 100%.
  const memMax = memPoints.length > 0 ? Math.max(100, Math.max(...memPoints.map((p) => p.value))) : 100

  return (
    <div className="space-y-4">
      {/* Stats Row */}
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <div className="rounded-lg bg-surface-container p-5">
          <p className="text-xs uppercase tracking-wider text-on-surface-variant">{t('component_usage.stat_average_usage')}</p>
          <p className="mt-2 font-headline text-3xl font-bold text-on-surface">{formatPct(cpuAvg)}</p>
        </div>
        <div className="rounded-lg bg-surface-container p-5">
          <p className="text-xs uppercase tracking-wider text-on-surface-variant">{t('component_usage.stat_peak_usage')}</p>
          <p className="mt-2 font-headline text-3xl font-bold text-on-surface">{formatPct(cpuPeak)}</p>
          {cpuPeak != null && cpuPeak >= 90 && (
            <span className="mt-1 inline-block rounded bg-error-container px-2 py-0.5 text-xs font-semibold text-on-error-container">
              {t('common.critical')}
            </span>
          )}
        </div>
        <div className="rounded-lg bg-surface-container p-5">
          <p className="text-xs uppercase tracking-wider text-on-surface-variant">{t('component_usage.stat_system_uptime')}</p>
          <p className="mt-2 font-headline text-3xl font-bold text-on-surface">{EM_DASH}</p>
          <p className="mt-1 text-xs text-on-surface-variant">{t('component_usage.uptime_unavailable')}</p>
        </div>
      </div>

      {/* Telemetry Chart */}
      <div className="rounded-lg bg-surface-container p-5">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="font-headline text-lg font-bold text-on-surface">{t('component_usage.section_telemetry_history')}</h2>
            <p className="text-xs text-on-surface-variant">{t('component_usage.telemetry_subtitle')}</p>
          </div>
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2 text-xs text-on-surface-variant">
              <span className="inline-block h-2 w-2 rounded-full bg-primary" /> {t('component_usage.legend_cpu')}
              <span className="ml-2 inline-block h-2 w-2 rounded-full bg-tertiary" /> {t('component_usage.legend_memory')}
            </div>
            <div className="flex gap-1">
              {Object.keys(TIME_RANGE_MAP).map((r) => (
                <button
                  key={r}
                  onClick={() => setActiveRange(r)}
                  className={`rounded px-3 py-1 text-xs font-medium ${
                    activeRange === r
                      ? 'bg-on-primary-container text-white'
                      : 'bg-surface-container-low text-on-surface-variant hover:bg-surface-container-high'
                  }`}
                >
                  {r}
                </button>
              ))}
            </div>
          </div>
        </div>
        <svg viewBox={`0 0 ${chartW} ${chartH}`} className="w-full">
          {[0, 25, 50, 75, 100].map((v) => {
            const y = padY + drawH - (v / 100) * drawH
            return (
              <g key={v}>
                <line x1={padX} y1={y} x2={chartW - padX} y2={y} stroke="#202b32" strokeWidth="1" />
                <text x={padX - 8} y={y + 4} textAnchor="end" fill="#8e9196" fontSize="10">{v}%</text>
              </g>
            )
          })}
          {hasChartData ? (
            <>
              {cpuPoints.map((d, i) => {
                if (cpuPoints.length < 2) return null
                const x = padX + (i / (cpuPoints.length - 1)) * drawW
                // Show ~6 ticks max to avoid crowding
                const stride = Math.max(1, Math.floor(cpuPoints.length / 6))
                if (i % stride !== 0) return null
                return (
                  <text key={`tick-${i}`} x={x} y={chartH - 2} textAnchor="middle" fill="#8e9196" fontSize="10">
                    {formatTickTime(d.time)}
                  </text>
                )
              })}
              <path d={buildLinePath(cpuPoints, chartW, chartH, 100)} fill="none" stroke="#9ecaff" strokeWidth="2.5" />
              <path
                d={buildLinePath(memPoints, chartW, chartH, memMax)}
                fill="none"
                stroke="#ffb5a0"
                strokeWidth="2.5"
                strokeDasharray="6 3"
              />
              {cpuPoints.map((d, i) => {
                if (cpuPoints.length < 2) return null
                const x = padX + (i / (cpuPoints.length - 1)) * drawW
                const y = padY + drawH - (Math.min(d.value, 100) / 100) * drawH
                return <circle key={`cpu-${i}`} cx={x} cy={y} r="3" fill="#9ecaff" />
              })}
            </>
          ) : (
            <text x={chartW / 2} y={chartH / 2} textAnchor="middle" fill="#8e9196" fontSize="14">
              {isChartLoading ? t('common.loading') : t('component_usage.no_telemetry_data')}
            </text>
          )}
        </svg>
      </div>

      {/* Bottom Section */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div className="rounded-lg bg-surface-container p-5">
          <h3 className="mb-1 font-headline text-sm font-bold uppercase tracking-wider text-on-surface-variant">
            {t('component_usage.section_operational_events')}
          </h3>
          <div className="mt-4 space-y-3">
            {operationalEvents.length === 0 ? (
              <div className="rounded bg-surface-container-low px-4 py-6 text-center text-sm text-on-surface-variant">
                {auditQuery.isLoading ? t('common.loading') : t('component_usage.no_recent_events')}
              </div>
            ) : (
              operationalEvents.map((ev) => (
                <div key={ev.id} className="flex items-center justify-between rounded bg-surface-container-low px-4 py-3">
                  <div>
                    <p className="text-sm font-medium text-on-surface">{ev.action}</p>
                    <p className="text-xs text-on-surface-variant">
                      {new Date(ev.created_at).toLocaleString()}
                    </p>
                  </div>
                  <span className="text-xs font-semibold uppercase text-on-surface-variant">{ev.module}</span>
                </div>
              ))
            )}
          </div>
        </div>
        <div className="rounded-lg bg-surface-container p-5">
          <h3 className="mb-1 font-headline text-sm font-bold uppercase tracking-wider text-on-surface-variant">
            {t('component_usage.section_component_integrity')}
          </h3>
          <div className="mt-4 space-y-3">
            <IntegrityRow label={t('component_usage.integrity_cpu_avg')} value={formatPct(cpuAvg)} />
            <IntegrityRow label={t('component_usage.integrity_memory_avg')} value={formatPct(avg(memPoints))} />
            <IntegrityRow label={t('component_usage.integrity_temperature_avg')} value={formatNumber(avg(tempPoints), '°C')} />
            <IntegrityRow label={t('component_usage.integrity_power_avg')} value={formatNumber(avg(powerPoints), 'kW', 2)} />
          </div>
        </div>
      </div>
    </div>
  )
}

function IntegrityRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between rounded bg-surface-container-low px-4 py-3">
      <span className="text-sm text-on-surface-variant">{label}</span>
      <span className="text-sm font-semibold text-on-surface">{value}</span>
    </div>
  )
}
