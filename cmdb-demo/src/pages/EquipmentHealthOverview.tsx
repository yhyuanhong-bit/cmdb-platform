import { toast } from 'sonner'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useAssets, useLifecycleStats, useCapacityPlanning } from '../hooks/useAssets'
import type { AlertEvent } from '../lib/api/monitoring'
import type { CapacityForecast } from '../lib/api/assets'
import { useAlerts, useFleetMetrics } from '../hooks/useMonitoring'

interface FleetMetricItem {
  name: string
  label?: string
  avg_value?: number | null
  p95_value?: number | null
  unit?: string
  data_points?: number
  sparkline?: number[]
}


/* ------------------------------------------------------------------ */
/*  Mini Sparkline                                                      */
/* ------------------------------------------------------------------ */

function MiniSparkline({ data, color = '#9ecaff' }: { data: number[]; color?: string }) {
  if (data.length < 2) return null
  const max = Math.max(...data)
  const min = Math.min(...data)
  const range = max - min || 1
  const w = 80
  const h = 24
  const points = data
    .map((v, i) => `${(i / (data.length - 1)) * w},${h - ((v - min) / range) * h}`)
    .join(' ')
  return (
    <svg width={w} height={h} className="ml-2 inline-block opacity-60">
      <polyline points={points} fill="none" stroke={color} strokeWidth="1.5" />
    </svg>
  )
}

/* ------------------------------------------------------------------ */
/*  Component                                                           */
/* ------------------------------------------------------------------ */

// The API Asset schema is minimal; the backend also returns lifecycle fields at runtime
type AssetWithLifecycle = import('../lib/api/assets').Asset & {
  warranty_end?: string | null
  eol_date?: string | null
  vendor?: string
  model?: string
}


export default function EquipmentHealthOverview() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  // Fetch server assets for health overview context
  const { data: apiData } = useAssets({ type: 'server' })
  const allAssets = apiData?.data ?? []
  const { data: alertsResp } = useAlerts({ severity: 'warning' })
  const allAlerts = alertsResp?.data ?? []

  // Lifecycle stats for warranty-aware scoring
  const { data: lifecycleResp } = useLifecycleStats()
  const lifecycleStats = lifecycleResp?.data ?? {
    by_status: {},
    total_purchase_cost: 0,
    warranty_active_count: 0,
    warranty_expired_count: 0,
    approaching_eol_count: 0,
  }

  // Capacity planning forecasts — useCapacityPlanning hook already unwraps via select()
  const { data: capacityData } = useCapacityPlanning()
  const capacityForecasts = capacityData ?? []

  // Fleet metrics from monitoring system
  const { data: fleetMetricsResp } = useFleetMetrics()
  const fleetMetrics: FleetMetricItem[] = (fleetMetricsResp as unknown as { data?: FleetMetricItem[] })?.data ?? []

  const getMetric = (name: string): FleetMetricItem | undefined => fleetMetrics.find((m) => m.name === name)

  const serverAssets = allAssets || []
  const operationalCount = serverAssets.filter((a: AssetWithLifecycle) => a.status === 'operational').length
  const totalCount = serverAssets.length || 1

  // Enhancement 2: Warranty-aware health score
  const warrantyExpiredCount = lifecycleStats.warranty_expired_count ?? 0
  const effectiveHealthy = operationalCount - Math.min(warrantyExpiredCount, operationalCount) * 0.2
  const healthScore = Math.round((effectiveHealthy / totalCount) * 100 * 10) / 10

  // Fleet metric lookups
  const diskMetric = getMetric('disk_usage')
  const networkMetric = getMetric('network_in_bytes')
  const cpuMetric = getMetric('cpu_usage')
  const memMetric = getMetric('memory_usage')
  const temperatureMetric = getMetric('temperature')
  const powerMetric = getMetric('power_kw')

  // Derived metric values with graceful fallback
  const storageStability =
    diskMetric?.avg_value != null
      ? Math.round(100 - diskMetric.avg_value) // low disk usage = high stability
      : Math.max(50, 100 - allAlerts.length * 5)
  const storageNoData = diskMetric?.avg_value == null

  const networkConnectivity =
    (networkMetric?.data_points ?? 0) > 0
      ? Math.min(100, Math.round(100 - (networkMetric?.p95_value ?? 0) / 1000))
      : null
  const networkNoData = networkConnectivity === null

  const metrics = [
    {
      labelKey: 'equipment_health_overview.metric_service_stability',
      value: Math.round((operationalCount / totalCount) * 100),
      max: 100,
      badgeKey: null,
      badgeColor: '',
      barColor: 'bg-[#69db7c]',
      noData: false,
      sparkline: cpuMetric?.sparkline ?? [],
    },
    {
      // Enhancement 1: Real warranty data for hardware lifespan
      labelKey: 'equipment_health_overview.metric_hardware_lifespan',
      value:
        totalCount > 0
          ? Math.round(((totalCount - (lifecycleStats.warranty_expired_count ?? 0)) / totalCount) * 100)
          : 100,
      max: 100,
      badgeKey: null,
      badgeColor: '',
      barColor: 'bg-primary',
      noData: false,
      sparkline: memMetric?.sparkline ?? [],
    },
    {
      labelKey: 'equipment_health_overview.metric_storage_stability',
      value: storageStability,
      max: 100,
      badgeKey: null,
      badgeColor: '',
      barColor: 'bg-[#ffa94d]',
      noData: storageNoData,
      sparkline: diskMetric?.sparkline ?? [],
    },
    {
      labelKey: 'equipment_health_overview.metric_network_connectivity',
      value: networkConnectivity ?? 0,
      max: 100,
      badgeKey: networkConnectivity !== null ? 'equipment_health_overview.badge_optimal' : null,
      badgeColor: 'bg-[#69db7c]/15 text-[#69db7c]',
      barColor: 'bg-[#69db7c]',
      noData: networkNoData,
      sparkline: networkMetric?.sparkline ?? [],
    },
  ]

  // Dynamic sensor readings
  const sensorReadings = [
    {
      id: 'TEMP',
      icon: 'thermostat',
      label: t('equipment_health_overview.sensor_temperature', 'Avg Temperature'),
      value:
        temperatureMetric?.avg_value != null
          ? `${temperatureMetric.avg_value.toFixed(1)}°C`
          : null,
      status:
        temperatureMetric?.avg_value != null
          ? temperatureMetric.avg_value > 35
            ? 'warning'
            : 'normal'
          : 'no_data',
    },
    {
      id: 'POWER',
      icon: 'electric_bolt',
      label: t('equipment_health_overview.sensor_power', 'Avg Power Draw'),
      value:
        powerMetric?.avg_value != null ? `${powerMetric.avg_value.toFixed(1)} kW` : null,
      status: powerMetric?.avg_value != null ? 'normal' : 'no_data',
    },
    {
      id: 'UPTIME',
      icon: 'schedule',
      label: t('equipment_health_overview.sensor_uptime', 'Fleet Uptime'),
      value: `${operationalCount}/${totalCount}`,
      status: operationalCount === totalCount ? 'normal' : 'warning',
    },
  ]

  const latestWarning = allAlerts[0]
  const warningMessage = latestWarning
    ? {
        titleKey: 'equipment_health_overview.warning_title',
        assetRef: latestWarning.message || 'Active Warning',
        body: `Asset: ${latestWarning.ci_id?.slice(0, 8) || 'Unknown'}`,
      }
    : {
        titleKey: 'equipment_health_overview.warning_title',
        assetRef: 'No active warnings',
        body: 'All systems operating normally',
      }

  const criticalCount = allAlerts.filter((a: AlertEvent) => a.severity === 'critical').length
  const riskAssessment = {
    titleKey: 'equipment_health_overview.risk_title',
    body:
      criticalCount > 0
        ? `${criticalCount} critical alert(s) detected across monitored assets.`
        : 'No critical alerts. Systems operating within normal parameters.',
    riskLevel: criticalCount > 3 ? 'HIGH' : criticalCount > 0 ? 'ELEVATED' : 'LOW',
    remainingLife: Math.max(5, 100 - criticalCount * 15),
  }

  // Enhancement 4: Download report handler
  const handleDownloadReport = () => {
    const report = {
      generated_at: new Date().toISOString(),
      health_score: healthScore,
      total_assets: totalCount,
      operational: operationalCount,
      warranty_expired: lifecycleStats.warranty_expired_count ?? 0,
      approaching_eol: lifecycleStats.approaching_eol_count ?? 0,
      risk_level: riskAssessment.riskLevel,
      critical_alerts: criticalCount,
      warning_alerts: allAlerts.length,
      fleet_metrics: fleetMetrics.map((m: FleetMetricItem) => ({
        name: m.name,
        label: m.label,
        avg_value: m.avg_value,
        unit: m.unit,
        data_points: m.data_points,
      })),
      assets: (serverAssets as AssetWithLifecycle[]).map((a) => ({
        asset_tag: a.asset_tag,
        name: a.name,
        type: a.type,
        status: a.status,
        vendor: a.vendor,
        model: a.model,
        warranty_end: a.warranty_end,
        eol_date: a.eol_date,
      })),
    }
    const blob = new Blob([JSON.stringify(report, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `health-report-${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(url)
    toast.success(t('equipment_health_overview.report_downloaded', 'Report downloaded'))
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
          <button
            onClick={handleDownloadReport}
            className="flex items-center gap-2 rounded bg-surface-container px-4 py-2.5 text-[10px] font-bold tracking-widest text-on-surface-variant transition-colors hover:bg-surface-container-high cursor-pointer"
          >
            <span className="material-symbols-outlined text-base">download</span>
            {t('equipment_health_overview.btn_download_report')}
          </button>
          <button
            onClick={() => navigate('/monitoring/sensors')}
            className="flex items-center gap-2 rounded bg-surface-container px-4 py-2.5 text-[10px] font-bold tracking-widest text-on-surface-variant transition-colors hover:bg-surface-container-high cursor-pointer"
          >
            <span className="material-symbols-outlined text-base">notifications</span>
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
                <svg className="absolute inset-0" viewBox="0 0 160 160" fill="none">
                  <circle cx="80" cy="80" r="68" stroke="#202b32" strokeWidth="10" />
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
              <div key={m.labelKey} className="rounded-lg bg-surface-container p-5">
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
                {m.noData ? (
                  <p className="mb-2 text-xs italic text-on-surface-variant">
                    {t('equipment_health_overview.awaiting_data', 'Awaiting monitoring data')}
                  </p>
                ) : (
                  <div className="mb-2 flex items-center font-mono text-2xl font-bold text-on-surface">
                    {m.value}
                    <span className="text-sm text-on-surface-variant">/{m.max}</span>
                    {m.sparkline.length > 1 && (
                      <MiniSparkline data={m.sparkline} />
                    )}
                  </div>
                )}
                <div className="h-1.5 overflow-hidden rounded-full bg-surface-container-low">
                  <div
                    className={`h-full rounded-full ${m.barColor}`}
                    style={{ width: `${m.noData ? 0 : (m.value / m.max) * 100}%` }}
                  />
                </div>
                {m.sparkline.length > 1 && (
                  <p className="mt-1 text-[10px] text-on-surface-variant">
                    {t('equipment_health_overview.trend_7d', '7-day trend')}
                  </p>
                )}
              </div>
            ))}
          </div>

          {/* Sensor readings */}
          <div className="rounded-lg bg-surface-container p-6">
            <h2 className="mb-4 text-[10px] font-bold tracking-widest text-on-surface-variant">
              {t('equipment_health_overview.section_sensor_readings')}
            </h2>
            <div className="grid grid-cols-3 gap-4">
              {sensorReadings.map((sensor) => (
                <div
                  key={sensor.id}
                  className="flex items-center gap-3 rounded-lg bg-surface-container-low p-3"
                >
                  <span className="material-symbols-outlined text-xl text-primary">
                    {sensor.icon}
                  </span>
                  <div className="flex-1">
                    <p className="text-xs text-on-surface-variant">{sensor.label}</p>
                    {sensor.value != null ? (
                      <p
                        className={`text-sm font-bold ${
                          sensor.status === 'warning' ? 'text-tertiary' : 'text-on-surface'
                        }`}
                      >
                        {sensor.value}
                      </p>
                    ) : (
                      <p className="text-xs italic text-on-surface-variant">
                        {t(
                          'equipment_health_overview.connect_monitoring',
                          'Connect monitoring to enable'
                        )}
                      </p>
                    )}
                  </div>
                  {sensor.status === 'normal' && (
                    <span className="h-2 w-2 rounded-full bg-[#69db7c]" />
                  )}
                  {sensor.status === 'warning' && (
                    <span className="h-2 w-2 rounded-full bg-tertiary" />
                  )}
                  {sensor.status === 'no_data' && (
                    <span className="h-2 w-2 rounded-full bg-on-surface-variant/30" />
                  )}
                </div>
              ))}
            </div>
          </div>

          {/* Warning section */}
          <div className="rounded-lg bg-[#ffa94d]/5 p-6">
            <div className="mb-3 flex items-center gap-2">
              <span className="material-symbols-outlined text-lg text-[#ffa94d]">warning</span>
              <h2 className="text-xs font-bold tracking-wider text-[#ffa94d]">
                {t(warningMessage.titleKey)}
              </h2>
            </div>
            <p className="mb-2 text-xs font-semibold text-on-surface">{warningMessage.assetRef}</p>
            <p className="text-sm leading-relaxed text-on-surface-variant">{warningMessage.body}</p>
          </div>

          {/* Device Health Table */}
          <div className="rounded-lg bg-surface-container p-6">
            <h3 className="mb-4 text-xs font-bold uppercase tracking-widest text-on-surface-variant">
              {t('equipment_health_overview.device_table_title', 'Device Health Breakdown')}
            </h3>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-surface-container-highest text-left text-xs text-on-surface-variant">
                    <th className="px-3 py-2">{t('equipment_health_overview.col_asset', 'Asset')}</th>
                    <th className="px-3 py-2">{t('equipment_health_overview.col_type', 'Type')}</th>
                    <th className="px-3 py-2">{t('equipment_health_overview.col_status', 'Status')}</th>
                    <th className="px-3 py-2">{t('equipment_health_overview.col_warranty', 'Warranty')}</th>
                    <th className="px-3 py-2">{t('equipment_health_overview.col_risk', 'Risk')}</th>
                  </tr>
                </thead>
                <tbody>
                  {(serverAssets as AssetWithLifecycle[]).slice(0, 20).map((asset) => {
                    const isExpiredWarranty =
                      asset.warranty_end && new Date(asset.warranty_end) < new Date()
                    const isOperational = asset.status === 'operational'
                    const hasAlerts = allAlerts.some(
                      (a: AlertEvent) => a.ci_id === asset.id
                    )
                    const risk = !isOperational
                      ? 'HIGH'
                      : hasAlerts
                        ? 'ELEVATED'
                        : isExpiredWarranty
                          ? 'MEDIUM'
                          : 'LOW'
                    const riskColor =
                      risk === 'HIGH'
                        ? 'text-error'
                        : risk === 'ELEVATED'
                          ? 'text-tertiary'
                          : risk === 'MEDIUM'
                            ? 'text-[#fbbf24]'
                            : 'text-[#69db7c]'
                    return (
                      <tr
                        key={asset.id}
                        className="cursor-pointer border-b border-surface-container-highest hover:bg-surface-container-high"
                        onClick={() => navigate(`/assets/${asset.id}`)}
                      >
                        <td className="px-3 py-2.5">
                          <div className="font-medium text-on-surface">{asset.name}</div>
                          <div className="text-xs text-on-surface-variant">{asset.asset_tag}</div>
                        </td>
                        <td className="px-3 py-2.5 text-on-surface-variant">{asset.type}</td>
                        <td className="px-3 py-2.5">
                          <span
                            className={`text-xs font-semibold ${
                              isOperational ? 'text-[#69db7c]' : 'text-error'
                            }`}
                          >
                            {asset.status?.toUpperCase()}
                          </span>
                        </td>
                        <td className="px-3 py-2.5">
                          {asset.warranty_end ? (
                            <span
                              className={`text-xs ${isExpiredWarranty ? 'text-error' : 'text-[#69db7c]'}`}
                            >
                              {isExpiredWarranty
                                ? t('equipment_health_overview.warranty_expired', 'Expired')
                                : `${t('equipment_health_overview.warranty_active', 'Until')} ${asset.warranty_end}`}
                            </span>
                          ) : (
                            <span className="text-xs text-on-surface-variant">—</span>
                          )}
                        </td>
                        <td className="px-3 py-2.5">
                          <span className={`text-xs font-bold ${riskColor}`}>{risk}</span>
                        </td>
                      </tr>
                    )
                  })}
                  {serverAssets.length === 0 && (
                    <tr>
                      <td colSpan={5} className="px-3 py-6 text-center text-xs text-on-surface-variant">
                        No asset data available.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
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
              <span
                className={`rounded px-2 py-0.5 text-[10px] font-bold tracking-widest ${
                  riskAssessment.riskLevel === 'HIGH'
                    ? 'bg-error/15 text-error'
                    : riskAssessment.riskLevel === 'ELEVATED'
                      ? 'bg-[#ffa94d]/15 text-[#ffa94d]'
                      : 'bg-[#69db7c]/15 text-[#69db7c]'
                }`}
              >
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
              className="mt-5 w-full cursor-pointer rounded bg-surface-container-high py-3 text-[10px] font-bold tracking-widest text-primary transition-colors hover:bg-surface-container-low"
            >
              {t('equipment_health_overview.btn_create_work_order')}
            </button>
          </div>

          {/* Enhancement 5: Capacity Summary */}
          <div className="rounded-lg bg-surface-container p-5">
            <h3 className="mb-3 text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">
              {t('equipment_health_overview.capacity_title', 'Capacity Overview')}
            </h3>
            <div className="space-y-3">
              {Array.isArray(capacityForecasts) &&
                capacityForecasts
                  .filter((f: CapacityForecast) => f.resource_type === 'infrastructure')
                  .map((f: CapacityForecast, i: number) => (
                    <div key={i}>
                      <div className="mb-1 flex justify-between text-xs">
                        <span className="text-on-surface-variant">{f.resource_name}</span>
                        <span
                          className={`font-semibold ${
                            f.severity === 'critical'
                              ? 'text-error'
                              : f.severity === 'warning'
                                ? 'text-tertiary'
                                : 'text-on-surface'
                          }`}
                        >
                          {f.usage_percent > 0
                            ? `${f.usage_percent}%`
                            : `${Math.round(f.current_usage)} total`}
                        </span>
                      </div>
                      {f.usage_percent > 0 && (
                        <div className="h-1.5 w-full overflow-hidden rounded-full bg-surface-container-lowest">
                          <div
                            className={`h-full rounded-full ${
                              f.severity === 'critical'
                                ? 'bg-error'
                                : f.severity === 'warning'
                                  ? 'bg-tertiary'
                                  : 'bg-primary'
                            }`}
                            style={{ width: `${Math.min(f.usage_percent, 100)}%` }}
                          />
                        </div>
                      )}
                      {f.months_until_full != null && (
                        <p className="mt-0.5 text-[10px] text-on-surface-variant">
                          ~{f.months_until_full} months until threshold
                        </p>
                      )}
                    </div>
                  ))}
              {(!Array.isArray(capacityForecasts) || capacityForecasts.length === 0) && (
                <p className="text-xs text-on-surface-variant">No capacity data yet.</p>
              )}
            </div>
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
                  onClick={() =>
                    item.action === '__export__'
                      ? toast.info('Coming Soon')
                      : item.action && navigate(item.action)
                  }
                  className="flex w-full cursor-pointer items-center gap-3 rounded bg-surface-container-low px-4 py-3 text-left text-xs font-bold tracking-wider text-on-surface transition-colors hover:bg-surface-container-high"
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
