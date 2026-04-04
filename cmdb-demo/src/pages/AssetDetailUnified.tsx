import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import Icon from '../components/Icon'
import StatusBadge from '../components/StatusBadge'
import { useAsset } from '../hooks/useAssets'

/* ================================================================== */
/*  SHARED DATA & HELPERS                                              */
/* ================================================================== */

// TODO: needs backend endpoints for warranty, uptime, financial, network detail
const assetDefaults = {
  description: '',
  tags: [] as string[],
  warranty: { status: 'Unknown', expiry: '-' },
  uptime: { days: 0, hours: 0, minutes: 0 },
  mtbf: '-',
  cpu: '-',
  memory: '-',
  formFactor: '-',
  storage: '-',
  network: '-',
  os: '-',
  facility: '-',
  room: '-',
  rackId: '-',
  uPosition: '-',
  inventoryNo: '-',
  poRef: '-',
  purchaseDate: '-',
  cost: '-',
  depreciation: '-',
  bookValue: '-',
  primaryIp: '-',
  mgmtIp: '-',
  domain: '-',
  vlan: '-',
}

const tabs = [
  { key: 'overview', label: '\u6982\u89bd', icon: 'dashboard' },
  { key: 'health', label: '\u5065\u5eb7\u76e3\u63a7', icon: 'monitor_heart' },
  { key: 'usage', label: '\u4f7f\u7528\u5206\u6790', icon: 'analytics' },
  { key: 'maintenance', label: '\u7dad\u8b77\u6b77\u53f2', icon: 'build' },
] as const

type TabKey = (typeof tabs)[number]['key']

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <h3 className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant mb-3">
      {children}
    </h3>
  )
}

function DataRow({
  label,
  value,
  mono,
  valueColor,
}: {
  label: string
  value: React.ReactNode
  mono?: boolean
  valueColor?: string
}) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">{label}</span>
      <span className={`text-sm ${valueColor ?? 'text-on-surface'} ${mono ? 'font-mono' : 'font-body'}`}>{value}</span>
    </div>
  )
}

function toSvgPath(
  points: { t: number; value: number }[],
  width: number,
  height: number,
  padding = 16,
): string {
  const maxT = Math.max(...points.map((p) => p.t))
  const maxV = 100
  const xScale = (width - padding * 2) / (maxT || 1)
  const yScale = (height - padding * 2) / maxV
  return points
    .map((p, i) => {
      const x = padding + p.t * xScale
      const y = height - padding - p.value * yScale
      return `${i === 0 ? 'M' : 'L'}${x},${y}`
    })
    .join(' ')
}

/* ================================================================== */
/*  TAB 1: OVERVIEW                                                    */
/* ================================================================== */

const telemetryPoints = [
  { t: 0, cpu: 38, mem: 52 },
  { t: 1, cpu: 42, mem: 55 },
  { t: 2, cpu: 35, mem: 54 },
  { t: 3, cpu: 50, mem: 58 },
  { t: 4, cpu: 44, mem: 60 },
  { t: 5, cpu: 39, mem: 57 },
  { t: 6, cpu: 46, mem: 62 },
  { t: 7, cpu: 41, mem: 59 },
  { t: 8, cpu: 48, mem: 61 },
  { t: 9, cpu: 42, mem: 59 },
]

function RackIllustration() {
  return (
    <div className="bg-surface-container-low rounded-lg p-4 flex items-center justify-center min-h-[140px]">
      <svg width="72" height="112" viewBox="0 0 72 112" fill="none" role="img" aria-label="Rack illustration">
        <rect x="8" y="4" width="56" height="104" rx="3" stroke="#44474c" strokeWidth="1.5" fill="none" />
        {Array.from({ length: 10 }).map((_, i) => {
          const y = 10 + i * 10
          const isHighlighted = i === 6 || i === 7
          return <rect key={i} x="14" y={y} width="44" height="8" rx="1" fill={isHighlighted ? '#0087df' : '#202b32'} opacity={isHighlighted ? 0.8 : 1} />
        })}
        <text x="64" y="80" fill="#9ecaff" fontSize="6" fontFamily="Inter" textAnchor="end">U14</text>
      </svg>
    </div>
  )
}

function OverviewTab({ t, navigate, asset }: { t: ReturnType<typeof useTranslation>['t']; navigate: ReturnType<typeof import('react-router-dom').useNavigate>; asset: any }) {
  const width = 480
  const height = 160
  const cpuPath = toSvgPath(telemetryPoints.map((p) => ({ t: p.t, value: p.cpu })), width, height)
  const memPath = toSvgPath(telemetryPoints.map((p) => ({ t: p.t, value: p.mem })), width, height)

  return (
    <div className="grid grid-cols-12 gap-4">
      {/* LEFT COLUMN */}
      <div className="col-span-12 lg:col-span-5 flex flex-col gap-4">
        {/* Asset Status */}
        <div className="bg-surface-container rounded-lg p-5">
          <SectionLabel>{t('asset_detail.section_asset_status')}</SectionLabel>
          <div className="grid grid-cols-2 gap-5">
            <DataRow
              label={t('asset_detail.label_status')}
              value={
                <span className="flex items-center gap-2">
                  <span className="w-2 h-2 rounded-full bg-[#34d399] inline-block" />
                  <span className="font-semibold text-[#34d399]">{asset.status}</span>
                </span>
              }
            />
            <DataRow label={t('asset_detail.label_bia_level')} value={asset.biaLevel} />
            <DataRow
              label={t('asset_detail.label_warranty_status')}
              value={
                <span className="flex items-center gap-2">
                  <span className="w-2 h-2 rounded-full bg-[#34d399] inline-block" />
                  <span>{asset.warranty.status} (Exp. {asset.warranty.expiry})</span>
                </span>
              }
              valueColor="text-[#34d399]"
            />
            <div className="flex gap-6">
              <DataRow label={t('asset_detail.label_uptime')} value={`${asset.uptime.days}d ${asset.uptime.hours}h ${String(asset.uptime.minutes).padStart(2, '0')}m`} mono />
              <DataRow label={t('asset_detail.label_mtbf')} value={asset.mtbf} mono />
            </div>
          </div>
        </div>

        {/* Location Mapping */}
        <div className="bg-surface-container rounded-lg p-5">
          <SectionLabel>{t('asset_detail.section_location_mapping')}</SectionLabel>
          <div className="flex gap-5">
            <div className="flex-1 grid grid-cols-2 gap-4">
              <div className="col-span-2">
                <DataRow
                  label={t('asset_detail.label_facility')}
                  value={
                    <span className="flex items-center gap-2">
                      <span className="material-symbols-outlined text-[16px] text-primary">domain</span>
                      {asset.facility}
                    </span>
                  }
                />
              </div>
              <DataRow label={t('asset_detail.label_room_hall')} value={asset.room} />
              <DataRow label={t('asset_detail.label_rack_id')} value={<span className="cursor-pointer text-primary hover:underline" onClick={() => navigate('/racks/detail')}>{asset.rackId}</span>} mono />
              <DataRow label={t('asset_detail.label_u_position')} value={asset.uPosition} mono />
            </div>
            <RackIllustration />
          </div>
        </div>

        {/* Financial & Lifecycle */}
        <div className="bg-surface-container rounded-lg p-5">
          <SectionLabel>{t('asset_detail.section_financial_lifecycle')}</SectionLabel>
          <div className="grid grid-cols-2 gap-4">
            <DataRow label={t('asset_detail.label_asset_inventory_no')} value={asset.inventoryNo} mono />
            <DataRow label={t('asset_detail.label_po_reference')} value={asset.poRef} mono />
            <DataRow label={t('asset_detail.label_purchase_date')} value={asset.purchaseDate} />
            <DataRow label={t('asset_detail.label_acquisition_cost')} value={asset.cost} mono />
            <DataRow label={t('asset_detail.label_depreciation_status')} value={asset.depreciation} />
            <DataRow
              label={t('asset_detail.label_book_value')}
              value={<span><span className="text-on-surface-variant text-[0.625rem] mr-1">VAL:</span>{asset.bookValue}</span>}
              mono
            />
          </div>
          <div className="mt-4 flex gap-3">
            <button
              onClick={() => navigate('/assets/lifecycle/timeline')}
              className="flex items-center gap-1.5 text-xs font-semibold text-primary hover:underline"
            >
              <span className="material-symbols-outlined text-[16px]">timeline</span>
              查看生命週期
            </button>
            <button
              onClick={() => navigate('/maintenance/add')}
              className="flex items-center gap-1.5 rounded-lg bg-on-primary-container px-4 py-2 text-xs font-semibold text-white hover:brightness-110 transition-all"
            >
              <span className="material-symbols-outlined text-[16px]">add</span>
              建立維護任務
            </button>
          </div>
        </div>
      </div>

      {/* RIGHT COLUMN */}
      <div className="col-span-12 lg:col-span-7 flex flex-col gap-4">
        {/* Physical Specifications */}
        <div className="bg-surface-container rounded-lg p-5">
          <SectionLabel>{t('asset_detail.section_physical_specs')}</SectionLabel>
          <div className="grid grid-cols-2 gap-4">
            <DataRow label={t('asset_detail.label_model')} value={asset.model} mono />
            <DataRow label={t('asset_detail.label_serial_number')} value={asset.serial} mono />
            <DataRow label={t('asset_detail.label_cpu_architecture')} value={asset.cpu} mono />
            <DataRow label={t('asset_detail.label_memory_ram')} value={asset.memory} mono />
            <DataRow label={t('asset_detail.label_form_factor')} value={asset.formFactor} />
            <DataRow label={t('asset_detail.label_storage')} value={asset.storage} mono />
            <DataRow label={t('asset_detail.label_network')} value={asset.network} mono />
            <DataRow label={t('asset_detail.label_os')} value={asset.os} mono />
          </div>
        </div>

        {/* Live Telemetry */}
        <div className="bg-surface-container rounded-lg p-5">
          <SectionLabel>{t('asset_detail.section_live_telemetry')}</SectionLabel>
          <svg viewBox={`0 0 ${width} ${height}`} className="w-full h-auto" role="img" aria-label="CPU and memory telemetry chart">
            {[0, 25, 50, 75, 100].map((v) => {
              const y = height - 16 - (v / 100) * (height - 32)
              return (
                <g key={v}>
                  <line x1={16} y1={y} x2={width - 16} y2={y} stroke="#202b32" strokeWidth="1" />
                  <text x={8} y={y + 3} fill="#8e9196" fontSize="8" fontFamily="Inter">{v}</text>
                </g>
              )
            })}
            <path d={cpuPath} stroke="#9ecaff" strokeWidth="2" fill="none" strokeLinecap="round" strokeLinejoin="round" />
            <path d={memPath} stroke="#ffb5a0" strokeWidth="2" fill="none" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
          <div className="flex items-center gap-5 mt-3">
            <div className="flex items-center gap-2">
              <span className="w-2.5 h-2.5 rounded-full bg-primary inline-block" />
              <span className="text-xs text-on-surface-variant font-label">CPU 42%</span>
            </div>
            <div className="flex items-center gap-2">
              <span className="w-2.5 h-2.5 rounded-full bg-tertiary inline-block" />
              <span className="text-xs text-on-surface-variant font-label">MEM 59%</span>
            </div>
          </div>
        </div>

        {/* Network Info */}
        <div className="bg-surface-container rounded-lg p-5">
          <SectionLabel>{t('asset_detail.section_network_info')}</SectionLabel>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
            <DataRow label={t('asset_detail.label_primary_ip')} value={asset.primaryIp} mono />
            <DataRow label={t('asset_detail.label_management_ip')} value={asset.mgmtIp} mono />
            <DataRow label={t('asset_detail.label_domain')} value={asset.domain} mono />
            <DataRow label={t('asset_detail.label_vlan')} value={asset.vlan} mono />
          </div>
        </div>
      </div>
    </div>
  )
}

/* ================================================================== */
/*  TAB 2: HEALTH MONITOR                                              */
/* ================================================================== */

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

function HealthTab({ t }: { t: ReturnType<typeof useTranslation>['t'] }) {
  const [activeRange, setActiveRange] = useState<string>('24H')

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
              <span className="font-headline text-3xl font-bold text-on-surface">38.2</span>
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
            <path
              d="M0,80 C30,75 60,70 90,65 C120,60 150,55 180,50 C210,45 240,55 270,60 C300,65 330,50 360,45 C390,40 420,35 450,38 C480,41 510,36 540,40 C570,44 585,42 600,40"
              fill="none" stroke="#9ecaff" strokeWidth="2"
            />
            <path
              d="M0,80 C30,75 60,70 90,65 C120,60 150,55 180,50 C210,45 240,55 270,60 C300,65 330,50 360,45 C390,40 420,35 450,38 C480,41 510,36 540,40 C570,44 585,42 600,40 L600,120 L0,120 Z"
              fill="url(#tempGradUnified)"
            />
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
              <span className="font-headline text-2xl font-bold text-on-surface">412</span>
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
            <path d="M0,35 C20,32 40,30 60,28 C80,26 100,30 120,32 C140,34 160,28 180,25 C200,22 220,28 240,30 C260,32 280,28 300,30" fill="none" stroke="#fbbf24" strokeWidth="2" />
            <path d="M0,35 C20,32 40,30 60,28 C80,26 100,30 120,32 C140,34 160,28 180,25 C200,22 220,28 240,30 C260,32 280,28 300,30 L300,60 L0,60 Z" fill="url(#powerGradUnified)" />
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

/* ================================================================== */
/*  TAB 3: USAGE ANALYTICS                                             */
/* ================================================================== */

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

function UsageTab({ t }: { t: ReturnType<typeof useTranslation>['t'] }) {
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
        {/* Operational Events */}
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

        {/* Component Integrity */}
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

/* ================================================================== */
/*  TAB 4: MAINTENANCE HISTORY                                         */
/* ================================================================== */

type MaintenanceEvent = {
  id: number
  date: string
  type: string
  description: string
  technician: string
  status: 'COMPLETED' | 'SCHEDULED'
  accent: string
}

const maintEvents: MaintenanceEvent[] = [
  { id: 1, date: '2024-03-15', type: 'Firmware Update', description: 'Patch v4.0.2 Security Hardening', technician: 'Alex J.', status: 'COMPLETED', accent: '#34d399' },
  { id: 2, date: '2024-03-28', type: 'Hardware Inspection', description: 'Routine check', technician: 'Marcus K.', status: 'SCHEDULED', accent: '#fbbf24' },
  { id: 3, date: '2024-02-12', type: 'Emergency PSU Swap', description: 'Power failure recovery', technician: 'Sarah L.', status: 'COMPLETED', accent: '#34d399' },
  { id: 4, date: '2024-01-05', type: 'Routine Database Optimization', description: 'Post-migration check', technician: 'Alex J.', status: 'COMPLETED', accent: '#34d399' },
  { id: 5, date: '2023-12-18', type: 'Thermal Paste Re-application', description: 'Maintenance re-adherence, high-temp warning', technician: 'Marcus K.', status: 'COMPLETED', accent: '#34d399' },
]

function MaintenanceTab({ t, navigate, asset }: { t: ReturnType<typeof useTranslation>['t']; navigate: ReturnType<typeof import('react-router-dom').useNavigate>; asset: any }) {
  return (
    <div className="space-y-6">
      {/* Title row */}
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
        <h2 className="font-headline font-bold text-lg tracking-tight text-on-surface">
          {t('asset_maint_history.title')}
        </h2>
        <div className="flex gap-3">
          <button className="bg-surface-container-high px-5 py-2.5 rounded-lg text-xs font-semibold tracking-wider text-on-surface-variant uppercase hover:bg-surface-container-highest transition-colors">
            <span className="flex items-center gap-2">
              <span className="material-symbols-outlined text-[16px]">filter_list</span>
              {t('asset_maint_history.btn_filter_logs')}
            </span>
          </button>
          <button className="bg-on-primary-container text-white px-5 py-2.5 rounded-lg text-xs font-semibold tracking-wider uppercase hover:brightness-110 transition-all">
            <span className="flex items-center gap-2">
              <span className="material-symbols-outlined text-[16px]">add</span>
              {t('asset_maint_history.btn_schedule_entry')}
            </span>
          </button>
        </div>
      </div>

      {/* Table */}
      <div className="bg-surface-container rounded-2xl overflow-hidden">
        <div className="grid grid-cols-[2.5fr_2fr_1.5fr_1fr] gap-4 px-6 py-4 bg-surface-container-low">
          <span className="text-[0.625rem] font-semibold tracking-widest text-on-surface-variant uppercase">{t('asset_maint_history.table_event_date')}</span>
          <span className="text-[0.625rem] font-semibold tracking-widest text-on-surface-variant uppercase">{t('asset_maint_history.table_type_of_work')}</span>
          <span className="text-[0.625rem] font-semibold tracking-widest text-on-surface-variant uppercase">{t('asset_maint_history.table_technician')}</span>
          <span className="text-[0.625rem] font-semibold tracking-widest text-on-surface-variant uppercase text-right">{t('asset_maint_history.table_workflow_status')}</span>
        </div>
        {maintEvents.map((evt) => (
          <div
            key={evt.id}
            className="grid grid-cols-[2.5fr_2fr_1.5fr_1fr] gap-4 px-6 py-5 items-center hover:bg-surface-container-high/50 transition-colors relative"
          >
            <div className="absolute left-0 top-2 bottom-2 w-[3px] rounded-r-full" style={{ backgroundColor: evt.accent }} />
            <div className="pl-2">
              <p className="text-on-surface text-sm font-medium">{evt.date}</p>
              <p className="text-on-surface-variant text-xs mt-0.5">{evt.description}</p>
            </div>
            <div><p className="text-on-surface text-sm font-medium">{evt.type}</p></div>
            <div><p className="text-on-surface text-sm">{evt.technician}</p></div>
            <div className="flex justify-end">
              <span
                className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-[0.625rem] font-semibold tracking-wider uppercase ${
                  evt.status === 'COMPLETED'
                    ? 'bg-[#064e3b] text-[#34d399]'
                    : 'bg-[#92400e] text-[#fbbf24]'
                }`}
              >
                <span className="w-1.5 h-1.5 rounded-full" style={{ backgroundColor: evt.status === 'COMPLETED' ? '#34d399' : '#fbbf24' }} />
                {evt.status}
              </span>
            </div>
          </div>
        ))}
      </div>

      {/* View all + Pagination */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <p className="text-xs text-on-surface-variant tracking-wider uppercase">
            {t('asset_maint_history.pagination_showing')}
          </p>
          <button
            onClick={() => navigate('/maintenance')}
            className="flex items-center gap-1 text-xs font-semibold text-primary hover:underline"
          >
            查看全部維護記錄
            <span className="material-symbols-outlined text-[14px]">arrow_forward</span>
          </button>
        </div>
        <div className="flex gap-2">
          <button className="bg-surface-container-high w-9 h-9 rounded-lg flex items-center justify-center hover:bg-surface-container-highest transition-colors text-on-surface-variant">
            <span className="material-symbols-outlined text-[18px]">chevron_left</span>
          </button>
          <button className="bg-surface-container-high w-9 h-9 rounded-lg flex items-center justify-center hover:bg-surface-container-highest transition-colors text-on-surface-variant">
            <span className="material-symbols-outlined text-[18px]">chevron_right</span>
          </button>
        </div>
      </div>

      {/* AI Insight Card */}
      <div className="bg-surface-container rounded-2xl p-6 flex items-start gap-4 max-w-xl ml-auto">
        <div className="w-10 h-10 rounded-xl bg-primary/10 flex items-center justify-center flex-shrink-0">
          <span className="material-symbols-outlined text-primary text-xl">auto_awesome</span>
        </div>
        <div>
          <p className="text-[0.625rem] font-semibold tracking-widest text-primary uppercase mb-1">{t('asset_maint_history.ai_insight_label')}</p>
          <p className="text-on-surface-variant text-sm leading-relaxed">
            {t('asset_maint_history.ai_insight_text')}
          </p>
        </div>
      </div>
    </div>
  )
}

/* ================================================================== */
/*  MAIN PAGE                                                          */
/* ================================================================== */

export default function AssetDetailUnified() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { assetId } = useParams<{ assetId: string }>()
  const [activeTab, setActiveTab] = useState<TabKey>('overview')

  // Fetch asset from API
  const assetQ = useAsset(assetId ?? '')
  const apiAsset = assetQ.data?.data

  // Merge API asset data with defaults for fields not in the API schema
  const asset = {
    ...assetDefaults,
    id: apiAsset?.asset_tag ?? assetId ?? '-',
    status: apiAsset?.status ?? 'Unknown',
    biaLevel: apiAsset?.bia_level ?? '-',
    model: apiAsset ? `${apiAsset.vendor} ${apiAsset.model}` : '-',
    serial: apiAsset?.serial_number ?? '-',
    tags: apiAsset?.tags ?? [],
    description: (apiAsset?.attributes?.description as string) ?? assetDefaults.description,
    // Map additional fields from attributes if backend provides them
    cpu: (apiAsset?.attributes?.cpu as string) ?? assetDefaults.cpu,
    memory: (apiAsset?.attributes?.memory as string) ?? assetDefaults.memory,
    formFactor: (apiAsset?.attributes?.form_factor as string) ?? assetDefaults.formFactor,
    storage: (apiAsset?.attributes?.storage as string) ?? assetDefaults.storage,
    network: (apiAsset?.attributes?.network as string) ?? assetDefaults.network,
    os: (apiAsset?.attributes?.os as string) ?? assetDefaults.os,
    primaryIp: (apiAsset?.attributes?.primary_ip as string) ?? assetDefaults.primaryIp,
    mgmtIp: (apiAsset?.attributes?.mgmt_ip as string) ?? assetDefaults.mgmtIp,
    domain: (apiAsset?.attributes?.domain as string) ?? assetDefaults.domain,
    vlan: (apiAsset?.attributes?.vlan as string) ?? assetDefaults.vlan,
  }

  if (assetQ.isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-sky-400 border-t-transparent" />
      </div>
    )
  }

  if (assetQ.error) {
    return (
      <div className="p-6">
        <div className="rounded-lg bg-red-900/20 p-4 text-red-300 text-sm">
          Failed to load asset detail.{' '}
          <button onClick={() => assetQ.refetch()} className="underline">Retry</button>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-surface font-body text-on-surface">
      {/* Back Button */}
      <div className="px-8 pt-6 pb-1">
        <button
          onClick={() => navigate('/assets')}
          className="flex items-center gap-1 text-sm text-on-surface-variant hover:text-primary transition-colors"
        >
          <span className="material-symbols-outlined text-[18px]">arrow_back</span>
          {t('asset_detail.back_to_assets')}
        </button>
      </div>

      {/* Header */}
      <div className="px-8 pt-5 pb-4">
        {/* Tags */}
        <div className="flex items-center gap-2 mb-3">
          {asset.tags.map((tag) => (
            <span
              key={tag}
              className="px-2.5 py-1 rounded text-[0.625rem] font-semibold uppercase tracking-wider bg-surface-container-high text-on-surface-variant"
            >
              {tag}
            </span>
          ))}
        </div>

        {/* Title row */}
        <div className="flex items-start justify-between gap-6">
          <div className="flex items-center gap-4">
            <h1 className="font-headline font-bold text-3xl text-on-surface leading-tight">
              {asset.id}
            </h1>
            <StatusBadge status={asset.status} />
          </div>
          <div className="flex items-center gap-3 shrink-0 pt-1">
            <button className="flex items-center gap-2 px-5 py-2.5 rounded-lg bg-on-primary-container text-[#ffffff] text-[0.75rem] font-semibold uppercase tracking-wider hover:opacity-90 transition-opacity">
              <span className="material-symbols-outlined text-[18px]">edit</span>
              {t('asset_detail.btn_edit_asset')}
            </button>
            <button
              onClick={() => navigate('/audit')}
              className="flex items-center gap-2 px-5 py-2.5 rounded-lg bg-surface-container-high text-on-surface-variant text-[0.75rem] font-semibold uppercase tracking-wider hover:bg-surface-container-highest transition-colors"
            >
              <span className="material-symbols-outlined text-[18px]">history</span>
              {t('asset_detail.btn_audit_log')}
            </button>
          </div>
        </div>

        <p className="mt-1.5 text-sm text-on-surface-variant max-w-2xl leading-relaxed">
          {asset.description}
        </p>
      </div>

      {/* Tabs */}
      <div className="px-8 flex items-center gap-1 border-b border-outline-variant/20 mb-6">
        {tabs.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`flex items-center gap-1.5 px-4 py-3 text-[0.75rem] font-semibold uppercase tracking-wider transition-colors border-b-2 ${
              activeTab === tab.key
                ? 'border-primary text-primary'
                : 'border-transparent text-on-surface-variant hover:text-on-surface hover:border-outline-variant/30'
            }`}
          >
            <Icon name={tab.icon} className="text-[18px]" />
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      <div className="px-8 pb-10">
        {activeTab === 'overview' && <OverviewTab t={t} navigate={navigate} asset={asset} />}
        {activeTab === 'health' && <HealthTab t={t} />}
        {activeTab === 'usage' && <UsageTab t={t} />}
        {activeTab === 'maintenance' && <MaintenanceTab t={t} navigate={navigate} asset={asset} />}
      </div>
    </div>
  )
}
