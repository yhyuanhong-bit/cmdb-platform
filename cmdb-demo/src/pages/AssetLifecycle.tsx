import { toast } from 'sonner'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import Icon from '../components/Icon'
import StatusBadge from '../components/StatusBadge'
import { useAssets, useLifecycleStats } from '../hooks/useAssets'
import type { Asset } from '../lib/api/assets'

// SummaryCard interface removed — cards are now inline with API data

// Summary cards will be populated dynamically from stageCounts in the component

// (changeColors removed — no longer needed with API-driven cards)

interface LifecycleStage {
  label: string
  icon: string
  count: number
  color: string
}

const lifecycleStagesDef: Omit<LifecycleStage, 'count'>[] = [
  { label: 'Procurement', icon: 'shopping_cart', color: 'bg-primary' },
  { label: 'Deployment', icon: 'rocket_launch', color: 'bg-[#818cf8]' },
  { label: 'Active', icon: 'check_circle', color: 'bg-[#34d399]' },
  { label: 'Maintenance', icon: 'build', color: 'bg-[#fbbf24]' },
  { label: 'Decommission', icon: 'power_off', color: 'bg-[#fb923c]' },
  { label: 'Disposal', icon: 'delete', color: 'bg-error' },
]

// LifecycleAsset is now derived from API Asset

const stageColors: Record<string, string> = {
  Procurement: 'text-primary',
  Deployment: 'text-[#818cf8]',
  Active: 'text-[#34d399]',
  Maintenance: 'text-[#fbbf24]',
  Decommission: 'text-[#fb923c]',
  Disposal: 'text-error',
}


export default function AssetLifecycle() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  // Fetch assets from API to populate lifecycle table
  const { data: apiData, isLoading, error, refetch } = useAssets()

  // Fetch aggregated lifecycle/financial stats
  const { data: statsData } = useLifecycleStats()
  const lcStats = statsData?.data
  const assets: Asset[] = apiData?.data ?? []
  const totalAssets = apiData?.pagination?.total ?? assets.length

  // Group by status for lifecycle stages (map API status to lifecycle stage)
  const statusToStage: Record<string, string> = {
    procurement: 'Procurement',
    deploying: 'Deployment',
    active: 'Active',
    operational: 'Active',
    maintenance: 'Maintenance',
    decommission: 'Decommission',
    disposed: 'Disposal',
    offline: 'Decommission',
  }

  const stageCounts = useMemo(() => {
    const counts: Record<string, number> = {
      Procurement: 0, Deployment: 0, Active: 0, Maintenance: 0, Decommission: 0, Disposal: 0,
    }
    assets.forEach((a) => {
      const stage = statusToStage[a.status?.toLowerCase()] ?? 'Active'
      counts[stage] = (counts[stage] ?? 0) + 1
    })
    return counts
  }, [assets])

  const lifecycleStages: LifecycleStage[] = lifecycleStagesDef.map((s) => ({
    ...s,
    count: stageCounts[s.label] ?? 0,
  }))

  return (
    <div className="min-h-screen bg-surface p-6 font-body text-on-surface">
      {/* Header */}
      <div className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="font-headline text-2xl font-bold tracking-tight text-on-surface">
            {t('asset_lifecycle.title')}
          </h1>
          <p className="mt-1 text-sm text-on-surface-variant">
            {t('asset_lifecycle.subtitle')}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => navigate('/assets/lifecycle/timeline/' + (assets[0]?.id ?? ''))}
            className="flex items-center gap-1.5 bg-surface-container-high px-4 py-2.5 text-sm font-medium text-on-surface rounded hover:bg-surface-container-highest transition-all"
          >
            <Icon name="timeline" className="text-[18px]" />
            {t('asset_lifecycle.view_timeline', 'Timeline')}
          </button>
          <button onClick={() => toast.info('Coming Soon')} className="flex items-center gap-1.5 bg-surface-container-high px-4 py-2.5 text-sm font-medium text-on-surface rounded hover:bg-surface-container-highest transition-all">
            <Icon name="filter_list" className="text-[18px]" />
            {t('common.filters')}
          </button>
          <button onClick={() => toast.info('Coming Soon')} className="flex items-center gap-1.5 bg-on-primary-container px-4 py-2.5 text-sm font-semibold text-white rounded hover:brightness-110 transition-all">
            <Icon name="download" className="text-[18px]" />
            {t('common.export_report')}
          </button>
        </div>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        {[
          { labelKey: 'asset_lifecycle.total_assets', value: totalAssets.toLocaleString(), icon: 'inventory_2', changeType: 'neutral' as const },
          { labelKey: 'asset_lifecycle.active', value: (stageCounts['Active'] ?? 0).toLocaleString(), icon: 'check_circle', changeType: 'neutral' as const },
          { labelKey: 'asset_lifecycle.approaching_eol', value: (stageCounts['Decommission'] ?? 0).toLocaleString(), icon: 'warning', changeType: 'up' as const },
          { labelKey: 'asset_lifecycle.disposed', value: (stageCounts['Disposal'] ?? 0).toLocaleString(), icon: 'delete_sweep', changeType: 'neutral' as const },
        ].map((card) => (
          <div key={card.labelKey} className="bg-surface-container rounded p-5">
            <div className="flex items-center justify-between mb-3">
              <span className="text-[0.6875rem] font-semibold uppercase tracking-wider text-on-surface-variant">
                {t(card.labelKey)}
              </span>
              <Icon name={card.icon} className="text-[22px] text-primary" />
            </div>
            <div className="font-headline text-3xl font-bold text-on-surface">{card.value}</div>
          </div>
        ))}
      </div>

      {/* Lifecycle Pipeline Visualization */}
      <div className="bg-surface-container rounded p-5 mb-6">
        <h2 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant mb-5">
          {t('asset_lifecycle.lifecycle_pipeline')}
        </h2>
        <div className="flex items-center gap-0 overflow-x-auto pb-2">
          {lifecycleStages.map((stage, i) => (
            <div key={stage.label} className="flex items-center flex-shrink-0">
              <div className="flex flex-col items-center min-w-[130px]">
                {/* Stage circle */}
                <div
                  className={`w-14 h-14 rounded-full ${stage.color}/20 flex items-center justify-center mb-2`}
                >
                  <Icon
                    name={stage.icon}
                    className={`text-[28px] ${stage.color.replace('bg-', 'text-')}`}
                  />
                </div>
                <span className="text-xs font-semibold text-on-surface mb-0.5">{stage.label}</span>
                <span className="text-lg font-bold font-headline text-on-surface">
                  {stage.count.toLocaleString()}
                </span>
                {/* Proportion bar */}
                <div className="mt-2 w-20 h-1.5 rounded-full bg-surface-container-lowest overflow-hidden">
                  <div
                    className={`h-full rounded-full ${stage.color}`}
                    style={{
                      width: `${Math.max(8, (stage.count / 11205) * 100)}%`,
                    }}
                  />
                </div>
              </div>
              {/* Connector arrow */}
              {i < lifecycleStages.length - 1 && (
                <div className="flex items-center mx-1 -mt-8 flex-shrink-0">
                  <div className="w-8 h-px bg-on-surface-variant/30" />
                  <Icon name="chevron_right" className="text-[16px] text-on-surface-variant/40 -ml-1" />
                </div>
              )}
            </div>
          ))}
        </div>
      </div>

      {/* Assets Table */}
      <div className="bg-surface-container rounded overflow-hidden mb-6">
        <div className="px-5 py-4 bg-surface-container-low">
          <h2 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
            {t('asset_lifecycle.asset_lifecycle_status')}
          </h2>
        </div>
        {/* Table Header */}
        <div className="grid grid-cols-[110px_1fr_90px_120px_110px_110px_110px_110px] items-center gap-2 px-5 py-3 bg-surface-container-low text-[0.6875rem] font-semibold uppercase tracking-wider text-on-surface-variant">
          <span>{t('asset_lifecycle.table_asset_id')}</span>
          <span>{t('asset_lifecycle.table_name')}</span>
          <span>{t('asset_lifecycle.table_type')}</span>
          <span>{t('asset_lifecycle.table_stage')}</span>
          <span>{t('asset_lifecycle.table_procured')}</span>
          <span>{t('asset_lifecycle.table_deployed')}</span>
          <span>{t('asset_lifecycle.table_eol_date')}</span>
          <span>{t('asset_lifecycle.table_status')}</span>
        </div>
        {/* Loading / Error */}
        {isLoading && (
          <div className="flex items-center justify-center py-10">
            <div className="animate-spin rounded-full h-6 w-6 border-2 border-sky-400 border-t-transparent" />
          </div>
        )}
        {error && (
          <div className="p-4 text-red-300 text-sm">
            Failed to load assets.{' '}
            <button onClick={() => refetch()} className="underline">Retry</button>
          </div>
        )}
        {/* Rows */}
        {assets.map((asset, i) => {
          const stage = statusToStage[asset.status?.toLowerCase()] ?? 'Active'
          return (
            <div
              key={asset.id}
              onClick={() => navigate(`/assets/${asset.id}`)}
              className={`grid grid-cols-[110px_1fr_90px_120px_110px_110px_110px_110px] items-center gap-2 px-5 py-3 text-sm transition-colors hover:bg-surface-container-high cursor-pointer ${
                i % 2 === 1 ? 'bg-surface-container-low/40' : ''
              }`}
            >
              <span className="font-mono text-primary text-xs font-semibold">{asset.asset_tag}</span>
              <span className="text-on-surface truncate">{asset.name}</span>
              <span className="text-on-surface-variant text-xs">{asset.type}</span>
              <span className={`text-xs font-semibold uppercase tracking-wider ${stageColors[stage] ?? 'text-on-surface-variant'}`}>
                {stage}
              </span>
              <span className="text-on-surface-variant text-xs font-mono">{asset.created_at?.slice(0, 10) ?? '-'}</span>
              <span className="text-on-surface-variant text-xs font-mono">{(asset.attributes?.deployed_date as string) ?? '-'}</span>
              <span className="text-on-surface-variant text-xs font-mono">{(asset.attributes?.eol_date as string) ?? '-'}</span>
              <span>
                <StatusBadge status={asset.status} />
              </span>
            </div>
          )
        })}
      </div>

      {/* Financial & Warranty Summary */}
      <div className="bg-surface-container rounded p-5">
        <div className="flex items-center gap-2 mb-5">
          <Icon name="account_balance" className="text-[22px] text-primary" />
          <h2 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
            {t('asset_lifecycle.financial_summary')}
          </h2>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {[
            {
              labelKey: 'asset_lifecycle.total_asset_value',
              value: lcStats?.total_purchase_cost != null
                ? `$${Number(lcStats.total_purchase_cost).toLocaleString()}`
                : '—',
              subtextKey: 'asset_lifecycle.current_book_value',
            },
            {
              labelKey: 'asset_lifecycle.under_warranty',
              value: lcStats != null ? `${lcStats.warranty_active_count} assets` : '—',
              subtextKey: 'asset_lifecycle.active_coverage',
            },
            {
              labelKey: 'asset_lifecycle.warranty_expired',
              value: lcStats != null ? `${lcStats.warranty_expired_count} assets` : '—',
              subtextKey: 'asset_lifecycle.coverage_lapsed',
            },
            {
              labelKey: 'asset_lifecycle.approaching_eol',
              value: lcStats != null ? `${lcStats.approaching_eol_count} assets` : '—',
              subtextKey: 'asset_lifecycle.within_6_months',
            },
          ].map((item) => (
            <div key={item.labelKey} className="bg-surface-container-low rounded p-4">
              <span className="text-[0.6875rem] font-semibold uppercase tracking-wider text-on-surface-variant">
                {t(item.labelKey, item.labelKey.split('.').pop() ?? item.labelKey)}
              </span>
              <div className="font-headline text-2xl font-bold text-on-surface mt-2">
                {item.value}
              </div>
              <p className="text-[0.6875rem] text-on-surface-variant mt-1">
                {t(item.subtextKey, item.subtextKey.split('.').pop() ?? item.subtextKey)}
              </p>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
