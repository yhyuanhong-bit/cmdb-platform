import { useState, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import Icon from '../components/Icon'
import { useDiscoveredAssets, useDiscoveryStats, useApproveAsset, useIgnoreAsset } from '../hooks/useDiscovery'
import ScanManagementTab from '../components/ScanManagementTab'

/* ------------------------------------------------------------------ */
/*  Types                                                              */
/* ------------------------------------------------------------------ */

interface DiscoveredAsset {
  id: string
  source: string
  external_id?: string
  hostname?: string
  ip_address?: string
  raw_data?: Record<string, any>
  status: string
  matched_asset_id?: string | null
  diff_details?: Record<string, any> | null
  discovered_at: string
  reviewed_by?: string | null
  reviewed_at?: string | null
}

interface DiscoveryStatsData {
  total: number
  pending: number
  conflict: number
  approved: number
  ignored: number
  matched: number
}

/* ------------------------------------------------------------------ */
/*  Source icon mapping                                                */
/* ------------------------------------------------------------------ */

const sourceIcon: Record<string, { icon: string; bg: string }> = {
  VMware:  { icon: 'cloud',           bg: 'bg-[#1e3a5f]' },
  SNMP:    { icon: 'router',          bg: 'bg-[#064e3b]' },
  IPMI:    { icon: 'developer_board', bg: 'bg-[#92400e]' },
  SSH:     { icon: 'terminal',        bg: 'bg-[#1a365d]' },
  manual:  { icon: 'upload_file',     bg: 'bg-[#4a1d6e]' },
}

/* ------------------------------------------------------------------ */
/*  Status badge                                                       */
/* ------------------------------------------------------------------ */

const statusStyle: Record<string, string> = {
  pending:  'bg-blue-500/20 text-blue-400',
  conflict: 'bg-amber-500/20 text-amber-400',
  approved: 'bg-emerald-500/20 text-emerald-400',
  ignored:  'bg-surface-container-highest text-on-surface-variant',
}

function StatusBadge({ status }: { status: string }) {
  return (
    <span className={`inline-block px-2.5 py-1 rounded text-[0.6875rem] font-semibold uppercase tracking-wider ${statusStyle[status] ?? statusStyle.pending}`}>
      {status}
    </span>
  )
}

/* ------------------------------------------------------------------ */
/*  Diff viewer                                                        */
/* ------------------------------------------------------------------ */

function DiffDetails({ diff }: { diff: Record<string, any> }) {
  const entries = Object.entries(diff)
  if (entries.length === 0) return null
  return (
    <div className="mt-2 space-y-1">
      {entries.map(([key, val]) => (
        <div key={key} className="flex items-center gap-2 text-xs">
          <span className="font-mono text-on-surface-variant">{key}:</span>
          <span className="text-red-400 line-through">{String((val as any)?.old ?? '')}</span>
          <Icon name="arrow_forward" className="text-[12px] text-on-surface-variant" />
          <span className="text-emerald-400">{String((val as any)?.new ?? '')}</span>
        </div>
      ))}
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Component                                                          */
/* ------------------------------------------------------------------ */

export default function AutoDiscovery() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  const [activeTab, setActiveTab] = useState<'review' | 'scan'>('review')
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [sourceFilter, setSourceFilter] = useState<string>('all')

  const queryParams: Record<string, string> = {}
  if (statusFilter !== 'all') queryParams.status = statusFilter

  const { data: listData, isLoading, error, refetch } = useDiscoveredAssets(queryParams)
  const { data: statsData } = useDiscoveryStats()
  const approveMutation = useApproveAsset()
  const ignoreMutation = useIgnoreAsset()

  const assets: DiscoveredAsset[] = (listData as any)?.data ?? []
  const stats: DiscoveryStatsData = (statsData as any)?.data ?? { total: 0, pending: 0, conflict: 0, approved: 0, ignored: 0, matched: 0 }

  /* Source filter is client-side since API only supports status */
  const filtered = assets.filter(a => {
    if (sourceFilter !== 'all' && a.source !== sourceFilter) return false
    return true
  })

  const handleApprove = useCallback((id: string) => {
    approveMutation.mutate(id)
  }, [approveMutation])

  const handleIgnore = useCallback((id: string) => {
    ignoreMutation.mutate(id)
  }, [ignoreMutation])

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      {/* ============================================================ */}
      {/*  Header                                                       */}
      {/* ============================================================ */}
      <header className="px-8 pt-6 pb-4">
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
        </div>
      </header>

      {/* ============================================================ */}
      {/*  Tab switcher                                                 */}
      {/* ============================================================ */}
      <div className="px-8 pb-2 flex gap-1">
        {(['review', 'scan'] as const).map(tab => (
          <button key={tab} onClick={() => setActiveTab(tab)}
            className={`px-4 py-2 rounded-lg text-sm font-semibold transition-colors ${
              activeTab === tab ? 'bg-primary text-on-primary' : 'text-on-surface-variant hover:bg-surface-container-high'
            }`}>
            {tab === 'review' ? t('auto_discovery.tab_review') : t('auto_discovery.tab_scan')}
          </button>
        ))}
      </div>

      {activeTab === 'review' && (<>

      {/* ============================================================ */}
      {/*  Stats row                                                    */}
      {/* ============================================================ */}
      <section className="px-8 pb-4">
        <div className="flex flex-wrap items-stretch gap-4">
          <StatCard label={t('auto_discovery.stat_total')} value={stats.total} icon="inventory_2" />
          <StatCard label={t('auto_discovery.stat_pending')} value={stats.pending} icon="pending_actions" color="text-blue-400" />
          <StatCard label={t('auto_discovery.stat_conflict')} value={stats.conflict} icon="warning" color="text-amber-400" />
          <StatCard label={t('auto_discovery.stat_approved')} value={stats.approved} icon="check_circle" color="text-emerald-400" />
          <StatCard label={t('auto_discovery.stat_ignored')} value={stats.ignored} icon="block" color="text-on-surface-variant" />
          <StatCard label={t('auto_discovery.stat_matched')} value={stats.matched} icon="link" color="text-primary" />
        </div>
      </section>

      {/* ============================================================ */}
      {/*  Filters                                                      */}
      {/* ============================================================ */}
      <section className="px-8 pb-4 flex flex-wrap items-center gap-3">
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
            <option value="SSH">SSH</option>
            <option value="IPMI">IPMI</option>
            <option value="manual">{t('auto_discovery.source_manual')}</option>
          </select>
          <Icon name="expand_more" className="absolute right-2 top-1/2 -translate-y-1/2 text-[16px] text-on-surface-variant pointer-events-none" />
        </div>

        <div className="relative">
          <select
            value={statusFilter}
            onChange={e => setStatusFilter(e.target.value)}
            className="appearance-none bg-surface-container-high text-on-surface text-sm rounded-lg pl-3 pr-8 py-2 outline-none cursor-pointer"
            aria-label="Filter by status"
          >
            <option value="all">{t('auto_discovery.filter_all_statuses')}</option>
            <option value="pending">{t('auto_discovery.status_pending')}</option>
            <option value="conflict">{t('auto_discovery.status_conflict')}</option>
            <option value="approved">{t('auto_discovery.status_approved')}</option>
            <option value="ignored">{t('auto_discovery.status_ignored')}</option>
          </select>
          <Icon name="expand_more" className="absolute right-2 top-1/2 -translate-y-1/2 text-[16px] text-on-surface-variant pointer-events-none" />
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
                <th className="px-4 py-3 text-left font-semibold">{t('auto_discovery.table_source')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('auto_discovery.table_hostname')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('auto_discovery.table_ip_address')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('auto_discovery.table_external_id')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('auto_discovery.table_status')}</th>
                <th className="px-4 py-3 text-left font-semibold">{t('auto_discovery.table_details')}</th>
                <th className="px-4 py-3 text-right font-semibold">{t('auto_discovery.table_actions')}</th>
              </tr>
            </thead>
            <tbody>
              {isLoading && (
                <tr><td colSpan={7} className="py-10 text-center">
                  <div className="inline-block animate-spin rounded-full h-6 w-6 border-2 border-sky-400 border-t-transparent" />
                </td></tr>
              )}
              {error && (
                <tr><td colSpan={7} className="py-4 text-center text-red-300 text-sm">
                  Failed to load discovered assets. <button onClick={() => refetch()} className="underline">Retry</button>
                </td></tr>
              )}
              {filtered.map(row => {
                const src = sourceIcon[row.source] ?? sourceIcon.SNMP
                return (
                  <tr key={row.id} className="bg-surface-container hover:bg-surface-container-high transition-colors border-t border-surface-container-high">
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <span className={`inline-flex items-center justify-center w-7 h-7 rounded-md ${src.bg}`}>
                          <Icon name={src.icon} className="text-[16px] text-on-surface" />
                        </span>
                        <span className="text-on-surface font-medium">{row.source}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-primary font-medium">{row.hostname ?? '-'}</td>
                    <td className="px-4 py-3 font-mono text-on-surface-variant">{row.ip_address ?? '-'}</td>
                    <td className="px-4 py-3 font-mono text-on-surface-variant text-xs">{row.external_id ?? '-'}</td>
                    <td className="px-4 py-3">
                      <StatusBadge status={row.status} />
                    </td>
                    <td className="px-4 py-3">
                      {row.status === 'conflict' && row.diff_details ? (
                        <DiffDetails diff={row.diff_details} />
                      ) : row.matched_asset_id ? (
                        <span className="text-xs text-on-surface-variant">{t('auto_discovery.matched_prefix')}: {row.matched_asset_id.slice(0, 8)}...</span>
                      ) : (
                        <span className="text-xs text-on-surface-variant">{t('auto_discovery.new_asset')}</span>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center justify-end gap-1">
                        {(row.status === 'pending' || row.status === 'conflict') && (
                          <>
                            <button
                              onClick={() => handleApprove(row.id)}
                              disabled={approveMutation.isPending}
                              className="p-1.5 rounded-md hover:bg-[#064e3b]/40 transition-colors"
                              aria-label={`Approve ${row.hostname}`}
                            >
                              <Icon name="check" className="text-[18px] text-[#34d399]" />
                            </button>
                            <button
                              onClick={() => handleIgnore(row.id)}
                              disabled={ignoreMutation.isPending}
                              className="p-1.5 rounded-md hover:bg-error-container/40 transition-colors"
                              aria-label={`Ignore ${row.hostname}`}
                            >
                              <Icon name="close" className="text-[18px] text-error" />
                            </button>
                          </>
                        )}
                      </div>
                    </td>
                  </tr>
                )
              })}
              {!isLoading && !error && filtered.length === 0 && (
                <tr><td colSpan={7} className="py-10 text-center text-on-surface-variant text-sm">
                  {t('auto_discovery.empty_state')}
                </td></tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      </>)}

      {activeTab === 'scan' && <ScanManagementTab />}
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Stat card sub-component                                            */
/* ------------------------------------------------------------------ */

function StatCard({ label, value, icon, color }: { label: string; value: number; icon: string; color?: string }) {
  return (
    <div className="bg-surface-container-low rounded-lg p-5 flex flex-col gap-2 min-w-[140px]">
      <div className="flex items-center justify-between">
        <span className="font-label text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant">
          {label}
        </span>
        <Icon name={icon} className={`text-[18px] ${color ?? 'text-on-surface-variant'}`} />
      </div>
      <div className={`font-headline font-bold text-2xl ${color ?? 'text-on-surface'}`}>{value}</div>
    </div>
  )
}
