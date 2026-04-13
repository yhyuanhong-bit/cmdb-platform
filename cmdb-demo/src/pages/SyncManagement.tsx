import { useState } from 'react'
import { toast } from 'sonner'
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, Cell } from 'recharts'
import { usePermission } from '../hooks/usePermission'
import { useSyncState, useSyncConflicts, useResolveConflict, useSyncStats } from '../hooks/useSync'
import type { SyncConflict } from '../lib/api/sync'

function formatTimeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h ago`
  return `${Math.floor(hours / 24)}d ago`
}

function syncStatusColor(status: string, lastSyncAt: string): { color: string; label: string } {
  if (status === 'error') return { color: 'bg-red-500', label: 'Error' }
  const hoursSince = (Date.now() - new Date(lastSyncAt).getTime()) / 3600000
  if (hoursSince > 24) return { color: 'bg-red-500', label: 'Error' }
  if (hoursSince > 1) return { color: 'bg-yellow-500', label: 'Lag' }
  return { color: 'bg-emerald-500', label: 'OK' }
}

export default function SyncManagement() {
  const [activeTab, setActiveTab] = useState<'status' | 'conflicts'>('status')
  const canResolve = usePermission('sync', 'write')

  return (
    <div className="p-6 max-w-6xl mx-auto">
      <h1 className="text-2xl font-bold text-on-surface mb-6">Sync Management</h1>

      <div className="flex gap-1 mb-6">
        {(['status', 'conflicts'] as const).map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-4 py-2 rounded-lg text-sm font-semibold transition-colors ${
              activeTab === tab
                ? 'bg-surface-container-high text-on-surface'
                : 'text-on-surface-variant hover:bg-surface-container'
            }`}
          >
            {tab === 'status' ? 'Sync Status' : 'Conflicts'}
          </button>
        ))}
      </div>

      {activeTab === 'status' && <SyncStatusTab />}
      {activeTab === 'conflicts' && <ConflictsTab canResolve={canResolve} />}
    </div>
  )
}

function SummaryCards({ states, conflictCount }: { states: any[]; conflictCount: number }) {
  const uniqueNodes = new Set(states.map((s: any) => s.node_id))
  const okCount = states.filter((s: any) => {
    if (s.status === 'error') return false
    const hoursSince = (Date.now() - new Date(s.last_sync_at).getTime()) / 3600000
    return hoursSince <= 1
  }).length
  const lagCount = states.filter((s: any) => {
    if (s.status === 'error') return false
    const hoursSince = (Date.now() - new Date(s.last_sync_at).getTime()) / 3600000
    return hoursSince > 1 && hoursSince <= 24
  }).length
  const errorCount = states.filter((s: any) => {
    if (s.status === 'error') return true
    const hoursSince = (Date.now() - new Date(s.last_sync_at).getTime()) / 3600000
    return hoursSince > 24
  }).length

  return (
    <div className="grid grid-cols-3 gap-4 mb-6">
      <div className="bg-surface-container rounded-lg p-4 text-center">
        <div className="text-2xl font-bold text-on-surface">{uniqueNodes.size}</div>
        <div className="text-xs text-on-surface-variant mt-1">Total Nodes</div>
      </div>
      <div className="bg-surface-container rounded-lg p-4 text-center">
        <div className="text-sm font-semibold text-on-surface">
          <span className="text-emerald-500">{okCount} OK</span>
          {lagCount > 0 && <span className="text-yellow-500 ml-2">{lagCount} Lag</span>}
          {errorCount > 0 && <span className="text-red-500 ml-2">{errorCount} Error</span>}
        </div>
        <div className="text-xs text-on-surface-variant mt-1">Sync Health</div>
      </div>
      <div className="bg-surface-container rounded-lg p-4 text-center">
        <div className="text-sm font-semibold text-on-surface">
          {conflictCount} conflicts · {errorCount} errors
        </div>
        <div className="text-xs text-on-surface-variant mt-1">Pending</div>
      </div>
    </div>
  )
}

function VersionGapChart({ stats }: { stats: any[] }) {
  const chartData = stats
    .map((s: any) => ({
      entity_type: s.entity_type.replace(/_/g, ' '),
      gap: Math.max(0, ...s.nodes.map((n: any) => n.gap), 0),
    }))
    .filter((d: any) => d.gap > 0)

  if (chartData.length === 0) {
    return (
      <div className="bg-surface-container rounded-lg p-4 mb-6 text-center text-on-surface-variant text-sm">
        All nodes are up to date — no version gaps.
      </div>
    )
  }

  return (
    <div className="bg-surface-container rounded-lg p-4 mb-6">
      <h3 className="text-sm font-bold text-on-surface mb-3">Version Gap by Entity Type</h3>
      <ResponsiveContainer width="100%" height={200}>
        <BarChart data={chartData}>
          <XAxis dataKey="entity_type" tick={{ fontSize: 11 }} />
          <YAxis tick={{ fontSize: 11 }} />
          <Tooltip />
          <Bar dataKey="gap" fill="#f59e0b" radius={[4, 4, 0, 0]}>
            {chartData.map((_: any, i: number) => (
              <Cell key={i} fill={chartData[i].gap > 50 ? '#ef4444' : '#f59e0b'} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}

function ErrorList({ states }: { states: any[] }) {
  const errors = states.filter((s: any) => s.status === 'error' || (Date.now() - new Date(s.last_sync_at).getTime()) / 3600000 > 24)

  if (errors.length === 0) return null

  return (
    <div className="mt-6">
      <h3 className="text-sm font-bold text-on-surface mb-3">Sync Errors</h3>
      <div className="space-y-2">
        {errors.map((s: any) => (
          <div key={`${s.node_id}-${s.entity_type}`} className="bg-red-500/10 border border-red-500/20 rounded-lg p-3">
            <div className="text-sm font-semibold text-on-surface">
              {s.node_id} / {s.entity_type}
            </div>
            <div className="text-xs text-on-surface-variant mt-1">
              {s.error_message || `Last sync: ${formatTimeAgo(s.last_sync_at)}`}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

function SyncStatusTab() {
  const { data: stateResp, isLoading: stateLoading } = useSyncState()
  const { data: conflictsResp } = useSyncConflicts()
  const { data: statsResp, isLoading: statsLoading } = useSyncStats()
  const states = (stateResp as any)?.data ?? []
  const conflicts = (conflictsResp as any)?.data ?? []
  const stats = (statsResp as any)?.data ?? []

  if (stateLoading) {
    return <div className="text-on-surface-variant">Loading sync state...</div>
  }

  if (states.length === 0) {
    return (
      <div className="bg-surface-container rounded-lg p-8 text-center text-on-surface-variant">
        No sync nodes registered yet.
      </div>
    )
  }

  const byNode: Record<string, typeof states> = {}
  for (const s of states) {
    if (!byNode[s.node_id]) byNode[s.node_id] = []
    byNode[s.node_id].push(s)
  }

  return (
    <div>
      <SummaryCards states={states} conflictCount={conflicts.length} />

      {!statsLoading && stats.length > 0 && <VersionGapChart stats={stats} />}

      <div className="space-y-4">
        {Object.entries(byNode).map(([nodeId, nodeStates]) => (
          <div key={nodeId} className="bg-surface-container rounded-lg p-4">
            <h3 className="text-sm font-bold text-on-surface mb-3 uppercase tracking-wide">{nodeId}</h3>
            <div className="grid grid-cols-[1fr_80px_100px_60px] gap-2 text-sm">
              <div className="text-on-surface-variant font-semibold">Entity</div>
              <div className="text-on-surface-variant font-semibold">Version</div>
              <div className="text-on-surface-variant font-semibold">Last Sync</div>
              <div className="text-on-surface-variant font-semibold">Status</div>
              {(nodeStates as any[]).map((s: any) => {
                const { color, label } = syncStatusColor(s.status, s.last_sync_at)
                return (
                  <div key={s.entity_type} className="contents">
                    <div className="text-on-surface">{s.entity_type}</div>
                    <div className="text-on-surface">{s.last_sync_version}</div>
                    <div className="text-on-surface-variant">{formatTimeAgo(s.last_sync_at)}</div>
                    <div className="flex items-center gap-1.5">
                      <span className={`inline-block w-2 h-2 rounded-full ${color}`} />
                      <span className="text-on-surface-variant">{label}</span>
                    </div>
                  </div>
                )
              })}
            </div>
            {(nodeStates as any[]).some((s: any) => s.error_message) && (
              <div className="mt-3 text-xs text-red-400">
                {(nodeStates as any[]).filter((s: any) => s.error_message).map((s: any) => (
                  <div key={s.entity_type}>{s.entity_type}: {s.error_message}</div>
                ))}
              </div>
            )}
          </div>
        ))}
      </div>

      <ErrorList states={states} />
    </div>
  )
}

function ConflictsTab({ canResolve }: { canResolve: boolean }) {
  const { data: resp, isLoading } = useSyncConflicts()
  const conflicts: SyncConflict[] = (resp as any)?.data ?? []
  const [selectedConflict, setSelectedConflict] = useState<SyncConflict | null>(null)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [filterType, setFilterType] = useState<string>('')
  const resolveConflict = useResolveConflict()

  const filtered = filterType
    ? conflicts.filter((c) => c.entity_type === filterType)
    : conflicts

  const entityTypes = [...new Set(conflicts.map((c) => c.entity_type))]

  const toggleSelect = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleAll = () => {
    if (selectedIds.size === filtered.length) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(filtered.map((c) => c.id)))
    }
  }

  const batchResolve = async (resolution: 'local_wins' | 'remote_wins') => {
    const promises = [...selectedIds].map((id) =>
      resolveConflict.mutateAsync({ id, resolution })
    )
    await Promise.all(promises)
    setSelectedIds(new Set())
    toast.success(`Resolved ${promises.length} conflicts as ${resolution.replace('_', ' ')}`)
  }

  const handleResolve = async (id: string, resolution: 'local_wins' | 'remote_wins') => {
    await resolveConflict.mutateAsync({ id, resolution })
    setSelectedConflict(null)
    toast.success('Conflict resolved')
  }

  if (isLoading) {
    return <div className="text-on-surface-variant">Loading conflicts...</div>
  }

  if (conflicts.length === 0) {
    return (
      <div className="bg-surface-container rounded-lg p-8 text-center text-on-surface-variant">
        No pending conflicts. Sync is running smoothly.
      </div>
    )
  }

  return (
    <div>
      <div className="flex items-center gap-3 mb-4">
        <select
          value={filterType}
          onChange={(e) => setFilterType(e.target.value)}
          className="bg-surface-container rounded-lg px-3 py-2 text-sm text-on-surface"
        >
          <option value="">All types</option>
          {entityTypes.map((t) => (
            <option key={t} value={t}>{t}</option>
          ))}
        </select>
        {canResolve && selectedIds.size > 0 && (
          <div className="flex gap-2">
            <button
              onClick={() => batchResolve('local_wins')}
              className="px-3 py-1.5 rounded-lg text-xs font-semibold bg-blue-600 text-white hover:bg-blue-700"
            >
              Local Wins ({selectedIds.size})
            </button>
            <button
              onClick={() => batchResolve('remote_wins')}
              className="px-3 py-1.5 rounded-lg text-xs font-semibold bg-amber-600 text-white hover:bg-amber-700"
            >
              Remote Wins ({selectedIds.size})
            </button>
          </div>
        )}
      </div>

      <div className="space-y-2">
        {canResolve && filtered.length > 0 && (
          <label className="flex items-center gap-2 text-xs text-on-surface-variant mb-1 cursor-pointer">
            <input type="checkbox" checked={selectedIds.size === filtered.length} onChange={toggleAll} />
            Select all
          </label>
        )}
        {filtered.map((conflict) => (
          <div key={conflict.id} className="bg-surface-container rounded-lg p-4 flex items-center gap-3">
            {canResolve && (
              <input
                type="checkbox"
                checked={selectedIds.has(conflict.id)}
                onChange={() => toggleSelect(conflict.id)}
              />
            )}
            <div className="flex-1">
              <div className="text-sm font-semibold text-on-surface">
                {conflict.entity_type} / {conflict.entity_id.slice(0, 8)}...
              </div>
              <div className="text-xs text-on-surface-variant">
                Local v{conflict.local_version} &harr; Remote v{conflict.remote_version} &middot; {formatTimeAgo(conflict.created_at)}
              </div>
            </div>
            <button
              onClick={() => setSelectedConflict(conflict)}
              className="px-3 py-1.5 rounded-lg text-xs font-semibold bg-surface-container-high text-on-surface hover:bg-surface-container-highest"
            >
              View Details
            </button>
          </div>
        ))}
      </div>

      {selectedConflict && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setSelectedConflict(null)}>
          <div className="bg-surface rounded-xl p-6 max-w-3xl w-full mx-4 max-h-[80vh] overflow-auto" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-bold text-on-surface">
                {selectedConflict.entity_type} / {selectedConflict.entity_id.slice(0, 8)}...
              </h3>
              <button onClick={() => setSelectedConflict(null)} className="text-on-surface-variant hover:text-on-surface text-xl">&times;</button>
            </div>

            <div className="grid grid-cols-2 gap-4 mb-6">
              <div>
                <div className="text-xs font-semibold text-on-surface-variant mb-2">Local (v{selectedConflict.local_version})</div>
                <pre className="bg-surface-container rounded-lg p-3 text-xs text-on-surface overflow-auto max-h-60">
                  {JSON.stringify(selectedConflict.local_diff, null, 2)}
                </pre>
              </div>
              <div>
                <div className="text-xs font-semibold text-on-surface-variant mb-2">Remote (v{selectedConflict.remote_version})</div>
                <pre className="bg-surface-container rounded-lg p-3 text-xs text-on-surface overflow-auto max-h-60">
                  {JSON.stringify(selectedConflict.remote_diff, null, 2)}
                </pre>
              </div>
            </div>

            {canResolve && (
              <div className="flex justify-end gap-3">
                <button
                  onClick={() => handleResolve(selectedConflict.id, 'local_wins')}
                  disabled={resolveConflict.isPending}
                  className="px-4 py-2 rounded-lg text-sm font-semibold bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50"
                >
                  Local Wins
                </button>
                <button
                  onClick={() => handleResolve(selectedConflict.id, 'remote_wins')}
                  disabled={resolveConflict.isPending}
                  className="px-4 py-2 rounded-lg text-sm font-semibold bg-amber-600 text-white hover:bg-amber-700 disabled:opacity-50"
                >
                  Remote Wins
                </button>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
