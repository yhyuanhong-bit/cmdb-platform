import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import Icon from '../components/Icon'
import { useSchedulerHealth } from '../hooks/useSchedulerHealth'
import type { SchedulerHealth, SchedulerStatus } from '../lib/api/schedulerHealth'

/* ------------------------------------------------------------------ */
/*  Style + helpers                                                    */
/* ------------------------------------------------------------------ */

const statusStyle: Record<SchedulerStatus, string> = {
  ok:           'bg-emerald-500/20 text-emerald-400',
  lagging:      'bg-amber-500/20 text-amber-400',
  stale:        'bg-red-500/20 text-red-400',
  never_ticked: 'bg-red-500/30 text-red-400',
}

const cardBorder: Record<SchedulerStatus, string> = {
  ok:           'border-l-4 border-emerald-500',
  lagging:      'border-l-4 border-amber-500',
  stale:        'border-l-4 border-red-500',
  never_ticked: 'border-l-4 border-red-500',
}

function fmtSeconds(s: number): string {
  if (s < 60) return `${s}s`
  if (s < 3600) return `${Math.round(s / 60)}m`
  if (s < 86400) return `${Math.round(s / 3600)}h`
  return `${Math.round(s / 86400)}d`
}

function fmtInterval(s?: number | null): string {
  if (!s || s <= 0) return '—'
  return fmtSeconds(s)
}

/** Pretty scheduler name. The backend uses snake_case identifiers; the
 *  UI prefers spaces and Title Case so a glance reads naturally. */
function prettyName(name: string): string {
  return name.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function SchedulerHealthPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const q = useSchedulerHealth()

  const report = q.data?.data
  const schedulers: SchedulerHealth[] = report?.schedulers ?? []
  const allHealthy = report?.all_healthy ?? true

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      <header className="px-8 pt-6 pb-4">
        <nav className="flex items-center gap-1.5 text-xs text-on-surface-variant mb-3">
          <span className="hover:text-primary cursor-pointer" onClick={() => navigate('/system')}>
            {t('scheduler_health.breadcrumb_system')}
          </span>
          <Icon name="chevron_right" className="text-[14px] text-on-surface-variant" />
          <span className="text-primary">{t('scheduler_health.title')}</span>
        </nav>

        <div className="flex items-start justify-between flex-wrap gap-4">
          <div>
            <h1 className="font-headline font-bold text-2xl text-on-surface">
              {t('scheduler_health.title')}
            </h1>
            <p className="text-sm text-on-surface-variant mt-1">{t('scheduler_health.subtitle')}</p>
          </div>
        </div>
      </header>

      {/* Verdict banner */}
      <section className="px-8 pb-4">
        {q.isLoading ? (
          <div className="bg-surface-container rounded-lg p-10 flex justify-center">
            <div className="animate-spin rounded-full h-6 w-6 border-2 border-sky-400 border-t-transparent" />
          </div>
        ) : q.error ? (
          <div className="rounded-lg bg-red-900/20 p-4 text-red-300 text-sm">
            {t('scheduler_health.load_failed')}{' '}
            <button onClick={() => q.refetch()} className="underline">{t('common.retry')}</button>
          </div>
        ) : (
          <div
            className={`rounded-lg p-5 flex items-center gap-4 ${
              allHealthy
                ? 'bg-emerald-500/10 border border-emerald-500/40'
                : 'bg-red-500/10 border border-red-500/40'
            }`}
          >
            <Icon
              name={allHealthy ? 'check_circle' : 'error'}
              className={`text-[36px] ${allHealthy ? 'text-emerald-400' : 'text-red-400'}`}
            />
            <div className="min-w-0 flex-1">
              <p className={`font-headline text-lg font-bold ${allHealthy ? 'text-emerald-400' : 'text-red-400'}`}>
                {allHealthy ? t('scheduler_health.banner_ok') : t('scheduler_health.banner_unhealthy')}
              </p>
              <p className="text-xs text-on-surface-variant mt-1">
                <Icon name="autorenew" className="text-[14px] inline mr-1" />
                {t('scheduler_health.auto_refresh_hint')}
              </p>
            </div>
          </div>
        )}
      </section>

      {/* Per-scheduler cards */}
      {!q.isLoading && !q.error && (
        <section className="px-8 pb-8">
          <h2 className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant mb-3">
            {t('scheduler_health.section_schedulers')}
          </h2>
          {schedulers.length === 0 ? (
            <div className="bg-surface-container rounded-lg p-10 text-center text-on-surface-variant text-sm">
              {t('scheduler_health.no_schedulers')}
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {schedulers.map((s) => {
                const since = s.seconds_since_tick ?? null
                const interval = s.expected_interval_seconds ?? null
                const ratio = since != null && interval ? since / interval : null
                return (
                  <div key={s.name} className={`bg-surface-container rounded-lg p-5 ${cardBorder[s.status]}`}>
                    <div className="flex items-start justify-between mb-3">
                      <h3 className="font-headline text-base font-bold text-on-surface truncate">
                        {prettyName(s.name)}
                      </h3>
                      <span className={`px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${statusStyle[s.status]}`}>
                        {s.status === 'never_ticked' ? 'never' : s.status}
                      </span>
                    </div>

                    <div className="space-y-1.5 text-xs">
                      <div className="flex items-center justify-between">
                        <span className="text-on-surface-variant">{t('scheduler_health.last_tick')}</span>
                        <span className="font-mono">
                          {since == null ? (
                            <span className="text-red-400 italic">{t('scheduler_health.never')}</span>
                          ) : (
                            <span>{fmtSeconds(since)} {t('scheduler_health.ago')}</span>
                          )}
                        </span>
                      </div>
                      <div className="flex items-center justify-between">
                        <span className="text-on-surface-variant">{t('scheduler_health.interval')}</span>
                        <span className="font-mono">{fmtInterval(interval)}</span>
                      </div>
                      {ratio != null && (
                        <div className="flex items-center justify-between">
                          <span className="text-on-surface-variant">{t('scheduler_health.overdue_ratio')}</span>
                          <span
                            className={`font-mono ${
                              ratio < 1
                                ? 'text-emerald-400'
                                : ratio < 2
                                ? 'text-amber-400'
                                : 'text-red-400'
                            }`}
                          >
                            {ratio.toFixed(1)}×
                          </span>
                        </div>
                      )}
                      {s.last_tick_at && (
                        <div className="flex items-center justify-between text-[0.6875rem]">
                          <span className="text-on-surface-variant">{t('scheduler_health.last_tick_at')}</span>
                          <span className="font-mono text-on-surface-variant">
                            {new Date(s.last_tick_at).toLocaleString()}
                          </span>
                        </div>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          )}

          {/* Legend */}
          <div className="mt-6 bg-surface-container rounded-lg p-4">
            <p className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant mb-2">
              {t('scheduler_health.legend_title')}
            </p>
            <ul className="text-xs text-on-surface-variant space-y-1">
              <li>
                <span className="inline-block w-2 h-2 rounded-full bg-emerald-400 mr-2" />
                <span className="font-semibold text-on-surface">ok</span>: {t('scheduler_health.legend_ok')}
              </li>
              <li>
                <span className="inline-block w-2 h-2 rounded-full bg-amber-400 mr-2" />
                <span className="font-semibold text-on-surface">lagging</span>: {t('scheduler_health.legend_lagging')}
              </li>
              <li>
                <span className="inline-block w-2 h-2 rounded-full bg-red-400 mr-2" />
                <span className="font-semibold text-on-surface">stale</span>: {t('scheduler_health.legend_stale')}
              </li>
              <li>
                <span className="inline-block w-2 h-2 rounded-full bg-red-500 mr-2" />
                <span className="font-semibold text-on-surface">never_ticked</span>: {t('scheduler_health.legend_never')}
              </li>
            </ul>
          </div>
        </section>
      )}
    </div>
  )
}
