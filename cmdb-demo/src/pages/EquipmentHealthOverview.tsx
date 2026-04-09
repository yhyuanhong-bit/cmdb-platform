import { toast } from 'sonner'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useAssets } from '../hooks/useAssets'
import { useAlerts } from '../hooks/useMonitoring'

/* ------------------------------------------------------------------ */
/*  Static Data                                                        */
/* ------------------------------------------------------------------ */

const sensorReadings = [
  {
    id: 'NODE_01_TEMP_CORE',
    icon: 'thermostat',
    value: '24.2°C',
    status: 'normal',
  },
  {
    id: 'NODE_01_ELT_INPUT',
    icon: 'electric_bolt',
    value: '248.1V',
    status: 'normal',
  },
  {
    id: 'NODE_01_UPTIME',
    icon: 'schedule',
    value: '1248h 15m',
    status: 'normal',
  },
]


/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export default function EquipmentHealthOverview() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [activeTab] = useState(0)

  // Fetch server assets for health overview context
  const { data: apiData } = useAssets({ type: 'server' })
  const allAssets = (apiData as any)?.data || []
  const { data: alertsResp } = useAlerts({ severity: 'warning' })
  const allAlerts = (alertsResp as any)?.data || []

  const serverAssets = allAssets || []
  const operationalCount = serverAssets.filter((a: any) => a.status === 'operational').length
  const totalCount = serverAssets.length || 1
  const healthScore = Math.round((operationalCount / totalCount) * 100 * 10) / 10

  const metrics = [
    { labelKey: 'equipment_health_overview.metric_service_stability', value: Math.round((operationalCount / totalCount) * 100), max: 100, badgeKey: null, badgeColor: '', barColor: 'bg-[#69db7c]' },
    { labelKey: 'equipment_health_overview.metric_hardware_lifespan', value: Math.round(((totalCount - serverAssets.filter((a: any) => a.status === 'maintenance').length) / totalCount) * 100), max: 100, badgeKey: null, badgeColor: '', barColor: 'bg-primary' },
    { labelKey: 'equipment_health_overview.metric_storage_stability', value: allAlerts.length > 0 ? Math.max(50, 100 - allAlerts.length * 5) : 100, max: 100, badgeKey: null, badgeColor: '', barColor: 'bg-[#ffa94d]' },
    { labelKey: 'equipment_health_overview.metric_network_connectivity', value: 95, max: 100, badgeKey: 'equipment_health_overview.badge_optimal', badgeColor: 'bg-[#69db7c]/15 text-[#69db7c]', barColor: 'bg-[#69db7c]' },
  ]

  const latestWarning = allAlerts[0]
  const warningMessage = latestWarning
    ? { titleKey: 'equipment_health_overview.warning_title', assetRef: latestWarning.message || 'Active Warning', body: `Asset: ${latestWarning.ci_id?.slice(0, 8) || 'Unknown'}` }
    : { titleKey: 'equipment_health_overview.warning_title', assetRef: 'No active warnings', body: 'All systems operating normally' }

  const criticalCount = allAlerts.filter((a: any) => a.severity === 'critical').length
  const riskAssessment = {
    titleKey: 'equipment_health_overview.risk_title',
    body: criticalCount > 0 ? `${criticalCount} critical alert(s) detected across monitored assets.` : 'No critical alerts. Systems operating within normal parameters.',
    riskLevel: criticalCount > 3 ? 'HIGH' : criticalCount > 0 ? 'ELEVATED' : 'LOW',
    remainingLife: Math.max(5, 100 - criticalCount * 15),
  }

  return (
    <div className="min-h-screen bg-surface p-6 font-body text-on-surface">
      {/* Header */}
      <div className="mb-8 flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="font-headline text-2xl font-bold text-on-surface">
            {t('equipment_health_overview.title')}
          </h1>
          <div className="mt-3 flex flex-wrap items-center gap-3">
            <span className="flex items-center gap-1.5 rounded bg-[#69db7c]/15 px-3 py-1 text-[10px] font-bold tracking-widest text-[#69db7c]">
              <span className="inline-block h-1.5 w-1.5 rounded-full bg-[#69db7c]" />
              {t('equipment_health_overview.status_normal')}
            </span>
            <span className="text-xs text-on-surface-variant">
              {t('equipment_health_overview.last_sync')}：{new Date().toLocaleString()}
            </span>
          </div>
        </div>
        <div className="flex gap-2">
          <button onClick={() => toast.info('Coming Soon')} className="flex items-center gap-2 rounded bg-surface-container px-4 py-2.5 text-[10px] font-bold tracking-widest text-on-surface-variant transition-colors hover:bg-surface-container-high cursor-pointer">
            <span className="material-symbols-outlined text-base">
              download
            </span>
            {t('equipment_health_overview.btn_download_report')}
          </button>
          <button onClick={() => navigate('/monitoring/sensors')} className="flex items-center gap-2 rounded bg-surface-container px-4 py-2.5 text-[10px] font-bold tracking-widest text-on-surface-variant transition-colors hover:bg-surface-container-high cursor-pointer">
            <span className="material-symbols-outlined text-base">
              notifications
            </span>
            {t('equipment_health_overview.btn_alert_settings')}
          </button>
        </div>
      </div>

      {/* Main grid */}
      <div className="grid gap-6 lg:grid-cols-[1fr_380px]">
        {/* ---- Left column ---- */}
        <div className="space-y-6">
          {/* Health Score */}
          <div className="rounded-lg bg-surface-container p-6">
            <div className="flex items-center gap-8">
              <div className="relative flex h-40 w-40 flex-shrink-0 items-center justify-center">
                {/* Score ring background */}
                <svg
                  className="absolute inset-0"
                  viewBox="0 0 160 160"
                  fill="none"
                >
                  <circle
                    cx="80"
                    cy="80"
                    r="68"
                    stroke="#202b32"
                    strokeWidth="10"
                  />
                  <circle
                    cx="80"
                    cy="80"
                    r="68"
                    stroke="#9ecaff"
                    strokeWidth="10"
                    strokeDasharray={`${(healthScore / 100) * 427.3} 427.3`}
                    strokeLinecap="round"
                    transform="rotate(-90 80 80)"
                  />
                </svg>
                <div className="text-center">
                  <div className="font-mono text-4xl font-bold text-on-surface">
                    {healthScore}
                    <span className="text-lg text-on-surface-variant">%</span>
                  </div>
                </div>
              </div>
              <div>
                <div className="text-[10px] tracking-widest text-on-surface-variant">
                  {t('equipment_health_overview.label_health_score')}
                </div>
                <p className="mt-2 text-sm leading-relaxed text-on-surface-variant">
                  {t('equipment_health_overview.health_score_description')}
                </p>
              </div>
            </div>
          </div>

          {/* Metric cards */}
          <div className="grid grid-cols-2 gap-4">
            {metrics.map((m) => (
              <div
                key={m.labelKey}
                className="rounded-lg bg-surface-container p-5"
              >
                <div className="mb-3 flex items-center justify-between">
                  <span className="text-xs font-bold tracking-wider text-on-surface">
                    {t(m.labelKey)}
                  </span>
                  {m.badgeKey && (
                    <span
                      className={`rounded px-2 py-0.5 text-[10px] font-bold tracking-widest ${m.badgeColor}`}
                    >
                      {t(m.badgeKey)}
                    </span>
                  )}
                </div>
                <div className="mb-2 font-mono text-2xl font-bold text-on-surface">
                  {m.value}
                  <span className="text-sm text-on-surface-variant">
                    /{m.max}
                  </span>
                </div>
                <div className="h-1.5 overflow-hidden rounded-full bg-surface-container-low">
                  <div
                    className={`h-full rounded-full ${m.barColor}`}
                    style={{ width: `${(m.value / m.max) * 100}%` }}
                  />
                </div>
              </div>
            ))}
          </div>

          {/* Sensor readings */}
          <div className="rounded-lg bg-surface-container p-6">
            <h2 className="mb-4 text-[10px] font-bold tracking-widest text-on-surface-variant">
              {t('equipment_health_overview.section_sensor_readings')}
            </h2>
            <div className="grid grid-cols-3 gap-4">
              {sensorReadings.map((s) => (
                <div
                  key={s.id}
                  className="rounded bg-surface-container-low p-4"
                >
                  <div className="mb-2 flex items-center gap-2">
                    <span className="material-symbols-outlined text-base text-primary">
                      {s.icon}
                    </span>
                    <span className="text-[10px] tracking-widest text-on-surface-variant">
                      {s.id}
                    </span>
                  </div>
                  <div className="font-mono text-xl font-bold text-on-surface">
                    {s.value}
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Warning section */}
          <div className="rounded-lg bg-[#ffa94d]/5 p-6">
            <div className="mb-3 flex items-center gap-2">
              <span className="material-symbols-outlined text-lg text-[#ffa94d]">
                warning
              </span>
              <h2 className="text-xs font-bold tracking-wider text-[#ffa94d]">
                {t(warningMessage.titleKey)}
              </h2>
            </div>
            <p className="mb-2 text-xs font-semibold text-on-surface">
              {warningMessage.assetRef}
            </p>
            <p className="text-sm leading-relaxed text-on-surface-variant">
              {warningMessage.body}
            </p>
          </div>
        </div>

        {/* ---- Right panel ---- */}
        <div className="space-y-6">
          {/* Risk Assessment */}
          <div className="rounded-lg bg-surface-container p-6">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-[10px] font-bold tracking-widest text-on-surface-variant">
                {t(riskAssessment.titleKey)}
              </h2>
              <span className={`rounded px-2 py-0.5 text-[10px] font-bold tracking-widest ${riskAssessment.riskLevel === 'HIGH' ? 'bg-error/15 text-error' : riskAssessment.riskLevel === 'ELEVATED' ? 'bg-[#ffa94d]/15 text-[#ffa94d]' : 'bg-[#69db7c]/15 text-[#69db7c]'}`}>
                {riskAssessment.riskLevel}
              </span>
            </div>
            <p className="mb-5 text-sm leading-relaxed text-on-surface-variant">
              {riskAssessment.body}
            </p>

            {/* Remaining life indicator */}
            <div className="rounded bg-surface-container-low p-4">
              <div className="mb-2 flex items-center justify-between">
                <span className="text-[10px] tracking-widest text-on-surface-variant">
                  {t('equipment_health_overview.label_remaining_life')}
                </span>
                <span className="font-mono text-sm font-bold text-error">
                  {riskAssessment.remainingLife}%
                </span>
              </div>
              <div className="h-2 overflow-hidden rounded-full bg-surface-container">
                <div
                  className="h-full rounded-full bg-error"
                  style={{ width: `${riskAssessment.remainingLife}%` }}
                />
              </div>
            </div>

            <button
              onClick={() => navigate('/maintenance/add')}
              className="mt-5 w-full rounded bg-surface-container-high py-3 text-[10px] font-bold tracking-widest text-primary transition-colors hover:bg-surface-container-low cursor-pointer"
            >
              {t('equipment_health_overview.btn_create_work_order')}
            </button>
          </div>

          {/* Quick actions */}
          <div className="rounded-lg bg-surface-container p-6">
            <h2 className="mb-4 text-[10px] font-bold tracking-widest text-on-surface-variant">
              {t('equipment_health_overview.section_quick_actions')}
            </h2>
            <div className="space-y-2">
              {[
                {
                  icon: 'history',
                  labelKey: 'equipment_health_overview.action_view_maint_history',
                  action: '/maintenance',
                },
                {
                  icon: 'assessment',
                  labelKey: 'equipment_health_overview.action_performance_trends',
                  action: '/predictive',
                },
                {
                  icon: 'inventory_2',
                  labelKey: 'equipment_health_overview.action_spare_parts_query',
                  action: '/inventory',
                },
                {
                  icon: 'description',
                  labelKey: 'equipment_health_overview.action_export_full_report',
                  action: '__export__',
                },
              ].map((item) => (
                <button
                  key={item.labelKey}
                  onClick={() => item.action === '__export__' ? toast.info('Coming Soon') : item.action && navigate(item.action)}
                  className="flex w-full items-center gap-3 rounded bg-surface-container-low px-4 py-3 text-left text-xs font-bold tracking-wider text-on-surface transition-colors hover:bg-surface-container-high cursor-pointer"
                >
                  <span className="material-symbols-outlined text-base text-primary">
                    {item.icon}
                  </span>
                  {t(item.labelKey)}
                  <span className="material-symbols-outlined ml-auto text-sm text-on-surface-variant">
                    chevron_right
                  </span>
                </button>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
