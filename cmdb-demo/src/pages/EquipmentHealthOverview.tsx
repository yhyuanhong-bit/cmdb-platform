import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useAssets } from '../hooks/useAssets'

/* ------------------------------------------------------------------ */
/*  Mock Data (health metrics need backend monitoring endpoint)         */
/* ------------------------------------------------------------------ */

const healthScore = 92.4

const metrics = [
  {
    labelKey: 'equipment_health_overview.metric_service_stability',
    value: 98,
    max: 100,
    badgeKey: 'equipment_health_overview.badge_optimal',
    badgeColor: 'bg-[#69db7c]/15 text-[#69db7c]',
    barColor: 'bg-[#69db7c]',
  },
  {
    labelKey: 'equipment_health_overview.metric_hardware_lifespan',
    value: 85,
    max: 100,
    badgeKey: null,
    badgeColor: '',
    barColor: 'bg-primary',
  },
  {
    labelKey: 'equipment_health_overview.metric_storage_stability',
    value: 72,
    max: 100,
    badgeKey: null,
    badgeColor: '',
    barColor: 'bg-[#ffa94d]',
  },
  {
    labelKey: 'equipment_health_overview.metric_network_connectivity',
    value: 95,
    max: 100,
    badgeKey: 'equipment_health_overview.badge_optimal',
    badgeColor: 'bg-[#69db7c]/15 text-[#69db7c]',
    barColor: 'bg-[#69db7c]',
  },
]

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

const warningMessage = {
  titleKey: 'equipment_health_overview.warning_title',
  assetRef: '本機 Asset A4 設備 01 號',
  body: '根據目前感測器數據分析，冷卻風扇模組 FAN-03 的轉速已連續 72 小時低於標準閾值。建議在下一個排定的維護窗口期間進行檢查與更換，以避免因散熱不足導致的非預期停機。',
}

const riskAssessment = {
  titleKey: 'equipment_health_overview.risk_title',
  body: '冷卻單元接近預期壽命上限。根據製造商規格，CU-04 模組已運行 18,200 小時（預期壽命 20,000 小時），剩餘壽命約 9%。若未能及時更換，可能導致機架溫度超過安全閾值，觸發自動降頻保護機制，影響整體運算效能。',
  riskLevel: 'ELEVATED',
  remainingLife: 9,
}

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export default function EquipmentHealthOverview() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [activeTab] = useState(0)

  // Fetch server assets for health overview context
  const { data: apiData } = useAssets({ type: 'server' })
  const serverCount = apiData?.pagination?.total ?? apiData?.data?.length ?? 0

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
              {t('equipment_health_overview.last_sync')}：2024-10-24 14:30:00 UTC
            </span>
          </div>
        </div>
        <div className="flex gap-2">
          <button onClick={() => alert('Coming Soon')} className="flex items-center gap-2 rounded bg-surface-container px-4 py-2.5 text-[10px] font-bold tracking-widest text-on-surface-variant transition-colors hover:bg-surface-container-high cursor-pointer">
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
              <span className="rounded bg-error/15 px-2 py-0.5 text-[10px] font-bold tracking-widest text-error">
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
                  action: null,
                },
              ].map((item) => (
                <button
                  key={item.labelKey}
                  onClick={() => item.action && navigate(item.action)}
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
