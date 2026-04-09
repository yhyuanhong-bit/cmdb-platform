import { useState } from 'react'
import { useTranslation } from 'react-i18next'

const cpuData = [
  { time: '03:00', cpu: 32, mem: 4.1 },
  { time: '05:00', cpu: 28, mem: 3.8 },
  { time: '07:00', cpu: 45, mem: 5.2 },
  { time: '09:00', cpu: 62, mem: 6.8 },
  { time: '11:00', cpu: 78, mem: 7.4 },
  { time: '13:00', cpu: 89, mem: 8.1 },
  { time: '14:00', cpu: 72, mem: 7.9 },
  { time: '15:00', cpu: 58, mem: 6.5 },
  { time: '17:00', cpu: 65, mem: 7.0 },
  { time: '19:00', cpu: 48, mem: 5.8 },
  { time: '21:00', cpu: 35, mem: 4.5 },
  { time: '23:00', cpu: 30, mem: 4.0 },
]

const operationalEvents = [
  { event: 'Auto-Scale Triggered', time: '14:34:18', tag: 'COMPUTE', tagColor: 'text-yellow-400' },
  { event: 'Heat Sink Threshold Alert', time: '13:45:11', tag: 'RESOLVED', tagColor: 'text-primary' },
  { event: 'Inventory Snapshot Taken', time: '12:09:35', tag: 'STORAGE', tagColor: 'text-on-surface-variant' },
]

const integrityItems = [
  { label: 'CPU Core Health', value: 'OPTIMAL', color: 'text-green-400' },
  { label: 'Disk I/O Latency', value: '4ms', color: 'text-on-surface' },
  { label: 'Ambient Temp', value: '42\u00b0C', color: 'text-orange-400' },
  { label: 'Network Throughput', value: '1.2 Gbps', color: 'text-on-surface' },
]

function buildLinePath(data: typeof cpuData, key: 'cpu' | 'mem', width: number, height: number): string {
  const padX = 50
  const padY = 20
  const drawW = width - padX * 2
  const drawH = height - padY * 2
  return data
    .map((d, i) => {
      const x = padX + (i / (data.length - 1)) * drawW
      const val = key === 'cpu' ? d.cpu : d.mem
      const yMax = key === 'cpu' ? 100 : 10
      const y = padY + drawH - (val / yMax) * drawH
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
    })
    .join(' ')
}

export default function UsageTab() {
  const { t } = useTranslation()
  const [activeRange, setActiveRange] = useState('24H')
  const chartW = 720
  const chartH = 280
  const padX = 50
  const padY = 20
  const drawW = chartW - padX * 2
  const drawH = chartH - padY * 2

  return (
    <div className="space-y-4">
      {/* Stats Row */}
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <div className="rounded-lg bg-surface-container p-5">
          <p className="text-xs uppercase tracking-wider text-on-surface-variant">{t('component_usage.stat_average_usage')}</p>
          <p className="mt-2 font-headline text-3xl font-bold text-on-surface">42.8%</p>
          <p className="mt-1 text-sm text-[#34d399]">+2.4% &#9650;</p>
        </div>
        <div className="rounded-lg bg-surface-container p-5">
          <p className="text-xs uppercase tracking-wider text-on-surface-variant">{t('component_usage.stat_peak_usage')}</p>
          <p className="mt-2 font-headline text-3xl font-bold text-on-surface">89.2%</p>
          <span className="mt-1 inline-block rounded bg-error-container px-2 py-0.5 text-xs font-semibold text-on-error-container">
            {t('common.critical')}
          </span>
        </div>
        <div className="rounded-lg bg-surface-container p-5">
          <p className="text-xs uppercase tracking-wider text-on-surface-variant">{t('component_usage.stat_system_uptime')}</p>
          <p className="mt-2 font-headline text-3xl font-bold text-on-surface">99.99%</p>
          <p className="mt-1 text-sm text-[#34d399]">{t('component_usage.stat_excellent')}</p>
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
              {['24H', '1W', '30D'].map((r) => (
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
          {cpuData.map((d, i) => {
            const x = padX + (i / (cpuData.length - 1)) * drawW
            return <text key={d.time} x={x} y={chartH - 2} textAnchor="middle" fill="#8e9196" fontSize="10">{d.time}</text>
          })}
          <path d={buildLinePath(cpuData, 'cpu', chartW, chartH)} fill="none" stroke="#9ecaff" strokeWidth="2.5" />
          <path d={buildLinePath(cpuData, 'mem', chartW, chartH)} fill="none" stroke="#ffb5a0" strokeWidth="2.5" strokeDasharray="6 3" />
          {cpuData.map((d, i) => {
            const x = padX + (i / (cpuData.length - 1)) * drawW
            const y = padY + drawH - (d.cpu / 100) * drawH
            return <circle key={i} cx={x} cy={y} r="3" fill="#9ecaff" />
          })}
        </svg>
      </div>

      {/* Bottom Section */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div className="rounded-lg bg-surface-container p-5">
          <h3 className="mb-1 font-headline text-sm font-bold uppercase tracking-wider text-on-surface-variant">
            {t('component_usage.section_operational_events')}
          </h3>
          <div className="mt-4 space-y-3">
            {operationalEvents.map((ev, i) => (
              <div key={i} className="flex items-center justify-between rounded bg-surface-container-low px-4 py-3">
                <div>
                  <p className="text-sm font-medium text-on-surface">{ev.event}</p>
                  <p className="text-xs text-on-surface-variant">{ev.time}</p>
                </div>
                <span className={`text-xs font-semibold ${ev.tagColor}`}>{ev.tag}</span>
              </div>
            ))}
          </div>
        </div>
        <div className="rounded-lg bg-surface-container p-5">
          <h3 className="mb-1 font-headline text-sm font-bold uppercase tracking-wider text-on-surface-variant">
            {t('component_usage.section_component_integrity')}
          </h3>
          <div className="mt-4 space-y-3">
            {integrityItems.map((item, i) => (
              <div key={i} className="flex items-center justify-between rounded bg-surface-container-low px-4 py-3">
                <span className="text-sm text-on-surface-variant">{item.label}</span>
                <span className={`text-sm font-semibold ${item.color}`}>{item.value}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
