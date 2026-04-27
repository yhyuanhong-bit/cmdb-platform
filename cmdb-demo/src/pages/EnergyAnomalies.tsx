import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import Icon from '../components/Icon'
import {
  useEnergyAnomalies,
  useTransitionAnomaly,
} from '../hooks/useEnergyPhase2'
import type {
  EnergyAnomaly,
  EnergyAnomalyKind,
  EnergyAnomalyStatus,
} from '../lib/api/energyBilling'

/* ------------------------------------------------------------------ */
/*  Helpers + style maps                                               */
/* ------------------------------------------------------------------ */

const todayISO = () => new Date().toISOString().slice(0, 10)
const daysAgoISO = (n: number) => {
  const d = new Date()
  d.setDate(d.getDate() - n)
  return d.toISOString().slice(0, 10)
}

function fmt(n: string | number, digits = 2): string {
  const v = Number(n)
  if (!isFinite(v)) return '—'
  return v.toLocaleString(undefined, {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })
}

const kindStyle: Record<EnergyAnomalyKind, string> = {
  high: 'bg-red-500/20 text-red-400',
  low:  'bg-blue-500/20 text-blue-400',
}

const statusStyle: Record<EnergyAnomalyStatus, string> = {
  open:     'bg-amber-500/20 text-amber-400',
  ack:      'bg-blue-500/20 text-blue-400',
  resolved: 'bg-emerald-500/20 text-emerald-400',
}

// Rules for which transitions are reachable from each status. Mirrors
// the backend's lax acceptance (no SQL state-machine guard on the
// energy_anomalies UPDATE), but we constrain the UI to sensible flows
// so operators don't accidentally re-open a row they meant to ack.
const allowedActions: Record<EnergyAnomalyStatus, EnergyAnomalyStatus[]> = {
  open:     ['ack', 'resolved'],
  ack:      ['resolved', 'open'],
  resolved: ['open'],
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function EnergyAnomalies() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  const [statusTab, setStatusTab] = useState<EnergyAnomalyStatus>('open')
  const [dayFrom, setDayFrom] = useState(daysAgoISO(30))
  const [dayTo, setDayTo] = useState(todayISO())

  const listQ = useEnergyAnomalies({
    status: statusTab,
    day_from: dayFrom,
    day_to: dayTo,
    page_size: 100,
  })
  const transition = useTransitionAnomaly()

  const anomalies: EnergyAnomaly[] = listQ.data?.data ?? []
  const total = listQ.data?.pagination?.total ?? anomalies.length

  const onTransition = (a: EnergyAnomaly, next: EnergyAnomalyStatus) => {
    const note =
      next === 'ack'
        ? window.prompt(t('energy_anomalies.prompt_ack_note')) ?? undefined
        : next === 'resolved'
        ? window.prompt(t('energy_anomalies.prompt_resolve_note')) ?? undefined
        : window.prompt(t('energy_anomalies.prompt_reopen_note')) ?? undefined
    transition.mutate(
      { assetId: a.asset_id, day: a.day, status: next, note: note ?? undefined },
      {
        onSuccess: () => toast.success(t(`energy_anomalies.toast_${next}`)),
        onError: (e: unknown) => toast.error(e instanceof Error ? e.message : t('common.unknown_error')),
      },
    )
  }

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      <header className="px-8 pt-6 pb-4">
        <nav className="flex items-center gap-1.5 text-xs text-on-surface-variant mb-3">
          <span className="hover:text-primary cursor-pointer" onClick={() => navigate('/monitoring/energy/bill')}>
            {t('energy_anomalies.breadcrumb_energy')}
          </span>
          <Icon name="chevron_right" className="text-[14px] text-on-surface-variant" />
          <span className="text-primary">{t('energy_anomalies.title')}</span>
        </nav>

        <div className="flex items-start justify-between flex-wrap gap-4">
          <div>
            <h1 className="font-headline font-bold text-2xl text-on-surface">
              {t('energy_anomalies.title')}
            </h1>
            <p className="text-sm text-on-surface-variant mt-1">{t('energy_anomalies.subtitle')}</p>
          </div>
          <button
            onClick={() => navigate('/monitoring/energy/pue')}
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-surface-container-high text-on-surface text-sm font-semibold hover:bg-surface-container-highest transition-colors"
          >
            <Icon name="speed" className="text-[18px]" />
            {t('energy_anomalies.btn_view_pue')}
          </button>
        </div>
      </header>

      {/* Status tabs + date range */}
      <section className="px-8 pb-4">
        <div className="bg-surface-container rounded-lg p-5">
          <div className="flex flex-wrap items-end gap-4">
            <div className="flex gap-1">
              {(['open', 'ack', 'resolved'] as const).map((s) => (
                <button
                  key={s}
                  onClick={() => setStatusTab(s)}
                  className={`px-4 py-2 rounded-lg text-sm font-semibold transition-colors ${
                    statusTab === s
                      ? 'bg-primary text-on-primary'
                      : 'text-on-surface-variant hover:bg-surface-container-high'
                  }`}
                >
                  {t(`energy_anomalies.tab_${s}`)}
                </button>
              ))}
            </div>
            <div>
              <label className="block text-xs text-on-surface-variant mb-1">
                {t('energy_anomalies.field_day_from')}
              </label>
              <input
                type="date"
                value={dayFrom}
                onChange={(e) => setDayFrom(e.target.value)}
                className="bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none"
              />
            </div>
            <div>
              <label className="block text-xs text-on-surface-variant mb-1">
                {t('energy_anomalies.field_day_to')}
              </label>
              <input
                type="date"
                value={dayTo}
                onChange={(e) => setDayTo(e.target.value)}
                className="bg-surface-container-high text-on-surface text-sm rounded-lg p-2.5 outline-none"
              />
            </div>
            <div className="ml-auto text-xs text-on-surface-variant">
              {t('energy_anomalies.results_count', { count: total })}
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
                <th className="px-4 py-3 text-left font-semibold">{t('energy_anomalies.col_asset')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('energy_anomalies.col_day')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('energy_anomalies.col_kind')}</th>
                <th className="px-4 py-3 text-right font-semibold">{t('energy_anomalies.col_observed')}</th>
                <th className="px-4 py-3 text-right font-semibold">{t('energy_anomalies.col_baseline')}</th>
                <th className="px-4 py-3 text-right font-semibold">{t('energy_anomalies.col_score')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('energy_anomalies.col_note')}</th>
                <th className="px-4 py-3 text-right font-semibold">{t('common.actions')}</th>
              </tr>
            </thead>
            <tbody>
              {listQ.isLoading && (
                <tr><td colSpan={8} className="py-10 text-center">
                  <div className="inline-block animate-spin rounded-full h-6 w-6 border-2 border-sky-400 border-t-transparent" />
                </td></tr>
              )}
              {listQ.error && (
                <tr><td colSpan={8} className="py-4 text-center text-red-300 text-sm">
                  {t('energy_anomalies.load_failed')}{' '}
                  <button onClick={() => listQ.refetch()} className="underline">{t('common.retry')}</button>
                </td></tr>
              )}
              {!listQ.isLoading && !listQ.error && anomalies.length === 0 && (
                <tr><td colSpan={8} className="py-10 text-center text-on-surface-variant text-sm">
                  {t(`energy_anomalies.empty_${statusTab}`)}
                </td></tr>
              )}
              {anomalies.map((a) => {
                const actions = allowedActions[a.status] ?? []
                const score = Number(a.score)
                return (
                  <tr key={`${a.asset_id}:${a.day}`} className="border-t border-surface-container-high">
                    <td className="px-4 py-3">
                      <button
                        onClick={() => navigate(`/assets/${a.asset_id}`)}
                        className="text-primary font-medium hover:underline text-left"
                      >
                        {a.asset_name ?? a.asset_id.slice(0, 8) + '…'}
                      </button>
                      {a.asset_tag && (
                        <p className="text-[0.6875rem] text-on-surface-variant font-mono mt-0.5">{a.asset_tag}</p>
                      )}
                    </td>
                    <td className="px-4 py-3 text-xs text-on-surface-variant font-mono">{a.day}</td>
                    <td className="px-4 py-3">
                      <span className={`px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${kindStyle[a.kind]}`}>
                        {a.kind}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-right font-mono">{fmt(a.observed_kwh)}</td>
                    <td className="px-4 py-3 text-right font-mono text-on-surface-variant">{fmt(a.baseline_median)}</td>
                    <td className="px-4 py-3 text-right">
                      <span className={`px-2 py-0.5 rounded text-[0.625rem] font-semibold font-mono ${
                        score >= 2.0 ? 'bg-red-500/20 text-red-400' :
                        score <= 0.5 ? 'bg-blue-500/20 text-blue-400' :
                        'bg-amber-500/20 text-amber-400'
                      }`}>
                        ×{fmt(score)}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-xs text-on-surface-variant max-w-xs truncate">
                      {a.note ?? '—'}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <span className={`mr-2 px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${statusStyle[a.status]}`}>
                        {a.status}
                      </span>
                      {actions.includes('ack') && (
                        <button
                          onClick={() => onTransition(a, 'ack')}
                          disabled={transition.isPending}
                          className="ml-1 px-2 py-1 rounded text-[0.6875rem] font-semibold bg-blue-500/20 text-blue-400 hover:bg-blue-500/30 transition-colors disabled:opacity-40"
                        >
                          {t('energy_anomalies.btn_ack')}
                        </button>
                      )}
                      {actions.includes('resolved') && (
                        <button
                          onClick={() => onTransition(a, 'resolved')}
                          disabled={transition.isPending}
                          className="ml-1 px-2 py-1 rounded text-[0.6875rem] font-semibold bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30 transition-colors disabled:opacity-40"
                        >
                          {t('energy_anomalies.btn_resolve')}
                        </button>
                      )}
                      {actions.includes('open') && (
                        <button
                          onClick={() => onTransition(a, 'open')}
                          disabled={transition.isPending}
                          className="ml-1 px-2 py-1 rounded text-[0.6875rem] font-semibold bg-amber-500/20 text-amber-400 hover:bg-amber-500/30 transition-colors disabled:opacity-40"
                        >
                          {t('energy_anomalies.btn_reopen')}
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
    </div>
  )
}
