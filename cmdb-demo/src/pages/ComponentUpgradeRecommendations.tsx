import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useAssets } from '../hooks/useAssets'

/* ------------------------------------------------------------------ */
/*  Static Recommendation Data                                         */
/*  TODO: needs backend recommendation engine endpoint                 */
/* ------------------------------------------------------------------ */

type FilterTab = 'ALL' | 'CPU' | 'RAM' | 'STORAGE' | 'NETWORKING'

const filters: { key: FilterTab; i18nKey: string }[] = [
  { key: 'ALL', i18nKey: 'component_upgrades.filter_all' },
  { key: 'CPU', i18nKey: 'component_upgrades.filter_cpu' },
  { key: 'RAM', i18nKey: 'component_upgrades.filter_ram' },
  { key: 'STORAGE', i18nKey: 'component_upgrades.filter_storage' },
  { key: 'NETWORKING', i18nKey: 'component_upgrades.filter_networking' },
]

const impactMetricKeys = [
  { labelKey: 'component_upgrades.metric_fleet_health', value: '+94%', icon: 'health_and_safety' },
  { labelKey: 'component_upgrades.metric_power_efficiency', value: '+15%', icon: 'bolt' },
]

interface UpgradeCard {
  id: string
  category: string
  categoryTag: 'COMPUTE' | 'MEMORY' | 'STORAGE' | 'NETWORK'
  filterKey: FilterTab
  title: string
  rcmLevel: string
  rcmColor: string
  current: string
  recommended: string
  metric: string
  metricValue: string
  costPerNode: string
  selected: boolean
}

const initialCards: UpgradeCard[] = [
  {
    id: 'u1',
    category: 'COMPUTE',
    categoryTag: 'COMPUTE',
    filterKey: 'CPU',
    title: 'Processor Upgrade',
    rcmLevel: 'RCM HIGH',
    rcmColor: 'bg-error/20 text-error',
    current: 'Intel Xeon Gold 6130',
    recommended: 'Intel Xeon Platinum 8338',
    metric: 'Performance Gain',
    metricValue: '+45% Throughput',
    costPerNode: '$2,450.00',
    selected: false,
  },
  {
    id: 'u2',
    category: 'MEMORY',
    categoryTag: 'MEMORY',
    filterKey: 'RAM',
    title: 'Density Expansion',
    rcmLevel: 'RCM MED',
    rcmColor: 'bg-[#ffa94d]/20 text-[#ffa94d]',
    current: '128GB DDR4 2666MHz',
    recommended: '512GB DDR4 3200MHz',
    metric: 'Memory Latency',
    metricValue: '-20% Wait Time',
    costPerNode: '$1,180.00',
    selected: false,
  },
  {
    id: 'u3',
    category: 'STORAGE',
    categoryTag: 'STORAGE',
    filterKey: 'STORAGE',
    title: 'NVMe Acceleration',
    rcmLevel: 'RCM CRITICAL',
    rcmColor: 'bg-error/30 text-error',
    current: '2TB SATA SSD Raid 1',
    recommended: '4TB NVMe Gen4',
    metric: 'IOPS Gain',
    metricValue: '12X',
    costPerNode: '$890.00',
    selected: false,
  },
  {
    id: 'u4',
    category: 'NETWORK',
    categoryTag: 'NETWORK',
    filterKey: 'NETWORKING',
    title: 'Fabric Interconnect',
    rcmLevel: 'RCM LOW',
    rcmColor: 'bg-primary/20 text-primary',
    current: '10GbE SFP+',
    recommended: '100GbE QSFP28',
    metric: 'Capacity',
    metricValue: '+900%',
    costPerNode: '$1,420.00',
    selected: false,
  },
]

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export default function ComponentUpgradeRecommendations() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [activeFilter, setActiveFilter] = useState<FilterTab>('ALL')
  const [cards, setCards] = useState(initialCards)

  // Fetch assets to show fleet context
  const { data: apiData } = useAssets()
  const assetCount = apiData?.pagination?.total ?? apiData?.data?.length ?? 0

  const toggleSelect = (id: string) => {
    setCards((prev) =>
      prev.map((c) => (c.id === id ? { ...c, selected: !c.selected } : c)),
    )
  }

  const selectedCards = cards.filter((c) => c.selected)
  const totalCost = selectedCards.reduce(
    (sum, c) => sum + parseFloat(c.costPerNode.replace(/[$,]/g, '')),
    0,
  )

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
        </p>
      </div>

      {/* Filters + Impact */}
      <div className="mt-6 mb-6 flex flex-wrap items-center justify-between gap-4">
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

      {/* Card grid */}
      <div className="grid gap-4 sm:grid-cols-2">
        {filtered.map((card) => (
          <div
            key={card.id}
            className={`rounded-lg p-5 transition-colors ${
              card.selected
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
                onClick={() => toggleSelect(card.id)}
                className={`flex-1 rounded py-2.5 text-[10px] font-bold tracking-widest transition-colors cursor-pointer ${
                  card.selected
                    ? 'bg-primary text-[#0a151a]'
                    : 'bg-surface-container-high text-primary hover:bg-primary/15'
                }`}
              >
                {card.selected ? t('component_upgrades.btn_selected') : t('component_upgrades.btn_request_upgrade')}
              </button>
              <button onClick={() => alert('Coming Soon')} className="rounded bg-surface-container-high px-4 py-2.5 text-[10px] font-bold tracking-widest text-on-surface-variant transition-colors hover:bg-surface-container-low cursor-pointer">
                {t('component_upgrades.btn_learn_more')}
              </button>
            </div>
          </div>
        ))}
      </div>

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
            onClick={() => alert('Coming Soon')}
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
