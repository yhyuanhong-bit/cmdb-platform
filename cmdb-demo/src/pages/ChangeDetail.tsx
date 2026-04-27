import { useCallback, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import Icon from '../components/Icon'
import {
  useChange,
  useChangeApprovals,
  useChangeComments,
  useChangeProblems,
  useSubmitChange,
  useStartChange,
  useMarkChangeSucceeded,
  useMarkChangeFailed,
  useMarkChangeRolledBack,
  useCastChangeVote,
  useAddChangeComment,
  useUnlinkChangeProblem,
} from '../hooks/useChanges'
import type {
  ChangeStatus,
  ChangeType,
  ChangeRisk,
  ChangeVote,
} from '../lib/api/changes'

/* ------------------------------------------------------------------ */
/*  Styling                                                            */
/* ------------------------------------------------------------------ */

const statusStyle: Record<ChangeStatus, string> = {
  draft:       'bg-surface-container-highest text-on-surface-variant',
  submitted:   'bg-blue-500/20 text-blue-400',
  approved:    'bg-emerald-500/20 text-emerald-400',
  rejected:    'bg-red-500/20 text-red-400',
  in_progress: 'bg-amber-500/20 text-amber-400',
  succeeded:   'bg-emerald-600/30 text-emerald-300',
  failed:      'bg-red-600/30 text-red-300',
  rolled_back: 'bg-purple-500/20 text-purple-400',
}

const typeStyle: Record<ChangeType, string> = {
  standard:  'bg-blue-500/20 text-blue-400',
  normal:    'bg-surface-container-highest text-on-surface-variant',
  emergency: 'bg-red-500/20 text-red-400',
}

const riskStyle: Record<ChangeRisk, string> = {
  low:      'bg-[#1e3a5f] text-on-primary-container',
  medium:   'bg-blue-500/20 text-blue-400',
  high:     'bg-[#92400e] text-[#fbbf24]',
  critical: 'bg-error-container text-on-error-container',
}

const voteStyle: Record<ChangeVote, string> = {
  approve: 'bg-emerald-500/20 text-emerald-400',
  reject:  'bg-red-500/20 text-red-400',
  abstain: 'bg-surface-container-highest text-on-surface-variant',
}

/* ------------------------------------------------------------------ */
/*  State machine — what can the operator do from each status?         */
/*  Mirrors the backend WHERE-status guards in queries/changes.sql.    */
/* ------------------------------------------------------------------ */

type LifecycleAction = 'submit' | 'start' | 'succeed' | 'fail' | 'rollback'

const allowedActions: Record<ChangeStatus, LifecycleAction[]> = {
  draft:       ['submit'],
  submitted:   [], // CAB voting drives the next transition
  approved:    ['start'],
  rejected:    [],
  in_progress: ['succeed', 'fail', 'rollback'],
  succeeded:   [],
  failed:      ['rollback'],
  rolled_back: [],
}

/* ------------------------------------------------------------------ */
/*  Small UI primitives                                                */
/* ------------------------------------------------------------------ */

function Badge({ kind, children }: { kind: string; children: React.ReactNode }) {
  return (
    <span className={`px-2.5 py-1 rounded text-[0.6875rem] font-semibold uppercase tracking-wider ${kind}`}>
      {children}
    </span>
  )
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
/*  Vote dialog (approve / reject / abstain with optional note)        */
/* ------------------------------------------------------------------ */

function VoteDialog({
  vote,
  onClose,
  onSubmit,
  submitting,
}: {
  vote: ChangeVote
  onClose: () => void
  onSubmit: (note: string) => void
  submitting: boolean
}) {
  const { t } = useTranslation()
  const [note, setNote] = useState('')
  const titleKey =
    vote === 'approve' ? 'change_detail.dialog_vote_approve' :
    vote === 'reject'  ? 'change_detail.dialog_vote_reject'  :
                         'change_detail.dialog_vote_abstain'
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" role="dialog" aria-modal="true">
      <div className="bg-surface-container rounded-lg max-w-lg w-full p-6 shadow-xl">
        <h2 className="font-headline font-bold text-lg text-on-surface mb-2">{t(titleKey)}</h2>
        <p className="text-sm text-on-surface-variant mb-4">
          {t('change_detail.dialog_vote_desc')}
        </p>
        <label className="block text-xs text-on-surface-variant mb-1">{t('change_detail.dialog_note_optional')}</label>
        <textarea
          value={note}
          onChange={(e) => setNote(e.target.value)}
          rows={3}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-3 outline-none resize-none"
          placeholder={t('change_detail.dialog_vote_placeholder')}
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
            onClick={() => onSubmit(note.trim())}
            disabled={submitting}
            className={`px-4 py-2 rounded-lg text-sm font-semibold disabled:opacity-40 ${
              vote === 'approve' ? 'bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30' :
              vote === 'reject'  ? 'bg-red-500/20 text-red-400 hover:bg-red-500/30' :
                                   'bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest'
            }`}
          >
            {submitting ? t('common.saving') : t('change_detail.dialog_vote_confirm')}
          </button>
        </div>
      </div>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  CAB voting panel                                                   */
/* ------------------------------------------------------------------ */

function CabPanel({
  changeId,
  status,
  approvalThreshold,
}: {
  changeId: string
  status: ChangeStatus
  approvalThreshold: number
}) {
  const { t } = useTranslation()
  const { data, isLoading } = useChangeApprovals(changeId)
  const cast = useCastChangeVote()
  const [voting, setVoting] = useState<ChangeVote | null>(null)

  const approvals = data?.data ?? []
  const approveCount = approvals.filter((a) => a.vote === 'approve').length
  const rejectCount = approvals.filter((a) => a.vote === 'reject').length

  const onCast = (vote: ChangeVote, note: string) => {
    cast.mutate({ id: changeId, vote, note: note || undefined }, {
      onSuccess: () => {
        toast.success(t('change_detail.toast_vote_recorded'))
        setVoting(null)
      },
      onError: (e: unknown) => {
        if (e instanceof Error && e.message.toLowerCase().includes('invalid_transition')) {
          toast.error(t('change_detail.toast_vote_too_late'))
        } else {
          toast.error(e instanceof Error ? e.message : t('common.unknown_error'))
        }
        setVoting(null)
      },
    })
  }

  const canVote = status === 'submitted'

  return (
    <section className="bg-surface-container rounded-lg p-5">
      <div className="flex items-center justify-between mb-3">
        <h2 className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant">
          {t('change_detail.section_cab')}
        </h2>
        <span className="text-xs text-on-surface-variant">
          {t('change_detail.cab_tally', {
            approve: approveCount,
            reject: rejectCount,
            threshold: approvalThreshold,
          })}
        </span>
      </div>

      {canVote && (
        <div className="flex flex-wrap items-center gap-2 mb-4">
          <span className="text-xs text-on-surface-variant">
            {t('change_detail.cast_your_vote')}
          </span>
          <button
            onClick={() => setVoting('approve')}
            className="px-3 py-1.5 rounded-md text-xs font-semibold bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30 transition-colors"
          >
            <Icon name="thumb_up" className="text-[14px] inline mr-1" />
            {t('change_detail.btn_vote_approve')}
          </button>
          <button
            onClick={() => setVoting('reject')}
            className="px-3 py-1.5 rounded-md text-xs font-semibold bg-red-500/20 text-red-400 hover:bg-red-500/30 transition-colors"
          >
            <Icon name="thumb_down" className="text-[14px] inline mr-1" />
            {t('change_detail.btn_vote_reject')}
          </button>
          <button
            onClick={() => setVoting('abstain')}
            className="px-3 py-1.5 rounded-md text-xs font-semibold bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest transition-colors"
          >
            <Icon name="remove_circle_outline" className="text-[14px] inline mr-1" />
            {t('change_detail.btn_vote_abstain')}
          </button>
        </div>
      )}
      {!canVote && status !== 'draft' && (
        <p className="text-xs text-on-surface-variant italic mb-4">
          {t('change_detail.voting_closed', { status })}
        </p>
      )}

      {isLoading && (
        <div className="py-4 flex justify-center">
          <div className="animate-spin rounded-full h-5 w-5 border-2 border-sky-400 border-t-transparent" />
        </div>
      )}
      {!isLoading && approvals.length === 0 && (
        <p className="text-xs text-on-surface-variant italic">{t('change_detail.no_votes_yet')}</p>
      )}
      {approvals.length > 0 && (
        <ul className="space-y-2">
          {approvals.map((a) => (
            <li key={a.id} className="flex items-start justify-between gap-3 rounded-lg bg-surface-container-low p-3">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-semibold text-on-surface">
                    {a.voter_username ?? a.voter_id.slice(0, 8) + '…'}
                  </span>
                  <span className={`px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${voteStyle[a.vote]}`}>
                    {a.vote}
                  </span>
                  <span className="text-xs text-on-surface-variant">
                    {new Date(a.voted_at).toLocaleString()}
                  </span>
                </div>
                {a.note && (
                  <p className="mt-1 text-xs text-on-surface whitespace-pre-wrap">{a.note}</p>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}

      {voting && (
        <VoteDialog
          vote={voting}
          onClose={() => setVoting(null)}
          onSubmit={(note) => onCast(voting, note)}
          submitting={cast.isPending}
        />
      )}
    </section>
  )
}

/* ------------------------------------------------------------------ */
/*  Linked Problems panel (read-only quick link; richer linking lives  */
/*  on the Problem detail page where the "Link change" pivot belongs)  */
/* ------------------------------------------------------------------ */

function LinkedProblemsPanel({ changeId }: { changeId: string }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { data, isLoading } = useChangeProblems(changeId)
  const unlink = useUnlinkChangeProblem()
  const linked = data?.data ?? []

  const onUnlink = (problemId: string) => {
    if (!window.confirm(t('change_detail.confirm_unlink_problem'))) return
    unlink.mutate({ changeId, problemId }, {
      onSuccess: () => toast.success(t('change_detail.toast_problem_unlinked')),
      onError: (e: unknown) => toast.error(e instanceof Error ? e.message : t('common.unknown_error')),
    })
  }

  return (
    <section className="bg-surface-container rounded-lg p-5">
      <div className="flex items-center justify-between mb-3">
        <h2 className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant">
          {t('change_detail.section_linked_problems')}
        </h2>
      </div>
      {isLoading && (
        <div className="py-4 flex justify-center">
          <div className="animate-spin rounded-full h-5 w-5 border-2 border-sky-400 border-t-transparent" />
        </div>
      )}
      {!isLoading && linked.length === 0 && (
        <p className="text-xs text-on-surface-variant italic">{t('change_detail.no_linked_problems')}</p>
      )}
      {linked.length > 0 && (
        <ul className="space-y-2">
          {linked.map((p) => (
            <li key={p.id} className="flex items-center justify-between rounded-lg bg-surface-container-low p-3">
              <button
                onClick={() => navigate(`/monitoring/problems/${p.id}`)}
                className="min-w-0 flex-1 text-left"
              >
                <p className="text-sm font-semibold text-primary truncate hover:underline">{p.title}</p>
                <p className="text-xs text-on-surface-variant">{p.status} · {p.severity}</p>
              </button>
              <button
                onClick={() => onUnlink(p.id)}
                disabled={unlink.isPending}
                className="ml-2 p-1.5 rounded-md hover:bg-error-container/40 transition-colors disabled:opacity-40"
                aria-label={t('change_detail.btn_unlink')}
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

function Timeline({ changeId }: { changeId: string }) {
  const { t } = useTranslation()
  const { data, isLoading, error } = useChangeComments(changeId)
  const addComment = useAddChangeComment()
  const [body, setBody] = useState('')

  const onAdd = () => {
    const trimmed = body.trim()
    if (!trimmed) return
    addComment.mutate({ id: changeId, body: trimmed }, {
      onSuccess: () => {
        setBody('')
        toast.success(t('change_detail.toast_comment_added'))
      },
      onError: (e: unknown) => toast.error(e instanceof Error ? e.message : t('common.unknown_error')),
    })
  }

  const comments = data?.data ?? []

  return (
    <section className="bg-surface-container rounded-lg p-5">
      <h2 className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant mb-4">
        {t('change_detail.section_timeline')}
      </h2>

      {isLoading && (
        <div className="py-6 flex justify-center">
          <div className="animate-spin rounded-full h-5 w-5 border-2 border-sky-400 border-t-transparent" />
        </div>
      )}
      {error && <p className="text-sm text-red-300">{t('change_detail.timeline_error')}</p>}
      {!isLoading && !error && comments.length === 0 && (
        <p className="text-xs text-on-surface-variant">{t('change_detail.timeline_empty')}</p>
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
                    {c.kind === 'system' ? t('change_detail.system') : (c.author_username ?? t('change_detail.unknown_user'))}
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
          placeholder={t('change_detail.add_comment_placeholder')}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-3 outline-none resize-none"
        />
        <div className="flex justify-end mt-2">
          <button
            onClick={onAdd}
            disabled={addComment.isPending || body.trim() === ''}
            className="px-4 py-2 rounded-lg text-xs font-semibold uppercase tracking-wider bg-primary text-on-primary hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed"
          >
            {addComment.isPending ? t('common.saving') : t('change_detail.btn_add_comment')}
          </button>
        </div>
      </div>
    </section>
  )
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function ChangeDetail() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const changeQ = useChange(id)
  const submit = useSubmitChange()
  const start = useStartChange()
  const succ = useMarkChangeSucceeded()
  const fail = useMarkChangeFailed()
  const rb = useMarkChangeRolledBack()

  const submitting = submit.isPending || start.isPending || succ.isPending || fail.isPending || rb.isPending

  const handleErr = useCallback((e: unknown) => {
    if (e instanceof Error && e.message.toLowerCase().includes('invalid_transition')) {
      toast.error(t('change_detail.toast_invalid_transition'))
      return
    }
    toast.error(e instanceof Error ? e.message : t('common.unknown_error'))
  }, [t])

  if (changeQ.isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-sky-400 border-t-transparent" />
      </div>
    )
  }

  if (changeQ.error || !changeQ.data?.data) {
    return (
      <div className="p-6">
        <div className="rounded-lg bg-red-900/20 p-4 text-red-300 text-sm">
          {t('change_detail.load_failed')}{' '}
          <button onClick={() => changeQ.refetch()} className="underline">{t('common.retry')}</button>
        </div>
      </div>
    )
  }

  if (!id) return null

  const change = changeQ.data.data
  const actions = allowedActions[change.status] ?? []

  const onSubmit = () => submit.mutate(id, {
    onSuccess: () => toast.success(t('change_detail.toast_submitted')),
    onError: handleErr,
  })
  const onStart = () => start.mutate(id, {
    onSuccess: () => toast.success(t('change_detail.toast_started')),
    onError: handleErr,
  })
  const onSucc = () => {
    const note = window.prompt(t('change_detail.prompt_succeed_note')) ?? ''
    succ.mutate({ id, note: note.trim() || undefined }, {
      onSuccess: () => toast.success(t('change_detail.toast_succeeded')),
      onError: handleErr,
    })
  }
  const onFail = () => {
    const note = window.prompt(t('change_detail.prompt_fail_note')) ?? ''
    fail.mutate({ id, note: note.trim() || undefined }, {
      onSuccess: () => toast.success(t('change_detail.toast_failed')),
      onError: handleErr,
    })
  }
  const onRollback = () => {
    const note = window.prompt(t('change_detail.prompt_rollback_note')) ?? ''
    rb.mutate({ id, note: note.trim() || undefined }, {
      onSuccess: () => toast.success(t('change_detail.toast_rolled_back')),
      onError: handleErr,
    })
  }

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      <header className="px-8 pt-6 pb-4">
        <button
          onClick={() => navigate('/monitoring/changes')}
          className="flex items-center gap-1 text-sm text-on-surface-variant hover:text-primary transition-colors mb-3"
        >
          <Icon name="arrow_back" className="text-[18px]" />
          {t('change_detail.back_to_changes')}
        </button>

        <div className="flex items-start justify-between gap-6 flex-wrap">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2 flex-wrap mb-2">
              <Badge kind={statusStyle[change.status]}>{change.status}</Badge>
              <Badge kind={typeStyle[change.type]}>{change.type}</Badge>
              <Badge kind={riskStyle[change.risk]}>{change.risk}</Badge>
            </div>
            <h1 className="font-headline font-bold text-2xl text-on-surface leading-tight break-words">
              {change.title}
            </h1>
            <p className="text-xs text-on-surface-variant mt-1">
              {t('change_detail.created_at')} {new Date(change.created_at).toLocaleString()}
              {change.submitted_at && (
                <> · {t('change_detail.submitted_at')} {new Date(change.submitted_at).toLocaleString()}</>
              )}
              {change.actual_end && (
                <> · {t('change_detail.ended_at')} {new Date(change.actual_end).toLocaleString()}</>
              )}
            </p>
          </div>

          <div className="flex items-center gap-2 flex-wrap">
            {actions.includes('submit') && (
              <ActionButton label={t('change_detail.btn_submit')} icon="send" onClick={onSubmit} disabled={submitting} />
            )}
            {actions.includes('start') && (
              <ActionButton label={t('change_detail.btn_start')} icon="play_arrow" onClick={onStart} disabled={submitting} variant="warning" />
            )}
            {actions.includes('succeed') && (
              <ActionButton label={t('change_detail.btn_succeed')} icon="check_circle" onClick={onSucc} disabled={submitting} variant="success" />
            )}
            {actions.includes('fail') && (
              <ActionButton label={t('change_detail.btn_fail')} icon="cancel" onClick={onFail} disabled={submitting} variant="danger" />
            )}
            {actions.includes('rollback') && (
              <ActionButton label={t('change_detail.btn_rollback')} icon="undo" onClick={onRollback} disabled={submitting} variant="neutral" />
            )}
          </div>
        </div>
      </header>

      <div className="px-8 pb-10 grid grid-cols-12 gap-4">
        {/* Left: details + timeline */}
        <div className="col-span-12 lg:col-span-7 flex flex-col gap-4">
          <section className="bg-surface-container rounded-lg p-5 space-y-4">
            <DetailField label={t('change_detail.field_description')} value={change.description} placeholder={t('change_detail.no_description')} />
            <DetailField label={t('change_detail.field_impact_summary')} value={change.impact_summary} placeholder={t('change_detail.no_impact_summary')} />
            <DetailField label={t('change_detail.field_rollback_plan')} value={change.rollback_plan} placeholder={t('change_detail.no_rollback_plan')} />

            <div className="grid grid-cols-2 gap-4">
              <PlannedField
                label={t('change_detail.planned_start')}
                value={change.planned_start}
                placeholder={t('change_detail.not_scheduled')}
              />
              <PlannedField
                label={t('change_detail.planned_end')}
                value={change.planned_end}
                placeholder={t('change_detail.not_scheduled')}
              />
            </div>
          </section>

          <Timeline changeId={id} />
        </div>

        {/* Right: CAB voting + linkage */}
        <div className="col-span-12 lg:col-span-5 flex flex-col gap-4">
          <CabPanel
            changeId={id}
            status={change.status}
            approvalThreshold={change.approval_threshold}
          />
          <LinkedProblemsPanel changeId={id} />
        </div>
      </div>
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

function PlannedField({ label, value, placeholder }: { label: string; value?: string | null; placeholder: string }) {
  return (
    <div>
      <p className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant mb-1">{label}</p>
      {value ? (
        <p className="text-sm text-on-surface font-mono">{new Date(value).toLocaleString()}</p>
      ) : (
        <p className="text-sm text-on-surface-variant italic">{placeholder}</p>
      )}
    </div>
  )
}
