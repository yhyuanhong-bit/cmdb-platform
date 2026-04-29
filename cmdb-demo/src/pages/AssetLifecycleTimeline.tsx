import { toast } from 'sonner'
import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useAsset, useAssetLifecycle, useAssetComplianceScan } from '../hooks/useAssets'
import EmptyState from '../components/EmptyState'
import type { LifecycleEvent } from '../lib/api/assets'

const EMPTY = '—'

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
  const complianceQ = useAssetComplianceScan(assetId ?? '')
  const apiAsset = assetQ.data?.data
  const lifecycleData = lifecycleQ.data?.data
  const summary = lifecycleData?.summary
  const complianceData = complianceQ.data?.data
  const complianceEvents = complianceData?.events ?? []
  const lastScanAt = complianceData?.last_scan_at ?? null

  const asset = {
    id: apiAsset?.asset_tag ?? assetId ?? EMPTY,
    status: apiAsset?.status?.toUpperCase() ?? EMPTY,
    lastSync: EMPTY,
    serial: apiAsset?.serial_number ?? EMPTY,
    primaryIp: (apiAsset?.attributes?.primary_ip as string) ?? EMPTY,
    avgLatency: (apiAsset?.attributes?.avg_latency as string) ?? EMPTY,
    uptime: (apiAsset?.attributes?.uptime as string) ?? EMPTY,
  }

  // Render timeline strictly from real audit_events + warranty milestones.
  // No fabricated TIMELINE_STAGES — when the API returns no events we show
  // an empty state.
  const hasRealTimeline = (lifecycleData?.timeline?.length ?? 0) > 0

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
            {hasRealTimeline ? (
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
              /* No real events yet — show empty state instead of a fake
                 "Procurement → Installation → Maintenance" timeline. */
              <EmptyState
                icon="timeline"
                title={t('common.empty_no_data_title')}
                description={t('common.empty_no_data_desc')}
                tone="neutral"
              />
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

          {/* Compliance Summary — last scan-style audit activity for this asset. */}
          <div className="rounded-lg bg-surface-container p-6">
            <h2 className="mb-5 text-[10px] font-bold tracking-widest text-on-surface-variant">
              {t('asset_lifecycle_timeline.section_compliance_summary')}
            </h2>
            {complianceQ.isLoading ? (
              <div className="flex items-center justify-center py-6">
                <div className="animate-spin rounded-full h-5 w-5 border-2 border-sky-400 border-t-transparent" />
              </div>
            ) : complianceEvents.length === 0 ? (
              <EmptyState
                icon="verified"
                title={t('asset_lifecycle_timeline.compliance_empty_title')}
                description={t('asset_lifecycle_timeline.compliance_empty_desc')}
                tone="neutral"
                compact
              />
            ) : (
              <div className="space-y-3">
                {lastScanAt && (
                  <div className="flex items-center justify-between text-[10px] tracking-widest text-on-surface-variant">
                    <span>{t('asset_lifecycle_timeline.compliance_last_scan')}</span>
                    <span className="font-mono text-on-surface">
                      {new Date(lastScanAt).toLocaleString()}
                    </span>
                  </div>
                )}
                <ul className="divide-y divide-outline-variant">
                  {complianceEvents.slice(0, 5).map((evt, idx) => (
                    <li key={`${evt.scanned_at}-${idx}`} className="py-2">
                      <div className="flex items-center justify-between text-xs">
                        <span className="font-mono text-on-surface">{evt.action}</span>
                        <span className="text-[10px] text-on-surface-variant">
                          {new Date(evt.scanned_at).toLocaleDateString()}
                        </span>
                      </div>
                      {(evt.module || evt.source) && (
                        <div className="mt-1 text-[10px] uppercase tracking-widest text-on-surface-variant">
                          {[evt.module, evt.source].filter(Boolean).join(' · ')}
                        </div>
                      )}
                    </li>
                  ))}
                </ul>
              </div>
            )}
            <button
              onClick={() => toast.info(t('common.coming_soon', 'Coming Soon'))}
              className="mt-5 w-full rounded bg-surface-container-high py-3 text-[10px] font-bold tracking-widest text-primary transition-colors hover:bg-surface-container-low"
            >
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
