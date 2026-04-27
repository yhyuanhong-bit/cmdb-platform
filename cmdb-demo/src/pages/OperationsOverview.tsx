import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import Icon from '../components/Icon'

import { monitoringApi } from '../lib/api/monitoring'
import { problemsApi } from '../lib/api/problems'
import { changesApi } from '../lib/api/changes'
import { predictiveRefreshApi } from '../lib/api/predictiveRefresh'
import { energyBillingApi } from '../lib/api/energyBilling'
import { metricSourcesApi } from '../lib/api/metricSources'
import { schedulerHealthApi } from '../lib/api/schedulerHealth'

/* ------------------------------------------------------------------ */
/*  Operations overview — single-pane summary across every health     */
/*  surface the platform exposes. Each tile shows a count + a status  */
/*  ribbon and links to the detail page.                              */
/*                                                                     */
/*  Polling cadence (60s) matches the metric-freshness page so an     */
/*  operator who keeps this open during a shift sees fresh numbers     */
/*  without manual reload, but we don't hammer the backend.            */
/* ------------------------------------------------------------------ */

const POLL_MS = 60_000

/* ------------------------------------------------------------------ */
/*  Per-domain hooks. Each exists in its own file but the overview     */
/*  needs a single read of "current state" across all of them, so we   */
/*  use react-query directly here with a stable key per domain.        */
/* ------------------------------------------------------------------ */

function useOpenIncidents() {
  return useQuery({
    queryKey: ['ops', 'incidents'],
    queryFn: () => monitoringApi.listIncidents({ status: 'open' }),
    refetchInterval: POLL_MS,
  })
}

function useOpenProblems() {
  return useQuery({
    queryKey: ['ops', 'problems'],
    queryFn: () => problemsApi.list({ status: 'open', page_size: 1 }),
    refetchInterval: POLL_MS,
  })
}

function useSubmittedChanges() {
  return useQuery({
    queryKey: ['ops', 'changes'],
    queryFn: () => changesApi.list({ status: 'submitted', page_size: 1 }),
    refetchInterval: POLL_MS,
  })
}

function useOpenRefreshRecs() {
  return useQuery({
    queryKey: ['ops', 'predictive'],
    queryFn: () => predictiveRefreshApi.list({ status: 'open', page_size: 1 }),
    refetchInterval: POLL_MS,
  })
}

function useEnergyAnomaliesCount() {
  // Open energy anomalies in the last 30 days. We pull a small page and
  // rely on the pagination metadata for the total — same pattern the
  // anomaly page uses.
  const dayTo = new Date().toISOString().slice(0, 10)
  const dayFromDate = new Date()
  dayFromDate.setDate(dayFromDate.getDate() - 30)
  const dayFrom = dayFromDate.toISOString().slice(0, 10)
  return useQuery({
    queryKey: ['ops', 'energy-anomalies', dayFrom, dayTo],
    queryFn: () =>
      energyBillingApi.listAnomalies({
        status: 'open',
        day_from: dayFrom,
        day_to: dayTo,
        page_size: 1,
      }),
    refetchInterval: POLL_MS,
  })
}

function useStaleMetricSources() {
  return useQuery({
    queryKey: ['ops', 'freshness'],
    queryFn: () => metricSourcesApi.freshness(),
    refetchInterval: POLL_MS,
  })
}

function useSchedulerHealthOps() {
  return useQuery({
    queryKey: ['ops', 'scheduler-health'],
    queryFn: () => schedulerHealthApi.get(),
    refetchInterval: POLL_MS,
  })
}

/* ------------------------------------------------------------------ */
/*  Tile component — uniform shape across all sections                 */
/* ------------------------------------------------------------------ */

interface TileProps {
  title: string
  count: number | null
  hint?: string
  status: 'ok' | 'warn' | 'alert' | 'loading' | 'error'
  icon: string
  onClick: () => void
}

const tileBorder: Record<TileProps['status'], string> = {
  ok:      'border-l-4 border-emerald-500',
  warn:    'border-l-4 border-amber-500',
  alert:   'border-l-4 border-red-500',
  loading: 'border-l-4 border-surface-container-highest',
  error:   'border-l-4 border-red-500',
}

const tileNumberColor: Record<TileProps['status'], string> = {
  ok:      'text-emerald-400',
  warn:    'text-amber-400',
  alert:   'text-red-400',
  loading: 'text-on-surface-variant',
  error:   'text-red-400',
}

function Tile({ title, count, hint, status, icon, onClick }: TileProps) {
  const { t } = useTranslation()
  return (
    <button
      onClick={onClick}
      className={`bg-surface-container rounded-lg p-5 text-left hover:bg-surface-container-high transition-colors w-full ${tileBorder[status]}`}
    >
      <div className="flex items-start justify-between mb-3">
        <h3 className="font-headline text-base font-bold text-on-surface">{title}</h3>
        <Icon name={icon} className="text-[20px] text-on-surface-variant" />
      </div>
      {status === 'loading' ? (
        <div className="animate-spin rounded-full h-5 w-5 border-2 border-sky-400 border-t-transparent" />
      ) : status === 'error' ? (
        <p className="text-sm text-red-400">{t('ops_overview.tile_error')}</p>
      ) : (
        <>
          <p className={`font-headline text-3xl font-bold ${tileNumberColor[status]}`}>
            {count ?? '—'}
          </p>
          {hint && <p className="text-xs text-on-surface-variant mt-1">{hint}</p>}
        </>
      )}
    </button>
  )
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function OperationsOverview() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  const incidents = useOpenIncidents()
  const problems = useOpenProblems()
  const changes = useSubmittedChanges()
  const refresh = useOpenRefreshRecs()
  const energy = useEnergyAnomaliesCount()
  const freshness = useStaleMetricSources()
  const sched = useSchedulerHealthOps()

  /* ITSM */
  const incidentCount = incidents.data?.data?.length ?? null
  const problemCount = problems.data?.pagination?.total ?? problems.data?.data?.length ?? null
  const changeCount = changes.data?.pagination?.total ?? changes.data?.data?.length ?? null

  /* Energy */
  const anomalyCount = energy.data?.pagination?.total ?? energy.data?.data?.length ?? null

  /* Predictive */
  const refreshCount = refresh.data?.pagination?.total ?? refresh.data?.data?.length ?? null

  /* Metrics pipeline */
  const staleSourcesCount = freshness.data?.data?.length ?? null

  /* Schedulers */
  const allHealthy = sched.data?.data?.all_healthy ?? null
  const unhealthySchedulers =
    sched.data?.data?.schedulers?.filter(
      (s) => s.status === 'stale' || s.status === 'never_ticked',
    ).length ?? null

  /* Status helpers — convert a count into the tile's traffic-light. */
  const status = (q: { isLoading: boolean; error: unknown }, count: number | null, alertOver = 0): TileProps['status'] => {
    if (q.isLoading) return 'loading'
    if (q.error) return 'error'
    if (count == null) return 'ok'
    if (count > alertOver) return 'alert'
    return 'ok'
  }
  const warnStatus = (q: { isLoading: boolean; error: unknown }, count: number | null, warnOver = 0): TileProps['status'] => {
    if (q.isLoading) return 'loading'
    if (q.error) return 'error'
    if (count == null) return 'ok'
    if (count > warnOver) return 'warn'
    return 'ok'
  }

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      <header className="px-8 pt-6 pb-4">
        <nav className="flex items-center gap-1.5 text-xs text-on-surface-variant mb-3">
          <span className="hover:text-primary cursor-pointer" onClick={() => navigate('/dashboard')}>
            {t('ops_overview.breadcrumb_dashboard')}
          </span>
          <Icon name="chevron_right" className="text-[14px] text-on-surface-variant" />
          <span className="text-primary">{t('ops_overview.title')}</span>
        </nav>

        <div>
          <h1 className="font-headline font-bold text-2xl text-on-surface">{t('ops_overview.title')}</h1>
          <p className="text-sm text-on-surface-variant mt-1">{t('ops_overview.subtitle')}</p>
          <p className="text-[0.6875rem] text-on-surface-variant italic mt-1">
            <Icon name="autorenew" className="text-[14px] inline mr-1" />
            {t('ops_overview.auto_refresh_hint')}
          </p>
        </div>
      </header>

      {/* ITSM */}
      <section className="px-8 pb-4">
        <h2 className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant mb-3">
          {t('ops_overview.section_itsm')}
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <Tile
            title={t('ops_overview.tile_incidents')}
            count={incidentCount}
            hint={t('ops_overview.tile_incidents_hint')}
            status={status(incidents, incidentCount)}
            icon="report"
            onClick={() => navigate('/monitoring')}
          />
          <Tile
            title={t('ops_overview.tile_problems')}
            count={problemCount}
            hint={t('ops_overview.tile_problems_hint')}
            status={warnStatus(problems, problemCount)}
            icon="bug_report"
            onClick={() => navigate('/monitoring/problems?status=open')}
          />
          <Tile
            title={t('ops_overview.tile_changes')}
            count={changeCount}
            hint={t('ops_overview.tile_changes_hint')}
            status={warnStatus(changes, changeCount)}
            icon="approval"
            onClick={() => navigate('/monitoring/changes?status=submitted')}
          />
        </div>
      </section>

      {/* Capacity & energy */}
      <section className="px-8 pb-4">
        <h2 className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant mb-3">
          {t('ops_overview.section_capacity')}
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Tile
            title={t('ops_overview.tile_refresh_recs')}
            count={refreshCount}
            hint={t('ops_overview.tile_refresh_recs_hint')}
            status={warnStatus(refresh, refreshCount)}
            icon="upgrade"
            onClick={() => navigate('/predictive/refresh?status=open')}
          />
          <Tile
            title={t('ops_overview.tile_energy_anomalies')}
            count={anomalyCount}
            hint={t('ops_overview.tile_energy_anomalies_hint')}
            status={warnStatus(energy, anomalyCount)}
            icon="bolt"
            onClick={() => navigate('/monitoring/energy/anomalies')}
          />
        </div>
      </section>

      {/* Data plane health */}
      <section className="px-8 pb-8">
        <h2 className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant mb-3">
          {t('ops_overview.section_data_plane')}
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Tile
            title={t('ops_overview.tile_stale_sources')}
            count={staleSourcesCount}
            hint={t('ops_overview.tile_stale_sources_hint')}
            status={status(freshness, staleSourcesCount)}
            icon="settings_input_component"
            onClick={() => navigate('/system/metrics-freshness')}
          />
          {/* Scheduler health is a boolean-of-truth, not a count. We
              show the "unhealthy count" but colour the tile alert-red
              when all_healthy=false regardless of the count, so a single
              never-ticked scheduler still surfaces as alert. */}
          <Tile
            title={t('ops_overview.tile_schedulers')}
            count={unhealthySchedulers}
            hint={
              allHealthy === false
                ? t('ops_overview.tile_schedulers_unhealthy_hint')
                : t('ops_overview.tile_schedulers_ok_hint')
            }
            status={
              sched.isLoading
                ? 'loading'
                : sched.error
                ? 'error'
                : allHealthy
                ? 'ok'
                : 'alert'
            }
            icon="schedule"
            onClick={() => navigate('/system/scheduler-health')}
          />
        </div>
      </section>
    </div>
  )
}
