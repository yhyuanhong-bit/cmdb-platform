import { toast } from 'sonner'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useAssets, useUpgradeRecommendations, useAcceptUpgradeRecommendation, useCapacityPlanning } from '../hooks/useAssets'
import { useUpgradeRules, useCreateUpgradeRule, useUpdateUpgradeRule, useDeleteUpgradeRule } from '../hooks/useUpgradeRules'
import type { CapacityForecast, Asset } from '../lib/api/assets'
import type { UpgradeRule } from '../lib/api/upgradeRules'

/* ------------------------------------------------------------------ */
/*  Types                                                               */
/* ------------------------------------------------------------------ */

type FilterTab = 'ALL' | 'CPU' | 'RAM' | 'STORAGE' | 'NETWORKING'

const filters: { key: FilterTab; i18nKey: string }[] = [
  { key: 'ALL', i18nKey: 'component_upgrades.filter_all' },
  { key: 'CPU', i18nKey: 'component_upgrades.filter_cpu' },
  { key: 'RAM', i18nKey: 'component_upgrades.filter_ram' },
  { key: 'STORAGE', i18nKey: 'component_upgrades.filter_storage' },
  { key: 'NETWORKING', i18nKey: 'component_upgrades.filter_networking' },
]

interface UpgradeCard {
  id: string
  category: string
  filterKey: FilterTab
  title: string
  rcmLevel: string
  rcmColor: string
  current: string
  recommended: string
  metric: string
  metricValue: string
  costPerNode: string | number
  selected: boolean
  alternatives?: string[]
}

const RULE_PRIORITIES = ['low', 'medium', 'high', 'critical']
const RULE_CATEGORIES = ['cpu', 'memory', 'storage', 'network', 'overall']

const emptyNewRule = {
  asset_type: 'server',
  category: 'cpu',
  metric_name: 'cpu_usage',
  threshold: 80,
  duration_days: 7,
  priority: 'medium',
  recommendation: '',
  enabled: true,
}

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */


interface UpgradeRecommendationItem {
  id: string;
  category: string;
  priority: string;
  recommendation: string;
  current_spec?: string;
  metric_name: string;
  avg_value: number;
  threshold: number;
  cost_estimate?: number | string;
  alternatives?: string[];
}

interface UpgradeRecommendationResponse {
  recommendations?: UpgradeRecommendationItem[];
  warranty_warning?: string;
}

export default function ComponentUpgradeRecommendations() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [activeFilter, setActiveFilter] = useState<FilterTab>('ALL')
  const [selectedAssetId, setSelectedAssetId] = useState('')
  const [expandedCard, setExpandedCard] = useState<string | null>(null)
  const [localSelected, setLocalSelected] = useState<Set<string>>(new Set())
  const [showRules, setShowRules] = useState(false)
  const [showAddRule, setShowAddRule] = useState(false)
  const [newRule, setNewRule] = useState({ ...emptyNewRule })

  // Upgrade rules management
  const rulesQ = useUpgradeRules()
  const rules: UpgradeRule[] = rulesQ.data?.data?.rules ?? []
  const createRule = useCreateUpgradeRule()
  const updateRule = useUpdateUpgradeRule()
  const deleteRule = useDeleteUpgradeRule()

  // Fetch server assets for dropdown
  const { data: assetsData } = useAssets({ type: 'server' })

  // Fetch fleet context
  const { data: apiData } = useAssets()
  const allAssets: Asset[] = apiData?.data ?? []
  const assetCount = allAssets?.length || 0
  const operationalPct = assetCount > 0 ? Math.round((allAssets.filter((a: Asset) => a.status === 'operational').length / assetCount) * 100) : 0
  const impactMetricKeys = [
    { labelKey: 'component_upgrades.metric_fleet_health', value: `${operationalPct}%`, icon: 'health_and_safety' },
    { labelKey: 'component_upgrades.metric_power_efficiency', value: String(assetCount), icon: 'inventory_2' },
  ]

  // Fetch capacity planning data
  const forecasts = useCapacityPlanning().data ?? []

  // Fetch upgrade recommendations for selected asset
  const { data: recData } = useUpgradeRecommendations(selectedAssetId)
  const acceptMutation = useAcceptUpgradeRecommendation()
  const warrantyWarning = (recData as UpgradeRecommendationResponse | undefined)?.warranty_warning

  // Map API data to card format
  const apiCards: UpgradeCard[] = ((recData as UpgradeRecommendationResponse | undefined)?.recommendations ?? []).map((r: UpgradeRecommendationItem) => ({
    id: r.id,
    category: r.category.toUpperCase(),
    filterKey: r.category.toUpperCase() as FilterTab,
    title: r.recommendation,
    rcmLevel: `RCM ${r.priority.toUpperCase()}`,
    rcmColor: r.priority === 'critical' ? 'bg-red-500/20 text-red-400' : r.priority === 'high' ? 'bg-amber-500/20 text-amber-400' : 'bg-blue-500/20 text-blue-400',
    current: r.current_spec || '-',
    recommended: r.recommendation,
    metric: r.metric_name,
    metricValue: `${r.avg_value}% avg (threshold: ${r.threshold}%)`,
    costPerNode: r.cost_estimate ?? '-',
    selected: localSelected.has(r.id),
    alternatives: r.alternatives ?? [],
  }))

  const cards = apiCards

  const toggleSelect = (id: string) => {
    setLocalSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  const selectedCards = cards.filter((c) => localSelected.has(c.id))
  const totalCost = selectedCards.reduce((sum, c) => {
    const val = parseFloat(String(c.costPerNode).replace(/[$,]/g, ''))
    return sum + (isNaN(val) ? 0 : val)
  }, 0)

  const filtered =
    activeFilter === 'ALL'
      ? cards
      : cards.filter((c) => c.filterKey === activeFilter)

  return (
    <div className="min-h-screen bg-surface p-6 font-body text-on-surface">
      {/* Header */}
      <div className="mb-2">
        <h1 className="font-headline text-2xl font-bold tracking-wide text-on-surface">
          {t('component_upgrades.title')}
        </h1>
        <p className="mt-2 max-w-3xl text-sm leading-relaxed text-on-surface-variant">
          {t('component_upgrades.subtitle')}
          <span className="text-xs text-on-surface-variant ml-2">Based on {assetCount} monitored assets</span>
        </p>
      </div>

      {/* Capacity Planning Section */}
      <div className="mb-8 mt-4">
        <h2 className="font-headline text-lg font-bold text-on-surface mb-4">
          {t('component_upgrades.capacity_title', 'Capacity Planning')}
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {(forecasts as CapacityForecast[]).map((f, i) => (
            <div key={i} className={`bg-surface-container rounded-lg p-4 border-l-4 ${
              f.severity === 'critical' ? 'border-error' :
              f.severity === 'warning' ? 'border-tertiary' : 'border-primary'
            }`}>
              <div className="flex items-center justify-between mb-2">
                <span className="text-sm font-bold text-on-surface">{f.resource_name}</span>
                <span className={`text-xs px-2 py-0.5 rounded font-semibold ${
                  f.trend === 'rising' ? 'bg-error/20 text-error' :
                  f.trend === 'declining' ? 'bg-primary/20 text-primary' :
                  'bg-surface-container-high text-on-surface-variant'
                }`}>
                  {f.trend === 'rising' ? '↑' : f.trend === 'declining' ? '↓' : '→'} {f.trend}
                </span>
              </div>
              {f.current_capacity > 0 && f.usage_percent > 0 && (
                <div className="mb-2">
                  <div className="flex justify-between text-xs text-on-surface-variant mb-1">
                    <span>{Math.round(f.current_usage)} / {Math.round(f.current_capacity)}</span>
                    <span>{f.usage_percent}%</span>
                  </div>
                  <div className="w-full h-2 bg-surface-container-lowest rounded-full overflow-hidden">
                    <div className={`h-full rounded-full ${
                      f.usage_percent >= 90 ? 'bg-error' :
                      f.usage_percent >= 75 ? 'bg-tertiary' : 'bg-primary'
                    }`} style={{ width: `${Math.min(f.usage_percent, 100)}%` }} />
                  </div>
                </div>
              )}
              {f.months_until_full !== null && f.months_until_full !== undefined && (
                <p className={`text-xs font-semibold ${
                  f.months_until_full <= 1 ? 'text-error' :
                  f.months_until_full <= 3 ? 'text-tertiary' : 'text-on-surface-variant'
                }`}>
                  {f.months_until_full <= 0
                    ? t('component_upgrades.at_capacity', 'At capacity!')
                    : `~${f.months_until_full} ${t('component_upgrades.months_until_full', 'months until threshold')}`}
                </p>
              )}
              {f.monthly_growth !== 0 && (
                <p className="text-xs text-on-surface-variant mt-1">
                  Growth: {f.monthly_growth > 0 ? '+' : ''}{f.monthly_growth}{t('component_upgrades.growth_per_month', '/month')}
                </p>
              )}
              <p className="text-xs text-on-surface-variant mt-2">{f.recommendation}</p>
            </div>
          ))}
          {forecasts.length === 0 && (
            <div className="col-span-full bg-surface-container rounded-lg p-6 text-center text-on-surface-variant text-sm">
              No capacity data available yet.
            </div>
          )}
        </div>
      </div>

      {/* Upgrade Rules Management */}
      <div className="mb-8">
        <button
          onClick={() => setShowRules((v) => !v)}
          className="flex items-center gap-2 text-sm font-bold text-on-surface-variant hover:text-on-surface transition-colors cursor-pointer"
        >
          <span className="material-symbols-outlined text-base text-primary">rule</span>
          <span className="text-[10px] font-bold tracking-widest">{t('component_upgrades.rules_title')}</span>
          <span className="material-symbols-outlined text-base">{showRules ? 'expand_less' : 'expand_more'}</span>
        </button>
        {showRules && (
          <div className="mt-4 bg-surface-container rounded-lg p-4">
            <div className="flex items-center justify-between mb-3">
              <p className="text-xs text-on-surface-variant">{t('component_upgrades.rules_subtitle')}</p>
              <button
                onClick={() => setShowAddRule((v) => !v)}
                className="flex items-center gap-1 rounded bg-primary/15 px-3 py-1.5 text-[10px] font-bold tracking-widest text-primary hover:bg-primary/25 transition-colors cursor-pointer"
              >
                <span className="material-symbols-outlined text-[14px]">add</span>
                {t('component_upgrades.add_rule')}
              </button>
            </div>

            {/* Add Rule Form */}
            {showAddRule && (
              <div className="mb-4 bg-surface-container-high rounded-lg p-4 grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-[10px] tracking-widest text-on-surface-variant mb-1">{t('component_upgrades.rule_asset_type')}</label>
                  <input
                    className="w-full bg-surface-container-highest text-on-surface text-xs rounded px-2 py-1.5 outline-none"
                    value={newRule.asset_type}
                    onChange={(e) => setNewRule({ ...newRule, asset_type: e.target.value })}
                  />
                </div>
                <div>
                  <label className="block text-[10px] tracking-widest text-on-surface-variant mb-1">{t('component_upgrades.rule_category')}</label>
                  <select
                    className="w-full bg-surface-container-highest text-on-surface text-xs rounded px-2 py-1.5 outline-none cursor-pointer"
                    value={newRule.category}
                    onChange={(e) => setNewRule({ ...newRule, category: e.target.value })}
                  >
                    {RULE_CATEGORIES.map((c) => <option key={c} value={c}>{c}</option>)}
                  </select>
                </div>
                <div>
                  <label className="block text-[10px] tracking-widest text-on-surface-variant mb-1">{t('component_upgrades.rule_metric')}</label>
                  <input
                    className="w-full bg-surface-container-highest text-on-surface text-xs rounded px-2 py-1.5 outline-none"
                    value={newRule.metric_name}
                    onChange={(e) => setNewRule({ ...newRule, metric_name: e.target.value })}
                  />
                </div>
                <div>
                  <label className="block text-[10px] tracking-widest text-on-surface-variant mb-1">{t('component_upgrades.rule_threshold')}</label>
                  <input
                    type="number"
                    className="w-full bg-surface-container-highest text-on-surface text-xs rounded px-2 py-1.5 outline-none"
                    value={newRule.threshold}
                    onChange={(e) => setNewRule({ ...newRule, threshold: parseFloat(e.target.value) || 0 })}
                  />
                </div>
                <div>
                  <label className="block text-[10px] tracking-widest text-on-surface-variant mb-1">{t('component_upgrades.rule_duration')}</label>
                  <input
                    type="number"
                    className="w-full bg-surface-container-highest text-on-surface text-xs rounded px-2 py-1.5 outline-none"
                    value={newRule.duration_days}
                    onChange={(e) => setNewRule({ ...newRule, duration_days: parseInt(e.target.value) || 7 })}
                  />
                </div>
                <div>
                  <label className="block text-[10px] tracking-widest text-on-surface-variant mb-1">{t('component_upgrades.rule_priority')}</label>
                  <select
                    className="w-full bg-surface-container-highest text-on-surface text-xs rounded px-2 py-1.5 outline-none cursor-pointer"
                    value={newRule.priority}
                    onChange={(e) => setNewRule({ ...newRule, priority: e.target.value })}
                  >
                    {RULE_PRIORITIES.map((p) => <option key={p} value={p}>{p}</option>)}
                  </select>
                </div>
                <div className="col-span-2">
                  <label className="block text-[10px] tracking-widest text-on-surface-variant mb-1">{t('component_upgrades.rule_recommendation')}</label>
                  <input
                    className="w-full bg-surface-container-highest text-on-surface text-xs rounded px-2 py-1.5 outline-none"
                    value={newRule.recommendation}
                    onChange={(e) => setNewRule({ ...newRule, recommendation: e.target.value })}
                    placeholder="Describe the recommended action..."
                  />
                </div>
                <div className="col-span-2 flex gap-2 justify-end">
                  <button
                    onClick={() => { setShowAddRule(false); setNewRule({ ...emptyNewRule }) }}
                    className="rounded bg-surface-container px-4 py-1.5 text-[10px] font-bold tracking-widest text-on-surface-variant hover:bg-surface-container-high cursor-pointer"
                  >
                    {t('common.cancel', 'Cancel')}
                  </button>
                  <button
                    onClick={() => {
                      if (!newRule.recommendation.trim()) {
                        toast.error('Recommendation text is required')
                        return
                      }
                      createRule.mutate(newRule, {
                        onSuccess: () => {
                          toast.success('Rule created')
                          setShowAddRule(false)
                          setNewRule({ ...emptyNewRule })
                        },
                        onError: () => toast.error('Failed to create rule'),
                      })
                    }}
                    disabled={createRule.isPending}
                    className="rounded bg-primary px-4 py-1.5 text-[10px] font-bold tracking-widest text-[#0a151a] hover:opacity-90 cursor-pointer disabled:opacity-50"
                  >
                    {t('component_upgrades.add_rule')}
                  </button>
                </div>
              </div>
            )}

            {/* Rules Table */}
            {rules.length === 0 ? (
              <p className="text-xs text-on-surface-variant text-center py-6">No rules configured.</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="text-[10px] tracking-widest text-on-surface-variant border-b border-outline-variant/20">
                      <th className="text-left pb-2 pr-3">{t('component_upgrades.rule_asset_type')}</th>
                      <th className="text-left pb-2 pr-3">{t('component_upgrades.rule_category')}</th>
                      <th className="text-left pb-2 pr-3">{t('component_upgrades.rule_metric')}</th>
                      <th className="text-right pb-2 pr-3">{t('component_upgrades.rule_threshold')}</th>
                      <th className="text-right pb-2 pr-3">{t('component_upgrades.rule_duration')}</th>
                      <th className="text-left pb-2 pr-3">{t('component_upgrades.rule_priority')}</th>
                      <th className="text-left pb-2 pr-3">{t('component_upgrades.rule_enabled')}</th>
                      <th className="text-right pb-2"></th>
                    </tr>
                  </thead>
                  <tbody>
                    {rules.map((rule) => (
                      <tr key={rule.id} className="border-b border-outline-variant/10 hover:bg-surface-container-high">
                        <td className="py-2 pr-3 text-on-surface">{rule.asset_type}</td>
                        <td className="py-2 pr-3 text-on-surface-variant">{rule.category}</td>
                        <td className="py-2 pr-3 text-on-surface-variant font-mono">{rule.metric_name}</td>
                        <td className="py-2 pr-3 text-right font-mono text-on-surface">{rule.threshold}%</td>
                        <td className="py-2 pr-3 text-right text-on-surface-variant">{rule.duration_days}d</td>
                        <td className="py-2 pr-3">
                          <span className={`px-2 py-0.5 rounded text-[10px] font-bold ${
                            rule.priority === 'critical' ? 'bg-red-500/20 text-red-400' :
                            rule.priority === 'high' ? 'bg-amber-500/20 text-amber-400' :
                            rule.priority === 'medium' ? 'bg-blue-500/20 text-blue-400' :
                            'bg-surface-container-highest text-on-surface-variant'
                          }`}>{rule.priority}</span>
                        </td>
                        <td className="py-2 pr-3">
                          <button
                            onClick={() => updateRule.mutate(
                              { id: rule.id, data: { enabled: !rule.enabled } },
                              { onError: () => toast.error('Failed to update rule') }
                            )}
                            className="cursor-pointer"
                          >
                            <span className={`material-symbols-outlined text-base ${rule.enabled ? 'text-primary' : 'text-on-surface-variant'}`}>
                              {rule.enabled ? 'toggle_on' : 'toggle_off'}
                            </span>
                          </button>
                        </td>
                        <td className="py-2 text-right">
                          <button
                            onClick={() => deleteRule.mutate(rule.id, {
                              onSuccess: () => toast.success('Rule deleted'),
                              onError: () => toast.error('Failed to delete rule'),
                            })}
                            className="text-on-surface-variant hover:text-error transition-colors cursor-pointer"
                          >
                            <span className="material-symbols-outlined text-base">delete</span>
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Asset selector */}
      <div className="px-0 pb-4 pt-4">
        <select
          value={selectedAssetId}
          onChange={e => setSelectedAssetId(e.target.value)}
          className="bg-surface-container-high text-on-surface text-sm rounded-lg px-3 py-2 outline-none cursor-pointer"
        >
          <option value="">{t('component_upgrades.select_asset')}</option>
          {(assetsData?.data ?? []).map((a: Asset) => (
            <option key={a.id} value={a.id}>{a.name} ({a.type})</option>
          ))}
        </select>
      </div>

      {/* Filters + Impact */}
      <div className="mt-2 mb-6 flex flex-wrap items-center justify-between gap-4">
        {/* Filter tabs */}
        <div className="flex gap-1">
          {filters.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setActiveFilter(tab.key)}
              className={`rounded px-4 py-2 text-[10px] font-bold tracking-widest transition-colors cursor-pointer ${
                activeFilter === tab.key
                  ? 'bg-primary/15 text-primary'
                  : 'bg-surface-container text-on-surface-variant hover:bg-surface-container-high'
              }`}
            >
              {t(tab.i18nKey)}
            </button>
          ))}
        </div>

        {/* Aggregated Impact */}
        <div className="flex gap-4">
          {impactMetricKeys.map((m) => (
            <div
              key={m.labelKey}
              className="flex items-center gap-2 rounded bg-surface-container px-4 py-2"
            >
              <span className="material-symbols-outlined text-base text-primary">
                {m.icon}
              </span>
              <div>
                <div className="text-[10px] tracking-widest text-on-surface-variant">
                  {t(m.labelKey)}
                </div>
                <div className="font-mono text-sm font-bold text-[#69db7c]">
                  {m.value}
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Upgrade ROI Map label */}
      <div className="mb-4 flex items-center gap-2">
        <span className="material-symbols-outlined text-base text-primary">
          insights
        </span>
        <h2 className="text-[10px] font-bold tracking-widest text-on-surface-variant">
          {t('component_upgrades.section_upgrade_roi_map')}
        </h2>
      </div>

      {/* No asset selected message */}
      {!selectedAssetId && (
        <div className="flex items-center justify-center rounded-lg bg-surface-container py-16 text-sm text-on-surface-variant">
          {t('component_upgrades.empty_select_asset')}
        </div>
      )}

      {/* Warranty warning */}
      {warrantyWarning && (
        <div className="bg-tertiary/10 border border-tertiary/30 rounded-lg p-3 mb-4 text-sm text-tertiary">
          ⚠️ {warrantyWarning}
        </div>
      )}

      {/* Card grid */}
      {selectedAssetId && (
        <div className="grid gap-4 sm:grid-cols-2">
          {filtered.length === 0 && (
            <div className="col-span-2 flex items-center justify-center rounded-lg bg-surface-container py-16 text-sm text-on-surface-variant">
              {t('component_upgrades.empty_no_recommendations')}
            </div>
          )}
          {filtered.map((card) => {
            const isSelected = localSelected.has(card.id)
            return (
              <div
                key={card.id}
                className={`rounded-lg p-5 transition-colors ${
                  isSelected
                    ? 'bg-primary/5 ring-1 ring-primary/30'
                    : 'bg-surface-container'
                }`}
              >
                {/* Category + RCM badge */}
                <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                  <div className="flex items-center gap-2">
                    <span className="text-[10px] tracking-widest text-on-surface-variant">
                      {t('component_upgrades.label_category')}
                    </span>
                    <span className="text-[10px] font-bold tracking-widest text-on-surface">
                      {card.category}
                    </span>
                  </div>
                  <span
                    className={`rounded px-2 py-0.5 text-[10px] font-bold tracking-widest ${card.rcmColor}`}
                  >
                    {card.rcmLevel}
                  </span>
                </div>

                {/* Title */}
                <h3 className="mb-4 font-headline text-lg font-bold text-on-surface">
                  {card.title}
                </h3>

                {/* Current vs Recommended */}
                <div className="mb-4 grid grid-cols-2 gap-3">
                  <div className="rounded bg-surface-container-low p-3">
                    <div className="text-[10px] tracking-widest text-on-surface-variant">
                      {t('component_upgrades.label_current')}
                    </div>
                    <div className="mt-1 text-xs font-semibold text-on-surface">
                      {card.current}
                    </div>
                  </div>
                  <div className="rounded bg-surface-container-low p-3">
                    <div className="text-[10px] tracking-widest text-on-surface-variant">
                      {t('component_upgrades.label_recommended')}
                    </div>
                    <div className="mt-1 text-xs font-semibold text-primary">
                      {card.recommended}
                    </div>
                  </div>
                </div>

                {/* Metric + Cost */}
                <div className="mb-5 flex items-center justify-between">
                  <div>
                    <div className="text-[10px] tracking-widest text-on-surface-variant">
                      {card.metric}
                    </div>
                    <div className="mt-0.5 font-mono text-sm font-bold text-[#69db7c]">
                      {card.metricValue}
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="text-[10px] tracking-widest text-on-surface-variant">
                      {t('component_upgrades.label_cost_per_node')}
                    </div>
                    <div className="mt-0.5 font-mono text-sm font-bold text-on-surface">
                      {card.costPerNode}
                    </div>
                  </div>
                </div>

                {/* Actions */}
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => {
                      // C-H10 (audit 2026-04-28): only fire the accept
                      // mutation on transition INTO the selected state.
                      // Previously this fired on every toggle (including
                      // deselect), creating duplicate accept writes.
                      if (!isSelected) {
                        acceptMutation.mutate({ assetId: selectedAssetId, category: card.filterKey.toLowerCase() })
                      }
                      toggleSelect(card.id)
                    }}
                    className={`flex-1 rounded py-2.5 text-[10px] font-bold tracking-widest transition-colors cursor-pointer ${
                      isSelected
                        ? 'bg-primary text-[#0a151a]'
                        : 'bg-surface-container-high text-primary hover:bg-primary/15'
                    }`}
                  >
                    {isSelected ? t('component_upgrades.btn_selected') : t('component_upgrades.btn_request_upgrade')}
                  </button>
                  <button
                    onClick={() => setExpandedCard(expandedCard === card.id ? null : card.id)}
                    className="rounded bg-surface-container-high px-4 py-2.5 text-[10px] font-bold tracking-widest text-on-surface-variant transition-colors hover:bg-surface-container-low cursor-pointer"
                  >
                    {t('component_upgrades.btn_learn_more')}
                  </button>
                </div>
                {expandedCard === card.id && (
                  <div className="mt-3 pt-3 border-t border-outline-variant/20 text-xs text-on-surface-variant">
                    <p>{t('component_upgrades.detail_benchmark')}</p>
                    <p className="mt-1">{t('component_upgrades.detail_implementation')}</p>
                    {card.alternatives && card.alternatives.length > 0 && (
                      <div className="mt-3 border-t border-surface-container-highest pt-3">
                        <p className="text-xs font-semibold text-on-surface-variant mb-2">
                          {t('component_upgrades.alternatives_title')}
                        </p>
                        <ul className="space-y-1">
                          {card.alternatives.map((alt: string, i: number) => (
                            <li key={i} className="text-xs text-on-surface-variant flex items-start gap-1.5">
                              <span className="material-symbols-outlined text-[14px] mt-0.5 text-primary">arrow_right</span>
                              {alt}
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}

      {/* Bottom action bar */}
      <div className="mt-6 flex flex-wrap items-center justify-between gap-4 rounded-lg bg-surface-container p-5">
        <div className="flex flex-wrap items-center gap-6">
          <div>
            <div className="text-[10px] tracking-widest text-on-surface-variant">
              {t('component_upgrades.label_selected_components')}
            </div>
            <div className="mt-0.5 font-mono text-lg font-bold text-on-surface">
              {selectedCards.length}
            </div>
          </div>
          <div>
            <div className="text-[10px] tracking-widest text-on-surface-variant">
              {t('component_upgrades.label_total_cost')}
            </div>
            <div className="mt-0.5 font-mono text-lg font-bold text-primary">
              ${totalCost.toLocaleString('en-US', { minimumFractionDigits: 2 })}
            </div>
          </div>
          <div>
            <div className="text-[10px] tracking-widest text-on-surface-variant">
              {t('component_upgrades.label_approval_status')}
            </div>
            <div className="mt-0.5 text-xs font-bold tracking-widest text-[#ffa94d]">
              {t('component_upgrades.status_pending_review')}
            </div>
          </div>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => navigate('/maintenance/add')}
            className="rounded bg-surface-container-high px-5 py-2.5 text-[10px] font-bold tracking-widest text-on-surface-variant transition-colors hover:bg-surface-container-low cursor-pointer"
          >
            {t('component_upgrades.btn_schedule_maintenance')}
          </button>
          <button
            onClick={() => {
              if (selectedCards.length > 0) {
                navigate('/maintenance/add')
              } else {
                toast.info(t('component_upgrades.alert_select_one'))
              }
            }}
            className={`rounded px-5 py-2.5 text-[10px] font-bold tracking-widest transition-colors cursor-pointer ${
              selectedCards.length > 0
                ? 'bg-primary text-[#0a151a]'
                : 'bg-surface-container-high text-on-surface-variant'
            }`}
          >
            {t('component_upgrades.btn_request_selection')}
          </button>
        </div>
      </div>
    </div>
  )
}
