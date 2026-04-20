import { useTranslation } from 'react-i18next'
import { useFailureDistribution } from '../../hooks/usePrediction'
import { useAlerts } from '../../hooks/useMonitoring'
import { Icon, GANTT_BAR_COLORS, INSIGHT_PRIORITY_COLORS, EmptyState } from './shared'

export function InsightsTab() {
  const { t } = useTranslation()
  const days = Array.from({ length: 7 }, (_, i) => `DAY ${String((i * 4) + 1).padStart(2, '0')}`)
  const { data: failDistData } = useFailureDistribution()
  const failureDist = failDistData?.distribution ?? []
  const { data: alertsResponse } = useAlerts({ status: 'firing' })
  const alerts = alertsResponse?.data ?? []

  const insightsStats = [
    { labelKey: 'predictive_hub.insights_critical_maintenance', value: failureDist.filter((d) => d.category === 'Thermal' || d.category === 'Electrical').reduce((s, d) => s + d.count, 0), statusKey: 'predictive_hub.insights_status_upcoming', color: 'text-error', bgColor: 'bg-error-container' },
    { labelKey: 'predictive_hub.insights_major_maintenance', value: failureDist.filter((d) => d.category === 'Mechanical').reduce((s, d) => s + d.count, 0), statusKey: 'predictive_hub.insights_status_pending', color: 'text-[#fbbf24]', bgColor: 'bg-[#92400e]' },
    { labelKey: 'predictive_hub.insights_minor_maintenance', value: failureDist.filter((d) => d.category === 'Software' || d.category === 'Other').reduce((s, d) => s + d.count, 0), statusKey: 'predictive_hub.insights_status_scheduled', color: 'text-primary', bgColor: 'bg-[#1e3a5f]' },
  ]

  return (
    <div className="space-y-6">
      {/* Stats row */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {insightsStats.map((s) => (
          <div key={s.labelKey} className="bg-surface-container-low rounded-lg p-5">
            <div className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase mb-2">
              {t(s.labelKey)}
            </div>
            <div className="flex items-end gap-3">
              <span className={`font-headline font-bold text-3xl ${s.color}`}>{s.value}</span>
              <span className={`${s.bgColor} ${s.color} text-[0.625rem] font-semibold tracking-wider uppercase px-2 py-0.5 rounded mb-1`}>
                {t(s.statusKey)}
              </span>
            </div>
          </div>
        ))}
      </div>

      {/* 30-Day Gantt Timeline */}
      <div className="bg-surface-container rounded-xl p-6">
        <div className="flex items-center gap-2 mb-1">
          <Icon name="timeline" className="text-primary text-[20px]" />
          <h2 className="font-headline text-sm font-bold tracking-wide text-on-surface uppercase">
            {t('predictive_insights.section_30day_timeline')}
          </h2>
        </div>
        <p className="text-on-surface-variant text-[0.6875rem] tracking-wide mb-6 ml-7">
          {t('predictive_insights.timeline_subtitle')}
        </p>

        <div className="flex gap-5 mb-5 ml-7">
          {[
            { labelKey: 'predictive_hub.legend_critical', color: 'bg-error' },
            { labelKey: 'predictive_hub.legend_major', color: 'bg-tertiary' },
            { labelKey: 'predictive_hub.legend_minor', color: 'bg-primary' },
          ].map((l) => (
            <div key={l.labelKey} className="flex items-center gap-1.5">
              <span className={`w-3 h-3 rounded-sm ${l.color}`} />
              <span className="text-[0.625rem] text-on-surface-variant uppercase tracking-wider">{t(l.labelKey)}</span>
            </div>
          ))}
        </div>

        <div className="space-y-3">
          {failureDist.length === 0 ? (
            <EmptyState message={t('predictive.no_data')} />
          ) : (
            failureDist.map((d) => {
              const barType = d.category === 'Thermal' || d.category === 'Electrical' ? 'critical'
                : d.category === 'Mechanical' ? 'major' : 'minor'
              const barStart = 2
              const barEnd = Math.min(28, barStart + Math.max(2, Math.round(d.count * 3)))
              return (
                <div key={d.category} className="flex items-center gap-4">
                  <div className="w-44 shrink-0">
                    <div className="text-xs font-semibold text-on-surface font-headline">{d.category}</div>
                    <div className="text-[0.5625rem] text-on-surface-variant tracking-wider uppercase">{d.count} {t('predictive_hub.legend_occurrences', { defaultValue: 'occurrences' })}</div>
                  </div>
                  <div className="flex-1 relative h-8 bg-surface-container-low rounded">
                    <div
                      className={`absolute top-1 bottom-1 rounded ${GANTT_BAR_COLORS[barType]} opacity-80`}
                      style={{ left: `${(barStart / 30) * 100}%`, width: `${((barEnd - barStart) / 30) * 100}%` }}
                    />
                  </div>
                </div>
              )
            })
          )}
        </div>

        <div className="flex ml-48 mt-2">
          {days.map((d) => (
            <span key={d} className="flex-1 text-[0.5625rem] text-on-surface-variant tracking-wider">{d}</span>
          ))}
        </div>
      </div>

      {/* Recommendation cards 2x2 */}
      <div>
        <div className="flex items-center gap-2 mb-4">
          <Icon name="tips_and_updates" className="text-primary text-[20px]" />
          <h2 className="font-headline text-sm font-bold tracking-wide text-on-surface uppercase">
            {t('predictive_insights.section_proactive_recommendations')}
          </h2>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {alerts.length === 0 ? (
            <div className="col-span-2">
              <EmptyState message={t('predictive.no_data')} />
            </div>
          ) : (
            alerts.slice(0, 4).map((alert) => {
              const priority = alert.severity.toLowerCase() === 'critical' ? 'CRITICAL'
                : alert.severity.toLowerCase() === 'high' ? 'HIGH' : 'MEDIUM'
              return (
                <div key={alert.id} className="bg-surface-container rounded-xl p-5 flex flex-col gap-3">
                  <div className="flex items-start justify-between gap-3">
                    <h3 className="text-sm font-semibold text-on-surface font-headline">
                      {alert.message} &mdash; {alert.ci_id}
                    </h3>
                    <span className={`shrink-0 px-2.5 py-1 rounded text-[0.625rem] font-semibold tracking-wider uppercase ${INSIGHT_PRIORITY_COLORS[priority] ?? INSIGHT_PRIORITY_COLORS.MEDIUM}`}>
                      {priority}
                    </span>
                  </div>
                  <p className="text-on-surface-variant text-xs leading-relaxed">
                    {t('predictive_insights.triggered_at', { defaultValue: 'Triggered at' })}: {new Date(alert.fired_at).toLocaleString()}
                  </p>
                  <div className="flex gap-2 mt-auto pt-1">
                    <button className="bg-on-primary-container/20 text-on-primary-container text-[0.6875rem] font-semibold tracking-wider uppercase px-4 py-2 rounded-lg hover:bg-on-primary-container/30 transition-colors">
                      {t('predictive_insights.btn_repair_now')}
                    </button>
                    <button className="bg-surface-container-high text-on-surface-variant text-[0.6875rem] font-semibold tracking-wider uppercase px-4 py-2 rounded-lg hover:bg-surface-container-highest transition-colors">
                      {t('predictive_insights.btn_detailed_report')}
                    </button>
                  </div>
                </div>
              )
            })
          )}
        </div>
      </div>
    </div>
  )
}
