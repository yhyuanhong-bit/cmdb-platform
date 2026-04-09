import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import Icon from '../../../components/Icon'
import { useMetrics } from '../../../hooks/useMetrics'

const hardwareItems = [
  { label: 'CPU', spec: 'AMD EPYC 7763 (64C/128T)', usage: 24, icon: 'memory' },
  { label: 'RAM', spec: '512GB ECC DDR4-3200', usage: 89, icon: 'developer_board' },
  { label: 'NVMe Storage', spec: '3.8 TB Samsung PM9A3', usage: 12, icon: 'hard_drive' },
]

const recentEvents = [
  { icon: 'system_update', title: 'FIRMWARE UPDATED', description: 'BMC firmware updated to v2.14.1', date: 'Jun 14, 2023', type: 'info' },
  { icon: 'thermostat', title: 'THERMAL WARNING', description: 'CPU zone exceeded 72 C threshold for 3 min', date: 'Jun 21, 2023', type: 'warning' },
  { icon: 'event_available', title: 'SCHEDULED HEALTH CHECK', description: 'Automated diagnostics passed all checks', date: 'Automated', type: 'success' },
]

const eventTypeColors: Record<string, string> = {
  info: 'text-primary',
  warning: 'text-[#fbbf24]',
  success: 'text-[#34d399]',
}

function usageColor(percent: number): string {
  if (percent >= 80) return 'bg-error'
  if (percent >= 60) return 'bg-[#fbbf24]'
  return 'bg-primary'
}

export default function HealthTab({ assetId }: { assetId?: string }) {
  const { t } = useTranslation()
  const [activeRange, setActiveRange] = useState<string>('24H')

  const timeRangeMap: Record<string, string> = { '1H': '1h', '6H': '6h', '24H': '24h', '7D': '7d' }
  const timeRange = timeRangeMap[activeRange] ?? '24h'

  const tempMetrics = useMetrics({ asset_id: assetId, metric_name: 'temperature', time_range: timeRange })
  const powerMetrics = useMetrics({ asset_id: assetId, metric_name: 'power_kw', time_range: timeRange })

  const tempPoints = tempMetrics.data?.data ?? []
  const powerPoints = powerMetrics.data?.data ?? []

  const latestTemp = tempPoints.length > 0 ? tempPoints[tempPoints.length - 1].value.toFixed(1) : null
  const latestPowerW = powerPoints.length > 0 ? (powerPoints[powerPoints.length - 1].value * 1000).toFixed(0) : null

  function buildPath(pts: { value: number }[], w: number, h: number, padX = 0, padY = 10, maxVal?: number): string {
    if (pts.length < 2) return ''
    const vMax = maxVal ?? Math.max(...pts.map((p) => p.value), 1)
    const vMin = Math.min(...pts.map((p) => p.value), 0)
    const range = vMax - vMin || 1
    return pts
      .map((p, i) => {
        const x = padX + (i / (pts.length - 1)) * (w - padX * 2)
        const y = padY + (1 - (p.value - vMin) / range) * (h - padY * 2)
        return `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`
      })
      .join(' ')
  }

  return (
    <div className="space-y-4">
      {/* Time range */}
      <div className="flex items-center justify-between">
        <span className="text-sm text-on-surface-variant font-semibold uppercase tracking-wider">Time Range</span>
        <div className="flex bg-surface-container-low rounded overflow-hidden">
          {['1H', '6H', '24H', '7D'].map((range) => (
            <button
              key={range}
              onClick={() => setActiveRange(range)}
              className={`px-4 py-2 text-xs font-semibold tracking-wider transition-colors ${
                activeRange === range
                  ? 'bg-on-primary-container text-white'
                  : 'text-on-surface-variant hover:bg-surface-container-high'
              }`}
            >
              {range}
            </button>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        {/* Temperature Trend */}
        <div className="lg:col-span-2 bg-surface-container rounded p-5">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <Icon name="thermostat" className="text-[20px] text-primary" />
              <h2 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
                {t('asset_health.temperature_trend')}
              </h2>
            </div>
            <div className="text-right">
              <span className="font-headline text-3xl font-bold text-on-surface">
                {latestTemp ?? (tempMetrics.isLoading ? '\u2026' : '\u2014')}
              </span>
              <span className="ml-1 text-sm text-on-surface-variant">{'\u00b0'}C</span>
            </div>
          </div>
          <svg viewBox="0 0 600 120" className="w-full h-32" preserveAspectRatio="none">
            <defs>
              <linearGradient id="tempGradUnified" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="#9ecaff" stopOpacity="0.3" />
                <stop offset="100%" stopColor="#9ecaff" stopOpacity="0" />
              </linearGradient>
            </defs>
            {tempPoints.length >= 2 ? (
              <>
                <path d={buildPath(tempPoints, 600, 120)} fill="none" stroke="#9ecaff" strokeWidth="2" />
                <path d={`${buildPath(tempPoints, 600, 120)} L600,120 L0,120 Z`} fill="url(#tempGradUnified)" />
              </>
            ) : (
              <text x="300" y="65" textAnchor="middle" fill="#8e9196" fontSize="12">
                {tempMetrics.isLoading ? 'Loading\u2026' : 'No data'}
              </text>
            )}
            <line x1="0" y1="25" x2="600" y2="25" stroke="#ff6b6b" strokeWidth="1" strokeDasharray="6,4" opacity="0.5" />
            <text x="560" y="20" fill="#ff6b6b" fontSize="10" opacity="0.7">72{'\u00b0'}C</text>
          </svg>
        </div>

        {/* Hardware Health Panel */}
        <div className="bg-surface-container rounded p-5">
          <div className="flex items-center gap-2 mb-5">
            <Icon name="developer_board" className="text-[20px] text-primary" />
            <h2 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
              {t('asset_health.hardware_health')}
            </h2>
          </div>
          <div className="space-y-5">
            {hardwareItems.map((item) => (
              <div key={item.label}>
                <div className="flex items-center justify-between mb-1.5">
                  <div className="flex items-center gap-2">
                    <Icon name={item.icon} className="text-[16px] text-on-surface-variant" />
                    <span className="text-sm font-semibold text-on-surface">{item.label}</span>
                  </div>
                  <span className="text-sm font-mono font-bold text-on-surface">{item.usage}%</span>
                </div>
                <p className="text-[0.6875rem] text-on-surface-variant mb-2">{item.spec}</p>
                <div className="h-2 w-full rounded-full bg-surface-container-lowest overflow-hidden">
                  <div className={`h-full rounded-full transition-all ${usageColor(item.usage)}`} style={{ width: `${item.usage}%` }} />
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Vibration */}
        <div className="bg-surface-container rounded p-5">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <Icon name="vibration" className="text-[20px] text-primary" />
              <h2 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
                {t('asset_health.vibration_intensity')}
              </h2>
            </div>
            <div className="text-right">
              <span className="font-headline text-2xl font-bold text-on-surface">1.2</span>
              <span className="ml-1 text-xs text-on-surface-variant">mm/s</span>
            </div>
          </div>
          <div className="flex items-end gap-1 h-20">
            {[35, 42, 38, 55, 48, 40, 36, 50, 45, 38, 42, 36, 30, 44, 48, 38, 34, 40, 46, 42].map((h, i) => (
              <div key={i} className="flex-1 rounded-t bg-primary/60 hover:bg-primary transition-colors" style={{ height: `${h}%` }} />
            ))}
          </div>
        </div>

        {/* Power Draw */}
        <div className="bg-surface-container rounded p-5">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <Icon name="bolt" className="text-[20px] text-[#fbbf24]" />
              <h2 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
                {t('asset_health.power_draw')}
              </h2>
            </div>
            <div className="text-right">
              <span className="font-headline text-2xl font-bold text-on-surface">
                {latestPowerW ?? (powerMetrics.isLoading ? '\u2026' : '\u2014')}
              </span>
              <span className="ml-1 text-xs text-on-surface-variant">{t('asset_health.w_avg')}</span>
            </div>
          </div>
          <svg viewBox="0 0 300 60" className="w-full h-16" preserveAspectRatio="none">
            <defs>
              <linearGradient id="powerGradUnified" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="#fbbf24" stopOpacity="0.2" />
                <stop offset="100%" stopColor="#fbbf24" stopOpacity="0" />
              </linearGradient>
            </defs>
            {powerPoints.length >= 2 ? (
              <>
                <path d={buildPath(powerPoints, 300, 60, 0, 5)} fill="none" stroke="#fbbf24" strokeWidth="2" />
                <path d={`${buildPath(powerPoints, 300, 60, 0, 5)} L300,60 L0,60 Z`} fill="url(#powerGradUnified)" />
              </>
            ) : (
              <text x="150" y="35" textAnchor="middle" fill="#8e9196" fontSize="10">
                {powerMetrics.isLoading ? 'Loading\u2026' : 'No data'}
              </text>
            )}
          </svg>
        </div>

        {/* Recent Events */}
        <div className="bg-surface-container rounded p-5">
          <div className="flex items-center gap-2 mb-4">
            <Icon name="history" className="text-[20px] text-primary" />
            <h2 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
              {t('asset_health.recent_events')}
            </h2>
          </div>
          <div className="space-y-4">
            {recentEvents.map((event, i) => (
              <div key={i} className="flex gap-3">
                <div className="flex-shrink-0 mt-0.5">
                  <Icon name={event.icon} className={`text-[20px] ${eventTypeColors[event.type]}`} />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-xs font-semibold uppercase tracking-wider text-on-surface">{event.title}</span>
                    <span className="text-[0.625rem] text-on-surface-variant whitespace-nowrap">{event.date}</span>
                  </div>
                  <p className="text-[0.6875rem] text-on-surface-variant mt-0.5">{event.description}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
