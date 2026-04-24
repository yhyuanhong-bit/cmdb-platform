import { useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import Icon from '../components/Icon'
import { useDiscoveredAssets, useDiscoveryStats, useApproveAsset, useIgnoreAsset } from '../hooks/useDiscovery'
import { useUrlState } from '../hooks/useUrlState'
import ScanManagementTab from '../components/ScanManagementTab'

// URL-persisted list state for the Auto Discovery / inventory review page:
// which tab (review vs scan management), and which status/source filters
// are active on the review table.
type DiscoveryTab = 'review' | 'scan'
const discoveryListDefaults = {
  activeTab: 'review' as DiscoveryTab,
  statusFilter: 'all',
  sourceFilter: 'all',
}

/* ------------------------------------------------------------------ */
/*  Types                                                              */
/* ------------------------------------------------------------------ */

interface DiscoveredAsset {
  id: string
  source: string
  external_id?: string
  hostname?: string
  ip_address?: string
  raw_data?: Record<string, unknown>
  status: string
  matched_asset_id?: string | null
  match_confidence?: number | null
  match_strategy?: string | null
  review_reason?: string | null
  diff_details?: Record<string, unknown> | null
  discovered_at: string
  reviewed_by?: string | null
  reviewed_at?: string | null
}

interface DiscoveryStatsData {
  total: number
  pending: number
  conflict: number
  approved: number
  ignored: number
  matched: number
}

type ReviewAction = 'approve' | 'ignore'

interface ReviewDialogState {
  action: ReviewAction
  ids: string[]
  reason: string
}

/* ------------------------------------------------------------------ */
/*  Source icon mapping                                                */
/* ------------------------------------------------------------------ */

const sourceIcon: Record<string, { icon: string; bg: string }> = {
  VMware:  { icon: 'cloud',           bg: 'bg-[#1e3a5f]' },
  SNMP:    { icon: 'router',          bg: 'bg-[#064e3b]' },
  IPMI:    { icon: 'developer_board', bg: 'bg-[#92400e]' },
  SSH:     { icon: 'terminal',        bg: 'bg-[#1a365d]' },
  manual:  { icon: 'upload_file',     bg: 'bg-[#4a1d6e]' },
}

/* ------------------------------------------------------------------ */
/*  Status badge                                                       */
/* ------------------------------------------------------------------ */

const statusStyle: Record<string, string> = {
  pending:  'bg-blue-500/20 text-blue-400',
  conflict: 'bg-amber-500/20 text-amber-400',
  approved: 'bg-emerald-500/20 text-emerald-400',
  ignored:  'bg-surface-container-highest text-on-surface-variant',
}

function StatusBadge({ status }: { status: string }) {
  return (
    <span className={`inline-block px-2.5 py-1 rounded text-[0.6875rem] font-semibold uppercase tracking-wider ${statusStyle[status] ?? statusStyle.pending}`}>
      {status}
    </span>
  )
}

/* ------------------------------------------------------------------ */
/*  Confidence pill                                                    */
/* ------------------------------------------------------------------ */

function ConfidencePill({ value, strategy }: { value?: number | null; strategy?: string | null }) {
  if (value == null) return <span className="text-xs text-on-surface-variant">-</span>
  const pct = Math.round(value * 100)
  const color =
    pct >= 90 ? 'bg-emerald-500/20 text-emerald-400' :
    pct >= 70 ? 'bg-amber-500/20 text-amber-400' :
                'bg-red-500/20 text-red-400'
  return (
    <div className="flex items-center gap-1.5">
      <span className={`inline-block px-2 py-0.5 rounded text-[0.6875rem] font-mono font-semibold ${color}`}>
        {pct}%
      </span>
      {strategy && (
        <span className="text-[0.6875rem] text-on-surface-variant font-mono">{strategy}</span>
      )}
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Diff viewer                                                        */
/* ------------------------------------------------------------------ */

function DiffDetails({ diff }: { diff: Record<string, unknown> }) {
  const entries = Object.entries(diff)
  if (entries.length === 0) return null
  return (
    <div className="mt-2 space-y-1">
      {entries.map(([key, val]) => (
        <div key={key} className="flex items-center gap-2 text-xs">
          <span className="font-mono text-on-surface-variant">{key}:</span>
          <span className="text-red-400 line-through">{String((val as { old?: unknown })?.old ?? '')}</span>
          <Icon name="arrow_forward" className="text-[12px] text-on-surface-variant" />
          <span className="text-emerald-400">{String((val as { new?: unknown })?.new ?? '')}</span>
        </div>
      ))}
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Review dialog (reason capture)                                     */
/* ------------------------------------------------------------------ */

function ReviewDialog({
  state,
  onChange,
  onClose,
  onSubmit,
  submitting,
}: {
  state: ReviewDialogState
  onChange: (reason: string) => void
  onClose: () => void
  onSubmit: () => void
  submitting: boolean
}) {
  const { t } = useTranslation()
  const reasonRequired = state.action === 'ignore'
  const disabled = submitting || (reasonRequired && state.reason.trim() === '')
  const count = state.ids.length
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="review-dialog-title"
    >
      <div className="bg-surface-container rounded-lg max-w-lg w-full p-6 shadow-xl">
        <h2 id="review-dialog-title" className="font-headline font-bold text-lg text-on-surface mb-2">
          {state.action === 'approve'
            ? t('auto_discovery.dialog_approve_title', { count })
            : t('auto_discovery.dialog_ignore_title', { count })}
        </h2>
        <p className="text-sm text-on-surface-variant mb-4">
          {state.action === 'approve'
            ? t('auto_discovery.dialog_approve_desc')
            : t('auto_discovery.dialog_ignore_desc')}
        </p>
        <label className="block text-xs text-on-surface-variant mb-1" htmlFor="review-reason">
          {reasonRequired
            ? t('auto_discovery.dialog_reason_required')
            : t('auto_discovery.dialog_reason_optional')}
        </label>
        <textarea
          id="review-reason"
          value={state.reason}
          onChange={(e) => onChange(e.target.value)}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-3 outline-none resize-none"
          rows={4}
          placeholder={
            state.action === 'approve'
              ? t('auto_discovery.dialog_reason_placeholder_approve')
              : t('auto_discovery.dialog_reason_placeholder_ignore')
          }
        />
        <div className="flex justify-end gap-2 mt-4">
          <button
            onClick={onClose}
            className="px-4 py-2 rounded-lg text-sm text-on-surface-variant hover:bg-surface-container-high transition-colors"
            disabled={submitting}
          >
            {t('common.cancel')}
          </button>
          <button
            onClick={onSubmit}
            disabled={disabled}
            className={`px-4 py-2 rounded-lg text-sm font-semibold transition-colors ${
              state.action === 'approve'
                ? 'bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30'
                : 'bg-red-500/20 text-red-400 hover:bg-red-500/30'
            } disabled:opacity-40 disabled:cursor-not-allowed`}
          >
            {submitting
              ? t('common.saving')
              : state.action === 'approve'
              ? t('auto_discovery.dialog_confirm_approve')
              : t('auto_discovery.dialog_confirm_ignore')}
          </button>
        </div>
      </div>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export default function AutoDiscovery() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  const [urlState, setUrlState] = useUrlState('discovery', discoveryListDefaults)
  const { activeTab, statusFilter, sourceFilter } = urlState

  const queryParams: Record<string, string> = {}
  if (statusFilter !== 'all') queryParams.status = statusFilter

  const { data: listData, isLoading, error, refetch } = useDiscoveredAssets(queryParams)
  const { data: statsData } = useDiscoveryStats()
  const approveMutation = useApproveAsset()
  const ignoreMutation = useIgnoreAsset()

  const assets: DiscoveredAsset[] = listData?.data ?? []
  const stats: DiscoveryStatsData = statsData?.data ?? { total: 0, pending: 0, conflict: 0, approved: 0, ignored: 0, matched: 0 }

  /* Source filter is client-side since API only supports status */
  const filtered = useMemo(
    () => assets.filter((a) => (sourceFilter === 'all' ? true : a.source === sourceFilter)),
    [assets, sourceFilter],
  )

  const actionableIds = useMemo(
    () => filtered.filter((a) => a.status === 'pending' || a.status === 'conflict').map((a) => a.id),
    [filtered],
  )

  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [dialog, setDialog] = useState<ReviewDialogState | null>(null)

  const allSelected = actionableIds.length > 0 && actionableIds.every((id) => selected.has(id))
  const toggleAll = useCallback(() => {
    setSelected((prev) => {
      if (actionableIds.every((id) => prev.has(id)) && actionableIds.length > 0) {
        return new Set()
      }
      return new Set(actionableIds)
    })
  }, [actionableIds])

  const toggleOne = useCallback((id: string) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [])

  const openDialog = useCallback((action: ReviewAction, ids: string[]) => {
    if (ids.length === 0) return
    setDialog({ action, ids, reason: '' })
  }, [])

  const submitDialog = useCallback(async () => {
    if (!dialog) return
    const { action, ids, reason } = dialog
    const trimmed = reason.trim()
    try {
      if (action === 'approve') {
        await Promise.all(ids.map((id) => approveMutation.mutateAsync({ id, reason: trimmed || undefined })))
      } else {
        if (trimmed === '') return
        await Promise.all(ids.map((id) => ignoreMutation.mutateAsync({ id, reason: trimmed })))
      }
      setSelected((prev) => {
        const next = new Set(prev)
        ids.forEach((id) => next.delete(id))
        return next
      })
      setDialog(null)
    } catch (_e: unknown) {
      // Mutation errors surface through the mutation hook's error state; the
      // react-query onSuccess still invalidates queries so the list refreshes.
      setDialog(null)
    }
  }, [dialog, approveMutation, ignoreMutation])

  const submitting = approveMutation.isPending || ignoreMutation.isPending

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      {/* ============================================================ */}
      {/*  Header                                                       */}
      {/* ============================================================ */}
      <header className="px-8 pt-6 pb-4">
        <nav className="flex items-center gap-1.5 text-xs text-on-surface-variant mb-3">
          <span className="hover:text-primary cursor-pointer" onClick={() => navigate('/assets')}>{t('auto_discovery.breadcrumb_asset_management')}</span>
          <Icon name="chevron_right" className="text-[14px] text-on-surface-variant" />
          <span className="text-primary">{t('auto_discovery.breadcrumb_auto_discovery')}</span>
        </nav>

        <div className="flex items-start justify-between flex-wrap gap-4">
          <div>
            <h1 className="font-headline font-bold text-2xl text-on-surface">
              {t('auto_discovery.title')}
            </h1>
            <p className="text-sm text-on-surface-variant mt-1">
              {t('auto_discovery.subtitle')}
            </p>
          </div>
        </div>
      </header>

      {/* ============================================================ */}
      {/*  Tab switcher                                                 */}
      {/* ============================================================ */}
      <div className="px-8 pb-2 flex gap-1">
        {(['review', 'scan'] as const).map(tab => (
          <button key={tab} onClick={() => setUrlState({ activeTab: tab })}
            className={`px-4 py-2 rounded-lg text-sm font-semibold transition-colors ${
              activeTab === tab ? 'bg-primary text-on-primary' : 'text-on-surface-variant hover:bg-surface-container-high'
            }`}>
            {tab === 'review' ? t('auto_discovery.tab_review') : t('auto_discovery.tab_scan')}
          </button>
        ))}
      </div>

      {activeTab === 'review' && (<>

      {/* ============================================================ */}
      {/*  Stats row                                                    */}
      {/* ============================================================ */}
      <section className="px-8 pb-4">
        <div className="flex flex-wrap items-stretch gap-4">
          <StatCard label={t('auto_discovery.stat_total')} value={stats.total} icon="inventory_2" />
          <StatCard label={t('auto_discovery.stat_pending')} value={stats.pending} icon="pending_actions" color="text-blue-400" />
          <StatCard label={t('auto_discovery.stat_conflict')} value={stats.conflict} icon="warning" color="text-amber-400" />
          <StatCard label={t('auto_discovery.stat_approved')} value={stats.approved} icon="check_circle" color="text-emerald-400" />
          <StatCard label={t('auto_discovery.stat_ignored')} value={stats.ignored} icon="block" color="text-on-surface-variant" />
          <StatCard label={t('auto_discovery.stat_matched')} value={stats.matched} icon="link" color="text-primary" />
        </div>
      </section>

      {/* ============================================================ */}
      {/*  Filters + bulk actions                                       */}
      {/* ============================================================ */}
      <section className="px-8 pb-4 flex flex-wrap items-center gap-3">
        <div className="relative">
          <select
            value={sourceFilter}
            onChange={e => setUrlState({ sourceFilter: e.target.value })}
            className="appearance-none bg-surface-container-high text-on-surface text-sm rounded-lg pl-3 pr-8 py-2 outline-none cursor-pointer"
            aria-label="Filter by source"
          >
            <option value="all">{t('auto_discovery.filter_all_sources')}</option>
            <option value="VMware">VMware</option>
            <option value="SNMP">SNMP</option>
            <option value="SSH">SSH</option>
            <option value="IPMI">IPMI</option>
            <option value="manual">{t('auto_discovery.source_manual')}</option>
          </select>
          <Icon name="expand_more" className="absolute right-2 top-1/2 -translate-y-1/2 text-[16px] text-on-surface-variant pointer-events-none" />
        </div>

        <div className="relative">
          <select
            value={statusFilter}
            onChange={e => setUrlState({ statusFilter: e.target.value })}
            className="appearance-none bg-surface-container-high text-on-surface text-sm rounded-lg pl-3 pr-8 py-2 outline-none cursor-pointer"
            aria-label="Filter by status"
          >
            <option value="all">{t('auto_discovery.filter_all_statuses')}</option>
            <option value="pending">{t('auto_discovery.status_pending')}</option>
            <option value="conflict">{t('auto_discovery.status_conflict')}</option>
            <option value="approved">{t('auto_discovery.status_approved')}</option>
            <option value="ignored">{t('auto_discovery.status_ignored')}</option>
          </select>
          <Icon name="expand_more" className="absolute right-2 top-1/2 -translate-y-1/2 text-[16px] text-on-surface-variant pointer-events-none" />
        </div>

        {selected.size > 0 && (
          <div className="flex items-center gap-2 ml-2">
            <span className="text-xs text-on-surface-variant">
              {t('auto_discovery.selected_count', { count: selected.size })}
            </span>
            <button
              onClick={() => openDialog('approve', Array.from(selected))}
              className="px-3 py-1.5 rounded-md bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30 text-xs font-semibold transition-colors"
            >
              {t('auto_discovery.btn_batch_approve')}
            </button>
            <button
              onClick={() => openDialog('ignore', Array.from(selected))}
              className="px-3 py-1.5 rounded-md bg-red-500/20 text-red-400 hover:bg-red-500/30 text-xs font-semibold transition-colors"
            >
              {t('auto_discovery.btn_ignore_selected')}
            </button>
            <button
              onClick={() => setSelected(new Set())}
              className="px-2 py-1.5 rounded-md text-on-surface-variant hover:bg-surface-container-high text-xs transition-colors"
            >
              {t('common.clear')}
            </button>
          </div>
        )}

        <div className="ml-auto text-xs text-on-surface-variant">
          {t('auto_discovery.showing_results', { count: filtered.length })}
        </div>
      </section>

      {/* ============================================================ */}
      {/*  Discovery table                                              */}
      {/* ============================================================ */}
      <section className="px-8 pb-4">
        <div className="bg-surface-container rounded-lg overflow-x-auto" role="table" aria-label="Discovery results">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-surface-container-high text-on-surface-variant text-[0.6875rem] uppercase tracking-wider">
                <th className="px-3 py-3 w-8">
                  <input
                    type="checkbox"
                    aria-label="Select all"
                    checked={allSelected}
                    onChange={toggleAll}
                    disabled={actionableIds.length === 0}
                    className="accent-primary cursor-pointer disabled:cursor-not-allowed"
                  />
                </th>
                <th className="px-4 py-3 text-left font-semibold">{t('auto_discovery.table_source')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('auto_discovery.table_hostname')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('auto_discovery.table_ip_address')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('auto_discovery.table_status')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('auto_discovery.table_confidence')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('auto_discovery.table_details')}</th>
                <th className="px-4 py-3 text-right font-semibold">{t('auto_discovery.table_actions')}</th>
              </tr>
            </thead>
            <tbody>
              {isLoading && (
                <tr><td colSpan={8} className="py-10 text-center">
                  <div className="inline-block animate-spin rounded-full h-6 w-6 border-2 border-sky-400 border-t-transparent" />
                </td></tr>
              )}
              {error && (
                <tr><td colSpan={8} className="py-4 text-center text-red-300 text-sm">
                  Failed to load discovered assets. <button onClick={() => refetch()} className="underline">Retry</button>
                </td></tr>
              )}
              {filtered.map(row => {
                const src = sourceIcon[row.source] ?? sourceIcon.SNMP
                const canAct = row.status === 'pending' || row.status === 'conflict'
                return (
                  <tr key={row.id} className="bg-surface-container hover:bg-surface-container-high transition-colors border-t border-surface-container-high">
                    <td className="px-3 py-3">
                      <input
                        type="checkbox"
                        aria-label={`Select ${row.hostname ?? row.id}`}
                        checked={selected.has(row.id)}
                        onChange={() => toggleOne(row.id)}
                        disabled={!canAct}
                        className="accent-primary cursor-pointer disabled:cursor-not-allowed disabled:opacity-30"
                      />
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <span className={`inline-flex items-center justify-center w-7 h-7 rounded-md ${src.bg}`}>
                          <Icon name={src.icon} className="text-[16px] text-on-surface" />
                        </span>
                        <span className="text-on-surface font-medium">{row.source}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-primary font-medium">{row.hostname ?? '-'}</td>
                    <td className="px-4 py-3 font-mono text-on-surface-variant">{row.ip_address ?? '-'}</td>
                    <td className="px-4 py-3">
                      <StatusBadge status={row.status} />
                    </td>
                    <td className="px-4 py-3">
                      <ConfidencePill value={row.match_confidence} strategy={row.match_strategy} />
                    </td>
                    <td className="px-4 py-3">
                      {row.status === 'conflict' && row.diff_details ? (
                        <DiffDetails diff={row.diff_details as Record<string, unknown>} />
                      ) : row.matched_asset_id ? (
                        <span className="text-xs text-on-surface-variant">{t('auto_discovery.matched_prefix')}: {row.matched_asset_id.slice(0, 8)}...</span>
                      ) : (
                        <span className="text-xs text-on-surface-variant">{t('auto_discovery.new_asset')}</span>
                      )}
                      {row.review_reason && (
                        <div className="text-[0.6875rem] text-on-surface-variant italic mt-1">
                          "{row.review_reason}"
                        </div>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center justify-end gap-1">
                        {canAct && (
                          <>
                            <button
                              onClick={() => openDialog('approve', [row.id])}
                              disabled={submitting}
                              className="p-1.5 rounded-md hover:bg-[#064e3b]/40 transition-colors disabled:opacity-40"
                              aria-label={`Approve ${row.hostname}`}
                            >
                              <Icon name="check" className="text-[18px] text-[#34d399]" />
                            </button>
                            <button
                              onClick={() => openDialog('ignore', [row.id])}
                              disabled={submitting}
                              className="p-1.5 rounded-md hover:bg-error-container/40 transition-colors disabled:opacity-40"
                              aria-label={`Ignore ${row.hostname}`}
                            >
                              <Icon name="close" className="text-[18px] text-error" />
                            </button>
                          </>
                        )}
                      </div>
                    </td>
                  </tr>
                )
              })}
              {!isLoading && !error && filtered.length === 0 && (
                <tr><td colSpan={8} className="py-10 text-center text-on-surface-variant text-sm">
                  {t('auto_discovery.empty_state')}
                </td></tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      </>)}

      {activeTab === 'scan' && <ScanManagementTab />}

      {dialog && (
        <ReviewDialog
          state={dialog}
          onChange={(reason) => setDialog((d) => (d ? { ...d, reason } : d))}
          onClose={() => setDialog(null)}
          onSubmit={submitDialog}
          submitting={submitting}
        />
      )}
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Stat card sub-component                                            */
/* ------------------------------------------------------------------ */

function StatCard({ label, value, icon, color }: { label: string; value: number; icon: string; color?: string }) {
  return (
    <div className="bg-surface-container-low rounded-lg p-5 flex flex-col gap-2 min-w-[140px]">
      <div className="flex items-center justify-between">
        <span className="font-label text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant">
          {label}
        </span>
        <Icon name={icon} className={`text-[18px] ${color ?? 'text-on-surface-variant'}`} />
      </div>
      <div className={`font-headline font-bold text-2xl ${color ?? 'text-on-surface'}`}>{value}</div>
    </div>
  )
}
