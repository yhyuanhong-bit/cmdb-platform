import { useState, useCallback, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import Icon from '../components/Icon'
import StatusBadge from '../components/StatusBadge'
import { useAssets } from '../hooks/useAssets'
import type { Asset } from '../lib/api/assets'

/* ------------------------------------------------------------------ */
/*  Types                                                              */
/* ------------------------------------------------------------------ */

type SourceType = 'VMware' | 'SNMP' | 'IPMI'
type DiscoveryStatus = 'Pending' | 'Approved' | 'Ignored'

interface DiscoveryRow {
  id: string
  source: SourceType
  name: string
  ip: string
  sn: string
  vendor: string
  model: string
  status: DiscoveryStatus
}

/* ------------------------------------------------------------------ */
/*  Map API assets to discovery rows                                   */
/* ------------------------------------------------------------------ */

function assetToDiscoveryRow(a: Asset): DiscoveryRow {
  return {
    id: a.id,
    source: ((a.attributes?.source as string) ?? 'SNMP') as SourceType,
    name: a.name,
    ip: (a.attributes?.ip as string) ?? '-',
    sn: a.serial_number,
    vendor: a.vendor,
    model: a.model,
    status: ((a.attributes?.discovery_status as string) ?? 'Pending') as DiscoveryStatus,
  }
}

/* ------------------------------------------------------------------ */
/*  Source icon mapping                                                */
/* ------------------------------------------------------------------ */

const sourceIcon: Record<SourceType, { icon: string; bg: string }> = {
  VMware: { icon: 'cloud',           bg: 'bg-[#1e3a5f]' },
  SNMP:   { icon: 'router',          bg: 'bg-[#064e3b]' },
  IPMI:   { icon: 'developer_board', bg: 'bg-[#92400e]' },
}

/* ------------------------------------------------------------------ */
/*  Status styling                                                     */
/* ------------------------------------------------------------------ */

const statusStyle: Record<DiscoveryStatus, string> = {
  Pending:  'bg-[#92400e]/20 text-[#fbbf24]',
  Approved: 'bg-[#064e3b]/20 text-[#34d399]',
  Ignored:  'bg-surface-container-highest text-on-surface-variant',
}

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export default function AutoDiscovery() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [sourceFilter, setSourceFilter] = useState<string>('all')
  const [statusFilter, setStatusFilter] = useState<string>('all')

  // Fetch existing assets from API
  const { data: apiData, isLoading, error, refetch } = useAssets()
  const discoveryData: DiscoveryRow[] = useMemo(
    () => (apiData?.data ?? []).map(assetToDiscoveryRow),
    [apiData],
  )

  /* Selection helpers */
  const allSelected = selected.size === discoveryData.length && discoveryData.length > 0
  const toggleAll = useCallback(() => {
    setSelected(prev =>
      prev.size === discoveryData.length
        ? new Set()
        : new Set(discoveryData.map(r => r.id)),
    )
  }, [discoveryData])
  const toggleRow = useCallback((id: string) => {
    setSelected(prev => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })
  }, [])

  /* Filtered data */
  const filtered = discoveryData.filter(r => {
    if (sourceFilter !== 'all' && r.source !== sourceFilter) return false
    if (statusFilter !== 'all' && r.status !== statusFilter) return false
    return true
  })

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      {/* ============================================================ */}
      {/*  Header                                                       */}
      {/* ============================================================ */}
      <header className="px-8 pt-6 pb-4">
        {/* Breadcrumb */}
        <nav className="flex items-center gap-1.5 text-xs text-on-surface-variant mb-3">
          <span className="hover:text-primary cursor-pointer" onClick={() => navigate('/assets')}>{t('auto_discovery.breadcrumb_asset_management')}</span>
          <Icon name="chevron_right" className="text-[14px] text-on-surface-variant" />
          <span className="text-primary">{t('auto_discovery.breadcrumb_auto_discovery')}</span>
        </nav>

        <div className="flex items-start justify-between flex-wrap gap-4">
          <div>
            <h1 className="font-headline font-bold text-2xl text-on-surface">
              {t('auto_discovery.title')}
            </h1>
            <p className="text-sm text-on-surface-variant mt-1">
              {t('auto_discovery.subtitle')}
            </p>
          </div>

          <div className="flex items-center gap-3">
            <button
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-surface-container-high text-on-surface-variant text-sm font-medium hover:bg-surface-container-highest transition-colors"
              aria-label="Ignore Selected"
            >
              <Icon name="close" className="text-[18px]" />
              {t('auto_discovery.btn_ignore_selected')}
            </button>
            <button
              className="flex items-center gap-2 px-4 py-2 rounded-lg machined-gradient text-[#001b34] text-sm font-semibold hover:opacity-90 transition-opacity"
              aria-label="Batch Approve"
            >
              <Icon name="check" className="text-[18px]" />
              {t('auto_discovery.btn_batch_approve')}
            </button>
          </div>
        </div>
      </header>

      {/* ============================================================ */}
      {/*  Stats row                                                    */}
      {/* ============================================================ */}
      <section className="px-8 pb-4">
        <div className="flex flex-wrap items-stretch gap-4">
          {/* Stat: 新增未審核 */}
          <div className="bg-surface-container-low rounded-lg p-5 flex flex-col gap-2 min-w-[180px]">
            <div className="flex items-center justify-between">
              <span className="font-label text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant">
                {t('auto_discovery.stat_new_unreviewed')}
              </span>
              <Icon name="new_releases" className="text-on-surface-variant text-[18px]" />
            </div>
            <div className="font-headline font-bold text-2xl text-on-surface">12</div>
            <span className="text-xs text-on-primary-container">{t('auto_discovery.stat_needs_manual_confirm')}</span>
          </div>

          {/* Stat: 待確認 (PENDING) */}
          <div className="bg-surface-container-low rounded-lg p-5 flex flex-col gap-2 min-w-[180px]">
            <div className="flex items-center justify-between">
              <span className="font-label text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant">
                {t('auto_discovery.stat_pending')}
              </span>
              <Icon name="pending_actions" className="text-on-surface-variant text-[18px]" />
            </div>
            <div className="font-headline font-bold text-2xl text-on-surface">1,284</div>
            <span className="text-xs text-[#fbbf24]">{t('auto_discovery.stat_awaiting_batch')}</span>
          </div>

          {/* Status banner */}
          <div className="bg-surface-container-low rounded-lg p-5 flex flex-col justify-between flex-1 min-w-[340px]">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Icon name="sync" className="text-[18px] text-primary animate-spin" />
                <span className="text-sm font-semibold text-on-surface">
                  {t('auto_discovery.sync_status')} — {t('auto_discovery.sync_accuracy')}
                </span>
              </div>
              <div className="flex items-center gap-1.5 text-xs text-[#34d399]">
                <span className="w-2 h-2 rounded-full bg-[#34d399] inline-block" />
                {t('auto_discovery.service_pulse')}
              </div>
            </div>
            {/* Progress bar */}
            <div className="mt-3">
              <div className="h-1.5 rounded-full bg-surface-container-highest overflow-hidden">
                <div
                  className="h-full rounded-full machined-gradient transition-all duration-500"
                  style={{ width: '98.2%' }}
                />
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ============================================================ */}
      {/*  Filters                                                      */}
      {/* ============================================================ */}
      <section className="px-8 pb-4 flex flex-wrap items-center gap-3">
        {/* Source filter */}
        <div className="relative">
          <select
            value={sourceFilter}
            onChange={e => setSourceFilter(e.target.value)}
            className="appearance-none bg-surface-container-high text-on-surface text-sm rounded-lg pl-3 pr-8 py-2 outline-none cursor-pointer"
            aria-label="Filter by source"
          >
            <option value="all">{t('auto_discovery.filter_all_sources')}</option>
            <option value="VMware">VMware</option>
            <option value="SNMP">SNMP</option>
            <option value="IPMI">IPMI</option>
          </select>
          <Icon
            name="expand_more"
            className="absolute right-2 top-1/2 -translate-y-1/2 text-[16px] text-on-surface-variant pointer-events-none"
          />
        </div>

        {/* Status filter */}
        <div className="relative">
          <select
            value={statusFilter}
            onChange={e => setStatusFilter(e.target.value)}
            className="appearance-none bg-surface-container-high text-on-surface text-sm rounded-lg pl-3 pr-8 py-2 outline-none cursor-pointer"
            aria-label="Filter by status"
          >
            <option value="all">{t('auto_discovery.filter_all_statuses')}</option>
            <option value="Pending">Pending</option>
            <option value="Approved">Approved</option>
            <option value="Ignored">Ignored</option>
          </select>
          <Icon
            name="expand_more"
            className="absolute right-2 top-1/2 -translate-y-1/2 text-[16px] text-on-surface-variant pointer-events-none"
          />
        </div>

        <div className="ml-auto text-xs text-on-surface-variant">
          {t('auto_discovery.showing_results', { count: filtered.length })}
        </div>
      </section>

      {/* ============================================================ */}
      {/*  Discovery table                                              */}
      {/* ============================================================ */}
      <section className="px-8 pb-4">
        <div className="bg-surface-container rounded-lg overflow-x-auto" role="table" aria-label="Discovery results">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-surface-container-high text-on-surface-variant text-[0.6875rem] uppercase tracking-wider">
                <th className="pl-4 py-3 w-10 text-left">
                  <input
                    type="checkbox"
                    checked={allSelected}
                    onChange={toggleAll}
                    className="accent-primary w-4 h-4 cursor-pointer"
                    aria-label="Select all rows"
                  />
                </th>
                <th className="px-4 py-3 text-left font-semibold">{t('auto_discovery.table_source')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('auto_discovery.table_name')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('auto_discovery.table_ip_address')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('auto_discovery.table_serial_number')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('auto_discovery.table_vendor')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('auto_discovery.table_model')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('auto_discovery.table_status')}</th>
                <th className="px-4 py-3 text-right font-semibold">{t('auto_discovery.table_actions')}</th>
              </tr>
            </thead>
            <tbody>
              {isLoading && (
                <tr><td colSpan={9} className="py-10 text-center">
                  <div className="inline-block animate-spin rounded-full h-6 w-6 border-2 border-sky-400 border-t-transparent" />
                </td></tr>
              )}
              {error && (
                <tr><td colSpan={9} className="py-4 text-center text-red-300 text-sm">
                  Failed to load assets. <button onClick={() => refetch()} className="underline">Retry</button>
                </td></tr>
              )}
              {filtered.map(row => {
                const src = sourceIcon[row.source]
                return (
                  <tr
                    key={row.id}
                    className="bg-surface-container hover:bg-surface-container-high transition-colors"
                  >
                    <td className="pl-4 py-3">
                      <input
                        type="checkbox"
                        checked={selected.has(row.id)}
                        onChange={() => toggleRow(row.id)}
                        className="accent-primary w-4 h-4 cursor-pointer"
                        aria-label={`Select ${row.name}`}
                      />
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <span className={`inline-flex items-center justify-center w-7 h-7 rounded-md ${src.bg}`}>
                          <Icon name={src.icon} className="text-[16px] text-on-surface" />
                        </span>
                        <span className="text-on-surface font-medium">{row.source}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-primary font-medium">{row.name}</td>
                    <td className="px-4 py-3 font-mono text-on-surface-variant">{row.ip}</td>
                    <td className="px-4 py-3 font-mono text-on-surface-variant">{row.sn}</td>
                    <td className="px-4 py-3 text-on-surface">{row.vendor}</td>
                    <td className="px-4 py-3 text-on-surface">{row.model}</td>
                    <td className="px-4 py-3">
                      <span
                        className={`inline-block px-2.5 py-1 rounded text-[0.6875rem] font-semibold uppercase tracking-wider ${statusStyle[row.status]}`}
                      >
                        {row.status}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center justify-end gap-1">
                        <button
                          className="p-1.5 rounded-md hover:bg-[#064e3b]/40 transition-colors"
                          aria-label={`Approve ${row.name}`}
                        >
                          <Icon name="check" className="text-[18px] text-[#34d399]" />
                        </button>
                        <button
                          className="p-1.5 rounded-md hover:bg-error-container/40 transition-colors"
                          aria-label={`Ignore ${row.name}`}
                        >
                          <Icon name="close" className="text-[18px] text-error" />
                        </button>
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      </section>

      {/* ============================================================ */}
      {/*  Pagination                                                   */}
      {/* ============================================================ */}
      <section className="px-8 pb-6 flex items-center justify-between text-xs text-on-surface-variant">
        <span>{t('auto_discovery.pagination_showing')}</span>
        <div className="flex items-center gap-1">
          <button
            className="px-3 py-1.5 rounded-md bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest transition-colors"
            disabled
            aria-label="Previous page"
          >
            <Icon name="chevron_left" className="text-[16px]" />
          </button>
          <button className="px-3 py-1.5 rounded-md machined-gradient text-[#001b34] font-semibold" aria-current="page">
            1
          </button>
          <button className="px-3 py-1.5 rounded-md bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest transition-colors">
            2
          </button>
          <button className="px-3 py-1.5 rounded-md bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest transition-colors">
            3
          </button>
          <span className="px-2 text-on-surface-variant">...</span>
          <button className="px-3 py-1.5 rounded-md bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest transition-colors">
            253
          </button>
          <button
            className="px-3 py-1.5 rounded-md bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest transition-colors"
            aria-label="Next page"
          >
            <Icon name="chevron_right" className="text-[16px]" />
          </button>
        </div>
      </section>

      {/* ============================================================ */}
      {/*  Bottom panels                                                */}
      {/* ============================================================ */}
      <section className="px-8 pb-8">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {/* Left: Smart Insights */}
          <div className="bg-surface-container rounded-lg p-6">
            <h2 className="font-headline font-bold text-lg text-on-surface mb-4 flex items-center gap-2">
              <Icon name="tips_and_updates" className="text-[20px] text-primary" />
              {t('auto_discovery.section_smart_insights')}
            </h2>

            <div className="flex flex-col gap-3">
              {/* Insight card 1 */}
              <div className="bg-surface-container-high rounded-lg p-4">
                <div className="flex items-start gap-3">
                  <span className="mt-0.5 inline-flex items-center justify-center w-8 h-8 rounded-md bg-[#92400e]/30">
                    <Icon name="merge_type" className="text-[18px] text-[#fbbf24]" />
                  </span>
                  <div>
                    <h3 className="text-sm font-semibold text-on-surface">{t('auto_discovery.insight_duplicates_title')}</h3>
                    <p className="text-xs text-on-surface-variant mt-1 leading-relaxed">
                      {t('auto_discovery.insight_duplicates_desc')}
                    </p>
                  </div>
                </div>
              </div>

              {/* Insight card 2 */}
              <div className="bg-surface-container-high rounded-lg p-4">
                <div className="flex items-start gap-3">
                  <span className="mt-0.5 inline-flex items-center justify-center w-8 h-8 rounded-md bg-[#1e3a5f]/60">
                    <Icon name="help_outline" className="text-[18px] text-primary" />
                  </span>
                  <div>
                    <h3 className="text-sm font-semibold text-on-surface">{t('auto_discovery.insight_unknown_model_title')}</h3>
                    <p className="text-xs text-on-surface-variant mt-1 leading-relaxed">
                      {t('auto_discovery.insight_unknown_model_desc')}
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Right: Schedule */}
          <div className="bg-surface-container rounded-lg p-6">
            <h2 className="font-headline font-bold text-lg text-on-surface mb-4 flex items-center gap-2">
              <Icon name="schedule" className="text-[20px] text-primary" />
              {t('auto_discovery.section_schedule_title')}
            </h2>

            <p className="text-sm text-on-surface-variant leading-relaxed mb-5">
              {t('auto_discovery.section_schedule_desc')}
            </p>

            <button className="flex items-center gap-2 px-5 py-2.5 rounded-lg machined-gradient text-[#001b34] text-sm font-semibold hover:opacity-90 transition-opacity">
              <Icon name="event_note" className="text-[18px]" />
              {t('auto_discovery.btn_manage_schedule')}
            </button>
          </div>
        </div>
      </section>
    </div>
  )
}
