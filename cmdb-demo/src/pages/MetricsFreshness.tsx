import { useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import Icon from '../components/Icon'
import { useMetricSourceFreshness } from '../hooks/useMetricSources'
import type { MetricSourceFreshness } from '../lib/api/metricSources'

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function fmtSeconds(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`
  if (seconds < 86400) return `${Math.round(seconds / 3600)}h`
  return `${Math.round(seconds / 86400)}d`
}

function fmtInterval(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`
  if (seconds < 86400) return `${Math.round(seconds / 3600)}h`
  return `${Math.round(seconds / 86400)}d`
}

/** Severity tier from the overdue ratio. The detector flags at 2× the
 *  expected interval; from there:
 *    2-4×  warning
 *    4-10× alert (yellow → orange)
 *    >10×  critical (red)
 *  Never-heartbeated is its own tier — operator hasn't proven the
 *  source can talk to us at all, so it goes red regardless of interval.
 */
function severityClass(row: MetricSourceFreshness): string {
  if (row.seconds_since_heartbeat == null) return 'border-l-4 border-red-500'
  const ratio = row.seconds_since_heartbeat / row.expected_interval_seconds
  if (ratio > 10) return 'border-l-4 border-red-500'
  if (ratio > 4)  return 'border-l-4 border-orange-500'
  return 'border-l-4 border-amber-500'
}

function severityBadge(row: MetricSourceFreshness): { label: string; cls: string } {
  if (row.seconds_since_heartbeat == null) {
    return { label: 'never', cls: 'bg-red-500/30 text-red-400' }
  }
  const ratio = row.seconds_since_heartbeat / row.expected_interval_seconds
  if (ratio > 10) return { label: 'critical', cls: 'bg-red-500/30 text-red-400' }
  if (ratio > 4)  return { label: 'alert',    cls: 'bg-orange-500/30 text-orange-400' }
  return { label: 'warning', cls: 'bg-amber-500/30 text-amber-400' }
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function MetricsFreshness() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const freshnessQ = useMetricSourceFreshness()
  const stale = freshnessQ.data?.data ?? []

  // Sort: never-heartbeated first, then by overdue ratio descending.
  const sorted = useMemo(() => {
    return [...stale].sort((a, b) => {
      if (a.seconds_since_heartbeat == null && b.seconds_since_heartbeat != null) return -1
      if (b.seconds_since_heartbeat == null && a.seconds_since_heartbeat != null) return 1
      if (a.seconds_since_heartbeat == null && b.seconds_since_heartbeat == null) return 0
      const ra = (a.seconds_since_heartbeat ?? 0) / a.expected_interval_seconds
      const rb = (b.seconds_since_heartbeat ?? 0) / b.expected_interval_seconds
      return rb - ra
    })
  }, [stale])

  // Tier counts for the summary strip.
  const tiers = useMemo(() => {
    let critical = 0
    let alert = 0
    let warning = 0
    let never = 0
    for (const r of stale) {
      if (r.seconds_since_heartbeat == null) {
        never += 1
        continue
      }
      const ratio = r.seconds_since_heartbeat / r.expected_interval_seconds
      if (ratio > 10) critical += 1
      else if (ratio > 4) alert += 1
      else warning += 1
    }
    return { critical, alert, warning, never }
  }, [stale])

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      <header className="px-8 pt-6 pb-4">
        <nav className="flex items-center gap-1.5 text-xs text-on-surface-variant mb-3">
          <span className="hover:text-primary cursor-pointer" onClick={() => navigate('/system')}>
            {t('metrics_freshness.breadcrumb_system')}
          </span>
          <Icon name="chevron_right" className="text-[14px] text-on-surface-variant" />
          <span className="text-primary">{t('metrics_freshness.title')}</span>
        </nav>

        <div className="flex items-start justify-between flex-wrap gap-4">
          <div>
            <h1 className="font-headline font-bold text-2xl text-on-surface">
              {t('metrics_freshness.title')}
            </h1>
            <p className="text-sm text-on-surface-variant mt-1">{t('metrics_freshness.subtitle')}</p>
          </div>
          <button
            onClick={() => navigate('/system/metrics-sources')}
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-surface-container-high text-on-surface text-sm font-semibold hover:bg-surface-container-highest transition-colors"
          >
            <Icon name="settings_input_component" className="text-[18px]" />
            {t('metrics_freshness.btn_manage_sources')}
          </button>
        </div>
      </header>

      {/* Tier summary */}
      <section className="px-8 pb-4">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          <div className="bg-surface-container rounded-lg p-4 border-l-4 border-red-500">
            <p className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant">
              {t('metrics_freshness.tier_never')}
            </p>
            <p className="font-headline text-2xl font-bold text-red-400">{tiers.never}</p>
          </div>
          <div className="bg-surface-container rounded-lg p-4 border-l-4 border-red-500">
            <p className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant">
              {t('metrics_freshness.tier_critical')}
            </p>
            <p className="font-headline text-2xl font-bold text-red-400">{tiers.critical}</p>
          </div>
          <div className="bg-surface-container rounded-lg p-4 border-l-4 border-orange-500">
            <p className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant">
              {t('metrics_freshness.tier_alert')}
            </p>
            <p className="font-headline text-2xl font-bold text-orange-400">{tiers.alert}</p>
          </div>
          <div className="bg-surface-container rounded-lg p-4 border-l-4 border-amber-500">
            <p className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant">
              {t('metrics_freshness.tier_warning')}
            </p>
            <p className="font-headline text-2xl font-bold text-amber-400">{tiers.warning}</p>
          </div>
        </div>
      </section>

      {/* Auto-refresh notice */}
      <section className="px-8 pb-2">
        <p className="text-[0.6875rem] text-on-surface-variant italic">
          <Icon name="autorenew" className="text-[14px] inline mr-1" />
          {t('metrics_freshness.auto_refresh_hint')}
        </p>
      </section>

      {/* Stale list */}
      <section className="px-8 pb-8">
        {freshnessQ.isLoading ? (
          <div className="bg-surface-container rounded-lg p-10 flex justify-center">
            <div className="animate-spin rounded-full h-6 w-6 border-2 border-sky-400 border-t-transparent" />
          </div>
        ) : freshnessQ.error ? (
          <div className="rounded-lg bg-red-900/20 p-4 text-red-300 text-sm">
            {t('metrics_freshness.load_failed')}{' '}
            <button onClick={() => freshnessQ.refetch()} className="underline">{t('common.retry')}</button>
          </div>
        ) : sorted.length === 0 ? (
          <div className="bg-surface-container rounded-lg p-10 text-center">
            <Icon name="check_circle" className="text-[40px] text-emerald-400 mb-2" />
            <p className="text-sm text-on-surface">{t('metrics_freshness.all_healthy')}</p>
            <p className="text-xs text-on-surface-variant mt-1">
              {t('metrics_freshness.all_healthy_hint')}
            </p>
          </div>
        ) : (
          <div className="space-y-2">
            {sorted.map((r) => {
              const sev = severityBadge(r)
              const ratio = r.seconds_since_heartbeat != null
                ? r.seconds_since_heartbeat / r.expected_interval_seconds
                : null
              return (
                <button
                  key={r.id}
                  onClick={() => navigate('/system/metrics-sources')}
                  className={`w-full bg-surface-container hover:bg-surface-container-high rounded-lg p-4 transition-colors text-left ${severityClass(r)}`}
                >
                  <div className="flex items-center justify-between flex-wrap gap-3">
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className={`px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${sev.cls}`}>
                          {sev.label}
                        </span>
                        <p className="text-sm text-primary font-semibold truncate">{r.name}</p>
                        <span className="px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider bg-surface-container-highest text-on-surface-variant">
                          {r.kind}
                        </span>
                      </div>
                      <p className="text-xs text-on-surface-variant mt-1">
                        {r.seconds_since_heartbeat == null ? (
                          <>
                            <Icon name="error" className="text-[14px] text-red-400 inline mr-1" />
                            {t('metrics_freshness.never_heartbeated_hint', { interval: fmtInterval(r.expected_interval_seconds) })}
                          </>
                        ) : (
                          <>
                            {t('metrics_freshness.last_seen', { ago: fmtSeconds(r.seconds_since_heartbeat) })} ·{' '}
                            {t('metrics_freshness.expected_every', { interval: fmtInterval(r.expected_interval_seconds) })}
                            {ratio != null && (
                              <span className="font-mono ml-1">({ratio.toFixed(1)}× overdue)</span>
                            )}
                          </>
                        )}
                      </p>
                    </div>
                    <Icon name="chevron_right" className="text-[20px] text-on-surface-variant" />
                  </div>
                </button>
              )
            })}
          </div>
        )}
      </section>
    </div>
  )
}
