import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import Icon from '../components/Icon'
import { useProblems, useCreateProblem } from '../hooks/useProblems'
import { useUrlState } from '../hooks/useUrlState'
import type { ProblemStatus, ProblemPriority } from '../lib/api/problems'

const statusStyle: Record<ProblemStatus, string> = {
  open:          'bg-red-500/20 text-red-400',
  investigating: 'bg-amber-500/20 text-amber-400',
  known_error:   'bg-purple-500/20 text-purple-400',
  resolved:      'bg-emerald-500/20 text-emerald-400',
  closed:        'bg-surface-container-highest text-on-surface-variant',
}

const priorityStyle: Record<string, string> = {
  p1: 'bg-error-container text-on-error-container',
  p2: 'bg-[#92400e] text-[#fbbf24]',
  p3: 'bg-blue-500/20 text-blue-400',
  p4: 'bg-surface-container-highest text-on-surface-variant',
}

const problemsListDefaults = {
  status: 'all' as ProblemStatus | 'all',
  priority: 'all' as ProblemPriority | 'all',
}

interface CreateDialogProps {
  open: boolean
  onClose: () => void
  onSubmit: (title: string, severity: string, priority: string, description: string) => void
  submitting: boolean
}

function CreateDialog({ open, onClose, onSubmit, submitting }: CreateDialogProps) {
  const { t } = useTranslation()
  const [title, setTitle] = useState('')
  const [severity, setSeverity] = useState('medium')
  const [priority, setPriority] = useState('p3')
  const [description, setDescription] = useState('')

  if (!open) return null
  const disabled = submitting || title.trim() === ''

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" role="dialog" aria-modal="true">
      <div className="bg-surface-container rounded-lg max-w-lg w-full p-6 shadow-xl">
        <h2 className="font-headline font-bold text-lg text-on-surface mb-2">
          {t('problems.dialog_create_title')}
        </h2>
        <p className="text-sm text-on-surface-variant mb-4">
          {t('problems.dialog_create_desc')}
        </p>

        <label className="block text-xs text-on-surface-variant mb-1">{t('problems.field_title')}</label>
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none mb-3"
          placeholder={t('problems.placeholder_title')}
        />

        <div className="grid grid-cols-2 gap-3 mb-3">
          <div>
            <label className="block text-xs text-on-surface-variant mb-1">{t('problems.field_severity')}</label>
            <select
              value={severity}
              onChange={(e) => setSeverity(e.target.value)}
              className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none"
            >
              <option value="critical">critical</option>
              <option value="high">high</option>
              <option value="medium">medium</option>
              <option value="low">low</option>
              <option value="info">info</option>
              <option value="warning">warning</option>
            </select>
          </div>
          <div>
            <label className="block text-xs text-on-surface-variant mb-1">{t('problems.field_priority')}</label>
            <select
              value={priority}
              onChange={(e) => setPriority(e.target.value)}
              className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none"
            >
              <option value="p1">P1</option>
              <option value="p2">P2</option>
              <option value="p3">P3</option>
              <option value="p4">P4</option>
            </select>
          </div>
        </div>

        <label className="block text-xs text-on-surface-variant mb-1">{t('problems.field_description')}</label>
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          rows={3}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-3 outline-none resize-none"
          placeholder={t('problems.placeholder_description')}
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
            onClick={() => onSubmit(title.trim(), severity, priority, description.trim())}
            disabled={disabled}
            className="px-4 py-2 rounded-lg text-sm font-semibold bg-primary text-on-primary hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed"
          >
            {submitting ? t('common.saving') : t('problems.btn_create')}
          </button>
        </div>
      </div>
    </div>
  )
}

export default function Problems() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [urlState, setUrlState] = useUrlState('problems', problemsListDefaults)
  const { status, priority } = urlState
  const [createOpen, setCreateOpen] = useState(false)

  const params: Parameters<typeof useProblems>[0] = { page_size: 50 }
  if (status !== 'all') params.status = status
  if (priority !== 'all') params.priority = priority

  const { data, isLoading, error, refetch } = useProblems(params)
  const create = useCreateProblem()
  const problems = data?.data ?? []

  const handleCreate = (title: string, severity: string, priorityVal: string, description: string) => {
    create.mutate(
      { title, severity, priority: priorityVal as ProblemPriority, description: description || undefined },
      {
        onSuccess: (resp) => {
          toast.success(t('problems.toast_created'))
          setCreateOpen(false)
          const id = resp.data?.id
          if (id) navigate(`/monitoring/problems/${id}`)
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
            {t('problems.breadcrumb_monitoring')}
          </span>
          <Icon name="chevron_right" className="text-[14px] text-on-surface-variant" />
          <span className="text-primary">{t('problems.title')}</span>
        </nav>

        <div className="flex items-start justify-between flex-wrap gap-4">
          <div>
            <h1 className="font-headline font-bold text-2xl text-on-surface">{t('problems.title')}</h1>
            <p className="text-sm text-on-surface-variant mt-1">{t('problems.subtitle')}</p>
          </div>
          <button
            onClick={() => setCreateOpen(true)}
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-on-primary text-sm font-semibold hover:opacity-90 transition-opacity"
          >
            <Icon name="add" className="text-[18px]" />
            {t('problems.btn_new')}
          </button>
        </div>
      </header>

      <section className="px-8 pb-4 flex flex-wrap items-center gap-3">
        <select
          value={status}
          onChange={(e) => setUrlState({ status: e.target.value as ProblemStatus | 'all' })}
          className="bg-surface-container-high text-on-surface text-sm rounded-lg pl-3 pr-8 py-2 outline-none cursor-pointer"
          aria-label="Filter by status"
        >
          <option value="all">{t('problems.filter_all_statuses')}</option>
          <option value="open">open</option>
          <option value="investigating">investigating</option>
          <option value="known_error">known_error</option>
          <option value="resolved">resolved</option>
          <option value="closed">closed</option>
        </select>
        <select
          value={priority}
          onChange={(e) => setUrlState({ priority: e.target.value as ProblemPriority | 'all' })}
          className="bg-surface-container-high text-on-surface text-sm rounded-lg pl-3 pr-8 py-2 outline-none cursor-pointer"
          aria-label="Filter by priority"
        >
          <option value="all">{t('problems.filter_all_priorities')}</option>
          <option value="p1">P1</option>
          <option value="p2">P2</option>
          <option value="p3">P3</option>
          <option value="p4">P4</option>
        </select>
        <div className="ml-auto text-xs text-on-surface-variant">
          {t('problems.results_count', { count: problems.length })}
        </div>
      </section>

      <section className="px-8 pb-8">
        <div className="bg-surface-container rounded-lg overflow-x-auto" role="table" aria-label="Problems list">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-surface-container-high text-on-surface-variant text-[0.6875rem] uppercase tracking-wider">
                <th className="px-4 py-3 text-left font-semibold">{t('problems.col_title')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('problems.col_status')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('problems.col_priority')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('problems.col_severity')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('problems.col_created')}</th>
              </tr>
            </thead>
            <tbody>
              {isLoading && (
                <tr><td colSpan={5} className="py-10 text-center">
                  <div className="inline-block animate-spin rounded-full h-6 w-6 border-2 border-sky-400 border-t-transparent" />
                </td></tr>
              )}
              {error && (
                <tr><td colSpan={5} className="py-4 text-center text-red-300 text-sm">
                  {t('problems.load_failed')}{' '}
                  <button onClick={() => refetch()} className="underline">{t('common.retry')}</button>
                </td></tr>
              )}
              {!isLoading && !error && problems.length === 0 && (
                <tr><td colSpan={5} className="py-10 text-center text-on-surface-variant text-sm">
                  {t('problems.empty_state')}
                </td></tr>
              )}
              {problems.map((p) => (
                <tr
                  key={p.id}
                  onClick={() => navigate(`/monitoring/problems/${p.id}`)}
                  className="bg-surface-container hover:bg-surface-container-high transition-colors border-t border-surface-container-high cursor-pointer"
                >
                  <td className="px-4 py-3 text-primary font-medium">{p.title}</td>
                  <td className="px-4 py-3">
                    <span className={`inline-block px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${statusStyle[p.status]}`}>
                      {p.status}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    {p.priority ? (
                      <span className={`inline-block px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${priorityStyle[p.priority]}`}>
                        {p.priority.toUpperCase()}
                      </span>
                    ) : (
                      <span className="text-xs text-on-surface-variant">—</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-xs text-on-surface-variant uppercase tracking-wider">{p.severity}</td>
                  <td className="px-4 py-3 text-xs text-on-surface-variant">{new Date(p.created_at).toLocaleString()}</td>
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
