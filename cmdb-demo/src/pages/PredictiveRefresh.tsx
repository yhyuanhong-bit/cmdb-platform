import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import Icon from '../components/Icon'
import {
  usePredictiveRefresh,
  useRunPredictiveRefreshScan,
  useTransitionPredictiveRefresh,
} from '../hooks/usePredictiveRefresh'
import type {
  PredictiveRefresh,
  PredictiveRefreshKind,
  PredictiveRefreshStatus,
} from '../lib/api/predictiveRefresh'

/* ------------------------------------------------------------------ */
/*  Style maps                                                         */
/* ------------------------------------------------------------------ */

const kindStyle: Record<PredictiveRefreshKind, string> = {
  warranty_expiring: 'bg-amber-500/20 text-amber-400',
  warranty_expired:  'bg-red-500/20 text-red-400',
  eol_approaching:   'bg-amber-500/20 text-amber-400',
  eol_passed:        'bg-red-500/20 text-red-400',
  aged_out:          'bg-purple-500/20 text-purple-400',
}

const statusStyle: Record<PredictiveRefreshStatus, string> = {
  open:     'bg-amber-500/20 text-amber-400',
  ack:      'bg-blue-500/20 text-blue-400',
  resolved: 'bg-emerald-500/20 text-emerald-400',
}

const allowedActions: Record<PredictiveRefreshStatus, PredictiveRefreshStatus[]> = {
  open:     ['ack', 'resolved'],
  ack:      ['resolved', 'open'],
  resolved: ['open'],
}

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function fmtScore(s: string): string {
  const n = Number(s)
  if (!isFinite(n)) return s
  return n.toFixed(0)
}

function scoreColor(score: string): string {
  const n = Number(score)
  if (!isFinite(n)) return 'bg-surface-container-highest text-on-surface-variant'
  if (n >= 90) return 'bg-red-500/30 text-red-400'
  if (n >= 70) return 'bg-amber-500/30 text-amber-400'
  if (n >= 40) return 'bg-blue-500/30 text-blue-400'
  return 'bg-surface-container-high text-on-surface-variant'
}

/* ------------------------------------------------------------------ */
/*  Transition note dialog                                             */
/* ------------------------------------------------------------------ */

interface TransitionTarget {
  row: PredictiveRefresh
  next: PredictiveRefreshStatus
}

function TransitionDialog({
  open,
  target,
  onClose,
  onSubmit,
  submitting,
}: {
  open: boolean
  target: TransitionTarget | null
  onClose: () => void
  onSubmit: (note: string) => void
  submitting: boolean
}) {
  const { t } = useTranslation()
  const [note, setNote] = useState('')
  const textareaRef = useRef<HTMLTextAreaElement | null>(null)

  useEffect(() => {
    if (!open) return
    setNote('')
    const handle = setTimeout(() => textareaRef.current?.focus(), 0)
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !submitting) onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => {
      clearTimeout(handle)
      window.removeEventListener('keydown', onKey)
    }
  }, [open, onClose, submitting])

  if (!open || !target) return null

  const titleKey =
    target.next === 'ack'
      ? 'predictive_refresh.dialog_ack_title'
      : target.next === 'resolved'
      ? 'predictive_refresh.dialog_resolve_title'
      : 'predictive_refresh.dialog_reopen_title'
  const descKey =
    target.next === 'ack'
      ? 'predictive_refresh.dialog_ack_desc'
      : target.next === 'resolved'
      ? 'predictive_refresh.dialog_resolve_desc'
      : 'predictive_refresh.dialog_reopen_desc'
  const placeholderKey =
    target.next === 'ack'
      ? 'predictive_refresh.prompt_ack'
      : target.next === 'resolved'
      ? 'predictive_refresh.prompt_resolve'
      : 'predictive_refresh.prompt_reopen'
  const confirmKey =
    target.next === 'ack'
      ? 'predictive_refresh.btn_ack'
      : target.next === 'resolved'
      ? 'predictive_refresh.btn_resolve'
      : 'predictive_refresh.btn_reopen'
  const confirmStyle =
    target.next === 'resolved'
      ? 'bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30'
      : target.next === 'ack'
      ? 'bg-blue-500/20 text-blue-400 hover:bg-blue-500/30'
      : 'bg-amber-500/20 text-amber-400 hover:bg-amber-500/30'

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="predictive-refresh-dialog-title"
    >
      <div className="bg-surface-container rounded-lg max-w-lg w-full p-6 shadow-xl">
        <h2 id="predictive-refresh-dialog-title" className="font-headline font-bold text-lg text-on-surface mb-2">
          {t(titleKey)}
        </h2>
        <p className="text-sm text-on-surface-variant mb-4">
          {t(descKey)}
        </p>
        <p className="text-xs text-on-surface-variant mb-3 font-mono">
          {target.row.asset_name ?? target.row.asset_id} · {target.row.kind}
        </p>
        <label htmlFor="predictive-refresh-dialog-note" className="block text-xs text-on-surface-variant mb-1">
          {t('predictive_refresh.dialog_note_label')}
        </label>
        <textarea
          id="predictive-refresh-dialog-note"
          ref={textareaRef}
          value={note}
          onChange={(e) => setNote(e.target.value)}
          rows={3}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-3 outline-none resize-none"
          placeholder={t(placeholderKey)}
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
            onClick={() => onSubmit(note.trim())}
            disabled={submitting}
            className={`px-4 py-2 rounded-lg text-sm font-semibold transition-colors disabled:opacity-40 ${confirmStyle}`}
          >
            {submitting ? t('common.saving') : t(confirmKey)}
          </button>
        </div>
      </div>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function PredictiveRefreshPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [search, setSearch] = useSearchParams()

  const status = (search.get('status') as PredictiveRefreshStatus | null) ?? 'open'
  const kind = (search.get('kind') as PredictiveRefreshKind | null) ?? null
  const monthFilter = search.get('month') ?? null // YYYY-MM

  // C-H3 (audit 2026-04-28): page_size: 200 with client-side month
  // filter is the current pragmatic shape — backend has no `month=`
  // query param yet. When /predictive-refresh grows that filter, pass
  // monthFilter through here and drop the client-side filter below.
  const listQ = usePredictiveRefresh({
    status,
    kind: kind ?? undefined,
    page_size: 200,
  })
  const runScan = useRunPredictiveRefreshScan()
  const transition = useTransitionPredictiveRefresh()
  // C-H2 (audit 2026-04-28): replace window.prompt() with an
  // accessible modal so transition notes can be collected without
  // blocking the UI thread or breaking screen-reader flows.
  const [pendingTransition, setPendingTransition] = useState<TransitionTarget | null>(null)

  const allRows: PredictiveRefresh[] = listQ.data?.data ?? []

  // Client-side month filter (capex chart click-through). Backend
  // doesn't support a target_month query param yet — when it grows
  // one we can swap this for a server filter.
  const rows = useMemo(() => {
    if (!monthFilter) return allRows
    return allRows.filter((r) => r.target_date?.slice(0, 7) === monthFilter)
  }, [allRows, monthFilter])

  const setStatus = (s: PredictiveRefreshStatus) => {
    const next = new URLSearchParams(search)
    next.set('status', s)
    setSearch(next, { replace: true })
  }
  const setKind = (k: PredictiveRefreshKind | null) => {
    const next = new URLSearchParams(search)
    if (k) next.set('kind', k)
    else next.delete('kind')
    setSearch(next, { replace: true })
  }
  const clearMonth = () => {
    const next = new URLSearchParams(search)
    next.delete('month')
    setSearch(next, { replace: true })
  }

  const onRunScan = () => {
    runScan.mutate(undefined, {
      onSuccess: (resp) => {
        const upserted = resp.data?.rows_upserted ?? 0
        toast.success(t('predictive_refresh.toast_scan_done', { count: upserted }))
      },
      onError: (e: unknown) => toast.error(e instanceof Error ? e.message : t('common.unknown_error')),
    })
  }

  const onTransition = (r: PredictiveRefresh, next: PredictiveRefreshStatus) => {
    setPendingTransition({ row: r, next })
  }
  const submitTransition = (note: string) => {
    if (!pendingTransition) return
    const { row, next } = pendingTransition
    transition.mutate(
      { assetId: row.asset_id, kind: row.kind, status: next, note: note || undefined },
      {
        onSuccess: () => {
          toast.success(t(`predictive_refresh.toast_${next}`))
          setPendingTransition(null)
        },
        onError: (e: unknown) => toast.error(e instanceof Error ? e.message : t('common.unknown_error')),
      },
    )
  }

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      <header className="px-8 pt-6 pb-4">
        <nav className="flex items-center gap-1.5 text-xs text-on-surface-variant mb-3">
          <span className="hover:text-primary cursor-pointer" onClick={() => navigate('/predictive')}>
            {t('predictive_refresh.breadcrumb_predictive')}
          </span>
          <Icon name="chevron_right" className="text-[14px] text-on-surface-variant" />
          <span className="text-primary">{t('predictive_refresh.title')}</span>
        </nav>

        <div className="flex items-start justify-between flex-wrap gap-4">
          <div>
            <h1 className="font-headline font-bold text-2xl text-on-surface">
              {t('predictive_refresh.title')}
            </h1>
            <p className="text-sm text-on-surface-variant mt-1">{t('predictive_refresh.subtitle')}</p>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => navigate('/predictive/capex')}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-surface-container-high text-on-surface text-sm font-semibold hover:bg-surface-container-highest transition-colors"
            >
              <Icon name="bar_chart" className="text-[18px]" />
              {t('predictive_refresh.btn_capex')}
            </button>
            <button
              onClick={onRunScan}
              disabled={runScan.isPending}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-on-primary text-sm font-semibold hover:opacity-90 transition-opacity disabled:opacity-40"
              title={t('predictive_refresh.run_scan_hint')}
            >
              <Icon name="refresh" className="text-[18px]" />
              {runScan.isPending ? t('common.saving') : t('predictive_refresh.btn_run_scan')}
            </button>
          </div>
        </div>
      </header>

      {/* Filter bar */}
      <section className="px-8 pb-4">
        <div className="bg-surface-container rounded-lg p-5">
          <div className="flex flex-wrap items-end gap-4">
            <div className="flex gap-1">
              {(['open', 'ack', 'resolved'] as const).map((s) => (
                <button
                  key={s}
                  onClick={() => setStatus(s)}
                  className={`px-4 py-2 rounded-lg text-sm font-semibold transition-colors ${
                    status === s
                      ? 'bg-primary text-on-primary'
                      : 'text-on-surface-variant hover:bg-surface-container-high'
                  }`}
                >
                  {t(`predictive_refresh.tab_${s}`)}
                </button>
              ))}
            </div>
            <div>
              <label className="block text-xs text-on-surface-variant mb-1">{t('predictive_refresh.field_kind')}</label>
              <select
                value={kind ?? 'all'}
                onChange={(e) => setKind(e.target.value === 'all' ? null : (e.target.value as PredictiveRefreshKind))}
                className="bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none"
              >
                <option value="all">{t('predictive_refresh.all_kinds')}</option>
                <option value="warranty_expiring">warranty_expiring</option>
                <option value="warranty_expired">warranty_expired</option>
                <option value="eol_approaching">eol_approaching</option>
                <option value="eol_passed">eol_passed</option>
                <option value="aged_out">aged_out</option>
              </select>
            </div>
            {monthFilter && (
              <div className="flex items-center gap-2">
                <span className="text-xs text-on-surface-variant">
                  {t('predictive_refresh.month_filter', { month: monthFilter })}
                </span>
                <button
                  onClick={clearMonth}
                  className="px-2 py-1 rounded text-xs bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest"
                >
                  {t('common.clear')}
                </button>
              </div>
            )}
            <div className="ml-auto text-xs text-on-surface-variant">
              {t('predictive_refresh.results_count', { count: rows.length })}
            </div>
          </div>
        </div>
      </section>

      {/* Table */}
      <section className="px-8 pb-8">
        <div className="bg-surface-container rounded-lg overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-surface-container-high text-on-surface-variant text-[0.6875rem] uppercase tracking-wider">
                <th className="px-4 py-3 text-left font-semibold">{t('predictive_refresh.col_asset')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('predictive_refresh.col_kind')}</th>
                <th className="px-4 py-3 text-right font-semibold">{t('predictive_refresh.col_score')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('predictive_refresh.col_target_date')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('predictive_refresh.col_reason')}</th>
                <th className="px-4 py-3 text-right font-semibold">{t('common.actions')}</th>
              </tr>
            </thead>
            <tbody>
              {listQ.isLoading && (
                <tr><td colSpan={6} className="py-10 text-center">
                  <div className="inline-block animate-spin rounded-full h-6 w-6 border-2 border-sky-400 border-t-transparent" />
                </td></tr>
              )}
              {listQ.error && (
                <tr><td colSpan={6} className="py-4 text-center text-red-300 text-sm">
                  {t('predictive_refresh.load_failed')}{' '}
                  <button onClick={() => listQ.refetch()} className="underline">{t('common.retry')}</button>
                </td></tr>
              )}
              {!listQ.isLoading && !listQ.error && rows.length === 0 && (
                <tr><td colSpan={6} className="py-10 text-center text-on-surface-variant text-sm">
                  {t(`predictive_refresh.empty_${status}`)}
                </td></tr>
              )}
              {rows.map((r) => {
                const actions = allowedActions[r.status] ?? []
                return (
                  <tr key={`${r.asset_id}:${r.kind}`} className="border-t border-surface-container-high">
                    <td className="px-4 py-3">
                      <button
                        onClick={() => navigate(`/assets/${r.asset_id}`)}
                        className="text-primary font-medium hover:underline text-left"
                      >
                        {r.asset_name ?? r.asset_id.slice(0, 8) + '…'}
                      </button>
                      {r.asset_tag && (
                        <p className="text-[0.6875rem] text-on-surface-variant font-mono mt-0.5">{r.asset_tag}</p>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      <span className={`px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${kindStyle[r.kind]}`}>
                        {r.kind}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <span className={`px-2 py-0.5 rounded text-[0.6875rem] font-mono font-semibold ${scoreColor(r.risk_score)}`}>
                        {fmtScore(r.risk_score)}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-xs font-mono">
                      {r.target_date ?? '—'}
                    </td>
                    <td className="px-4 py-3 text-xs text-on-surface-variant max-w-md">
                      <p className="truncate">{r.reason}</p>
                      {r.recommended_action && (
                        <p className="text-[0.6875rem] italic text-on-surface-variant mt-0.5 truncate">
                          → {r.recommended_action}
                        </p>
                      )}
                    </td>
                    <td className="px-4 py-3 text-right whitespace-nowrap">
                      <span className={`mr-2 px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${statusStyle[r.status]}`}>
                        {r.status}
                      </span>
                      {actions.includes('ack') && (
                        <button
                          onClick={() => onTransition(r, 'ack')}
                          disabled={transition.isPending}
                          className="ml-1 px-2 py-1 rounded text-[0.6875rem] font-semibold bg-blue-500/20 text-blue-400 hover:bg-blue-500/30 transition-colors disabled:opacity-40"
                        >
                          {t('predictive_refresh.btn_ack')}
                        </button>
                      )}
                      {actions.includes('resolved') && (
                        <button
                          onClick={() => onTransition(r, 'resolved')}
                          disabled={transition.isPending}
                          className="ml-1 px-2 py-1 rounded text-[0.6875rem] font-semibold bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30 transition-colors disabled:opacity-40"
                        >
                          {t('predictive_refresh.btn_resolve')}
                        </button>
                      )}
                      {actions.includes('open') && (
                        <button
                          onClick={() => onTransition(r, 'open')}
                          disabled={transition.isPending}
                          className="ml-1 px-2 py-1 rounded text-[0.6875rem] font-semibold bg-amber-500/20 text-amber-400 hover:bg-amber-500/30 transition-colors disabled:opacity-40"
                        >
                          {t('predictive_refresh.btn_reopen')}
                        </button>
                      )}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      </section>
      <TransitionDialog
        open={pendingTransition !== null}
        target={pendingTransition}
        onClose={() => setPendingTransition(null)}
        onSubmit={submitTransition}
        submitting={transition.isPending}
      />
    </div>
  )
}
