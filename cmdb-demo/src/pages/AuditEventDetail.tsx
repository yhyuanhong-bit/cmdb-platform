import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useAuditEventDetail } from '../hooks/useAudit'


/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export default function AuditEventDetail() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const eventId = searchParams.get('id') ?? ''
  const [diffMode, setDiffMode] = useState<'side-by-side' | 'inline'>('side-by-side')

  const { data: eventData, isLoading } = useAuditEventDetail(eventId)
  const apiEvent = eventData?.event

  // Build display event from API data
  const displayEvent = apiEvent ? {
    id: apiEvent.id,
    category: apiEvent.module,
    status: 'SUCCESSFUL',
    title: `${apiEvent.module}.${apiEvent.action}`,
    subtitle: `${apiEvent.target_type} ${apiEvent.action}`,
    timestamp: apiEvent.created_at,
    performedBy: { name: apiEvent.operator_name || 'System', role: 'Operator' },
    sourceIp: '-',
    origin: apiEvent.source || 'web',
    target: { id: apiEvent.target_id, type: apiEvent.target_type },
    systemComment: apiEvent.systemComment,
    metadata: apiEvent.metadata,
  } : null

  // Build diff from API data
  const displayDiff = apiEvent?.diff
    ? (typeof apiEvent.diff === 'string' ? JSON.parse(apiEvent.diff) : apiEvent.diff)
    : {}
  const diffLines = Object.entries(displayDiff).map(([field, val]: [string, any]) => ({
    field,
    label: field.replace(/_/g, ' '),
    prev: val?.old ?? '-',
    next: val?.new ?? '-',
  }))

  const event = displayEvent

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-primary border-t-transparent" />
      </div>
    )
  }

  if (!event) {
    return (
      <div className="flex flex-col items-center justify-center py-20 gap-3 text-on-surface-variant">
        <span className="material-symbols-outlined text-4xl opacity-30">event_note</span>
        <p className="text-sm">{eventId ? 'Event not found.' : 'No event ID specified.'}</p>
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
        <button onClick={() => {
          const data = JSON.stringify(event || {}, null, 2)
          const blob = new Blob([data], { type: 'application/json' })
          const url = URL.createObjectURL(blob)
          const a = document.createElement('a')
          a.href = url
          a.download = `audit-event-${event?.id || 'unknown'}.json`
          a.click()
          URL.revokeObjectURL(url)
        }} className="flex items-center gap-2 rounded bg-surface-container-high px-5 py-2.5 text-xs font-bold tracking-widest text-on-surface transition-colors hover:bg-surface-container">
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
                    {event.performedBy.name.substring(0, 2).toUpperCase()}
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
              <div className="col-span-2">
                <div className="text-[10px] tracking-widest text-on-surface-variant">
                  {t('audit_event_detail.label_hardware_model')}
                </div>
                <div className="mt-1 text-sm text-on-surface">
                  {event.target.type || '-'}
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
              &ldquo;{event.systemComment || `${event.title} recorded at ${event.timestamp}.`}&rdquo;
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
                  {t('audit_event_detail.label_lines')}: <span className="text-on-surface font-bold">{diffLines.length}</span>
                </span>
                <span>
                  {t('audit_event_detail.label_mode')}:{' '}
                  <span className="text-on-surface font-bold">
                    {diffMode === 'side-by-side' ? t('audit_event_detail.mode_side_by_side') : t('audit_event_detail.mode_inline')}
                  </span>
                </span>
              </div>
              <button onClick={() => setDiffMode(m => m === 'side-by-side' ? 'inline' : 'side-by-side')}
                className="px-3 py-1.5 rounded-lg bg-surface-container-high text-xs text-on-surface-variant hover:text-on-surface transition-colors">
                {diffMode === 'side-by-side' ? 'Inline View' : 'Side-by-Side'}
              </button>
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
                  value: event.metadata?.userAgent || '-',
                },
                {
                  icon: 'fingerprint',
                  label: t('audit_event_detail.label_request_id'),
                  value: event.metadata?.requestId || event.id,
                },
                {
                  icon: 'verified',
                  label: t('audit_event_detail.label_validation_hash'),
                  value: event.metadata?.validationHash || '-',
                },
                {
                  icon: 'shield',
                  label: t('audit_event_detail.label_auth_provider'),
                  value: event.metadata?.authProvider || '-',
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
