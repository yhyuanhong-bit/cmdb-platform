import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import SectionLabel from '../components/SectionLabel'
import DataRow from '../components/DataRow'
import RackIllustration from '../components/RackIllustration'
import { toSvgPath } from '../components/MetricsChart'
import { useMetrics } from '../../../hooks/useMetrics'

export default function OverviewTab({ asset, assetId, impactedSystems = [] }: { asset: any; assetId?: string; impactedSystems?: any[] }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const width = 480
  const height = 160

  const cpuMetrics = useMetrics({ asset_id: assetId, metric_name: 'cpu_usage', time_range: '1h' })
  const memMetrics = useMetrics({ asset_id: assetId, metric_name: 'memory_usage', time_range: '1h' })

  const cpuPoints = (cpuMetrics.data?.data ?? []).map((p: any, i: number) => ({ t: i, value: p.value }))
  const memPoints = (memMetrics.data?.data ?? []).map((p: any, i: number) => ({ t: i, value: p.value }))

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
              {t('asset_detail.btn_view_lifecycle')}
            </button>
            <button
              onClick={() => navigate('/maintenance/add')}
              className="flex items-center gap-1.5 rounded-lg bg-on-primary-container px-4 py-2 text-xs font-semibold text-white hover:brightness-110 transition-all"
            >
              <span className="material-symbols-outlined text-[16px]">add</span>
              {t('asset_detail.btn_create_maintenance')}
            </button>
          </div>
        </div>

        {/* BIA Impact */}
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
            <DataRow label={t('asset_detail.field_property_number')} value={asset.property_number || '-'} mono />
            <DataRow label={t('asset_detail.field_control_number')} value={asset.control_number || '-'} mono />
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
