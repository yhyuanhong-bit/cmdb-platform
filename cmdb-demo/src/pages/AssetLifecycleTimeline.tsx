import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useAsset } from '../hooks/useAssets'

/* ------------------------------------------------------------------ */
/*  API-driven asset header                                            */
/* ------------------------------------------------------------------ */

// Default values for fields not available in the API Asset schema
const assetFallback = {
  id: '-',
  status: 'UNKNOWN',
  lastSync: '-',
  serial: '-',
  primaryIp: '-',
  avgLatency: '-',
  uptime: '-',
}

const timelineStages = [
  {
    id: 1,
    phaseKey: 'asset_lifecycle_timeline.phase_procurement',
    statusKey: 'asset_lifecycle_timeline.status_completed',
    statusColor: 'text-[#69db7c]',
    dotColor: 'bg-[#69db7c]',
    lineColor: 'bg-[#69db7c]/40',
    date: '2023.01.12',
    technician: 'M. Sterling',
    description:
      'Approved vendor bid accepted. Hardware order placed with Dell Technologies under PO-2023-8842.',
    hasDetail: true,
  },
  {
    id: 2,
    phaseKey: 'asset_lifecycle_timeline.phase_installation_deployment',
    statusKey: 'asset_lifecycle_timeline.status_completed',
    statusColor: 'text-[#69db7c]',
    dotColor: 'bg-[#69db7c]',
    lineColor: 'bg-[#69db7c]/40',
    date: '2023.01.20',
    technician: 'K. Vance',
    description:
      'Physical rack installation at IDC-NORTH-01. OS provisioning and network configuration completed.',
    hasDetail: true,
  },
  {
    id: 3,
    phaseKey: 'asset_lifecycle_timeline.phase_maintenance_cycle',
    statusKey: 'asset_lifecycle_timeline.status_ongoing',
    statusColor: 'text-[#ffa94d]',
    dotColor: 'bg-[#ffa94d]',
    lineColor: 'bg-[#ffa94d]/40',
    date: '2023.09.15',
    technician: 'J. Thorne',
    description:
      'Scheduled firmware update v4.2.1 applied. Cooling unit inspection flagged for follow-up.',
    hasDetail: false,
    highlighted: true,
  },
  {
    id: 4,
    phaseKey: 'asset_lifecycle_timeline.phase_infrastructure_upgrade',
    statusKey: 'asset_lifecycle_timeline.status_planned',
    statusColor: 'text-on-surface-variant',
    dotColor: 'bg-on-surface-variant/50',
    lineColor: 'bg-on-surface-variant/20',
    date: 'Expected: Q1 2024',
    technician: 'SysOps Alpha',
    description:
      'NVMe storage tier migration and memory expansion to 512GB. Budget allocated under CAPEX-2024-Q1.',
    hasDetail: false,
    costEstimate: '$14,200.00',
  },
  {
    id: 5,
    phaseKey: 'asset_lifecycle_timeline.phase_decommission_recycle',
    statusKey: 'asset_lifecycle_timeline.status_done',
    statusColor: 'text-on-surface-variant',
    dotColor: 'bg-on-surface-variant/50',
    lineColor: 'bg-transparent',
    date: '2027.01.12',
    technician: 'Standard 48-Mo Lifecycle',
    description:
      'End-of-life scheduled per corporate asset rotation policy. Data wipe and certified recycling.',
    hasDetail: false,
  },
]

const financials = {
  acquisitionCost: '$248,500.00',
  depreciatedValue: '$192,420.00',
  maintenanceRoi: 12.4,
}

const compliance = [
  {
    label: 'ISO 27001 CERTIFICATION',
    icon: 'verified',
    status: 'pass',
    color: 'text-[#69db7c]',
  },
  {
    label: 'SECURITY PATCHING V4.2',
    icon: 'security',
    status: 'pass',
    color: 'text-[#69db7c]',
  },
  {
    label: 'PHYSICAL ACCESS AUDIT',
    icon: 'warning',
    status: 'warn',
    color: 'text-error',
  },
]

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export default function AssetLifecycleTimeline() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { assetId } = useParams<{ assetId: string }>()
  const [expandedStage, setExpandedStage] = useState<number | null>(3)

  // Fetch asset from API
  const assetQ = useAsset(assetId ?? '')
  const apiAsset = assetQ.data?.data

  const asset = {
    ...assetFallback,
    id: apiAsset?.asset_tag ?? assetId ?? assetFallback.id,
    status: apiAsset?.status?.toUpperCase() ?? assetFallback.status,
    serial: apiAsset?.serial_number ?? assetFallback.serial,
    primaryIp: (apiAsset?.attributes?.primary_ip as string) ?? assetFallback.primaryIp,
    avgLatency: (apiAsset?.attributes?.avg_latency as string) ?? assetFallback.avgLatency,
    uptime: (apiAsset?.attributes?.uptime as string) ?? assetFallback.uptime,
  }

  if (assetQ.isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-sky-400 border-t-transparent" />
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-surface p-6 font-body text-on-surface">
      {/* Breadcrumb */}
      <div className="mb-6 flex items-center gap-2 text-xs tracking-widest text-on-surface-variant">
        <button
          onClick={() => navigate('/assets/lifecycle')}
          className="flex items-center gap-1 hover:text-primary transition-colors cursor-pointer"
        >
          <span className="material-symbols-outlined text-sm">arrow_back</span>
          {t('asset_lifecycle_timeline.breadcrumb_asset_lifecycle')}
        </button>
      </div>

      {/* Header */}
      <div className="mb-8 flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="font-headline text-2xl font-bold tracking-wide text-on-surface">
            {t('asset_lifecycle_timeline.title')}
          </h1>
          <div className="mt-3 flex flex-wrap items-center gap-3">
            <span className="font-mono text-lg font-bold text-primary">
              {asset.id}
            </span>
            <span className="flex items-center gap-1.5 rounded bg-[#69db7c]/15 px-3 py-1 text-[10px] font-bold tracking-widest text-[#69db7c]">
              <span className="inline-block h-1.5 w-1.5 rounded-full bg-[#69db7c]" />
              {asset.status}
            </span>
          </div>
        </div>
        <div className="text-right text-xs text-on-surface-variant">
          <div className="text-[10px] tracking-widest">{t('asset_lifecycle_timeline.label_last_sync')}</div>
          <div className="mt-1 font-mono">{asset.lastSync}</div>
        </div>
      </div>

      {/* Main grid */}
      <div className="grid gap-6 lg:grid-cols-[1fr_380px]">
        {/* ---- Timeline ---- */}
        <div className="rounded-lg bg-surface-container p-6">
          <h2 className="mb-6 text-[10px] font-bold tracking-widest text-on-surface-variant">
            {t('asset_lifecycle_timeline.section_event_sequence')}
          </h2>

          <div className="relative pl-8">
            {timelineStages.map((stage, idx) => (
              <div key={stage.id} className="relative pb-8 last:pb-0">
                {/* Vertical line */}
                {idx < timelineStages.length - 1 && (
                  <div
                    className={`absolute left-[-20px] top-4 h-full w-0.5 ${stage.lineColor}`}
                  />
                )}
                {/* Dot */}
                <div
                  className={`absolute left-[-24px] top-1 h-3 w-3 rounded-full ${stage.dotColor} ${
                    stage.highlighted ? 'ring-4 ring-[#ffa94d]/20' : ''
                  }`}
                />

                {/* Stage content */}
                <div
                  className={`rounded-lg p-4 transition-colors cursor-pointer ${
                    stage.highlighted
                      ? 'bg-[#ffa94d]/5'
                      : expandedStage === stage.id
                        ? 'bg-surface-container-high'
                        : 'bg-surface-container-low hover:bg-surface-container-high'
                  }`}
                  onClick={() =>
                    setExpandedStage(
                      expandedStage === stage.id ? null : stage.id,
                    )
                  }
                >
                  <div className="flex flex-wrap items-center gap-3">
                    <span className="text-xs font-bold tracking-widest text-on-surface">
                      {t(stage.phaseKey)}
                    </span>
                    <span
                      className={`text-[10px] font-bold tracking-widest ${stage.statusColor}`}
                    >
                      {t(stage.statusKey)}
                    </span>
                  </div>
                  <div className="mt-2 flex flex-wrap items-center gap-4 text-[10px] tracking-widest text-on-surface-variant">
                    <span className="flex items-center gap-1">
                      <span className="material-symbols-outlined text-xs">
                        calendar_today
                      </span>
                      {stage.date}
                    </span>
                    <span className="flex items-center gap-1">
                      <span className="material-symbols-outlined text-xs">
                        person
                      </span>
                      {stage.technician}
                    </span>
                  </div>

                  {expandedStage === stage.id && (
                    <div className="mt-3 space-y-3">
                      <p className="text-sm leading-relaxed text-on-surface-variant">
                        {stage.description}
                      </p>
                      {stage.costEstimate && (
                        <div className="text-xs text-on-surface-variant">
                          {t('asset_lifecycle_timeline.label_cost_estimate')}:{' '}
                          <span className="font-mono text-primary">
                            {stage.costEstimate}
                          </span>
                        </div>
                      )}
                      {stage.hasDetail && (
                        <button onClick={() => setExpandedStage(stage.id === expandedStage ? null : stage.id)} className="flex items-center gap-1 text-[10px] font-bold tracking-widest text-primary hover:underline cursor-pointer">
                          {t('asset_lifecycle_timeline.btn_view_details')}
                          <span className="material-symbols-outlined text-sm">
                            arrow_forward
                          </span>
                        </button>
                      )}
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* ---- Right Sidebar ---- */}
        <div className="space-y-6">
          {/* Financial Overview */}
          <div className="rounded-lg bg-surface-container p-6">
            <h2 className="mb-5 text-[10px] font-bold tracking-widest text-on-surface-variant">
              {t('asset_lifecycle_timeline.section_financial_overview')}
            </h2>
            <div className="space-y-4">
              <div>
                <div className="text-[10px] tracking-widest text-on-surface-variant">
                  {t('asset_lifecycle_timeline.label_acquisition_cost')}
                </div>
                <div className="mt-1 font-mono text-xl font-bold text-on-surface">
                  {financials.acquisitionCost}
                </div>
              </div>
              <div>
                <div className="text-[10px] tracking-widest text-on-surface-variant">
                  {t('asset_lifecycle_timeline.label_depreciated_value')}
                </div>
                <div className="mt-1 font-mono text-xl font-bold text-[#69db7c]">
                  {financials.depreciatedValue}
                </div>
              </div>
              <div>
                <div className="text-[10px] tracking-widest text-on-surface-variant">
                  {t('asset_lifecycle_timeline.label_maintenance_roi')}
                </div>
                <div className="mt-2 flex items-center gap-3">
                  <span className="font-mono text-lg font-bold text-on-surface">
                    {financials.maintenanceRoi}%
                  </span>
                  <div className="flex-1">
                    <div className="h-2 overflow-hidden rounded-full bg-surface-container-low">
                      <div
                        className="h-full rounded-full bg-primary"
                        style={{ width: `${financials.maintenanceRoi * 5}%` }}
                      />
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Compliance Summary */}
          <div className="rounded-lg bg-surface-container p-6">
            <h2 className="mb-5 text-[10px] font-bold tracking-widest text-on-surface-variant">
              {t('asset_lifecycle_timeline.section_compliance_summary')}
            </h2>
            <div className="space-y-3">
              {compliance.map((item) => (
                <div
                  key={item.label}
                  className="flex items-center justify-between rounded bg-surface-container-low px-4 py-3"
                >
                  <span className="text-xs font-bold tracking-wider text-on-surface">
                    {item.label}
                  </span>
                  <span
                    className={`material-symbols-outlined text-lg ${item.color}`}
                  >
                    {item.icon}
                  </span>
                </div>
              ))}
            </div>
            <button onClick={() => alert('Coming Soon')} className="mt-5 w-full rounded bg-surface-container-high py-3 text-[10px] font-bold tracking-widest text-primary transition-colors hover:bg-surface-container-low">
              {t('asset_lifecycle_timeline.btn_generate_audit_report')}
            </button>
          </div>

          {/* Core Metadata */}
          <div className="rounded-lg bg-surface-container p-6">
            <h2 className="mb-5 text-[10px] font-bold tracking-widest text-on-surface-variant">
              {t('asset_lifecycle_timeline.section_core_metadata')}
            </h2>
            <div className="grid grid-cols-2 gap-4">
              {[
                { label: t('asset_lifecycle_timeline.label_serial'), value: asset.serial },
                { label: t('asset_lifecycle_timeline.label_primary_ip'), value: asset.primaryIp },
                { label: t('asset_lifecycle_timeline.label_avg_latency'), value: asset.avgLatency },
                { label: t('asset_lifecycle_timeline.label_uptime'), value: asset.uptime },
              ].map((item) => (
                <div key={item.label}>
                  <div className="text-[10px] tracking-widest text-on-surface-variant">
                    {item.label}
                  </div>
                  <div className="mt-1 font-mono text-sm text-on-surface">
                    {item.value}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
