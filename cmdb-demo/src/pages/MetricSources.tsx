import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import Icon from '../components/Icon'
import {
  useMetricSources,
  useCreateMetricSource,
  useUpdateMetricSource,
  useDeleteMetricSource,
} from '../hooks/useMetricSources'
import type {
  MetricSource,
  MetricSourceKind,
  MetricSourceStatus,
  CreateMetricSourceInput,
  UpdateMetricSourceInput,
} from '../lib/api/metricSources'

/* ------------------------------------------------------------------ */
/*  Style maps                                                         */
/* ------------------------------------------------------------------ */

const kindStyle: Record<MetricSourceKind, string> = {
  snmp:     'bg-blue-500/20 text-blue-400',
  ipmi:     'bg-purple-500/20 text-purple-400',
  agent:    'bg-emerald-500/20 text-emerald-400',
  pipeline: 'bg-amber-500/20 text-amber-400',
  manual:   'bg-surface-container-highest text-on-surface-variant',
}

const statusStyle: Record<MetricSourceStatus, string> = {
  active:   'bg-emerald-500/20 text-emerald-400',
  disabled: 'bg-surface-container-highest text-on-surface-variant',
}

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function fmtInterval(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`
  if (seconds < 86400) return `${Math.round(seconds / 3600)}h`
  return `${Math.round(seconds / 86400)}d`
}

function fmtAgo(iso: string): string {
  const d = new Date(iso)
  const secs = Math.round((Date.now() - d.getTime()) / 1000)
  if (secs < 60) return `${secs}s ago`
  if (secs < 3600) return `${Math.round(secs / 60)}m ago`
  if (secs < 86400) return `${Math.round(secs / 3600)}h ago`
  return `${Math.round(secs / 86400)}d ago`
}

/* ------------------------------------------------------------------ */
/*  Form dialog (used for both create and edit)                        */
/* ------------------------------------------------------------------ */

interface FormState {
  name: string
  kind: MetricSourceKind
  intervalSeconds: number
  status: MetricSourceStatus
  notes: string
}

interface FormDialogProps {
  open: boolean
  mode: 'create' | 'edit'
  initial: FormState
  onClose: () => void
  onSubmit: (state: FormState) => void
  submitting: boolean
}

function FormDialog({ open, mode, initial, onClose, onSubmit, submitting }: FormDialogProps) {
  const { t } = useTranslation()
  const [state, setState] = useState<FormState>(initial)
  if (!open) return null
  const disabled = submitting || state.name.trim() === '' || state.intervalSeconds <= 0

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" role="dialog" aria-modal="true">
      <div className="bg-surface-container rounded-lg max-w-lg w-full p-6 shadow-xl">
        <h2 className="font-headline font-bold text-lg text-on-surface mb-2">
          {mode === 'create' ? t('metric_sources.dialog_create_title') : t('metric_sources.dialog_edit_title')}
        </h2>
        <p className="text-sm text-on-surface-variant mb-4">
          {t('metric_sources.dialog_desc')}
        </p>

        <label className="block text-xs text-on-surface-variant mb-1">{t('metric_sources.field_name')}</label>
        <input
          value={state.name}
          onChange={(e) => setState((s) => ({ ...s, name: e.target.value }))}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none mb-3"
          placeholder={t('metric_sources.placeholder_name')}
        />

        <div className="grid grid-cols-2 gap-3 mb-3">
          <div>
            <label className="block text-xs text-on-surface-variant mb-1">{t('metric_sources.field_kind')}</label>
            <select
              value={state.kind}
              onChange={(e) => setState((s) => ({ ...s, kind: e.target.value as MetricSourceKind }))}
              className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none"
            >
              <option value="snmp">snmp</option>
              <option value="ipmi">ipmi</option>
              <option value="agent">agent</option>
              <option value="pipeline">pipeline</option>
              <option value="manual">manual</option>
            </select>
          </div>
          <div>
            <label className="block text-xs text-on-surface-variant mb-1">{t('metric_sources.field_interval')}</label>
            <input
              type="number"
              min={1}
              value={state.intervalSeconds}
              onChange={(e) => setState((s) => ({ ...s, intervalSeconds: Number(e.target.value) }))}
              className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none"
            />
            <p className="text-[0.6875rem] text-on-surface-variant mt-1">
              {t('metric_sources.interval_hint')}
            </p>
          </div>
        </div>

        <label className="block text-xs text-on-surface-variant mb-1">{t('metric_sources.field_status')}</label>
        <select
          value={state.status}
          onChange={(e) => setState((s) => ({ ...s, status: e.target.value as MetricSourceStatus }))}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none mb-3"
        >
          <option value="active">active</option>
          <option value="disabled">disabled</option>
        </select>
        <p className="text-[0.6875rem] text-on-surface-variant -mt-2 mb-3">
          {t('metric_sources.status_hint')}
        </p>

        <label className="block text-xs text-on-surface-variant mb-1">{t('metric_sources.field_notes')}</label>
        <textarea
          value={state.notes}
          onChange={(e) => setState((s) => ({ ...s, notes: e.target.value }))}
          rows={2}
          className="w-full bg-surface-container-high text-on-surface text-sm rounded-lg p-3 outline-none resize-none"
          placeholder={t('metric_sources.placeholder_notes')}
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
            onClick={() => onSubmit(state)}
            disabled={disabled}
            className="px-4 py-2 rounded-lg text-sm font-semibold bg-primary text-on-primary hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed"
          >
            {submitting ? t('common.saving') : t('common.save')}
          </button>
        </div>
      </div>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function MetricSources() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [statusFilter, setStatusFilter] = useState<MetricSourceStatus | 'all'>('all')
  const [kindFilter, setKindFilter] = useState<MetricSourceKind | 'all'>('all')
  const listQ = useMetricSources(
    statusFilter === 'all' && kindFilter === 'all'
      ? undefined
      : {
          status: statusFilter === 'all' ? undefined : statusFilter,
          kind: kindFilter === 'all' ? undefined : kindFilter,
        },
  )
  const create = useCreateMetricSource()
  const update = useUpdateMetricSource()
  const remove = useDeleteMetricSource()

  const sources = listQ.data?.data ?? []

  const [createOpen, setCreateOpen] = useState(false)
  const [editing, setEditing] = useState<MetricSource | null>(null)

  const handleApiError = (e: unknown, fallback: string) => {
    const msg = e instanceof Error ? e.message : ''
    if (msg.includes('METRIC_SOURCE_DUPLICATE') || msg.toLowerCase().includes('duplicate')) {
      toast.error(t('metric_sources.toast_duplicate'))
      return
    }
    toast.error(msg || t(fallback))
  }

  const handleCreate = (s: FormState) => {
    const body: CreateMetricSourceInput = {
      name: s.name.trim(),
      kind: s.kind,
      expected_interval_seconds: s.intervalSeconds,
      status: s.status,
    }
    if (s.notes.trim()) body.notes = s.notes.trim()
    create.mutate(body, {
      onSuccess: () => {
        toast.success(t('metric_sources.toast_created'))
        setCreateOpen(false)
      },
      onError: (e: unknown) => handleApiError(e, 'common.unknown_error'),
    })
  }

  const handleEdit = (s: FormState) => {
    if (!editing) return
    const body: UpdateMetricSourceInput = {
      name: s.name.trim(),
      kind: s.kind,
      expected_interval_seconds: s.intervalSeconds,
      status: s.status,
      notes: s.notes.trim(),
    }
    update.mutate(
      { id: editing.id, body },
      {
        onSuccess: () => {
          toast.success(t('metric_sources.toast_updated'))
          setEditing(null)
        },
        onError: (e: unknown) => handleApiError(e, 'common.unknown_error'),
      },
    )
  }

  const handleDelete = (src: MetricSource) => {
    if (!window.confirm(t('metric_sources.confirm_delete', { name: src.name }))) return
    remove.mutate(src.id, {
      onSuccess: () => toast.success(t('metric_sources.toast_deleted')),
      onError: (e: unknown) => handleApiError(e, 'common.unknown_error'),
    })
  }

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      <header className="px-8 pt-6 pb-4">
        <nav className="flex items-center gap-1.5 text-xs text-on-surface-variant mb-3">
          <span className="hover:text-primary cursor-pointer" onClick={() => navigate('/system')}>
            {t('metric_sources.breadcrumb_system')}
          </span>
          <Icon name="chevron_right" className="text-[14px] text-on-surface-variant" />
          <span className="text-primary">{t('metric_sources.title')}</span>
        </nav>

        <div className="flex items-start justify-between flex-wrap gap-4">
          <div>
            <h1 className="font-headline font-bold text-2xl text-on-surface">{t('metric_sources.title')}</h1>
            <p className="text-sm text-on-surface-variant mt-1">{t('metric_sources.subtitle')}</p>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => navigate('/system/metrics-freshness')}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-surface-container-high text-on-surface text-sm font-semibold hover:bg-surface-container-highest transition-colors"
            >
              <Icon name="health_and_safety" className="text-[18px]" />
              {t('metric_sources.btn_freshness')}
            </button>
            <button
              onClick={() => setCreateOpen(true)}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-primary text-on-primary text-sm font-semibold hover:opacity-90 transition-opacity"
            >
              <Icon name="add" className="text-[18px]" />
              {t('metric_sources.btn_new')}
            </button>
          </div>
        </div>
      </header>

      {/* Filters */}
      <section className="px-8 pb-4 flex flex-wrap items-center gap-3">
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value as MetricSourceStatus | 'all')}
          className="bg-surface-container-high text-on-surface text-sm rounded-lg pl-3 pr-8 py-2 outline-none cursor-pointer"
        >
          <option value="all">{t('metric_sources.filter_all_statuses')}</option>
          <option value="active">active</option>
          <option value="disabled">disabled</option>
        </select>
        <select
          value={kindFilter}
          onChange={(e) => setKindFilter(e.target.value as MetricSourceKind | 'all')}
          className="bg-surface-container-high text-on-surface text-sm rounded-lg pl-3 pr-8 py-2 outline-none cursor-pointer"
        >
          <option value="all">{t('metric_sources.filter_all_kinds')}</option>
          <option value="snmp">snmp</option>
          <option value="ipmi">ipmi</option>
          <option value="agent">agent</option>
          <option value="pipeline">pipeline</option>
          <option value="manual">manual</option>
        </select>
        <div className="ml-auto text-xs text-on-surface-variant">
          {t('metric_sources.results_count', { count: sources.length })}
        </div>
      </section>

      {/* Table */}
      <section className="px-8 pb-8">
        <div className="bg-surface-container rounded-lg overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-surface-container-high text-on-surface-variant text-[0.6875rem] uppercase tracking-wider">
                <th className="px-4 py-3 text-left font-semibold">{t('metric_sources.col_name')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('metric_sources.col_kind')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('metric_sources.col_interval')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('metric_sources.col_last_heartbeat')}</th>
                <th className="px-4 py-3 text-right font-semibold">{t('metric_sources.col_lifetime_samples')}</th>
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
                  {t('metric_sources.load_failed')}{' '}
                  <button onClick={() => listQ.refetch()} className="underline">{t('common.retry')}</button>
                </td></tr>
              )}
              {!listQ.isLoading && !listQ.error && sources.length === 0 && (
                <tr><td colSpan={6} className="py-10 text-center text-on-surface-variant text-sm">
                  {t('metric_sources.empty_state')}
                </td></tr>
              )}
              {sources.map((src) => (
                <tr key={src.id} className={`border-t border-surface-container-high transition-colors ${src.status === 'disabled' ? 'opacity-60' : ''}`}>
                  <td className="px-4 py-3">
                    <p className="text-primary font-medium">{src.name}</p>
                    {src.notes && (
                      <p className="text-[0.6875rem] text-on-surface-variant italic mt-0.5 truncate max-w-md">
                        {src.notes}
                      </p>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <span className={`inline-block px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${kindStyle[src.kind]}`}>
                      {src.kind}
                    </span>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs">{fmtInterval(src.expected_interval_seconds)}</td>
                  <td className="px-4 py-3 text-xs">
                    {src.last_heartbeat_at ? (
                      <span className="font-mono text-on-surface-variant">{fmtAgo(src.last_heartbeat_at)}</span>
                    ) : (
                      <span className="italic text-amber-400">{t('metric_sources.never_heartbeated')}</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right font-mono text-xs">
                    {src.last_sample_count.toLocaleString()}
                  </td>
                  <td className="px-4 py-3 text-right whitespace-nowrap">
                    <span className={`mr-2 px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${statusStyle[src.status]}`}>
                      {src.status}
                    </span>
                    <button
                      onClick={() => setEditing(src)}
                      className="p-1.5 rounded-md hover:bg-surface-container-high transition-colors"
                      aria-label={t('common.edit')}
                    >
                      <Icon name="edit" className="text-[18px] text-primary" />
                    </button>
                    <button
                      onClick={() => handleDelete(src)}
                      disabled={remove.isPending}
                      className="ml-1 p-1.5 rounded-md hover:bg-error-container/40 transition-colors disabled:opacity-40"
                      aria-label={t('common.delete')}
                    >
                      <Icon name="delete" className="text-[18px] text-error" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <FormDialog
        open={createOpen}
        mode="create"
        initial={{ name: '', kind: 'agent', intervalSeconds: 60, status: 'active', notes: '' }}
        onClose={() => setCreateOpen(false)}
        onSubmit={handleCreate}
        submitting={create.isPending}
      />
      {editing && (
        <FormDialog
          open={true}
          mode="edit"
          initial={{
            name: editing.name,
            kind: editing.kind,
            intervalSeconds: editing.expected_interval_seconds,
            status: editing.status,
            notes: editing.notes ?? '',
          }}
          onClose={() => setEditing(null)}
          onSubmit={handleEdit}
          submitting={update.isPending}
        />
      )}
    </div>
  )
}
