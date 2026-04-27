import { useCallback, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import Icon from '../components/Icon'
import {
  useIncident,
  useIncidentComments,
  useAcknowledgeIncident,
  useStartInvestigatingIncident,
  useResolveIncident,
  useCloseIncident,
  useReopenIncident,
  useAddIncidentComment,
} from '../hooks/useMonitoring'
import type { IncidentStatus } from '../lib/api/monitoring'

/* ------------------------------------------------------------------ */
/*  Status & priority styling                                          */
/* ------------------------------------------------------------------ */

const statusStyle: Record<IncidentStatus, string> = {
  open:          'bg-red-500/20 text-red-400',
  acknowledged:  'bg-blue-500/20 text-blue-400',
  investigating: 'bg-amber-500/20 text-amber-400',
  resolved:      'bg-emerald-500/20 text-emerald-400',
  closed:        'bg-surface-container-highest text-on-surface-variant',
}

const severityStyle: Record<string, string> = {
  critical: 'bg-error-container text-on-error-container',
  high:     'bg-[#92400e] text-[#fbbf24]',
  warning:  'bg-amber-500/20 text-amber-400',
  medium:   'bg-blue-500/20 text-blue-400',
  low:      'bg-[#1e3a5f] text-on-primary-container',
  info:     'bg-surface-container-highest text-on-surface-variant',
}

const priorityStyle: Record<string, string> = {
  p1: 'bg-error-container text-on-error-container',
  p2: 'bg-[#92400e] text-[#fbbf24]',
  p3: 'bg-blue-500/20 text-blue-400',
  p4: 'bg-surface-container-highest text-on-surface-variant',
}

/* ------------------------------------------------------------------ */
/*  Action availability per status                                     */
/* ------------------------------------------------------------------ */

// State machine snapshot. Match the backend's WHERE-status guards in
// queries/incidents.sql so the UI never offers a transition the server
// will reject. Keep this map next to the styling so any future state
// addition forces a review here.
type LifecycleAction = 'acknowledge' | 'investigate' | 'resolve' | 'close' | 'reopen'

const allowedActions: Record<IncidentStatus, LifecycleAction[]> = {
  open:          ['acknowledge', 'resolve'],
  acknowledged:  ['investigate', 'resolve'],
  investigating: ['resolve'],
  resolved:      ['close', 'reopen'],
  closed:        [],
}

/* ------------------------------------------------------------------ */
/*  Lifecycle action button                                            */
/* ------------------------------------------------------------------ */

interface ActionButtonProps {
  label: string
  icon: string
  onClick: () => void
  disabled?: boolean
  variant?: 'primary' | 'success' | 'warning' | 'danger' | 'neutral'
}

function ActionButton({ label, icon, onClick, disabled, variant = 'primary' }: ActionButtonProps) {
  const variantClass = {
    primary:  'bg-primary text-on-primary hover:opacity-90',
    success:  'bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30',
    warning:  'bg-amber-500/20 text-amber-400 hover:bg-amber-500/30',
    danger:   'bg-red-500/20 text-red-400 hover:bg-red-500/30',
    neutral:  'bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest',
  }[variant]

  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={`flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-semibold uppercase tracking-wider transition-colors disabled:opacity-40 disabled:cursor-not-allowed ${variantClass}`}
    >
      <Icon name={icon} className="text-[16px]" />
      {label}
    </button>
  )
}

/* ------------------------------------------------------------------ */
/*  Resolve dialog (captures root_cause)                               */
/* ------------------------------------------------------------------ */

function ResolveDialog({
  open,
  onClose,
  onSubmit,
  submitting,
}: {
  open: boolean
  onClose: () => void
  onSubmit: (rootCause: string, note: string) => void
  submitting: boolean
}) {
  const { t } = useTranslation()
  const [rootCause, setRootCause] = useState('')
  const [note, setNote] = useState('')
  if (!open) return null
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" role="dialog" aria-modal="true">
      <div className="bg-surface-container rounded-lg max-w-lg w-full p-6 shadow-xl">
        <h2 className="font-headline font-bold text-lg text-on-surface mb-2">
          {t('incident_detail.dialog_resolve_title')}
        </h2>
        <p className="text-sm text-on-surface-variant mb-4">
          {t('incident_detail.dialog_resolve_desc')}
        </p>
        <label className="block text-xs text-on-surface-variant mb-1">
          {t('incident_detail.field_root_cause')}
        </label>
        <textarea
          value={rootCause}
          onChange={(e) => setRootCause(e.target.value)}
          rows={3}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-3 outline-none resize-none mb-3"
          placeholder={t('incident_detail.dialog_resolve_placeholder_root')}
        />
        <label className="block text-xs text-on-surface-variant mb-1">
          {t('incident_detail.dialog_resolve_note')}
        </label>
        <textarea
          value={note}
          onChange={(e) => setNote(e.target.value)}
          rows={2}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-3 outline-none resize-none"
          placeholder={t('incident_detail.dialog_resolve_placeholder_note')}
        />
        <div className="flex justify-end gap-2 mt-4">
          <button
            onClick={onClose}
            disabled={submitting}
            className="px-4 py-2 rounded-lg text-sm text-on-surface-variant hover:bg-surface-container-high transition-colors"
          >
            {t('common.cancel')}
          </button>
          <button
            onClick={() => onSubmit(rootCause.trim(), note.trim())}
            disabled={submitting}
            className="px-4 py-2 rounded-lg text-sm font-semibold bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30 transition-colors disabled:opacity-40"
          >
            {submitting ? t('common.saving') : t('incident_detail.dialog_resolve_confirm')}
          </button>
        </div>
      </div>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Timeline                                                           */
/* ------------------------------------------------------------------ */

function Timeline({ id }: { id: string }) {
  const { t } = useTranslation()
  const { data, isLoading, error } = useIncidentComments(id)
  const addComment = useAddIncidentComment()
  const [body, setBody] = useState('')

  const onAdd = () => {
    const trimmed = body.trim()
    if (!trimmed) return
    addComment.mutate({ id, body: trimmed }, {
      onSuccess: () => {
        setBody('')
        toast.success(t('incident_detail.toast_comment_added'))
      },
      onError: (e: unknown) => toast.error(e instanceof Error ? e.message : t('common.unknown_error')),
    })
  }

  const comments = data?.data ?? []

  return (
    <section className="bg-surface-container rounded-lg p-5">
      <h2 className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant mb-4">
        {t('incident_detail.section_timeline')}
      </h2>

      {isLoading && (
        <div className="py-6 flex justify-center">
          <div className="animate-spin rounded-full h-5 w-5 border-2 border-sky-400 border-t-transparent" />
        </div>
      )}
      {error && (
        <p className="text-sm text-red-300">{t('incident_detail.timeline_error')}</p>
      )}
      {!isLoading && !error && comments.length === 0 && (
        <p className="text-xs text-on-surface-variant">{t('incident_detail.timeline_empty')}</p>
      )}
      {comments.length > 0 && (
        <ol className="space-y-3 mb-4">
          {comments.map((c) => (
            <li key={c.id} className="flex items-start gap-3">
              <span
                className={`shrink-0 w-7 h-7 rounded-full flex items-center justify-center ${
                  c.kind === 'system'
                    ? 'bg-surface-container-high text-on-surface-variant'
                    : 'bg-primary/20 text-primary'
                }`}
              >
                <Icon name={c.kind === 'system' ? 'auto_awesome' : 'person'} className="text-[14px]" />
              </span>
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2 text-xs">
                  <span className={c.kind === 'system' ? 'text-on-surface-variant italic' : 'text-on-surface font-semibold'}>
                    {c.kind === 'system' ? t('incident_detail.system') : (c.author_username ?? t('incident_detail.unknown_user'))}
                  </span>
                  <span className="text-on-surface-variant">{new Date(c.created_at).toLocaleString()}</span>
                </div>
                <pre className="mt-1 whitespace-pre-wrap font-body text-sm text-on-surface">{c.body}</pre>
              </div>
            </li>
          ))}
        </ol>
      )}

      <div className="border-t border-surface-container-high pt-3">
        <textarea
          value={body}
          onChange={(e) => setBody(e.target.value)}
          rows={3}
          placeholder={t('incident_detail.add_comment_placeholder')}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-3 outline-none resize-none"
        />
        <div className="flex justify-end mt-2">
          <button
            onClick={onAdd}
            disabled={addComment.isPending || body.trim() === ''}
            className="px-4 py-2 rounded-lg text-xs font-semibold uppercase tracking-wider bg-primary text-on-primary hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed"
          >
            {addComment.isPending ? t('common.saving') : t('incident_detail.btn_add_comment')}
          </button>
        </div>
      </div>
    </section>
  )
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function IncidentDetail() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const incidentQ = useIncident(id)
  const ack = useAcknowledgeIncident()
  const start = useStartInvestigatingIncident()
  const resolve = useResolveIncident()
  const close = useCloseIncident()
  const reopen = useReopenIncident()

  const [resolveOpen, setResolveOpen] = useState(false)

  const submitting = ack.isPending || start.isPending || resolve.isPending || close.isPending || reopen.isPending

  const handleErr = useCallback((e: unknown) => {
    if (e instanceof Error && e.message.toLowerCase().includes('invalid_transition')) {
      toast.error(t('incident_detail.toast_invalid_transition'))
      return
    }
    toast.error(e instanceof Error ? e.message : t('common.unknown_error'))
  }, [t])

  if (incidentQ.isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-sky-400 border-t-transparent" />
      </div>
    )
  }

  if (incidentQ.error || !incidentQ.data?.data) {
    return (
      <div className="p-6">
        <div className="rounded-lg bg-red-900/20 p-4 text-red-300 text-sm">
          {t('incident_detail.load_failed')}{' '}
          <button onClick={() => incidentQ.refetch()} className="underline">{t('common.retry')}</button>
        </div>
      </div>
    )
  }

  const incident = incidentQ.data.data
  const actions = allowedActions[incident.status] ?? []

  if (!id) return null

  const onAck = () => ack.mutate({ id }, {
    onSuccess: () => toast.success(t('incident_detail.toast_acknowledged')),
    onError: handleErr,
  })
  const onStart = () => start.mutate(id, {
    onSuccess: () => toast.success(t('incident_detail.toast_investigating')),
    onError: handleErr,
  })
  const onResolveSubmit = (rootCause: string, note: string) => resolve.mutate(
    { id, rootCause: rootCause || undefined, note: note || undefined },
    {
      onSuccess: () => {
        toast.success(t('incident_detail.toast_resolved'))
        setResolveOpen(false)
      },
      onError: handleErr,
    },
  )
  const onClose = () => close.mutate(id, {
    onSuccess: () => toast.success(t('incident_detail.toast_closed')),
    onError: handleErr,
  })
  const onReopen = () => {
    const reason = window.prompt(t('incident_detail.prompt_reopen_reason')) ?? ''
    reopen.mutate({ id, reason: reason.trim() || undefined }, {
      onSuccess: () => toast.success(t('incident_detail.toast_reopened')),
      onError: handleErr,
    })
  }

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      <header className="px-8 pt-6 pb-4">
        <button
          onClick={() => navigate('/monitoring')}
          className="flex items-center gap-1 text-sm text-on-surface-variant hover:text-primary transition-colors mb-3"
        >
          <Icon name="arrow_back" className="text-[18px]" />
          {t('incident_detail.back_to_monitoring')}
        </button>

        <div className="flex items-start justify-between gap-6 flex-wrap">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2 flex-wrap mb-2">
              <span className={`px-2.5 py-1 rounded text-[0.6875rem] font-semibold uppercase tracking-wider ${statusStyle[incident.status]}`}>
                {incident.status}
              </span>
              <span className={`px-2.5 py-1 rounded text-[0.6875rem] font-semibold uppercase tracking-wider ${severityStyle[incident.severity] ?? severityStyle.info}`}>
                {incident.severity}
              </span>
              {incident.priority && (
                <span className={`px-2.5 py-1 rounded text-[0.6875rem] font-semibold uppercase tracking-wider ${priorityStyle[incident.priority]}`}>
                  {incident.priority}
                </span>
              )}
            </div>
            <h1 className="font-headline font-bold text-2xl text-on-surface leading-tight break-words">
              {incident.title}
            </h1>
            <p className="text-xs text-on-surface-variant mt-1">
              {t('incident_detail.started_at')} {new Date(incident.started_at).toLocaleString()}
              {incident.resolved_at && (
                <> · {t('incident_detail.resolved_at')} {new Date(incident.resolved_at).toLocaleString()}</>
              )}
            </p>
          </div>

          {/* Lifecycle action bar */}
          <div className="flex items-center gap-2 flex-wrap">
            {actions.includes('acknowledge') && (
              <ActionButton label={t('incident_detail.btn_acknowledge')} icon="visibility" onClick={onAck} disabled={submitting} />
            )}
            {actions.includes('investigate') && (
              <ActionButton label={t('incident_detail.btn_investigate')} icon="search" onClick={onStart} disabled={submitting} variant="warning" />
            )}
            {actions.includes('resolve') && (
              <ActionButton label={t('incident_detail.btn_resolve')} icon="check_circle" onClick={() => setResolveOpen(true)} disabled={submitting} variant="success" />
            )}
            {actions.includes('reopen') && (
              <ActionButton label={t('incident_detail.btn_reopen')} icon="refresh" onClick={onReopen} disabled={submitting} variant="warning" />
            )}
            {actions.includes('close') && (
              <ActionButton label={t('incident_detail.btn_close')} icon="lock" onClick={onClose} disabled={submitting} variant="neutral" />
            )}
          </div>
        </div>
      </header>

      <div className="px-8 pb-10 grid grid-cols-12 gap-4">
        {/* Left column: details */}
        <div className="col-span-12 lg:col-span-7 flex flex-col gap-4">
          {/* Description / Impact / Root cause */}
          <section className="bg-surface-container rounded-lg p-5 space-y-4">
            <DetailField label={t('incident_detail.field_description')} value={incident.description} placeholder={t('incident_detail.no_description')} />
            <DetailField label={t('incident_detail.field_impact')} value={incident.impact} placeholder={t('incident_detail.no_impact')} />
            <DetailField label={t('incident_detail.field_root_cause')} value={incident.root_cause} placeholder={t('incident_detail.no_root_cause')} />
          </section>

          <Timeline id={id} />
        </div>

        {/* Right column: linked entities */}
        <div className="col-span-12 lg:col-span-5 flex flex-col gap-4">
          <section className="bg-surface-container rounded-lg p-5">
            <h2 className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant mb-3">
              {t('incident_detail.section_linked')}
            </h2>
            <div className="space-y-2">
              <LinkedRow
                label={t('incident_detail.affected_asset')}
                value={incident.affected_asset_id}
                onOpen={() => incident.affected_asset_id && navigate(`/assets/${incident.affected_asset_id}`)}
                emptyText={t('incident_detail.unlinked_asset')}
              />
              <LinkedRow
                label={t('incident_detail.affected_service')}
                value={incident.affected_service_id}
                onOpen={() => incident.affected_service_id && navigate(`/services/${incident.affected_service_id}`)}
                emptyText={t('incident_detail.unlinked_service')}
              />
            </div>
          </section>

        </div>
      </div>

      <ResolveDialog
        open={resolveOpen}
        onClose={() => setResolveOpen(false)}
        onSubmit={onResolveSubmit}
        submitting={resolve.isPending}
      />
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function DetailField({ label, value, placeholder }: { label: string; value?: string | null; placeholder: string }) {
  return (
    <div>
      <p className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant mb-1">{label}</p>
      {value ? (
        <p className="text-sm text-on-surface whitespace-pre-wrap">{value}</p>
      ) : (
        <p className="text-sm text-on-surface-variant italic">{placeholder}</p>
      )}
    </div>
  )
}

function LinkedRow({
  label,
  value,
  onOpen,
  emptyText,
}: {
  label: string
  value?: string | null
  onOpen: () => void
  emptyText: string
}) {
  return (
    <div className="flex items-center justify-between rounded-lg bg-surface-container-low p-3">
      <span className="text-xs text-on-surface-variant font-label uppercase tracking-wider">{label}</span>
      {value ? (
        <button
          onClick={onOpen}
          className="text-sm text-primary font-mono hover:underline flex items-center gap-1"
        >
          {value.slice(0, 8)}…
          <Icon name="open_in_new" className="text-[14px]" />
        </button>
      ) : (
        <span className="text-xs text-on-surface-variant italic">{emptyText}</span>
      )}
    </div>
  )
}

