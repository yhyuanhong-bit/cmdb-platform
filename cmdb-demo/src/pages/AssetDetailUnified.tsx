import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import Icon from '../components/Icon'
import StatusBadge from '../components/StatusBadge'
import { useAsset, useUpdateAsset, useDeleteAsset } from '../hooks/useAssets'
import { useBIAImpact } from '../hooks/useBIA'
import { useMetrics } from '../hooks/useMetrics'
import { useRootLocations, useLocationDescendants } from '../hooks/useTopology'

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

function OverviewTab({ t, navigate, asset, assetId, impactedSystems = [] }: { t: ReturnType<typeof useTranslation>['t']; navigate: ReturnType<typeof import('react-router-dom').useNavigate>; asset: any; assetId?: string; impactedSystems?: any[] }) {
  const width = 480
  const height = 160

  const cpuMetrics = useMetrics({ asset_id: assetId, metric_name: 'cpu_usage', time_range: '1h' })
  const memMetrics = useMetrics({ asset_id: assetId, metric_name: 'memory_usage', time_range: '1h' })

  const cpuPoints = (cpuMetrics.data?.data ?? []).map((p, i) => ({ t: i, value: p.value }))
  const memPoints = (memMetrics.data?.data ?? []).map((p, i) => ({ t: i, value: p.value }))

  const hasData = cpuPoints.length > 0 || memPoints.length > 0
  const cpuPath = cpuPoints.length > 0 ? toSvgPath(cpuPoints, width, height) : null
  const memPath = memPoints.length > 0 ? toSvgPath(memPoints, width, height) : null

  const latestCpu = cpuPoints.length > 0 ? cpuPoints[cpuPoints.length - 1].value.toFixed(0) : null
  const latestMem = memPoints.length > 0 ? memPoints[memPoints.length - 1].value.toFixed(0) : null

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
            <DataRow label={t('asset_detail.label_bia_level')} value={
              <span className={`px-2.5 py-1 rounded text-[0.6875rem] font-semibold uppercase tracking-wider ${
                asset.biaLevel === 'critical' ? 'bg-error-container text-on-error-container' :
                asset.biaLevel === 'important' ? 'bg-[#92400e] text-[#fbbf24]' :
                asset.biaLevel === 'normal' ? 'bg-[#1e3a5f] text-on-primary-container' :
                'bg-surface-container-highest text-on-surface-variant'
              }`}>
                {asset.biaLevel}
              </span>
            } />
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

        {/* BIA Impact — Dependent Business Systems */}
        {impactedSystems.length > 0 && (
          <div className="mt-5 rounded-lg bg-surface-container p-5">
            <div className="mb-3 flex items-center gap-2">
              <span className="material-symbols-outlined text-primary text-xl">device_hub</span>
              <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
                BIA Impact — Dependent Business Systems
              </h3>
            </div>
            <div className="space-y-2">
              {impactedSystems.map((sys: any) => (
                <div key={sys.id} className="flex items-center justify-between rounded-lg bg-surface-container-low p-3">
                  <div>
                    <p className="text-sm font-semibold text-on-surface">{sys.system_name}</p>
                    <p className="text-xs text-on-surface-variant">{sys.system_code}</p>
                  </div>
                  <div className="flex items-center gap-3">
                    <span className={`px-2.5 py-1 rounded text-[0.6875rem] font-semibold uppercase tracking-wider
                      ${sys.tier === 'critical' ? 'bg-error-container text-on-error-container' :
                        sys.tier === 'important' ? 'bg-[#92400e] text-[#fbbf24]' :
                        'bg-[#1e3a5f] text-on-primary-container'}`}>
                      {sys.tier}
                    </span>
                    <span className="text-sm font-bold text-on-surface">{sys.bia_score}</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
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
          {(cpuMetrics.isLoading || memMetrics.isLoading) ? (
            <div className="flex items-center justify-center h-[160px]">
              <div className="animate-spin rounded-full h-6 w-6 border-2 border-sky-400 border-t-transparent" />
            </div>
          ) : !hasData ? (
            <div className="flex items-center justify-center h-[160px] text-on-surface-variant text-sm">
              No data
            </div>
          ) : (
            <>
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
                {cpuPath && <path d={cpuPath} stroke="#9ecaff" strokeWidth="2" fill="none" strokeLinecap="round" strokeLinejoin="round" />}
                {memPath && <path d={memPath} stroke="#ffb5a0" strokeWidth="2" fill="none" strokeLinecap="round" strokeLinejoin="round" />}
              </svg>
              <div className="flex items-center gap-5 mt-3">
                <div className="flex items-center gap-2">
                  <span className="w-2.5 h-2.5 rounded-full bg-primary inline-block" />
                  <span className="text-xs text-on-surface-variant font-label">
                    CPU {latestCpu != null ? `${latestCpu}%` : '—'}
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="w-2.5 h-2.5 rounded-full bg-tertiary inline-block" />
                  <span className="text-xs text-on-surface-variant font-label">
                    MEM {latestMem != null ? `${latestMem}%` : '—'}
                  </span>
                </div>
              </div>
            </>
          )}
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

function HealthTab({ t, assetId }: { t: ReturnType<typeof useTranslation>['t']; assetId?: string }) {
  const [activeRange, setActiveRange] = useState<string>('24H')

  const timeRangeMap: Record<string, string> = { '1H': '1h', '6H': '6h', '24H': '24h', '7D': '7d' }
  const timeRange = timeRangeMap[activeRange] ?? '24h'

  const tempMetrics = useMetrics({ asset_id: assetId, metric_name: 'temperature', time_range: timeRange })
  const powerMetrics = useMetrics({ asset_id: assetId, metric_name: 'power_kw', time_range: timeRange })

  const tempPoints = tempMetrics.data?.data ?? []
  const powerPoints = powerMetrics.data?.data ?? []

  const latestTemp = tempPoints.length > 0 ? tempPoints[tempPoints.length - 1].value.toFixed(1) : null
  const latestPowerW = powerPoints.length > 0 ? (powerPoints[powerPoints.length - 1].value * 1000).toFixed(0) : null

  // Build SVG paths for temperature (viewBox 600x120) and power (viewBox 300x60)
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
                {latestTemp ?? (tempMetrics.isLoading ? '…' : '—')}
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
                {tempMetrics.isLoading ? 'Loading…' : 'No data'}
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
                {latestPowerW ?? (powerMetrics.isLoading ? '…' : '—')}
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
                {powerMetrics.isLoading ? 'Loading…' : 'No data'}
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
          <button onClick={() => alert('Coming Soon')} className="bg-surface-container-high px-5 py-2.5 rounded-lg text-xs font-semibold tracking-wider text-on-surface-variant uppercase hover:bg-surface-container-highest transition-colors">
            <span className="flex items-center gap-2">
              <span className="material-symbols-outlined text-[16px]">filter_list</span>
              {t('asset_maint_history.btn_filter_logs')}
            </span>
          </button>
          <button onClick={() => navigate('/maintenance/add')} className="bg-on-primary-container text-white px-5 py-2.5 rounded-lg text-xs font-semibold tracking-wider uppercase hover:brightness-110 transition-all">
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
  const [editing, setEditing] = useState(false)
  const [editData, setEditData] = useState<Record<string, string>>({})
  const updateAsset = useUpdateAsset()
  const deleteAsset = useDeleteAsset()

  const rootLocQ = useRootLocations()
  const firstTerritoryId = rootLocQ.data?.data?.[0]?.id ?? ''
  const descQ = useLocationDescendants(firstTerritoryId)
  const allLocations = (descQ.data?.data ?? []).filter((l: any) =>
    ['room', 'module', 'idc', 'campus'].includes(l.level)
  )

  const tabLabels: Record<string, string> = {
    overview: t('asset_detail.tab_overview'),
    health: t('asset_detail.tab_health'),
    usage: t('asset_detail.tab_usage'),
    maintenance: t('asset_detail.tab_maintenance'),
  }

  // Fetch asset from API
  const assetQ = useAsset(assetId ?? '')
  const apiAsset = assetQ.data?.data

  // Fetch BIA impact (dependent business systems)
  const { data: impactResp } = useBIAImpact(assetId || '')
  const impactedSystems: any[] = (impactResp as any)?.data || []

  // Merge API asset data with defaults for fields not in the API schema
  // Helper to read an attribute with a string fallback
  const attr = (key: string): string | undefined => {
    const v = apiAsset?.attributes?.[key]
    return v != null ? String(v) : undefined
  }

  // Derive OS string: prefer attributes.os, then combine os_type + os_version
  const osValue = (() => {
    if (attr('os')) return attr('os') as string
    const osType = attr('os_type')
    const osVer = attr('os_version')
    if (osType && osVer) return `${osType} ${osVer}`
    return osType ?? osVer ?? assetDefaults.os
  })()

  // Primary IP: prefer direct field, fall back to attributes
  const primaryIpValue =
    (apiAsset as any)?.ip_address as string | undefined ??
    attr('ip_address') ??
    attr('primary_ip') ??
    assetDefaults.primaryIp

  // CPU: prefer attributes.cpu_arch, then attributes.cpu
  const cpuValue = attr('cpu_arch') ?? attr('cpu') ?? assetDefaults.cpu

  // Memory: prefer attributes.memory_gb (numeric → append GB), then attributes.memory
  const memoryValue = (() => {
    const gb = attr('memory_gb')
    if (gb) return `${gb} GB`
    return attr('memory') ?? assetDefaults.memory
  })()

  // Storage: prefer attributes.storage_tb (numeric → append TB), then attributes.storage
  const storageValue = (() => {
    const tb = attr('storage_tb')
    if (tb) return `${tb} TB`
    return attr('storage') ?? assetDefaults.storage
  })()

  const asset = {
    ...assetDefaults,
    id: apiAsset?.asset_tag ?? assetId ?? '-',
    status: apiAsset?.status ?? 'Unknown',
    biaLevel: apiAsset?.bia_level ?? '-',
    model: apiAsset ? `${apiAsset.vendor} ${apiAsset.model}` : '-',
    serial: apiAsset?.serial_number ?? '-',
    tags: apiAsset?.tags ?? [],
    description: attr('description') ?? assetDefaults.description,
    cpu: cpuValue,
    memory: memoryValue,
    formFactor: attr('form_factor') ?? assetDefaults.formFactor,
    storage: storageValue,
    network: attr('network') ?? assetDefaults.network,
    os: osValue,
    primaryIp: primaryIpValue,
    mgmtIp: attr('mgmt_ip') ?? assetDefaults.mgmtIp,
    domain: attr('domain') ?? assetDefaults.domain,
    vlan: attr('vlan') ?? assetDefaults.vlan,
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
            <button onClick={() => {
              setEditing(true)
              setEditData({
                name: apiAsset?.name || '',
                status: apiAsset?.status || '',
                vendor: apiAsset?.vendor || '',
                model: apiAsset?.model || '',
                bia_level: apiAsset?.bia_level || '',
                serial_number: apiAsset?.serial_number || '',
                ip_address: (apiAsset as any)?.ip_address || '',
                location_id: (apiAsset as any)?.location_id || '',
                tags: (apiAsset?.tags || []).join(', '),
              })
            }} className="flex items-center gap-2 px-5 py-2.5 rounded-lg bg-on-primary-container text-[#ffffff] text-[0.75rem] font-semibold uppercase tracking-wider hover:opacity-90 transition-opacity">
              <span className="material-symbols-outlined text-[18px]">edit</span>
              {t('asset_detail.btn_edit_asset')}
            </button>
            <button onClick={() => {
              if (confirm(t('asset_detail.confirm_delete'))) {
                deleteAsset.mutate(assetId!, { onSuccess: () => navigate('/assets') })
              }
            }} className="flex items-center gap-2 px-5 py-2.5 rounded-lg bg-red-500/20 text-red-400 text-[0.75rem] font-semibold uppercase tracking-wider hover:bg-red-500/30 transition-colors">
              <span className="material-symbols-outlined text-[18px]">delete</span>
              {deleteAsset.isPending ? t('asset_detail.btn_deleting') : t('asset_detail.btn_delete')}
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
            {tabLabels[tab.key] ?? tab.label}
          </button>
        ))}
      </div>

      {/* Inline Edit Panel */}
      {editing && (
        <div className="px-8 py-4">
          <div className="bg-surface-container rounded-lg p-5 space-y-4">
            <h3 className="font-headline text-sm font-bold text-on-surface uppercase tracking-wider">{t('asset_detail.edit_title')}</h3>
            <div className="grid grid-cols-3 gap-4">
              {/* Name */}
              <div className="flex flex-col gap-1">
                <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">{t('asset_detail.field_name')}</label>
                <input
                  value={editData.name ?? ''}
                  onChange={e => setEditData(p => ({ ...p, name: e.target.value }))}
                  className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm"
                />
              </div>
              {/* Status */}
              <div className="flex flex-col gap-1">
                <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">{t('asset_detail.field_status')}</label>
                <select
                  value={editData.status ?? ''}
                  onChange={e => setEditData(p => ({ ...p, status: e.target.value }))}
                  className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm"
                >
                  <option value=""></option>
                  <option value="inventoried">{t('asset_detail.status_inventoried')}</option>
                  <option value="operational">{t('asset_detail.status_operational')}</option>
                  <option value="deployed">{t('asset_detail.status_deployed')}</option>
                  <option value="maintenance">{t('asset_detail.status_maintenance')}</option>
                  <option value="retired">{t('asset_detail.status_retired')}</option>
                </select>
              </div>
              {/* BIA Level */}
              <div className="flex flex-col gap-1">
                <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">{t('asset_detail.field_bia_level')}</label>
                <select
                  value={editData.bia_level ?? ''}
                  onChange={e => setEditData(p => ({ ...p, bia_level: e.target.value }))}
                  className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm"
                >
                  <option value=""></option>
                  <option value="critical">{t('asset_detail.bia_critical')}</option>
                  <option value="important">{t('asset_detail.bia_important')}</option>
                  <option value="normal">{t('asset_detail.bia_normal')}</option>
                  <option value="minor">{t('asset_detail.bia_minor')}</option>
                </select>
              </div>
              {/* Vendor */}
              <div className="flex flex-col gap-1">
                <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">{t('asset_detail.field_vendor')}</label>
                <input
                  value={editData.vendor ?? ''}
                  onChange={e => setEditData(p => ({ ...p, vendor: e.target.value }))}
                  className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm"
                />
              </div>
              {/* Model */}
              <div className="flex flex-col gap-1">
                <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">{t('asset_detail.field_model')}</label>
                <input
                  value={editData.model ?? ''}
                  onChange={e => setEditData(p => ({ ...p, model: e.target.value }))}
                  className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm"
                />
              </div>
              {/* Serial Number */}
              <div className="flex flex-col gap-1">
                <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">{t('asset_detail.field_serial_number')}</label>
                <input
                  value={editData.serial_number ?? ''}
                  onChange={e => setEditData(p => ({ ...p, serial_number: e.target.value }))}
                  className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm"
                />
              </div>
              {/* IP Address */}
              <div className="flex flex-col gap-1">
                <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">{t('asset_detail.field_ip_address')}</label>
                <input
                  value={editData.ip_address ?? ''}
                  onChange={e => setEditData(p => ({ ...p, ip_address: e.target.value }))}
                  placeholder="192.168.1.100"
                  className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm"
                />
              </div>
              {/* Location */}
              <div className="flex flex-col gap-1">
                <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">{t('asset_detail.field_location')}</label>
                <select
                  value={editData.location_id ?? ''}
                  onChange={e => setEditData(p => ({ ...p, location_id: e.target.value }))}
                  className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm"
                >
                  <option value="">{t('asset_detail.select_location')}</option>
                  {allLocations.map((loc: any) => (
                    <option key={loc.id} value={loc.id}>{loc.name}</option>
                  ))}
                </select>
              </div>
              {/* Tags */}
              <div className="flex flex-col gap-1">
                <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
                  {t('asset_detail.field_tags')}
                  <span className="ml-1 normal-case text-[0.6rem] opacity-60">({t('asset_detail.tags_hint')})</span>
                </label>
                <input
                  value={editData.tags ?? ''}
                  onChange={e => setEditData(p => ({ ...p, tags: e.target.value }))}
                  placeholder="production, tier-1"
                  className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm"
                />
              </div>
            </div>
            <div className="flex gap-2 mt-4">
              <button onClick={() => {
                const payload: Record<string, any> = { ...editData }
                if (typeof payload.tags === 'string') {
                  payload.tags = payload.tags.split(',').map((s: string) => s.trim()).filter(Boolean)
                }
                Object.keys(payload).forEach(k => { if (payload[k] === '') delete payload[k] })
                updateAsset.mutate({ id: assetId!, data: payload }, {
                  onSuccess: (resp: any) => {
                    setEditing(false)
                    if (resp?.meta?.change_order_id) {
                      alert(`Critical asset change recorded. Audit order: ${resp.meta.change_order_id}`)
                    }
                  }
                })
              }} disabled={updateAsset.isPending}
                className="px-4 py-2 rounded bg-blue-600 text-white text-sm">
                {updateAsset.isPending ? t('asset_detail.edit_saving') : t('asset_detail.edit_save')}
              </button>
              <button onClick={() => setEditing(false)}
                className="px-4 py-2 rounded bg-gray-700 text-white text-sm">{t('asset_detail.edit_cancel')}</button>
            </div>
          </div>
        </div>
      )}

      {/* Tab Content */}
      <div className="px-8 pb-10">
        {activeTab === 'overview' && <OverviewTab t={t} navigate={navigate} asset={asset} assetId={assetId} impactedSystems={impactedSystems} />}
        {activeTab === 'health' && <HealthTab t={t} assetId={assetId} />}
        {activeTab === 'usage' && <UsageTab t={t} />}
        {activeTab === 'maintenance' && <MaintenanceTab t={t} navigate={navigate} asset={asset} />}
      </div>
    </div>
  )
}
