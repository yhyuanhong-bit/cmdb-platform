import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import Icon from '../components/Icon'
import { useChanges, useCreateChange } from '../hooks/useChanges'
import { useUrlState } from '../hooks/useUrlState'
import type { ChangeStatus, ChangeType, ChangeRisk } from '../lib/api/changes'

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

const changesListDefaults = {
  status: 'all' as ChangeStatus | 'all',
  type:   'all' as ChangeType | 'all',
  risk:   'all' as ChangeRisk | 'all',
}

interface CreateDialogProps {
  open: boolean
  onClose: () => void
  onSubmit: (input: {
    title: string
    type: ChangeType
    risk: ChangeRisk
    threshold: number
    description: string
    rollbackPlan: string
    impactSummary: string
  }) => void
  submitting: boolean
}

function CreateDialog({ open, onClose, onSubmit, submitting }: CreateDialogProps) {
  const { t } = useTranslation()
  const [title, setTitle] = useState('')
  const [type, setType] = useState<ChangeType>('normal')
  const [risk, setRisk] = useState<ChangeRisk>('medium')
  const [threshold, setThreshold] = useState(1)
  const [description, setDescription] = useState('')
  const [rollbackPlan, setRollbackPlan] = useState('')
  const [impactSummary, setImpactSummary] = useState('')

  if (!open) return null
  const disabled = submitting || title.trim() === ''

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" role="dialog" aria-modal="true">
      <div className="bg-surface-container rounded-lg max-w-lg w-full p-6 shadow-xl max-h-[90vh] overflow-y-auto">
        <h2 className="font-headline font-bold text-lg text-on-surface mb-2">
          {t('changes.dialog_create_title')}
        </h2>
        <p className="text-sm text-on-surface-variant mb-4">
          {t('changes.dialog_create_desc')}
        </p>

        <label className="block text-xs text-on-surface-variant mb-1">{t('changes.field_title')}</label>
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none mb-3"
          placeholder={t('changes.placeholder_title')}
        />

        <div className="grid grid-cols-3 gap-3 mb-3">
          <div>
            <label className="block text-xs text-on-surface-variant mb-1">{t('changes.field_type')}</label>
            <select
              value={type}
              onChange={(e) => setType(e.target.value as ChangeType)}
              className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none"
            >
              <option value="standard">standard</option>
              <option value="normal">normal</option>
              <option value="emergency">emergency</option>
            </select>
          </div>
          <div>
            <label className="block text-xs text-on-surface-variant mb-1">{t('changes.field_risk')}</label>
            <select
              value={risk}
              onChange={(e) => setRisk(e.target.value as ChangeRisk)}
              className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none"
            >
              <option value="low">low</option>
              <option value="medium">medium</option>
              <option value="high">high</option>
              <option value="critical">critical</option>
            </select>
          </div>
          <div>
            <label className="block text-xs text-on-surface-variant mb-1">{t('changes.field_threshold')}</label>
            <input
              type="number"
              min={0}
              value={threshold}
              onChange={(e) => setThreshold(Number(e.target.value))}
              className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none"
            />
          </div>
        </div>

        {type === 'standard' && (
          <p className="text-xs text-amber-400 mb-3 italic">
            {t('changes.standard_threshold_note')}
          </p>
        )}

        <label className="block text-xs text-on-surface-variant mb-1">{t('changes.field_description')}</label>
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          rows={3}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-3 outline-none resize-none mb-3"
          placeholder={t('changes.placeholder_description')}
        />

        <label className="block text-xs text-on-surface-variant mb-1">{t('changes.field_impact_summary')}</label>
        <textarea
          value={impactSummary}
          onChange={(e) => setImpactSummary(e.target.value)}
          rows={2}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-3 outline-none resize-none mb-3"
          placeholder={t('changes.placeholder_impact_summary')}
        />

        <label className="block text-xs text-on-surface-variant mb-1">{t('changes.field_rollback_plan')}</label>
        <textarea
          value={rollbackPlan}
          onChange={(e) => setRollbackPlan(e.target.value)}
          rows={2}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-3 outline-none resize-none"
          placeholder={t('changes.placeholder_rollback_plan')}
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
            onClick={() =>
              onSubmit({
                title: title.trim(),
                type,
                risk,
                threshold,
                description: description.trim(),
                rollbackPlan: rollbackPlan.trim(),
                impactSummary: impactSummary.trim(),
              })
            }
            disabled={disabled}
            className="px-4 py-2 rounded-lg text-sm font-semibold bg-primary text-on-primary hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed"
          >
            {submitting ? t('common.saving') : t('changes.btn_create')}
          </button>
        </div>
      </div>
    </div>
  )
}

export default function Changes() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [urlState, setUrlState] = useUrlState('changes', changesListDefaults)
  const { status, type, risk } = urlState
  const [createOpen, setCreateOpen] = useState(false)

  const params: Parameters<typeof useChanges>[0] = { page_size: 50 }
  if (status !== 'all') params.status = status
  if (type !== 'all') params.type = type
  if (risk !== 'all') params.risk = risk

  const { data, isLoading, error, refetch } = useChanges(params)
  const create = useCreateChange()
  const changes = data?.data ?? []

  const handleCreate = (input: {
    title: string
    type: ChangeType
    risk: ChangeRisk
    threshold: number
    description: string
    rollbackPlan: string
    impactSummary: string
  }) => {
    create.mutate(
      {
        title: input.title,
        type: input.type,
        risk: input.risk,
        approval_threshold: input.threshold,
        description: input.description || undefined,
        rollback_plan: input.rollbackPlan || undefined,
        impact_summary: input.impactSummary || undefined,
      },
      {
        onSuccess: (resp) => {
          toast.success(t('changes.toast_created'))
          setCreateOpen(false)
          const id = resp.data?.id
          if (id) navigate(`/monitoring/changes/${id}`)
        },
        onError: (e: unknown) => toast.error(e instanceof Error ? e.message : t('common.unknown_error')),
      },
    )
  }

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      <header className="px-8 pt-6 pb-4">
        <nav className="flex items-center gap-1.5 text-xs text-on-surface-variant mb-3">
          <span className="hover:text-primary cursor-pointer" onClick={() => navigate('/monitoring')}>
            {t('changes.breadcrumb_monitoring')}
          </span>
          <Icon name="chevron_right" className="text-[14px] text-on-surface-variant" />
          <span className="text-primary">{t('changes.title')}</span>
        </nav>

        <div className="flex items-start justify-between flex-wrap gap-4">
          <div>
            <h1 className="font-headline font-bold text-2xl text-on-surface">{t('changes.title')}</h1>
            <p className="text-sm text-on-surface-variant mt-1">{t('changes.subtitle')}</p>
          </div>
          <button
            onClick={() => setCreateOpen(true)}
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-on-primary text-sm font-semibold hover:opacity-90 transition-opacity"
          >
            <Icon name="add" className="text-[18px]" />
            {t('changes.btn_new')}
          </button>
        </div>
      </header>

      <section className="px-8 pb-4 flex flex-wrap items-center gap-3">
        <select
          value={status}
          onChange={(e) => setUrlState({ status: e.target.value as ChangeStatus | 'all' })}
          className="bg-surface-container-high text-on-surface text-sm rounded-lg pl-3 pr-8 py-2 outline-none cursor-pointer"
          aria-label="Filter by status"
        >
          <option value="all">{t('changes.filter_all_statuses')}</option>
          <option value="draft">draft</option>
          <option value="submitted">submitted</option>
          <option value="approved">approved</option>
          <option value="rejected">rejected</option>
          <option value="in_progress">in_progress</option>
          <option value="succeeded">succeeded</option>
          <option value="failed">failed</option>
          <option value="rolled_back">rolled_back</option>
        </select>
        <select
          value={type}
          onChange={(e) => setUrlState({ type: e.target.value as ChangeType | 'all' })}
          className="bg-surface-container-high text-on-surface text-sm rounded-lg pl-3 pr-8 py-2 outline-none cursor-pointer"
          aria-label="Filter by type"
        >
          <option value="all">{t('changes.filter_all_types')}</option>
          <option value="standard">standard</option>
          <option value="normal">normal</option>
          <option value="emergency">emergency</option>
        </select>
        <select
          value={risk}
          onChange={(e) => setUrlState({ risk: e.target.value as ChangeRisk | 'all' })}
          className="bg-surface-container-high text-on-surface text-sm rounded-lg pl-3 pr-8 py-2 outline-none cursor-pointer"
          aria-label="Filter by risk"
        >
          <option value="all">{t('changes.filter_all_risks')}</option>
          <option value="low">low</option>
          <option value="medium">medium</option>
          <option value="high">high</option>
          <option value="critical">critical</option>
        </select>
        <div className="ml-auto text-xs text-on-surface-variant">
          {t('changes.results_count', { count: changes.length })}
        </div>
      </section>

      <section className="px-8 pb-8">
        <div className="bg-surface-container rounded-lg overflow-x-auto" role="table" aria-label="Changes list">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-surface-container-high text-on-surface-variant text-[0.6875rem] uppercase tracking-wider">
                <th className="px-4 py-3 text-left font-semibold">{t('changes.col_title')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('changes.col_type')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('changes.col_risk')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('changes.col_status')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('changes.col_planned')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('changes.col_created')}</th>
              </tr>
            </thead>
            <tbody>
              {isLoading && (
                <tr><td colSpan={6} className="py-10 text-center">
                  <div className="inline-block animate-spin rounded-full h-6 w-6 border-2 border-sky-400 border-t-transparent" />
                </td></tr>
              )}
              {error && (
                <tr><td colSpan={6} className="py-4 text-center text-red-300 text-sm">
                  {t('changes.load_failed')}{' '}
                  <button onClick={() => refetch()} className="underline">{t('common.retry')}</button>
                </td></tr>
              )}
              {!isLoading && !error && changes.length === 0 && (
                <tr><td colSpan={6} className="py-10 text-center text-on-surface-variant text-sm">
                  {t('changes.empty_state')}
                </td></tr>
              )}
              {changes.map((c) => (
                <tr
                  key={c.id}
                  onClick={() => navigate(`/monitoring/changes/${c.id}`)}
                  className="bg-surface-container hover:bg-surface-container-high transition-colors border-t border-surface-container-high cursor-pointer"
                >
                  <td className="px-4 py-3 text-primary font-medium">{c.title}</td>
                  <td className="px-4 py-3">
                    <span className={`inline-block px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${typeStyle[c.type]}`}>
                      {c.type}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className={`inline-block px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${riskStyle[c.risk]}`}>
                      {c.risk}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className={`inline-block px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${statusStyle[c.status]}`}>
                      {c.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-xs text-on-surface-variant">
                    {c.planned_start ? new Date(c.planned_start).toLocaleString() : '—'}
                  </td>
                  <td className="px-4 py-3 text-xs text-on-surface-variant">{new Date(c.created_at).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <CreateDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onSubmit={handleCreate}
        submitting={create.isPending}
      />
    </div>
  )
}
