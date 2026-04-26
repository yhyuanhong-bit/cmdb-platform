import { useCallback, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import Icon from '../components/Icon'
import {
  useProblem,
  useProblemComments,
  useStartInvestigatingProblem,
  useMarkProblemKnownError,
  useResolveProblem,
  useCloseProblem,
  useReopenProblem,
  useAddProblemComment,
  useIncidentsForProblem,
  useUnlinkIncidentFromProblem,
} from '../hooks/useProblems'
import type { ProblemStatus } from '../lib/api/problems'

const statusStyle: Record<ProblemStatus, string> = {
  open:          'bg-red-500/20 text-red-400',
  investigating: 'bg-amber-500/20 text-amber-400',
  known_error:   'bg-purple-500/20 text-purple-400',
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

// Lifecycle map matching the backend WHERE-status guards in
// queries/problems.sql. Keeping this next to the styling so any future
// state addition forces a review here.
type LifecycleAction = 'investigate' | 'known_error' | 'resolve' | 'close' | 'reopen'
const allowedActions: Record<ProblemStatus, LifecycleAction[]> = {
  open:          ['investigate'],
  investigating: ['known_error', 'resolve'],
  known_error:   ['resolve'],
  resolved:      ['close', 'reopen'],
  closed:        [],
}

interface ActionButtonProps {
  label: string
  icon: string
  onClick: () => void
  disabled?: boolean
  variant?: 'primary' | 'success' | 'warning' | 'danger' | 'neutral'
}

function ActionButton({ label, icon, onClick, disabled, variant = 'primary' }: ActionButtonProps) {
  const variantClass = {
    primary: 'bg-primary text-on-primary hover:opacity-90',
    success: 'bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30',
    warning: 'bg-amber-500/20 text-amber-400 hover:bg-amber-500/30',
    danger:  'bg-red-500/20 text-red-400 hover:bg-red-500/30',
    neutral: 'bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest',
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
/*  Known-error dialog (workaround required)                           */
/* ------------------------------------------------------------------ */

function KnownErrorDialog({
  open, onClose, onSubmit, submitting,
}: {
  open: boolean
  onClose: () => void
  onSubmit: (workaround: string, note: string) => void
  submitting: boolean
}) {
  const { t } = useTranslation()
  const [workaround, setWorkaround] = useState('')
  const [note, setNote] = useState('')
  if (!open) return null
  const disabled = submitting || workaround.trim() === ''
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" role="dialog" aria-modal="true">
      <div className="bg-surface-container rounded-lg max-w-lg w-full p-6 shadow-xl">
        <h2 className="font-headline font-bold text-lg text-on-surface mb-2">
          {t('problem_detail.dialog_known_error_title')}
        </h2>
        <p className="text-sm text-on-surface-variant mb-4">
          {t('problem_detail.dialog_known_error_desc')}
        </p>
        <label className="block text-xs text-on-surface-variant mb-1">
          {t('problem_detail.field_workaround_required')}
        </label>
        <textarea
          value={workaround}
          onChange={(e) => setWorkaround(e.target.value)}
          rows={3}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-3 outline-none resize-none mb-3"
          placeholder={t('problem_detail.dialog_known_error_placeholder')}
        />
        <label className="block text-xs text-on-surface-variant mb-1">
          {t('problem_detail.dialog_note_optional')}
        </label>
        <textarea
          value={note}
          onChange={(e) => setNote(e.target.value)}
          rows={2}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-3 outline-none resize-none"
        />
        <div className="flex justify-end gap-2 mt-4">
          <button
            onClick={onClose}
            disabled={submitting}
            className="px-4 py-2 rounded-lg text-sm text-on-surface-variant hover:bg-surface-container-high"
          >
            {t('common.cancel')}
          </button>
          <button
            onClick={() => onSubmit(workaround.trim(), note.trim())}
            disabled={disabled}
            className="px-4 py-2 rounded-lg text-sm font-semibold bg-amber-500/20 text-amber-400 hover:bg-amber-500/30 disabled:opacity-40 disabled:cursor-not-allowed"
          >
            {submitting ? t('common.saving') : t('problem_detail.dialog_known_error_confirm')}
          </button>
        </div>
      </div>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Resolve dialog (root_cause + resolution + note)                    */
/* ------------------------------------------------------------------ */

function ResolveDialog({
  open, onClose, onSubmit, submitting,
}: {
  open: boolean
  onClose: () => void
  onSubmit: (rootCause: string, resolution: string, note: string) => void
  submitting: boolean
}) {
  const { t } = useTranslation()
  const [rootCause, setRootCause] = useState('')
  const [resolution, setResolution] = useState('')
  const [note, setNote] = useState('')
  if (!open) return null
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" role="dialog" aria-modal="true">
      <div className="bg-surface-container rounded-lg max-w-lg w-full p-6 shadow-xl">
        <h2 className="font-headline font-bold text-lg text-on-surface mb-2">
          {t('problem_detail.dialog_resolve_title')}
        </h2>
        <p className="text-sm text-on-surface-variant mb-4">
          {t('problem_detail.dialog_resolve_desc')}
        </p>
        <label className="block text-xs text-on-surface-variant mb-1">{t('problem_detail.field_root_cause')}</label>
        <textarea value={rootCause} onChange={(e) => setRootCause(e.target.value)} rows={2}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-3 outline-none resize-none mb-3" />
        <label className="block text-xs text-on-surface-variant mb-1">{t('problem_detail.field_resolution')}</label>
        <textarea value={resolution} onChange={(e) => setResolution(e.target.value)} rows={2}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-3 outline-none resize-none mb-3" />
        <label className="block text-xs text-on-surface-variant mb-1">{t('problem_detail.dialog_note_optional')}</label>
        <textarea value={note} onChange={(e) => setNote(e.target.value)} rows={2}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-3 outline-none resize-none" />
        <div className="flex justify-end gap-2 mt-4">
          <button onClick={onClose} disabled={submitting}
            className="px-4 py-2 rounded-lg text-sm text-on-surface-variant hover:bg-surface-container-high">
            {t('common.cancel')}
          </button>
          <button onClick={() => onSubmit(rootCause.trim(), resolution.trim(), note.trim())} disabled={submitting}
            className="px-4 py-2 rounded-lg text-sm font-semibold bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30 disabled:opacity-40">
            {submitting ? t('common.saving') : t('problem_detail.dialog_resolve_confirm')}
          </button>
        </div>
      </div>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Linked Incidents panel                                             */
/* ------------------------------------------------------------------ */

function LinkedIncidents({ problemId }: { problemId: string }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { data, isLoading } = useIncidentsForProblem(problemId)
  const unlink = useUnlinkIncidentFromProblem()
  const incidents = data?.data ?? []

  const onUnlink = (incidentId: string) => {
    if (!window.confirm(t('problem_detail.confirm_unlink'))) return
    unlink.mutate({ incidentId, problemId }, {
      onSuccess: () => toast.success(t('problem_detail.toast_unlinked')),
      onError: (e: unknown) => toast.error(e instanceof Error ? e.message : t('common.unknown_error')),
    })
  }

  return (
    <section className="bg-surface-container rounded-lg p-5">
      <div className="flex items-center justify-between mb-3">
        <h2 className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant">
          {t('problem_detail.section_linked_incidents')}
        </h2>
        <span className="text-xs text-on-surface-variant">
          {t('problem_detail.linked_count', { count: incidents.length })}
        </span>
      </div>
      {isLoading && (
        <div className="py-4 flex justify-center">
          <div className="animate-spin rounded-full h-5 w-5 border-2 border-sky-400 border-t-transparent" />
        </div>
      )}
      {!isLoading && incidents.length === 0 && (
        <p className="text-xs text-on-surface-variant italic">{t('problem_detail.no_linked_incidents')}</p>
      )}
      {incidents.length > 0 && (
        <ul className="space-y-2">
          {incidents.map((inc) => (
            <li key={inc.id} className="flex items-center justify-between rounded-lg bg-surface-container-low p-3">
              <button
                onClick={() => navigate(`/monitoring/incidents/${inc.id}`)}
                className="min-w-0 flex-1 text-left"
              >
                <p className="text-sm font-semibold text-primary truncate hover:underline">{inc.title}</p>
                <p className="text-xs text-on-surface-variant">
                  {inc.status} · {new Date(inc.started_at).toLocaleString()}
                </p>
              </button>
              <button
                onClick={() => onUnlink(inc.id)}
                disabled={unlink.isPending}
                className="ml-2 p-1.5 rounded-md hover:bg-error-container/40 transition-colors disabled:opacity-40"
                aria-label={t('problem_detail.btn_unlink')}
              >
                <Icon name="link_off" className="text-[18px] text-error" />
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

/* ------------------------------------------------------------------ */
/*  Timeline                                                           */
/* ------------------------------------------------------------------ */

function Timeline({ problemId }: { problemId: string }) {
  const { t } = useTranslation()
  const { data, isLoading, error } = useProblemComments(problemId)
  const addComment = useAddProblemComment()
  const [body, setBody] = useState('')

  const onAdd = () => {
    const trimmed = body.trim()
    if (!trimmed) return
    addComment.mutate({ id: problemId, body: trimmed }, {
      onSuccess: () => {
        setBody('')
        toast.success(t('problem_detail.toast_comment_added'))
      },
      onError: (e: unknown) => toast.error(e instanceof Error ? e.message : t('common.unknown_error')),
    })
  }

  const comments = data?.data ?? []

  return (
    <section className="bg-surface-container rounded-lg p-5">
      <h2 className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant mb-4">
        {t('problem_detail.section_timeline')}
      </h2>

      {isLoading && (
        <div className="py-6 flex justify-center">
          <div className="animate-spin rounded-full h-5 w-5 border-2 border-sky-400 border-t-transparent" />
        </div>
      )}
      {error && <p className="text-sm text-red-300">{t('problem_detail.timeline_error')}</p>}
      {!isLoading && !error && comments.length === 0 && (
        <p className="text-xs text-on-surface-variant">{t('problem_detail.timeline_empty')}</p>
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
                    {c.kind === 'system' ? t('problem_detail.system') : (c.author_username ?? t('problem_detail.unknown_user'))}
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
          placeholder={t('problem_detail.add_comment_placeholder')}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-3 outline-none resize-none"
        />
        <div className="flex justify-end mt-2">
          <button
            onClick={onAdd}
            disabled={addComment.isPending || body.trim() === ''}
            className="px-4 py-2 rounded-lg text-xs font-semibold uppercase tracking-wider bg-primary text-on-primary hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed"
          >
            {addComment.isPending ? t('common.saving') : t('problem_detail.btn_add_comment')}
          </button>
        </div>
      </div>
    </section>
  )
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function ProblemDetail() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const problemQ = useProblem(id)
  const start = useStartInvestigatingProblem()
  const knownErr = useMarkProblemKnownError()
  const resolve = useResolveProblem()
  const close = useCloseProblem()
  const reopen = useReopenProblem()

  const [knownErrorOpen, setKnownErrorOpen] = useState(false)
  const [resolveOpen, setResolveOpen] = useState(false)

  const submitting = start.isPending || knownErr.isPending || resolve.isPending || close.isPending || reopen.isPending

  const handleErr = useCallback((e: unknown) => {
    if (e instanceof Error && e.message.toLowerCase().includes('invalid_transition')) {
      toast.error(t('problem_detail.toast_invalid_transition'))
      return
    }
    toast.error(e instanceof Error ? e.message : t('common.unknown_error'))
  }, [t])

  if (problemQ.isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-sky-400 border-t-transparent" />
      </div>
    )
  }

  if (problemQ.error || !problemQ.data?.data) {
    return (
      <div className="p-6">
        <div className="rounded-lg bg-red-900/20 p-4 text-red-300 text-sm">
          {t('problem_detail.load_failed')}{' '}
          <button onClick={() => problemQ.refetch()} className="underline">{t('common.retry')}</button>
        </div>
      </div>
    )
  }

  if (!id) return null

  const problem = problemQ.data.data
  const actions = allowedActions[problem.status] ?? []

  const onStart = () => start.mutate({ id }, {
    onSuccess: () => toast.success(t('problem_detail.toast_investigating')),
    onError: handleErr,
  })
  const onKnownError = (workaround: string, note: string) => knownErr.mutate(
    { id, workaround, note: note || undefined },
    {
      onSuccess: () => {
        toast.success(t('problem_detail.toast_known_error'))
        setKnownErrorOpen(false)
      },
      onError: handleErr,
    },
  )
  const onResolve = (rootCause: string, resolution: string, note: string) => resolve.mutate(
    {
      id,
      rootCause: rootCause || undefined,
      resolution: resolution || undefined,
      note: note || undefined,
    },
    {
      onSuccess: () => {
        toast.success(t('problem_detail.toast_resolved'))
        setResolveOpen(false)
      },
      onError: handleErr,
    },
  )
  const onClose = () => close.mutate(id, {
    onSuccess: () => toast.success(t('problem_detail.toast_closed')),
    onError: handleErr,
  })
  const onReopen = () => {
    const reason = window.prompt(t('problem_detail.prompt_reopen_reason')) ?? ''
    reopen.mutate({ id, reason: reason.trim() || undefined }, {
      onSuccess: () => toast.success(t('problem_detail.toast_reopened')),
      onError: handleErr,
    })
  }

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      <header className="px-8 pt-6 pb-4">
        <button
          onClick={() => navigate('/monitoring/problems')}
          className="flex items-center gap-1 text-sm text-on-surface-variant hover:text-primary transition-colors mb-3"
        >
          <Icon name="arrow_back" className="text-[18px]" />
          {t('problem_detail.back_to_problems')}
        </button>

        <div className="flex items-start justify-between gap-6 flex-wrap">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2 flex-wrap mb-2">
              <span className={`px-2.5 py-1 rounded text-[0.6875rem] font-semibold uppercase tracking-wider ${statusStyle[problem.status]}`}>
                {problem.status}
              </span>
              <span className={`px-2.5 py-1 rounded text-[0.6875rem] font-semibold uppercase tracking-wider ${severityStyle[problem.severity] ?? severityStyle.info}`}>
                {problem.severity}
              </span>
              {problem.priority && (
                <span className={`px-2.5 py-1 rounded text-[0.6875rem] font-semibold uppercase tracking-wider ${priorityStyle[problem.priority]}`}>
                  {problem.priority}
                </span>
              )}
            </div>
            <h1 className="font-headline font-bold text-2xl text-on-surface leading-tight break-words">
              {problem.title}
            </h1>
            <p className="text-xs text-on-surface-variant mt-1">
              {t('problem_detail.created_at')} {new Date(problem.created_at).toLocaleString()}
              {problem.resolved_at && (
                <> · {t('problem_detail.resolved_at')} {new Date(problem.resolved_at).toLocaleString()}</>
              )}
            </p>
          </div>

          <div className="flex items-center gap-2 flex-wrap">
            {actions.includes('investigate') && (
              <ActionButton label={t('problem_detail.btn_investigate')} icon="search" onClick={onStart} disabled={submitting} variant="warning" />
            )}
            {actions.includes('known_error') && (
              <ActionButton label={t('problem_detail.btn_known_error')} icon="bug_report" onClick={() => setKnownErrorOpen(true)} disabled={submitting} variant="warning" />
            )}
            {actions.includes('resolve') && (
              <ActionButton label={t('problem_detail.btn_resolve')} icon="check_circle" onClick={() => setResolveOpen(true)} disabled={submitting} variant="success" />
            )}
            {actions.includes('reopen') && (
              <ActionButton label={t('problem_detail.btn_reopen')} icon="refresh" onClick={onReopen} disabled={submitting} variant="warning" />
            )}
            {actions.includes('close') && (
              <ActionButton label={t('problem_detail.btn_close')} icon="lock" onClick={onClose} disabled={submitting} variant="neutral" />
            )}
          </div>
        </div>
      </header>

      <div className="px-8 pb-10 grid grid-cols-12 gap-4">
        <div className="col-span-12 lg:col-span-7 flex flex-col gap-4">
          <section className="bg-surface-container rounded-lg p-5 space-y-4">
            <DetailField label={t('problem_detail.field_description')} value={problem.description} placeholder={t('problem_detail.no_description')} />
            <DetailField label={t('problem_detail.field_workaround')} value={problem.workaround} placeholder={t('problem_detail.no_workaround')} />
            <DetailField label={t('problem_detail.field_root_cause')} value={problem.root_cause} placeholder={t('problem_detail.no_root_cause')} />
            <DetailField label={t('problem_detail.field_resolution')} value={problem.resolution} placeholder={t('problem_detail.no_resolution')} />
          </section>

          <Timeline problemId={id} />
        </div>

        <div className="col-span-12 lg:col-span-5 flex flex-col gap-4">
          <LinkedIncidents problemId={id} />
        </div>
      </div>

      <KnownErrorDialog
        open={knownErrorOpen}
        onClose={() => setKnownErrorOpen(false)}
        onSubmit={onKnownError}
        submitting={knownErr.isPending}
      />
      <ResolveDialog
        open={resolveOpen}
        onClose={() => setResolveOpen(false)}
        onSubmit={onResolve}
        submitting={resolve.isPending}
      />
    </div>
  )
}

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
