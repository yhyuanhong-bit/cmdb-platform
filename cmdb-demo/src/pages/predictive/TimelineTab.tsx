import { useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useAlerts } from '../../hooks/useMonitoring'
import { useWorkOrders } from '../../hooks/useMaintenance'
import EmptyStateCard from '../../components/EmptyState'
import { Icon, SEVERITY_CONFIG, BUTTON_STYLES, EmptyState, LoadingSpinner } from './shared'

export function TimelineTab() {
  const { t } = useTranslation()
  const [filter, setFilter] = useState<'all' | 'critical' | 'scheduled'>('all')
  const { data: alertsResponse, isLoading: alertsLoading } = useAlerts()
  const { data: workOrdersResponse, isLoading: woLoading } = useWorkOrders()
  const alerts = alertsResponse?.data ?? []
  const workOrders = workOrdersResponse?.data ?? []

  const filters = [
    { key: 'all' as const, label: t('predictive_timeline.filter_all_events') },
    { key: 'critical' as const, label: t('predictive_timeline.filter_critical_only') },
    { key: 'scheduled' as const, label: t('predictive_timeline.filter_scheduled') },
  ]

  const timelineEvents = useMemo(() => {
    const fromAlerts = alerts.map((a) => {
      const severity: 'CRITICAL' | 'POTENTIAL ISSUE' | 'SCHEDULED' =
        a.severity.toLowerCase() === 'critical' ? 'CRITICAL' : 'POTENTIAL ISSUE'
      return {
        time: new Date(a.fired_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        severity,
        asset: a.ci_id,
        description: a.message,
        sortKey: new Date(a.fired_at).getTime(),
        buttonLabelKey: severity === 'CRITICAL' ? 'predictive_timeline.btn_execute_emergency' : 'predictive_timeline.btn_dispatch_inspection',
        buttonVariant: (severity === 'CRITICAL' ? 'danger' : 'warning') as 'danger' | 'warning' | 'default',
      }
    })
    const fromWO = workOrders.map((wo) => ({
      time: new Date(wo.scheduled_start).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
      severity: 'SCHEDULED' as const,
      asset: wo.title,
      description: wo.description || wo.title,
      sortKey: new Date(wo.scheduled_start).getTime(),
      buttonLabelKey: 'predictive_timeline.btn_confirmed',
      buttonVariant: 'default' as const,
    }))
    return [...fromAlerts, ...fromWO].sort((a, b) => b.sortKey - a.sortKey)
  }, [alerts, workOrders])

  const filteredEvents = timelineEvents.filter((e) => {
    if (filter === 'critical') return e.severity === 'CRITICAL'
    if (filter === 'scheduled') return e.severity === 'SCHEDULED'
    return true
  })

  const isLoading = alertsLoading || woLoading

  return (
    <div className="space-y-6">
      {/* Filters */}
      <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
        <div className="flex gap-1">
          {filters.map((f) => (
            <button
              key={f.key}
              onClick={() => setFilter(f.key)}
              className={`px-4 py-2 rounded-lg text-[0.6875rem] font-semibold tracking-wider uppercase transition-colors ${
                filter === f.key
                  ? 'bg-surface-container-high text-primary'
                  : 'bg-surface-container text-on-surface-variant hover:bg-surface-container-high'
              }`}
            >
              {f.label}
            </button>
          ))}
        </div>
        <div className="flex items-center gap-2 bg-surface-container px-4 py-2 rounded-lg">
          <Icon name="calendar_month" className="text-on-surface-variant text-[18px]" />
          <span className="text-on-surface-variant text-[0.6875rem] font-semibold tracking-wider">
            2026-03-28 &mdash; 2026-03-28
          </span>
        </div>
      </div>

      {/* TODAY marker */}
      <div className="flex items-center gap-3">
        <div className="bg-primary px-3 py-1 rounded">
          <span className="text-[0.6875rem] font-bold tracking-wider text-on-primary uppercase">{t('predictive_timeline.label_today')}</span>
        </div>
        <div className="flex-1 h-px bg-primary/30" />
        <span className="text-[0.625rem] text-on-surface-variant font-mono">2026-03-28 UTC</span>
      </div>

      {/* Vertical timeline */}
      {isLoading ? (
        <LoadingSpinner />
      ) : filteredEvents.length === 0 ? (
        <EmptyState message={t('predictive.no_data')} />
      ) : (
        <div className="relative">
          <div className="absolute left-[72px] top-0 bottom-0 w-px bg-surface-container-highest" />
          <div className="space-y-6">
            {filteredEvents.map((event, idx) => {
              const config = SEVERITY_CONFIG[event.severity]
              return (
                <div key={idx} className="flex gap-4">
                  <div className="w-16 shrink-0 pt-5">
                    <span className="text-xs font-mono text-on-surface-variant">{event.time}</span>
                  </div>
                  <div className="relative shrink-0 flex flex-col items-center pt-5">
                    <div className={`w-3.5 h-3.5 rounded-full ${config.dot} z-10`} />
                  </div>
                  <div className="flex-1 bg-surface-container rounded-xl p-5">
                    <div className="flex items-center gap-2 mb-2">
                      <span className={`${config.bg} ${config.label} text-[0.625rem] font-semibold tracking-wider uppercase px-2 py-0.5 rounded`}>
                        {event.severity}
                      </span>
                      <span className="text-sm font-semibold text-on-surface font-headline">{event.asset}</span>
                    </div>
                    <p className="text-on-surface-variant text-sm leading-relaxed mb-3">{event.description}</p>
                    <button className={`${BUTTON_STYLES[event.buttonVariant]} text-[0.6875rem] font-semibold tracking-wider uppercase px-4 py-2 rounded-lg transition-colors`}>
                      {t(event.buttonLabelKey)}
                    </button>
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}

      {/* Bottom panels */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Rack Occupancy Visualizer */}
        <div className="bg-surface-container rounded-xl p-5">
          <div className="flex items-center gap-2 mb-4">
            <Icon name="grid_view" className="text-primary text-[20px]" />
            <h2 className="font-headline text-sm font-bold tracking-wide text-on-surface uppercase">
              {t('predictive_timeline.section_rack_occupancy')}
            </h2>
          </div>
          {/* TODO(phase-3.10): wire up GET /racks/{id}/occupancy once the
              backend exposes a per-rack U-slot occupancy endpoint. Previously
              rendered a fabricated 42-slot grid with hardcoded critical
              indices. */}
          <EmptyStateCard
            icon="grid_view"
            title={t('common.empty_not_wired_title')}
            description={t('common.empty_not_wired_desc')}
            tone="neutral"
            compact
          />
        </div>

        {/* Environment Context */}
        <div className="bg-surface-container rounded-xl p-5">
          <div className="flex items-center gap-2 mb-4">
            <Icon name="thermostat" className="text-primary text-[20px]" />
            <h2 className="font-headline text-sm font-bold tracking-wide text-on-surface uppercase">
              {t('predictive_timeline.section_environment_context')}
            </h2>
          </div>
          {/* TODO(phase-3.10): wire up GET /metrics/environmental (temperature,
              humidity, grid stability) once the telemetry endpoint ships.
              Previously rendered hardcoded 23.4 C / 44% / 99.7% values. */}
          <EmptyStateCard
            icon="thermostat"
            title={t('common.empty_not_wired_title')}
            description={t('common.empty_not_wired_desc')}
            tone="neutral"
            compact
          />
        </div>
      </div>
    </div>
  )
}
