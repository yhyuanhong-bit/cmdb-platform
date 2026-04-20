import { toast } from 'sonner'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import StatusBadge from '../../components/StatusBadge'
import { useAlerts } from '../../hooks/useMonitoring'
import {
  Icon,
  CHART_WIDTH,
  CHART_HEIGHT,
  CHART_PADDING,
  INNER_H,
  INNER_W,
  EmptyState,
  LoadingSpinner,
} from './shared'

export function ForecastTab() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { data: alertsResponse, isLoading: alertsLoading } = useAlerts({ status: 'firing' })
  const alerts = alertsResponse?.data ?? []

  const forecastTasks = useMemo(() => alerts.slice(0, 5).map((alert) => {
    const urgency = alert.severity.toLowerCase() === 'critical' ? 'CRITICAL' as const
      : alert.severity.toLowerCase() === 'high' ? 'HIGH' as const : 'MEDIUM' as const
    const probability = alert.severity.toLowerCase() === 'critical' ? 91
      : alert.severity.toLowerCase() === 'high' ? 64 : 32
    return {
      asset: alert.ci_id,
      failure: alert.message,
      probability,
      urgency,
    }
  }), [alerts])

  return (
    <div className="space-y-6">
      {/* Header row: Stats + Immediate Attention */}
      <div className="flex flex-col lg:flex-row gap-6">
        <div className="flex-1 grid grid-cols-2 gap-4">
          <div className="bg-surface-container-low rounded-lg p-5">
            <div className="flex items-center justify-between mb-2">
              <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('failure_forecast.stat_critical_threats')}</span>
              <Icon name="warning" className="text-error text-[18px]" />
            </div>
            <div className="font-headline font-bold text-3xl text-error">03</div>
            <span className="text-xs text-error">{t('failure_forecast.stat_active_threat_vectors')}</span>
          </div>
          <div className="bg-surface-container-low rounded-lg p-5">
            <div className="flex items-center justify-between mb-2">
              <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('failure_forecast.stat_risk_index')}</span>
              <Icon name="speed" className="text-tertiary text-[18px]" />
            </div>
            <div className="font-headline font-bold text-3xl text-tertiary">12.4%</div>
            <span className="text-xs text-tertiary">{t('failure_forecast.stat_composite_fleet_risk')}</span>
          </div>
        </div>

        {/* Immediate Attention warning box */}
        <div className="lg:w-96 bg-error-container rounded-xl p-5">
          <div className="flex items-center gap-2 mb-3">
            <Icon name="crisis_alert" className="text-on-error-container text-[20px]" />
            <span className="text-[0.6875rem] font-bold tracking-wider text-on-error-container uppercase">{t('failure_forecast.immediate_attention')}</span>
          </div>
          <p className="text-on-error-container text-sm leading-relaxed mb-2">
            {t('predictive_hub.forecast_attention_desc')}
          </p>
          <div className="flex items-center gap-3 mb-4">
            <div className="flex items-center gap-1.5">
              <Icon name="timer" className="text-on-error-container text-[16px]" />
              <span className="text-xs text-on-error-container">{t('predictive_hub.forecast_time_to_failure')}</span>
            </div>
            <div className="flex items-center gap-1.5">
              <Icon name="database" className="text-on-error-container text-[16px]" />
              <span className="text-xs text-on-error-container">{t('predictive_hub.forecast_data_at_risk')}</span>
            </div>
          </div>
          <button onClick={() => toast.info('Coming Soon')} className="bg-error text-on-error text-[0.6875rem] font-bold tracking-wider uppercase px-5 py-2.5 rounded-lg hover:bg-error/80 transition-colors w-full">
            {t('failure_forecast.btn_isolate_node')}
          </button>
        </div>
      </div>

      {/* SVG line chart */}
      <div className="bg-surface-container rounded-xl p-5">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Icon name="show_chart" className="text-primary text-[20px]" />
            <h2 className="font-headline text-sm font-bold tracking-wide text-on-surface uppercase">
              {t('failure_forecast.section_failure_rate_chart')}
            </h2>
          </div>
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-1.5">
              <span className="w-5 h-0.5 bg-error rounded" />
              <span className="text-[0.625rem] text-on-surface-variant tracking-wider uppercase">{t('failure_forecast.legend_server_nodes')}</span>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="w-5 h-0.5 bg-primary rounded" />
              <span className="text-[0.625rem] text-on-surface-variant tracking-wider uppercase">{t('failure_forecast.legend_ups_units')}</span>
            </div>
          </div>
        </div>

        {alerts.length === 0 ? (
          <EmptyState message={t('predictive.no_data', { defaultValue: 'Insufficient data for forecast' })} />
        ) : (
          <div className="w-full overflow-x-auto">
            <svg viewBox={`0 0 ${CHART_WIDTH} ${CHART_HEIGHT}`} className="w-full min-w-[500px]" preserveAspectRatio="xMidYMid meet">
              {[0, 25, 50, 75, 100].map((val) => {
                const y = CHART_PADDING.top + INNER_H - (val / 100) * INNER_H
                return (
                  <g key={val}>
                    <line x1={CHART_PADDING.left} y1={y} x2={CHART_PADDING.left + INNER_W} y2={y} stroke="#2b363d" strokeWidth="1" />
                    <text x={CHART_PADDING.left - 8} y={y + 4} fill="#c4c6cc" fontSize="9" textAnchor="end" fontFamily="Inter">{val}%</text>
                  </g>
                )
              })}
              {['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC'].map((m, i) => {
                const x = CHART_PADDING.left + (i / 11) * INNER_W
                return <text key={m} x={x} y={CHART_HEIGHT - 5} fill="#c4c6cc" fontSize="8" textAnchor="middle" fontFamily="Inter">{m}</text>
              })}
              <text x={CHART_WIDTH / 2} y={CHART_HEIGHT / 2} fill="#c4c6cc" fontSize="12" textAnchor="middle" fontFamily="Inter">
                {t('predictive.no_data', { defaultValue: 'Collecting metrics data...' })}
              </text>
              <defs>
                <linearGradient id="serverGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#ffb4ab" />
                  <stop offset="100%" stopColor="#ffb4ab" stopOpacity="0" />
                </linearGradient>
                <linearGradient id="upsGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor="#9ecaff" />
                  <stop offset="100%" stopColor="#9ecaff" stopOpacity="0" />
                </linearGradient>
              </defs>
            </svg>
          </div>
        )}
      </div>

      {/* Proactive Maintenance Tasks table */}
      <div className="bg-surface-container rounded-xl overflow-hidden">
        <div className="px-6 py-4 bg-surface-container-high">
          <div className="flex items-center gap-2">
            <Icon name="build" className="text-primary text-[20px]" />
            <h2 className="font-headline text-sm font-bold tracking-wide text-on-surface uppercase">
              {t('failure_forecast.section_proactive_tasks')}
            </h2>
          </div>
        </div>

        <div className="grid grid-cols-[1.2fr_1.5fr_0.8fr_0.7fr_0.7fr] gap-4 px-6 py-3 bg-surface-container-high">
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('failure_forecast.table_asset')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('failure_forecast.table_failure_mode')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('failure_forecast.table_probability')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('failure_forecast.table_urgency')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase text-right">{t('failure_forecast.table_action')}</span>
        </div>

        {alertsLoading ? (
          <LoadingSpinner />
        ) : forecastTasks.length === 0 ? (
          <EmptyState message={t('predictive.no_data')} />
        ) : (
          forecastTasks.map((task, idx) => {
            const probColor = task.probability >= 80 ? 'bg-error' : task.probability >= 50 ? 'bg-tertiary' : 'bg-[#fbbf24]'
            return (
              <div
                key={task.asset + idx}
                className={`grid grid-cols-[1.2fr_1.5fr_0.8fr_0.7fr_0.7fr] gap-4 px-6 py-4 items-center ${
                  idx % 2 === 0 ? 'bg-surface-container' : 'bg-surface-container-low'
                }`}
              >
                <div className="flex items-center gap-3">
                  <Icon name="dns" className="text-primary text-[20px]" />
                  <span className="text-sm font-semibold text-on-surface font-headline">{task.asset}</span>
                </div>
                <span className="text-sm text-on-surface-variant">{task.failure}</span>
                <div className="flex items-center gap-2">
                  <div className="flex-1 h-2 bg-surface-container-low rounded-full overflow-hidden">
                    <div className={`h-full rounded-full ${probColor}`} style={{ width: `${task.probability}%` }} />
                  </div>
                  <span className="text-xs font-mono text-on-surface-variant w-10 text-right">{task.probability}%</span>
                </div>
                <div>
                  <StatusBadge status={task.urgency} />
                </div>
                <div className="flex justify-end">
                  <button
                    onClick={(e) => { e.stopPropagation(); navigate('/maintenance/add'); }}
                    className="bg-on-primary-container/20 text-on-primary-container text-[0.6875rem] font-semibold tracking-wider uppercase px-3 py-2 rounded-lg hover:bg-on-primary-container/30 transition-colors whitespace-nowrap"
                  >
                    {t('failure_forecast.btn_initiate_task')}
                  </button>
                </div>
              </div>
            )
          })
        )}
      </div>

      {/* System health footer */}
      <div className="flex justify-end">
        <div className="bg-surface-container rounded-xl px-5 py-3 flex items-center gap-3">
          <span className="w-2.5 h-2.5 rounded-full bg-[#34d399] animate-pulse" />
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('failure_forecast.system_health_label')}</span>
          <span className="text-sm font-bold font-headline text-[#34d399]">{t('failure_forecast.system_health_optimal')}</span>
        </div>
      </div>
    </div>
  )
}
