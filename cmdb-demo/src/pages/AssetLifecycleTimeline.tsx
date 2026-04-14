import { toast } from 'sonner'
import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useAsset, useAssetLifecycle } from '../hooks/useAssets'
import { ASSET_FALLBACK, TIMELINE_STAGES, COMPLIANCE } from '../data/fallbacks/lifecycle'
import type { LifecycleEvent } from '../lib/api/assets'

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

// Map an API lifecycle event type to an icon name and color class.
function eventIconAndColor(evt: LifecycleEvent): { icon: string; color: string } {
  switch (evt.type) {
    case 'created': return { icon: 'rocket_launch', color: 'text-primary' }
    case 'status_change': return { icon: 'sync_alt', color: 'text-[#818cf8]' }
    case 'warranty_start': return { icon: 'shield', color: 'text-[#34d399]' }
    case 'warranty_end': return { icon: 'shield_with_heart', color: 'text-[#fbbf24]' }
    case 'eol': return { icon: 'power_off', color: 'text-error' }
    case 'deleted': return { icon: 'delete', color: 'text-error' }
    default: return { icon: 'edit', color: 'text-on-surface-variant' }
  }
}

function eventDotColor(evt: LifecycleEvent): string {
  switch (evt.type) {
    case 'created': return 'bg-primary'
    case 'status_change': return 'bg-[#818cf8]'
    case 'warranty_start': return 'bg-[#34d399]'
    case 'warranty_end': return 'bg-[#fbbf24]'
    case 'eol': return 'bg-error'
    case 'deleted': return 'bg-error'
    default: return 'bg-on-surface-variant'
  }
}

export default function AssetLifecycleTimeline() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { assetId } = useParams<{ assetId: string }>()
  const [expandedIndex, setExpandedIndex] = useState<number | null>(null)

  // Fetch asset basic info and lifecycle timeline
  const assetQ = useAsset(assetId ?? '')
  const lifecycleQ = useAssetLifecycle(assetId ?? '')
  const apiAsset = assetQ.data?.data
  const lifecycleData = lifecycleQ.data?.data
  const summary = lifecycleData?.summary

  const asset = {
    ...ASSET_FALLBACK,
    id: apiAsset?.asset_tag ?? assetId ?? ASSET_FALLBACK.id,
    status: apiAsset?.status?.toUpperCase() ?? ASSET_FALLBACK.status,
    serial: apiAsset?.serial_number ?? ASSET_FALLBACK.serial,
    primaryIp: (apiAsset?.attributes?.primary_ip as string) ?? ASSET_FALLBACK.primaryIp,
    avgLatency: (apiAsset?.attributes?.avg_latency as string) ?? ASSET_FALLBACK.avgLatency,
    uptime: (apiAsset?.attributes?.uptime as string) ?? ASSET_FALLBACK.uptime,
  }

  // Use real timeline events when available, fall back to TIMELINE_STAGES for demo
  const hasRealTimeline = (lifecycleData?.timeline?.length ?? 0) > 0
  const useRealTimeline = hasRealTimeline

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
            {useRealTimeline ? (
              /* Real timeline from audit_events + warranty milestones */
              lifecycleData!.timeline.map((evt, idx) => {
                const { icon, color } = eventIconAndColor(evt)
                const dotColor = eventDotColor(evt)
                const isExpanded = expandedIndex === idx
                const dateLabel = new Date(evt.date).toLocaleDateString()
                return (
                  <div key={idx} className="relative pb-8 last:pb-0">
                    {idx < lifecycleData!.timeline.length - 1 && (
                      <div className="absolute left-[-20px] top-4 h-full w-0.5 bg-surface-container-high" />
                    )}
                    <div className={`absolute left-[-24px] top-1 h-3 w-3 rounded-full ${dotColor}`} />
                    <div
                      className={`rounded-lg p-4 transition-colors cursor-pointer ${
                        isExpanded ? 'bg-surface-container-high' : 'bg-surface-container-low hover:bg-surface-container-high'
                      }`}
                      onClick={() => setExpandedIndex(isExpanded ? null : idx)}
                    >
                      <div className="flex flex-wrap items-center gap-2">
                        <span className={`material-symbols-outlined text-base ${color}`}>{icon}</span>
                        <span className="text-xs font-bold tracking-widest text-on-surface">
                          {evt.description ?? evt.action}
                        </span>
                        {evt.from_status && evt.to_status && (
                          <span className="text-[10px] font-mono text-on-surface-variant">
                            {evt.from_status} → {evt.to_status}
                          </span>
                        )}
                      </div>
                      <div className="mt-2 flex flex-wrap items-center gap-4 text-[10px] tracking-widest text-on-surface-variant">
                        <span className="flex items-center gap-1">
                          <span className="material-symbols-outlined text-xs">calendar_today</span>
                          {dateLabel}
                        </span>
                        {evt.operator_id && (
                          <span className="flex items-center gap-1">
                            <span className="material-symbols-outlined text-xs">person</span>
                            {evt.operator_id.slice(0, 8)}…
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                )
              })
            ) : (
              /* Fallback demo timeline when no real events exist */
              TIMELINE_STAGES.map((stage, idx) => (
                <div key={stage.id} className="relative pb-8 last:pb-0">
                  {idx < TIMELINE_STAGES.length - 1 && (
                    <div className={`absolute left-[-20px] top-4 h-full w-0.5 ${stage.lineColor}`} />
                  )}
                  <div
                    className={`absolute left-[-24px] top-1 h-3 w-3 rounded-full ${stage.dotColor} ${
                      stage.highlighted ? 'ring-4 ring-[#ffa94d]/20' : ''
                    }`}
                  />
                  <div
                    className={`rounded-lg p-4 transition-colors cursor-pointer ${
                      stage.highlighted
                        ? 'bg-[#ffa94d]/5'
                        : expandedIndex === stage.id
                          ? 'bg-surface-container-high'
                          : 'bg-surface-container-low hover:bg-surface-container-high'
                    }`}
                    onClick={() => setExpandedIndex(expandedIndex === stage.id ? null : stage.id)}
                  >
                    <div className="flex flex-wrap items-center gap-3">
                      <span className="text-xs font-bold tracking-widest text-on-surface">
                        {t(stage.phaseKey)}
                      </span>
                      <span className={`text-[10px] font-bold tracking-widest ${stage.statusColor}`}>
                        {t(stage.statusKey)}
                      </span>
                    </div>
                    <div className="mt-2 flex flex-wrap items-center gap-4 text-[10px] tracking-widest text-on-surface-variant">
                      <span className="flex items-center gap-1">
                        <span className="material-symbols-outlined text-xs">calendar_today</span>
                        {stage.date}
                      </span>
                      <span className="flex items-center gap-1">
                        <span className="material-symbols-outlined text-xs">person</span>
                        {stage.technician}
                      </span>
                    </div>
                    {expandedIndex === stage.id && (
                      <div className="mt-3 space-y-3">
                        <p className="text-sm leading-relaxed text-on-surface-variant">
                          {stage.description}
                        </p>
                        {stage.costEstimate && (
                          <div className="text-xs text-on-surface-variant">
                            {t('asset_lifecycle_timeline.label_cost_estimate')}:{' '}
                            <span className="font-mono text-primary">{stage.costEstimate}</span>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                </div>
              ))
            )}
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
                  {summary?.purchase_cost != null
                    ? `$${Number(summary.purchase_cost).toLocaleString()}`
                    : '—'}
                </div>
              </div>
              {summary?.warranty_vendor && (
                <div>
                  <div className="text-[10px] tracking-widest text-on-surface-variant">
                    {t('asset_lifecycle_timeline.label_warranty_vendor', 'Warranty Vendor')}
                  </div>
                  <div className="mt-1 font-mono text-sm font-bold text-on-surface">
                    {summary.warranty_vendor}
                  </div>
                </div>
              )}
              {summary?.warranty_end && (
                <div>
                  <div className="text-[10px] tracking-widest text-on-surface-variant">
                    {t('asset_lifecycle_timeline.label_warranty_end', 'Warranty Expires')}
                  </div>
                  <div className="mt-1 font-mono text-sm font-bold text-[#fbbf24]">
                    {summary.warranty_end}
                  </div>
                </div>
              )}
              {summary?.eol_date && (
                <div>
                  <div className="text-[10px] tracking-widest text-on-surface-variant">
                    {t('asset_lifecycle_timeline.label_eol_date', 'End of Life')}
                  </div>
                  <div className="mt-1 font-mono text-sm font-bold text-error">
                    {summary.eol_date}
                  </div>
                </div>
              )}
              {!summary && (
                <p className="text-[10px] text-on-surface-variant italic">No financial data available.</p>
              )}
            </div>
          </div>

          {/* Compliance Summary */}
          <div className="rounded-lg bg-surface-container p-6">
            <h2 className="mb-5 text-[10px] font-bold tracking-widest text-on-surface-variant">
              {t('asset_lifecycle_timeline.section_compliance_summary')}
            </h2>
            <div className="space-y-3">
              {COMPLIANCE.map((item) => (
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
            <button onClick={() => toast.info('Coming Soon')} className="mt-5 w-full rounded bg-surface-container-high py-3 text-[10px] font-bold tracking-widest text-primary transition-colors hover:bg-surface-container-low">
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
