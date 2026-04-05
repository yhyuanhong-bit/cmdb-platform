import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useAuditEvents } from '../hooks/useAudit'

/* ------------------------------------------------------------------ */
/*  Fallback Data                                                      */
/* ------------------------------------------------------------------ */

const FALLBACK_EVENT = {
  id: 'EVT-20241024-00347',
  category: 'SYSTEM ALERT',
  status: 'SUCCESSFUL',
  title: 'Configuration Update',
  subtitle:
    'Manual adjustment of resource scaling limits for production database tier assets. Change authorized under maintenance window MW-2024-1881.',
  timestamp: '2024-10-24 14:22:01.422',
  performedBy: {
    name: 'Marcus Vance',
    role: 'SENIOR INFRASTRUCTURE ARCHITECT',
    avatar: 'MV',
  },
  sourceIp: '10.244.12.89',
  origin: 'REST API',
  target: {
    id: 'SRV-PROD-001',
    model: 'PowerEdge R750',
    site: 'IDC-NORTH-01',
    rack: 'Rack A02 (U12-U14)',
  },
  systemComment:
    'Scaling policy override applied under authorized maintenance window MW-2024-1881. All configuration deltas have been validated against the approved change request CR-7721. No anomalous side-effects detected during post-apply verification.',
  metadata: {
    userAgent:
      'IronGrid-CLI/3.4.1 (Linux x86_64) libcurl/7.81.0 OpenSSL/3.0.2',
    requestId: 'req_9f84c2e1-47ab-4d02-b8e3-6c10aef77d12',
    validationHash: 'sha256:e3b0c44298fc1c149afbf4c8996fb924',
    authProvider: 'SAML 2.0 / Okta Federation',
  },
}

const FALLBACK_DIFF = [
  {
    field: 'cpu_core_limit',
    label: 'CPU Core Limit',
    prev: '4.0 vCPU',
    next: '8.0 vCPU',
  },
  {
    field: 'memory_allocation_gb',
    label: 'Memory Allocation',
    prev: '64 GB',
    next: '128 GB',
  },
  {
    field: 'scaling_strategy',
    label: 'Scaling Strategy',
    prev: 'MANUAL_STATIC',
    next: 'DYNAMIC_BURST',
  },
  {
    field: 'burst_ceiling',
    label: 'Burst Ceiling',
    prev: 'N/A',
    next: '16.0 vCPU',
  },
  {
    field: 'cooldown_period_sec',
    label: 'Cooldown Period',
    prev: '0',
    next: '120',
  },
  {
    field: 'auto_revert',
    label: 'Auto Revert',
    prev: 'false',
    next: 'true',
  },
]

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export default function AuditEventDetail() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const eventId = searchParams.get('id') ?? ''
  const [diffMode] = useState<'side-by-side' | 'inline'>('side-by-side')

  const params: Record<string, string> = {}
  if (eventId) params.target_id = eventId
  const { data: eventsResponse, isLoading } = useAuditEvents(params)
  const apiEvent = eventsResponse?.data?.[0]

  // Build event from API data or fall back to static
  const event = apiEvent ? {
    ...FALLBACK_EVENT,
    id: apiEvent.id,
    category: apiEvent.module?.toUpperCase() ?? FALLBACK_EVENT.category,
    title: `${apiEvent.action} on ${apiEvent.target_type}`,
    subtitle: `Action: ${apiEvent.action} | Module: ${apiEvent.module} | Target: ${apiEvent.target_id}`,
    timestamp: apiEvent.created_at ? new Date(apiEvent.created_at).toLocaleString() : FALLBACK_EVENT.timestamp,
    performedBy: {
      name: apiEvent.operator_id ?? FALLBACK_EVENT.performedBy.name,
      role: FALLBACK_EVENT.performedBy.role,
      avatar: (apiEvent.operator_id ?? 'SY').substring(0, 2).toUpperCase(),
    },
    target: {
      ...FALLBACK_EVENT.target,
      id: apiEvent.target_id,
    },
  } : FALLBACK_EVENT

  // Build diff lines from API diff object or fall back
  const diffLines = apiEvent?.diff && Object.keys(apiEvent.diff).length > 0
    ? Object.entries(apiEvent.diff).map(([key, val]) => ({
        field: key,
        label: key.replace(/_/g, ' '),
        prev: typeof val === 'object' && val !== null && 'old' in (val as Record<string, unknown>) ? String((val as Record<string, unknown>).old) : '—',
        next: typeof val === 'object' && val !== null && 'new' in (val as Record<string, unknown>) ? String((val as Record<string, unknown>).new) : String(val),
      }))
    : FALLBACK_DIFF

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-primary border-t-transparent" />
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-surface p-6 font-body text-on-surface">
      {/* Breadcrumb */}
      <div className="mb-6 flex items-center gap-2 text-xs tracking-widest text-on-surface-variant">
        <button
          onClick={() => navigate('/audit')}
          className="hover:text-primary transition-colors cursor-pointer"
        >
          {t('audit_event_detail.breadcrumb_audit_logs')}
        </button>
        <span className="material-symbols-outlined text-sm">chevron_right</span>
        <span className="text-on-surface">{t('audit_event_detail.breadcrumb_event_details')}</span>
      </div>

      {/* Header row */}
      <div className="mb-6 flex flex-wrap items-start justify-between gap-4">
        <div className="flex-1">
          <div className="mb-3 flex flex-wrap items-center gap-3">
            <span className="rounded bg-surface-container-high px-3 py-1 text-[10px] font-bold tracking-widest text-on-surface-variant">
              {event.category}
            </span>
            <span className="rounded bg-[#69db7c]/15 px-3 py-1 text-[10px] font-bold tracking-widest text-[#69db7c]">
              {event.status}
            </span>
            <span className="text-[10px] tracking-widest text-on-surface-variant">
              {event.id}
            </span>
          </div>
          <h1 className="font-headline text-3xl font-bold text-on-surface">
            {event.title}
          </h1>
          <p className="mt-2 max-w-2xl text-sm leading-relaxed text-on-surface-variant">
            {event.subtitle}
          </p>
        </div>
        <button onClick={() => alert('Export: Coming Soon')} className="flex items-center gap-2 rounded bg-surface-container-high px-5 py-2.5 text-xs font-bold tracking-widest text-on-surface transition-colors hover:bg-surface-container">
          <span className="material-symbols-outlined text-base">download</span>
          {t('audit_event_detail.btn_export_log')}
        </button>
      </div>

      {/* Main content grid */}
      <div className="grid gap-6 lg:grid-cols-2">
        {/* ---- Left Panel ---- */}
        <div className="space-y-6">
          {/* Core Telemetry */}
          <div className="rounded-lg bg-surface-container p-6">
            <h2 className="mb-5 text-[10px] font-bold tracking-widest text-on-surface-variant">
              {t('audit_event_detail.section_core_telemetry')}
            </h2>
            <div className="space-y-5">
              {/* Timestamp */}
              <div>
                <div className="text-[10px] tracking-widest text-on-surface-variant">
                  {t('audit_event_detail.label_timestamp_utc')}
                </div>
                <div className="mt-1 font-mono text-sm text-on-surface">
                  {event.timestamp}
                </div>
              </div>
              {/* Performed By */}
              <div>
                <div className="text-[10px] tracking-widest text-on-surface-variant">
                  {t('audit_event_detail.label_performed_by')}
                </div>
                <div className="mt-2 flex items-center gap-3">
                  <div className="flex h-9 w-9 items-center justify-center rounded-full bg-primary/20 text-xs font-bold text-primary">
                    {event.performedBy.avatar}
                  </div>
                  <div>
                    <div className="text-sm font-semibold text-on-surface">
                      {event.performedBy.name}
                    </div>
                    <div className="text-[10px] tracking-widest text-on-surface-variant">
                      {event.performedBy.role}
                    </div>
                  </div>
                </div>
              </div>
              {/* Source IP & Origin */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <div className="text-[10px] tracking-widest text-on-surface-variant">
                    {t('audit_event_detail.label_source_ip')}
                  </div>
                  <div className="mt-1 font-mono text-sm text-on-surface">
                    {event.sourceIp}
                  </div>
                </div>
                <div>
                  <div className="text-[10px] tracking-widest text-on-surface-variant">
                    {t('audit_event_detail.label_origin')}
                  </div>
                  <div className="mt-1 flex items-center gap-2 text-sm text-on-surface">
                    <span className="material-symbols-outlined text-base text-primary">
                      api
                    </span>
                    {event.origin}
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Target Asset */}
          <div className="rounded-lg bg-surface-container p-6">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-[10px] font-bold tracking-widest text-on-surface-variant">
                {t('audit_event_detail.section_target_asset')}
              </h2>
              <button
                onClick={() => navigate('/assets/detail')}
                className="flex items-center gap-1 text-[10px] font-bold tracking-widest text-primary hover:underline cursor-pointer"
              >
                {t('audit_event_detail.btn_view_details')}
                <span className="material-symbols-outlined text-sm">
                  arrow_forward
                </span>
              </button>
            </div>
            <div className="mb-4 flex items-center gap-3">
              <span className="material-symbols-outlined text-2xl text-primary">
                dns
              </span>
              <span className="font-mono text-lg font-bold text-on-surface">
                {event.target.id}
              </span>
            </div>
            <div className="grid grid-cols-2 gap-y-4 gap-x-6">
              <div>
                <div className="text-[10px] tracking-widest text-on-surface-variant">
                  {t('audit_event_detail.label_hardware_model')}
                </div>
                <div className="mt-1 text-sm text-on-surface">
                  {event.target.model}
                </div>
              </div>
              <div>
                <div className="text-[10px] tracking-widest text-on-surface-variant">
                  {t('audit_event_detail.label_site_location')}
                </div>
                <div className="mt-1 text-sm text-on-surface">
                  {event.target.site}
                </div>
              </div>
              <div className="col-span-2">
                <div className="text-[10px] tracking-widest text-on-surface-variant">
                  {t('audit_event_detail.label_rack_position')}
                </div>
                <div className="mt-1 text-sm text-on-surface">
                  {event.target.rack}
                </div>
              </div>
            </div>
          </div>

          {/* System Comments */}
          <div className="rounded-lg bg-surface-container p-6">
            <h2 className="mb-4 text-[10px] font-bold tracking-widest text-on-surface-variant">
              {t('audit_event_detail.section_system_comments')}
            </h2>
            <div className="rounded bg-surface-container-low p-4 text-sm leading-relaxed text-on-surface-variant italic border-l-2 border-primary/40">
              &ldquo;{event.systemComment}&rdquo;
            </div>
          </div>
        </div>

        {/* ---- Right Panel ---- */}
        <div className="space-y-6">
          {/* Configuration Delta */}
          <div className="rounded-lg bg-surface-container p-6">
            <div className="mb-1 flex items-center gap-2">
              <span className="material-symbols-outlined text-base text-primary">
                compare_arrows
              </span>
              <h2 className="text-[10px] font-bold tracking-widest text-on-surface-variant">
                {t('audit_event_detail.section_config_delta')}
              </h2>
            </div>
            <p className="mb-5 text-xs text-on-surface-variant">
              {t('audit_event_detail.config_delta_subtitle')}
            </p>

            {/* Diff controls */}
            <div className="mb-5 flex items-center justify-between">
              <div className="flex gap-4 text-[10px] tracking-widest text-on-surface-variant">
                <span>
                  {t('audit_event_detail.label_lines')}: <span className="text-on-surface font-bold">12</span>
                </span>
                <span>
                  {t('audit_event_detail.label_mode')}:{' '}
                  <span className="text-on-surface font-bold">
                    {diffMode === 'side-by-side' ? t('audit_event_detail.mode_side_by_side') : t('audit_event_detail.mode_inline')}
                  </span>
                </span>
              </div>
            </div>

            {/* Diff table */}
            <div className="rounded-lg bg-surface-container-low overflow-hidden">
              {/* Header */}
              <div className="grid grid-cols-3 gap-px">
                <div className="bg-surface-container-high px-4 py-2 text-[10px] font-bold tracking-widest text-on-surface-variant">
                  {t('audit_event_detail.table_field')}
                </div>
                <div className="bg-surface-container-high px-4 py-2 text-[10px] font-bold tracking-widest text-error/80">
                  {t('audit_event_detail.table_previous_state')}
                </div>
                <div className="bg-surface-container-high px-4 py-2 text-[10px] font-bold tracking-widest text-[#69db7c]/80">
                  {t('audit_event_detail.table_updated_state')}
                </div>
              </div>
              {/* Rows */}
              {diffLines.map((line) => (
                <div
                  key={line.field}
                  className="grid grid-cols-3 gap-px"
                >
                  <div className="bg-surface-container px-4 py-3 font-mono text-xs text-on-surface-variant">
                    {line.label}
                  </div>
                  <div className="bg-error/5 px-4 py-3 font-mono text-xs text-error">
                    - {line.prev}
                  </div>
                  <div className="bg-[#69db7c]/5 px-4 py-3 font-mono text-xs text-[#69db7c]">
                    + {line.next}
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Source Metadata & Headers */}
          <div className="rounded-lg bg-surface-container p-6">
            <h2 className="mb-5 text-[10px] font-bold tracking-widest text-on-surface-variant">
              {t('audit_event_detail.section_source_metadata')}
            </h2>
            <div className="space-y-4">
              {[
                {
                  icon: 'devices',
                  label: t('audit_event_detail.label_user_agent'),
                  value: event.metadata.userAgent,
                },
                {
                  icon: 'fingerprint',
                  label: t('audit_event_detail.label_request_id'),
                  value: event.metadata.requestId,
                },
                {
                  icon: 'verified',
                  label: t('audit_event_detail.label_validation_hash'),
                  value: event.metadata.validationHash,
                },
                {
                  icon: 'shield',
                  label: t('audit_event_detail.label_auth_provider'),
                  value: event.metadata.authProvider,
                },
              ].map((item) => (
                <div key={item.label}>
                  <div className="flex items-center gap-2 text-[10px] tracking-widest text-on-surface-variant">
                    <span className="material-symbols-outlined text-sm text-primary">
                      {item.icon}
                    </span>
                    {item.label}
                  </div>
                  <div className="mt-1 break-all font-mono text-xs text-on-surface">
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
