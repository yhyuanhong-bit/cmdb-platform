import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import StatCard from '../../components/StatCard'
import StatusBadge from '../../components/StatusBadge'
import { useAlerts } from '../../hooks/useMonitoring'
import { Icon, RISK_COLOR, ConfidenceBar, EmptyState, LoadingSpinner } from './shared'

export function RecommendationsTab() {
  const { t } = useTranslation()
  const { data: alertsResponse, isLoading: alertsLoading } = useAlerts({ status: 'firing' })
  const alerts = alertsResponse?.data ?? []

  const recRows = useMemo(() => alerts.slice(0, 6).map((alert) => {
    const urgency = alert.severity.toLowerCase() === 'critical' ? 'CRITICAL' as const
      : alert.severity.toLowerCase() === 'high' ? 'HIGH' as const
      : alert.severity.toLowerCase() === 'medium' ? 'MEDIUM' as const : 'LOW' as const
    const confidence = alert.severity.toLowerCase() === 'critical' ? 94
      : alert.severity.toLowerCase() === 'high' ? 87
      : alert.severity.toLowerCase() === 'medium' ? 72 : 55
    return {
      id: alert.id,
      asset: alert.ci_id,
      failureMode: alert.message,
      urgency,
      confidence,
      action: alert.message,
    }
  }), [alerts])

  return (
    <div className="space-y-6">
      {/* Stats row */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCard icon="warning" label={t('predictive_recommendations.stat_critical_assets_at_risk')} value={alerts.filter(a => a.severity.toLowerCase() === 'critical').length || 0} sub={t('predictive_recommendations.stat_critical_sub')} subColor="text-error" />
        <StatCard icon="schedule" label={t('predictive_recommendations.stat_downtime_saved')} value="128.5h" sub={t('predictive_recommendations.stat_downtime_sub')} subColor="text-[#34d399]" />
        <StatCard icon="verified" label={t('predictive_recommendations.stat_system_reliability')} value="99.98%" sub={t('predictive_recommendations.stat_reliability_sub')} subColor="text-[#34d399]" />
        <StatCard icon="query_stats" label={t('predictive_recommendations.stat_roi_diagnostics')} value="87%" sub={t('predictive_recommendations.stat_roi_sub')} subColor="text-primary" />
      </div>

      {/* Confidence table */}
      <div className="bg-surface-container rounded-xl overflow-hidden">
        <div className="grid grid-cols-[1fr_1.5fr_0.7fr_1fr_1.5fr] gap-4 px-6 py-3 bg-surface-container-high">
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('predictive_recommendations.table_asset_identity')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('predictive_recommendations.table_predicted_failure_mode')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('predictive_recommendations.table_urgency')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('predictive_recommendations.table_confidence')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('predictive_recommendations.table_recommended_action')}</span>
        </div>

        {alertsLoading ? (
          <LoadingSpinner />
        ) : recRows.length === 0 ? (
          <EmptyState message={t('predictive.no_data')} />
        ) : (
          recRows.map((row, idx) => (
            <div
              key={row.id}
              className={`grid grid-cols-[1fr_1.5fr_0.7fr_1fr_1.5fr] gap-4 px-6 py-4 items-center ${
                idx % 2 === 0 ? 'bg-surface-container' : 'bg-surface-container-low'
              }`}
            >
              <div className="flex items-center gap-3">
                <Icon name="dns" className="text-primary text-[20px]" />
                <span className="text-sm font-semibold text-on-surface font-headline">{row.asset}</span>
              </div>
              <span className="text-sm text-on-surface-variant">{row.failureMode}</span>
              <div>
                <StatusBadge status={row.urgency} />
              </div>
              <ConfidenceBar value={row.confidence} />
              <span className="text-xs text-on-surface-variant leading-relaxed">{row.action}</span>
            </div>
          ))
        )}
      </div>

      {/* Bottom panels */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Regional Risk Heatmap */}
        <div className="bg-surface-container rounded-xl p-5">
          <div className="flex items-center gap-2 mb-4">
            <Icon name="map" className="text-primary text-[20px]" />
            <h2 className="font-headline text-sm font-bold tracking-wide text-on-surface uppercase">
              {t('predictive_recommendations.section_regional_risk_heatmap')}
            </h2>
          </div>
          <div className="grid grid-cols-3 gap-2">
            {alerts.length === 0 ? (
              <div className="col-span-3">
                <EmptyState message={t('predictive.no_data')} />
              </div>
            ) : (
              alerts.slice(0, 12).map((alert, idx) => {
                const risk = alert.severity.toLowerCase() === 'critical' ? 'critical'
                  : alert.severity.toLowerCase() === 'high' ? 'high'
                  : alert.severity.toLowerCase() === 'medium' ? 'medium' : 'low'
                return (
                  <div
                    key={alert.id ?? idx}
                    className={`${RISK_COLOR[risk]} rounded-lg p-3 flex flex-col items-center justify-center min-h-[60px]`}
                  >
                    <span className="text-[0.625rem] font-semibold text-on-surface tracking-wider uppercase">{alert.ci_id.slice(0, 10)}</span>
                    <span className="text-[0.5625rem] text-on-surface-variant uppercase tracking-wider mt-0.5">{risk}</span>
                  </div>
                )
              })
            )}
          </div>
          <div className="flex gap-4 mt-4">
            {['critical', 'high', 'medium', 'low'].map((level) => (
              <div key={level} className="flex items-center gap-1.5">
                <span className={`w-2.5 h-2.5 rounded-sm ${RISK_COLOR[level]}`} />
                <span className="text-[0.5625rem] text-on-surface-variant uppercase tracking-wider">{level}</span>
              </div>
            ))}
          </div>
        </div>

        {/* AI Model Health */}
        <div className="bg-surface-container rounded-xl p-5">
          <div className="flex items-center gap-2 mb-4">
            <Icon name="model_training" className="text-primary text-[20px]" />
            <h2 className="font-headline text-sm font-bold tracking-wide text-on-surface uppercase">
              {t('predictive_recommendations.section_ai_model_health')}
            </h2>
          </div>
          <div className="space-y-5">
            <div>
              <div className="flex items-center justify-between mb-2">
                <span className="text-[0.6875rem] text-on-surface-variant tracking-wider uppercase font-semibold">
                  {t('predictive_recommendations.label_prediction_accuracy')}
                </span>
                <span className="text-sm font-bold font-headline text-[#34d399]">94.2%</span>
              </div>
              <div className="h-2.5 bg-surface-container-low rounded-full overflow-hidden">
                <div className="h-full bg-[#34d399] rounded-full" style={{ width: '94.2%' }} />
              </div>
            </div>
            <div>
              <div className="flex items-center justify-between mb-2">
                <span className="text-[0.6875rem] text-on-surface-variant tracking-wider uppercase font-semibold">
                  {t('predictive_recommendations.label_data_ingestion_latency')}
                </span>
                <span className="text-sm font-bold font-headline text-primary">1.1ms</span>
              </div>
              <div className="h-2.5 bg-surface-container-low rounded-full overflow-hidden">
                <div className="h-full bg-primary rounded-full" style={{ width: '5%' }} />
              </div>
            </div>
            <div className="bg-surface-container-low rounded-lg p-4 space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-[0.625rem] text-on-surface-variant tracking-wider uppercase">{t('predictive_recommendations.label_model_version')}</span>
                <span className="text-xs text-on-surface font-mono">v3.8.1-stable</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-[0.625rem] text-on-surface-variant tracking-wider uppercase">{t('predictive_recommendations.label_last_retrained')}</span>
                <span className="text-xs text-on-surface font-mono">2026-03-25 03:00 UTC</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-[0.625rem] text-on-surface-variant tracking-wider uppercase">{t('predictive_recommendations.label_training_samples')}</span>
                <span className="text-xs text-on-surface font-mono">2,847,392</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-[0.625rem] text-on-surface-variant tracking-wider uppercase">{t('predictive_recommendations.label_status')}</span>
                <span className="flex items-center gap-1.5">
                  <span className="w-2 h-2 rounded-full bg-[#34d399]" />
                  <span className="text-xs text-[#34d399] font-semibold">{t('common.operational')}</span>
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
