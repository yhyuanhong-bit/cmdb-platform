import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import StatusBadge from '../../components/StatusBadge'
import { useAlerts } from '../../hooks/useMonitoring'
import { Icon, ALERT_FILTER_TABS, EmptyState, LoadingSpinner } from './shared'

export function AlertsTab() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [activeFilter, setActiveFilter] = useState('ALL ASSETS')
  const { data: alertsResponse, isLoading: alertsLoading } = useAlerts({ status: 'firing' })
  const alerts = alertsResponse?.data ?? []

  const urgencyFromSeverity = (severity: string): 'HIGH' | 'MEDIUM' | 'LOW' => {
    const s = severity.toLowerCase()
    if (s === 'critical' || s === 'high') return 'HIGH'
    if (s === 'medium' || s === 'warning') return 'MEDIUM'
    return 'LOW'
  }

  return (
    <div className="space-y-6">
      {/* Filter tabs + sort */}
      <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
        <div className="flex gap-1">
          {ALERT_FILTER_TABS.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setActiveFilter(tab.key)}
              className={`px-4 py-2 rounded-lg text-[0.6875rem] font-semibold tracking-wider uppercase transition-colors ${
                activeFilter === tab.key
                  ? 'bg-surface-container-high text-primary'
                  : 'bg-surface-container text-on-surface-variant hover:bg-surface-container-high'
              }`}
            >
              {t(tab.labelKey)}
            </button>
          ))}
        </div>
        <div className="flex items-center gap-2 bg-surface-container px-4 py-2 rounded-lg">
          <Icon name="sort" className="text-on-surface-variant text-[18px]" />
          <span className="text-on-surface-variant text-[0.6875rem] font-semibold tracking-wider uppercase">
            {t('predictive_alerts.sort_by_urgency')}
          </span>
        </div>
      </div>

      {/* Alert table */}
      <div className="bg-surface-container rounded-xl overflow-hidden">
        <div className="grid grid-cols-[1fr_1.5fr_0.7fr_0.8fr_1fr] gap-4 px-6 py-3 bg-surface-container-high">
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('predictive_alerts.table_asset_identity')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('predictive_alerts.table_predicted_issue')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('predictive_alerts.table_urgency')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">{t('predictive_alerts.table_failure_window')}</span>
          <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase text-right">{t('predictive_alerts.table_actions')}</span>
        </div>

        {alertsLoading ? (
          <LoadingSpinner />
        ) : alerts.length === 0 ? (
          <EmptyState message={t('predictive.no_data')} />
        ) : (
          alerts.map((alert, idx) => (
            <div
              key={alert.id}
              className={`grid grid-cols-[1fr_1.5fr_0.7fr_0.8fr_1fr] gap-4 px-6 py-4 items-center ${
                idx % 2 === 0 ? 'bg-surface-container' : 'bg-surface-container-low'
              }`}
            >
              <div className="flex items-center gap-3">
                <Icon name="dns" className="text-primary text-[20px]" />
                <span className="text-sm font-semibold text-on-surface font-headline">{alert.ci_id}</span>
              </div>
              <span className="text-sm text-on-surface-variant">{alert.message}</span>
              <div>
                <StatusBadge status={urgencyFromSeverity(alert.severity)} />
              </div>
              <span className="text-sm text-on-surface-variant font-mono">{new Date(alert.fired_at).toLocaleDateString()}</span>
              <div className="flex justify-end">
                <button
                  onClick={(e) => { e.stopPropagation(); navigate('/maintenance/add'); }}
                  className="bg-surface-container-high hover:bg-surface-container-highest text-primary text-[0.6875rem] font-semibold tracking-wider uppercase px-4 py-2 rounded-lg transition-colors"
                >
                  {t('predictive_alerts.btn_schedule_maintenance')}
                </button>
              </div>
            </div>
          ))
        )}
      </div>

      {/* Telemetry stream */}
      <div className="flex justify-end">
        <div className="bg-surface-container rounded-xl p-5 w-full max-w-md">
          <div className="flex items-center gap-2 mb-3">
            <Icon name="stream" className="text-primary text-[18px]" />
            <span className="text-[0.6875rem] font-semibold tracking-wider text-on-surface-variant uppercase">
              {t('predictive_alerts.section_telemetry_stream')}
            </span>
            <span className="ml-auto w-2 h-2 rounded-full bg-[#34d399] animate-pulse" />
          </div>
          <div className="bg-surface-container-low rounded-lg p-3 font-mono text-[0.625rem] text-on-surface-variant space-y-1.5 max-h-32 overflow-y-auto">
            <div className="opacity-80">STREAM_IN &gt; node:SRV-PROD-01 | temp:72.4 C | fan_rpm:1820</div>
            <div className="opacity-80">STREAM_IN &gt; node:DB-CLUSTER-04 | ssd_wear:94.2% | iops:12400</div>
            <div className="opacity-80">STREAM_IN &gt; node:NET-CORE-SWITCH-B | cap_temp:68.1 C | pkt_loss:0.02%</div>
            <div className="opacity-80">STREAM_IN &gt; node:UPS-ZONE-04 | voltage:13.2V | cycles:842</div>
          </div>
        </div>
      </div>
    </div>
  )
}
