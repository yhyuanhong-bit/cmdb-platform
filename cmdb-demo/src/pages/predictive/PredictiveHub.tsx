import { memo, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { usePredictionModels } from '../../hooks/usePrediction'
import { useAlerts } from '../../hooks/useMonitoring'
import { usePredictiveRefresh } from '../../hooks/usePredictiveRefresh'
import CreateRCAModal from '../../components/CreateRCAModal'
import { Icon, TAB_DEFINITIONS, type TabKey } from './shared'
import { PredictionOverviewTab } from './PredictionOverviewTab'
import { AlertsTab } from './AlertsTab'
import { InsightsTab } from './InsightsTab'
import { RecommendationsTab } from './RecommendationsTab'
import { TimelineTab } from './TimelineTab'
import { ForecastTab } from './ForecastTab'

const PLACEHOLDER = '—'

const PredictiveHub = memo(function PredictiveHub() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [activeTab, setActiveTab] = useState<TabKey>('overview')
  const [showCreateRCA, setShowCreateRCA] = useState(false)
  const { data: modelsResponse } = usePredictionModels()
  const models = modelsResponse?.data ?? []
  // C-H4/H5/H6 (audit 2026-04-28): the previous hardcoded values
  // ('42' / '12' / '158h' / '14:20:05' / '98.4%') showed fabricated
  // metrics in production. We now derive what the API exposes and
  // mark the rest as unavailable so users can tell live data from
  // missing data.
  const { data: openRefreshResp } = usePredictiveRefresh({ status: 'open', page_size: 200 })
  const { data: critAlertsResp } = useAlerts({ status: 'firing', severity: 'critical' })

  const openRefreshRows = openRefreshResp?.data ?? []
  const criticalAlerts = critAlertsResp?.data ?? []

  const enabledModels = models.filter((m) => m.enabled)
  const accuracies = enabledModels
    .map((m) => (typeof m.accuracy === 'number' ? m.accuracy : null))
    .filter((v): v is number => v !== null)
  const avgAccuracy = accuracies.length > 0
    ? accuracies.reduce((s, n) => s + n, 0) / accuracies.length
    : null
  const accuracyDisplay = avgAccuracy !== null
    ? `${(avgAccuracy * (avgAccuracy <= 1 ? 100 : 1)).toFixed(1)}%`
    : PLACEHOLDER

  const lastTrained = useMemo(() => {
    const stamps = models
      .map((m) => (m.last_trained ? Date.parse(m.last_trained) : NaN))
      .filter((n) => Number.isFinite(n)) as number[]
    if (stamps.length === 0) return null
    const max = Math.max(...stamps)
    return new Date(max)
  }, [models])
  const lastUpdateDisplay = lastTrained
    ? lastTrained.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    : PLACEHOLDER

  const totalAtRisk = openRefreshRows.length
  const totalAtRiskDisplay = openRefreshResp ? String(totalAtRisk) : PLACEHOLDER
  const highPriorityDisplay = critAlertsResp ? String(criticalAlerts.length) : PLACEHOLDER
  // Downtime saved aggregate is not exposed by any current endpoint.
  // Surface as unavailable rather than fabricate.
  const downtimeSavedDisplay = PLACEHOLDER

  type StatCard = {
    labelKey: string
    value: string
    icon: string
    deltaKey: string
    deltaColor: string
    isUnavailable: boolean
  }
  const statCards: StatCard[] = [
    {
      labelKey: 'predictive_hub.stat_total_assets_risk',
      value: totalAtRiskDisplay,
      icon: 'warning',
      deltaKey: 'predictive_hub.stat_total_assets_delta',
      deltaColor: 'text-tertiary',
      isUnavailable: totalAtRiskDisplay === PLACEHOLDER,
    },
    {
      labelKey: 'predictive_hub.stat_high_priority',
      value: highPriorityDisplay,
      icon: 'priority_high',
      deltaKey: 'predictive_hub.stat_high_priority_delta',
      deltaColor: 'text-error',
      isUnavailable: highPriorityDisplay === PLACEHOLDER,
    },
    {
      labelKey: 'predictive_hub.stat_downtime_saved',
      value: downtimeSavedDisplay,
      icon: 'timer',
      deltaKey: 'predictive_hub.stat_downtime_delta',
      deltaColor: 'text-primary',
      isUnavailable: true,
    },
  ]

  const renderTabContent = () => {
    switch (activeTab) {
      case 'overview':
        return <PredictionOverviewTab />
      case 'alerts':
        return <AlertsTab />
      case 'insights':
        return <InsightsTab />
      case 'recommendations':
        return <RecommendationsTab />
      case 'timeline':
        return <TimelineTab />
      case 'forecast':
        return <ForecastTab />
      default:
        return <PredictionOverviewTab />
    }
  }

  return (
    <div className="min-h-screen bg-surface p-6 font-body text-on-surface">
      {/* Breadcrumb */}
      <button
        onClick={() => navigate('/dashboard')}
        className="flex items-center gap-1 text-on-surface-variant text-sm mb-6 hover:text-primary transition-colors"
      >
        <Icon name="arrow_back" className="text-[18px]" />
        <span className="uppercase tracking-wider text-[0.6875rem] font-semibold">
          {t('common.back_to_dashboard')}
        </span>
      </button>

      {/* ── Shared Header (always visible) ──────── */}
      <div className="flex items-start justify-between mb-6">
        <div>
          <h1 className="font-headline text-3xl font-bold tracking-tight text-on-surface">
            {t('predictive.title_zh')}
          </h1>
          <p className="text-on-surface-variant text-sm mt-1 font-label tracking-widest uppercase">
            {t('predictive_hub.subtitle_predictive_hub')}
          </p>
        </div>
        <div className="flex items-center gap-6">
          <button
            onClick={() => setShowCreateRCA(true)}
            className="bg-primary/20 hover:bg-primary/30 px-4 py-2.5 rounded-lg flex items-center gap-2 text-sm font-semibold text-primary transition-colors"
          >
            <Icon name="psychology" className="text-primary text-xl" />
            {t('predictive_hub.btn_new_rca')}
          </button>
          <button
            onClick={() => navigate('/maintenance')}
            className="bg-surface-container-high hover:bg-surface-container-highest px-4 py-2.5 rounded-lg flex items-center gap-2 text-sm font-semibold text-on-surface transition-colors"
          >
            <Icon name="build" className="text-primary text-xl" />
            {t('predictive_hub.btn_maintenance_mgmt')}
          </button>
          <div className="bg-surface-container-high px-4 py-2 rounded-lg flex items-center gap-2">
            <Icon name="model_training" className="text-primary text-xl" />
            <div>
              <p className="text-[10px] text-on-surface-variant font-label uppercase tracking-widest">
                {t('predictive.model_accuracy')}
              </p>
              <p className={`font-headline text-xl font-bold leading-tight ${accuracyDisplay === PLACEHOLDER ? 'text-on-surface-variant' : 'text-primary'}`}>
                {accuracyDisplay}
              </p>
              {accuracyDisplay === PLACEHOLDER && (
                <p className="text-[9px] text-on-surface-variant font-label">
                  {t('common.metric_unavailable')}
                </p>
              )}
            </div>
          </div>
          <div className="bg-surface-container-high px-4 py-2 rounded-lg flex items-center gap-2">
            <Icon name="schedule" className="text-on-surface-variant text-xl" />
            <div>
              <p className="text-[10px] text-on-surface-variant font-label uppercase tracking-widest">
                {t('predictive.last_update')}
              </p>
              <p className={`font-headline text-xl font-bold tabular-nums leading-tight ${lastUpdateDisplay === PLACEHOLDER ? 'text-on-surface-variant' : 'text-on-surface'}`}>
                {lastUpdateDisplay}
              </p>
              {lastUpdateDisplay === PLACEHOLDER && (
                <p className="text-[9px] text-on-surface-variant font-label">
                  {t('common.metric_unavailable')}
                </p>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Stat cards row */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        {statCards.map((s) => (
          <div key={s.labelKey} className="bg-surface-container rounded-xl p-5 flex flex-col gap-3">
            <div className="flex items-center gap-2">
              <div className="bg-surface-container-high rounded-lg p-2">
                <Icon name={s.icon} className="text-primary text-xl" />
              </div>
              <span className="text-xs text-on-surface-variant font-label uppercase tracking-widest">
                {t(s.labelKey)}
              </span>
            </div>
            <p className={`font-headline text-4xl font-extrabold tracking-tight ${s.isUnavailable ? 'text-on-surface-variant' : 'text-on-surface'}`}>
              {s.value}
            </p>
            {s.isUnavailable ? (
              <p className="text-xs font-label text-on-surface-variant">
                {t('common.metric_unavailable')}
              </p>
            ) : (
              <p className={`text-xs font-label ${s.deltaColor}`}>{t(s.deltaKey)}</p>
            )}
          </div>
        ))}
      </div>

      {/* ── Tab Navigation ──────────────────────── */}
      <div className="flex gap-1 mb-6 border-b border-surface-container-highest pb-0">
        {TAB_DEFINITIONS.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`px-5 py-3 rounded-t-lg text-[0.6875rem] font-semibold tracking-wider transition-colors relative ${
              activeTab === tab.key
                ? 'bg-surface-container text-primary'
                : 'text-on-surface-variant hover:bg-surface-container-high hover:text-on-surface'
            }`}
          >
            <span className="block">{t(tab.labelKey)}</span>
            {activeTab === tab.key && (
              <span className="absolute bottom-0 left-0 right-0 h-0.5 bg-primary rounded-t" />
            )}
          </button>
        ))}
      </div>

      {/* ── Tab Content ─────────────────────────── */}
      {renderTabContent()}

      <CreateRCAModal open={showCreateRCA} onClose={() => setShowCreateRCA(false)} />
    </div>
  )
})

export default PredictiveHub
