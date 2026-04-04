import { useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import Icon from '../components/Icon'
import StatusBadge from '../components/StatusBadge'
import { useAssets } from '../hooks/useAssets'
import type { Asset } from '../lib/api/assets'

const typeIcons: Record<string, string> = {
  server: 'dns',
  network: 'lan',
  storage: 'storage',
  ups: 'battery_charging_full',
}

const biaColors: Record<string, string> = {
  critical: 'bg-error-container text-on-error-container',
  important: 'bg-[#92400e] text-[#fbbf24]',
  normal: 'bg-surface-container-highest text-on-surface-variant',
  Critical: 'bg-error-container text-on-error-container',
  Important: 'bg-[#92400e] text-[#fbbf24]',
  Normal: 'bg-surface-container-highest text-on-surface-variant',
}

const statusDotColor: Record<string, string> = {
  operational: 'bg-green-400',
  maintenance: 'bg-yellow-400',
  offline: 'bg-red-400',
  Operational: 'bg-green-400',
  Maintenance: 'bg-yellow-400',
  Offline: 'bg-red-400',
}

const statusTextColor: Record<string, string> = {
  operational: 'text-green-400',
  maintenance: 'text-yellow-400',
  offline: 'text-red-400',
  Operational: 'text-green-400',
  Maintenance: 'text-yellow-400',
  Offline: 'text-red-400',
}

/* ------------------------------------------------------------------ */
/*  Card View Component                                                */
/* ------------------------------------------------------------------ */

function AssetCard({ asset, onClick }: { asset: Asset; onClick: () => void }) {
  const icon = typeIcons[asset.type?.toLowerCase()] ?? 'dns';
  return (
    <div
      onClick={onClick}
      className="group rounded-lg bg-surface-container p-5 transition-colors hover:bg-surface-container-high cursor-pointer relative"
    >
      {/* Header */}
      <div className="mb-3 flex items-start justify-between">
        <div>
          <h3 className="font-headline text-base font-bold text-on-surface">{asset.name}</h3>
          <div className="mt-1 flex items-center gap-2">
            <span className="rounded bg-surface-container-highest px-2 py-0.5 text-xs text-on-surface-variant">
              {asset.type}
            </span>
            <span className="flex items-center gap-1">
              <span className={`inline-block h-2 w-2 rounded-full ${statusDotColor[asset.status] ?? 'bg-gray-400'}`} />
              <span className={`text-xs font-semibold ${statusTextColor[asset.status] ?? 'text-gray-400'}`}>
                {asset.status}
              </span>
            </span>
          </div>
        </div>
        <Icon name={icon} className="text-[22px] text-on-surface-variant" />
      </div>

      {/* Info */}
      <div className="space-y-1 mb-3 text-xs text-on-surface-variant">
        <div>{asset.vendor} {asset.model}</div>
        <div className="font-mono">{asset.serial_number}</div>
      </div>

      {/* Footer info */}
      <div className="flex items-center justify-between text-xs text-on-surface-variant">
        <span className={`px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${biaColors[asset.bia_level] ?? ''}`}>
          {asset.bia_level}
        </span>
        <span className="font-mono text-[10px]">{asset.asset_tag}</span>
      </div>

      {/* Hover overlay */}
      <div className="absolute inset-0 flex items-center justify-center rounded-lg bg-surface/60 opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none">
        <span className="rounded-lg bg-on-primary-container px-4 py-2 text-xs font-semibold text-white shadow-lg">
          View Details
        </span>
      </div>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Main Page                                                          */
/* ------------------------------------------------------------------ */

export default function AssetManagementUnified() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  const [viewMode, setViewMode] = useState<'table' | 'card'>('table')
  const [search, setSearch] = useState('')
  const [typeFilter, setTypeFilter] = useState('All')
  const [statusFilter, setStatusFilter] = useState('All Status')
  const [locationFilter, setLocationFilter] = useState('All Locations')

  // Build query params from filter state
  const queryParams = useMemo(() => {
    const params: Record<string, string> = {}
    if (typeFilter !== 'All') params.type = typeFilter.toLowerCase()
    if (statusFilter !== 'All Status') params.status = statusFilter.toLowerCase()
    if (search) params.search = search
    return params
  }, [typeFilter, statusFilter, search])

  // Fetch assets from API
  const { data: apiData, isLoading, error, refetch } = useAssets(queryParams)
  const assets: Asset[] = apiData?.data ?? []
  const totalCount = apiData?.pagination?.total ?? assets.length

  const filtered = assets.filter((a) => {
    // Client-side location filter (not a backend param)
    if (locationFilter !== 'All Locations') return false // TODO: filter by location when backend supports it
    return true
  })

  return (
    <div className="min-h-screen bg-surface p-6 font-body text-on-surface">
      {/* Header */}
      <div className="mb-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="font-headline text-2xl font-bold tracking-tight text-on-surface">
              {t('assets.title_zh')} / {t('assets.title')}
            </h1>
            <p className="mt-1 text-sm text-on-surface-variant">
              {t('assets.subtitle')}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => navigate('/assets/lifecycle')}
              className="flex items-center gap-1.5 bg-surface-container-high px-3 py-2 text-xs font-semibold text-on-surface-variant rounded hover:bg-surface-container-highest hover:text-on-surface transition-all"
            >
              <Icon name="cycle" className="text-[16px]" />
              生命週期
            </button>
            <button
              onClick={() => navigate('/assets/discovery')}
              className="flex items-center gap-1.5 bg-surface-container-high px-3 py-2 text-xs font-semibold text-on-surface-variant rounded hover:bg-surface-container-highest hover:text-on-surface transition-all"
            >
              <Icon name="search" className="text-[16px]" />
              自動發現
            </button>
          </div>
        </div>
      </div>

      {/* Toolbar */}
      <div className="mb-4 flex flex-wrap items-center gap-3">
        {/* Search */}
        <div className="relative flex-1 min-w-[260px]">
          <Icon
            name="search"
            className="absolute left-3 top-1/2 -translate-y-1/2 text-on-surface-variant text-[20px]"
          />
          <input
            type="text"
            placeholder={t('assets.search_placeholder')}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full bg-surface-container-low py-2.5 pl-10 pr-4 text-sm text-on-surface placeholder:text-on-surface-variant/50 rounded focus:outline-none focus:ring-1 focus:ring-primary/40"
          />
        </div>

        {/* Type Filter */}
        <select
          value={typeFilter}
          onChange={(e) => setTypeFilter(e.target.value)}
          className="bg-surface-container-low py-2.5 px-3 text-sm text-on-surface rounded appearance-none cursor-pointer focus:outline-none focus:ring-1 focus:ring-primary/40"
        >
          <option value="All">{t('assets.all_types')}</option>
          <option value="Server">{t('assets.type_server')}</option>
          <option value="Network">{t('assets.type_network')}</option>
          <option value="Storage">{t('assets.type_storage')}</option>
          <option value="UPS">{t('assets.type_ups')}</option>
        </select>

        {/* Status Filter */}
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="bg-surface-container-low py-2.5 px-3 text-sm text-on-surface rounded appearance-none cursor-pointer focus:outline-none focus:ring-1 focus:ring-primary/40"
        >
          <option>{t('assets.all_status')}</option>
          <option>{t('common.operational')}</option>
          <option>{t('common.maintenance')}</option>
          <option>{t('common.offline')}</option>
        </select>

        {/* Location Filter */}
        <select
          value={locationFilter}
          onChange={(e) => setLocationFilter(e.target.value)}
          className="bg-surface-container-low py-2.5 px-3 text-sm text-on-surface rounded appearance-none cursor-pointer focus:outline-none focus:ring-1 focus:ring-primary/40"
        >
          <option>{t('assets.all_locations')}</option>
          <option>DC-1</option>
          <option>DC-2</option>
        </select>

        {/* View Toggle */}
        <div className="flex bg-surface-container-low rounded overflow-hidden">
          <button
            onClick={() => setViewMode('table')}
            className={`flex items-center gap-1.5 px-3 py-2 text-xs font-semibold transition-colors ${
              viewMode === 'table'
                ? 'bg-on-primary-container text-white'
                : 'text-on-surface-variant hover:bg-surface-container-high'
            }`}
          >
            <Icon name="table_rows" className="text-[16px]" />
            {t('assets.view_table') ?? '\u8868\u683c'}
          </button>
          <button
            onClick={() => setViewMode('card')}
            className={`flex items-center gap-1.5 px-3 py-2 text-xs font-semibold transition-colors ${
              viewMode === 'card'
                ? 'bg-on-primary-container text-white'
                : 'text-on-surface-variant hover:bg-surface-container-high'
            }`}
          >
            <Icon name="grid_view" className="text-[16px]" />
            {t('assets.view_card') ?? '\u5361\u7247'}
          </button>
        </div>

        {/* Spacer */}
        <div className="flex-1" />

        {/* Actions */}
        <button className="flex items-center gap-1.5 bg-on-primary-container px-4 py-2.5 text-sm font-semibold text-white rounded hover:brightness-110 transition-all">
          <Icon name="add" className="text-[18px]" />
          {t('assets.add_asset')}
        </button>
        <button className="flex items-center gap-1.5 bg-surface-container-high px-4 py-2.5 text-sm font-medium text-on-surface rounded hover:bg-surface-container-highest transition-all">
          <Icon name="upload" className="text-[18px]" />
          {t('common.import')}
        </button>
        <button className="flex items-center gap-1.5 bg-surface-container-high px-4 py-2.5 text-sm font-medium text-on-surface rounded hover:bg-surface-container-highest transition-all">
          <Icon name="download" className="text-[18px]" />
          {t('common.export_csv')}
        </button>
      </div>

      {/* ============================================================ */}
      {/*  TABLE VIEW                                                   */}
      {/* ============================================================ */}
      {viewMode === 'table' && (
        <div className="bg-surface-container rounded overflow-hidden">
          {/* Table Header */}
          <div className="grid grid-cols-[120px_1fr_100px_1fr_1fr_110px_120px_80px] items-center gap-2 bg-surface-container-low px-4 py-3 text-[0.6875rem] font-semibold uppercase tracking-wider text-on-surface-variant">
            <span>{t('assets.table_asset_no')}</span>
            <span>{t('assets.table_name')}</span>
            <span>{t('assets.table_type')}</span>
            <span>{t('assets.table_vendor_model')}</span>
            <span>{t('assets.table_location')}</span>
            <span>{t('assets.table_bia_level')}</span>
            <span>{t('assets.table_status')}</span>
            <span className="text-center">{t('assets.table_actions')}</span>
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
          {filtered.map((asset, i) => (
            <div
              key={asset.id}
              onClick={() => navigate(`/assets/${asset.id}`)}
              className={`grid grid-cols-[120px_1fr_100px_1fr_1fr_110px_120px_80px] items-center gap-2 px-4 py-3 text-sm transition-colors hover:bg-surface-container-high cursor-pointer ${
                i % 2 === 1 ? 'bg-surface-container-low/40' : ''
              }`}
            >
              <span className="font-mono text-primary text-xs font-semibold">
                {asset.asset_tag}
              </span>
              <span className="text-on-surface truncate">{asset.name}</span>
              <span className="flex items-center gap-1.5 text-on-surface-variant">
                <Icon name={typeIcons[asset.type?.toLowerCase()] ?? 'dns'} className="text-[18px]" />
                <span className="text-xs">{asset.type}</span>
              </span>
              <span className="text-on-surface-variant text-xs">
                {asset.vendor} {asset.model}
              </span>
              <span className="text-on-surface-variant text-xs font-mono">
                {asset.serial_number}
              </span>
              <span>
                <span
                  className={`px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${biaColors[asset.bia_level] ?? ''}`}
                >
                  {asset.bia_level}
                </span>
              </span>
              <span>
                <StatusBadge status={asset.status} />
              </span>
              <span className="flex items-center justify-center gap-1">
                <button className="p-1 rounded hover:bg-surface-container-highest transition-colors">
                  <Icon name="visibility" className="text-[18px] text-on-surface-variant" />
                </button>
                <button className="p-1 rounded hover:bg-surface-container-highest transition-colors">
                  <Icon name="more_vert" className="text-[18px] text-on-surface-variant" />
                </button>
              </span>
            </div>
          ))}
        </div>
      )}

      {/* ============================================================ */}
      {/*  CARD VIEW                                                    */}
      {/* ============================================================ */}
      {viewMode === 'card' && (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {isLoading && (
            <div className="col-span-full flex items-center justify-center py-10">
              <div className="animate-spin rounded-full h-6 w-6 border-2 border-sky-400 border-t-transparent" />
            </div>
          )}
          {filtered.map((asset) => (
            <AssetCard
              key={asset.id}
              asset={asset}
              onClick={() => navigate(`/assets/${asset.id}`)}
            />
          ))}
        </div>
      )}

      {/* Pagination */}
      <div className="mt-4 flex items-center justify-between text-sm text-on-surface-variant">
        <span>{t('common.showing')} 1-{filtered.length} {t('common.of')} {totalCount.toLocaleString()} {t('common.entries')}</span>
        <div className="flex items-center gap-1">
          <button className="px-3 py-1.5 rounded bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest transition-colors">
            <Icon name="chevron_left" className="text-[18px]" />
          </button>
          <button className="px-3 py-1.5 rounded bg-on-primary-container text-white text-xs font-semibold min-w-[32px]">
            1
          </button>
          <button className="px-3 py-1.5 rounded bg-surface-container-high text-on-surface-variant text-xs hover:bg-surface-container-highest transition-colors min-w-[32px]">
            2
          </button>
          <button className="px-3 py-1.5 rounded bg-surface-container-high text-on-surface-variant text-xs hover:bg-surface-container-highest transition-colors min-w-[32px]">
            3
          </button>
          <span className="px-2 text-on-surface-variant">...</span>
          <button className="px-3 py-1.5 rounded bg-surface-container-high text-on-surface-variant text-xs hover:bg-surface-container-highest transition-colors min-w-[32px]">
            1,606
          </button>
          <button className="px-3 py-1.5 rounded bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest transition-colors">
            <Icon name="chevron_right" className="text-[18px]" />
          </button>
        </div>
      </div>
    </div>
  )
}
