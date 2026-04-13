import { toast } from 'sonner'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useAssets, useUpgradeRecommendations, useAcceptUpgradeRecommendation } from '../hooks/useAssets'

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
}

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export default function ComponentUpgradeRecommendations() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [activeFilter, setActiveFilter] = useState<FilterTab>('ALL')
  const [selectedAssetId, setSelectedAssetId] = useState('')
  const [expandedCard, setExpandedCard] = useState<string | null>(null)
  const [localSelected, setLocalSelected] = useState<Set<string>>(new Set())

  // Fetch server assets for dropdown
  const { data: assetsData } = useAssets({ type: 'server' })

  // Fetch fleet context
  const { data: apiData } = useAssets()
  const allAssets = (apiData as any)?.data || []
  const assetCount = allAssets?.length || 0
  const operationalPct = assetCount > 0 ? Math.round((allAssets.filter((a: any) => a.status === 'operational').length / assetCount) * 100) : 0
  const impactMetricKeys = [
    { labelKey: 'component_upgrades.metric_fleet_health', value: `${operationalPct}%`, icon: 'health_and_safety' },
    { labelKey: 'component_upgrades.metric_power_efficiency', value: String(assetCount), icon: 'inventory_2' },
  ]

  // Fetch upgrade recommendations for selected asset
  const { data: recData } = useUpgradeRecommendations(selectedAssetId)
  const acceptMutation = useAcceptUpgradeRecommendation()

  // Map API data to card format
  const apiCards: UpgradeCard[] = ((recData as any)?.recommendations ?? []).map((r: any) => ({
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

      {/* Asset selector */}
      <div className="px-0 pb-4 pt-4">
        <select
          value={selectedAssetId}
          onChange={e => setSelectedAssetId(e.target.value)}
          className="bg-surface-container-high text-on-surface text-sm rounded-lg px-3 py-2 outline-none cursor-pointer"
        >
          <option value="">{t('component_upgrades.select_asset')}</option>
          {((assetsData as any)?.data ?? []).map((a: any) => (
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
                      toggleSelect(card.id)
                      acceptMutation.mutate({ assetId: selectedAssetId, category: card.filterKey.toLowerCase() })
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
