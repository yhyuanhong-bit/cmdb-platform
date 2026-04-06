import { memo, useState } from 'react'
import { Link } from 'react-router-dom'
import { useBIAAssessments, useBIADependencies, useCreateBIADependency } from '../../hooks/useBIA'
import { useAssets } from '../../hooks/useAssets'

const TIER_BADGE: Record<string, string> = {
  critical:  'bg-[#7f1d1d] text-[#fca5a5]',
  important: 'bg-[#78350f] text-[#fde68a]',
  normal:    'bg-[#1e3a5f] text-[#93c5fd]',
  low:       'bg-[#374151] text-[#d1d5db]',
}

const DEP_TYPE_COLORS: Record<string, string> = {
  runs_on:    'bg-[#1e3a5f] text-on-primary-container',
  depends_on: 'bg-[#92400e] text-[#fbbf24]',
  backed_by:  'bg-[#064e3b] text-[#34d399]',
}

const CRITICALITY_COLORS: Record<string, string> = {
  critical: 'bg-[#7f1d1d] text-[#fca5a5]',
  high:     'bg-[#78350f] text-[#fde68a]',
  medium:   'bg-[#1e3a5f] text-[#93c5fd]',
  low:      'bg-[#374151] text-[#d1d5db]',
}

function Icon({ name, className = '' }: { name: string; className?: string }) {
  return <span className={`material-symbols-outlined ${className}`}>{name}</span>
}

function getBadge(tier: string) {
  return TIER_BADGE[tier.toLowerCase()] || TIER_BADGE.low
}

function DependencyMap() {
  const { data: assessResp, isLoading: assessLoading } = useBIAAssessments()
  const assessments = (assessResp?.data as any)?.data || []

  const [selectedId, setSelectedId] = useState('')
  const { data: depsResp, isLoading: depsLoading } = useBIADependencies(selectedId)
  const deps = (depsResp?.data as any)?.data || []

  const { data: assetsResp } = useAssets()
  const assets = (assetsResp?.data as any)?.data || []

  const createDep = useCreateBIADependency()

  const [showModal, setShowModal] = useState(false)
  const [newDep, setNewDep] = useState({ asset_id: '', dependency_type: 'runs_on', criticality: 'medium' })

  const selectedAssessment = assessments.find((a) => a.id === selectedId)

  function handleAddDependency() {
    if (!selectedId || !newDep.asset_id) return
    createDep.mutate(
      { assessmentId: selectedId, data: newDep },
      {
        onSuccess: () => {
          setShowModal(false)
          setNewDep({ asset_id: '', dependency_type: 'runs_on', criticality: 'medium' })
        },
      }
    )
  }

  // Map asset IDs to names for display
  const assetNameMap = new Map(assets.map((a) => [a.id, a.name || a.id]))

  return (
    <div className="space-y-6">
      {/* Breadcrumb + Header */}
      <div>
        <div className="flex items-center gap-1.5 text-[0.6875rem] uppercase tracking-wider text-on-surface-variant mb-2">
          <Link to="/bia" className="hover:text-on-surface transition-colors">BIA</Link>
          <Icon name="chevron_right" className="text-base" />
          <span className="text-on-surface">Dependency Map</span>
        </div>
        <h1 className="font-headline font-bold text-2xl text-on-surface">Dependency Map</h1>
      </div>

      {/* Assessment Selector */}
      <div className="rounded-lg bg-surface-container p-5">
        <label className="block text-[0.6875rem] uppercase tracking-wider text-on-surface-variant mb-2">
          Select Business System
        </label>
        <select
          value={selectedId}
          onChange={(e) => setSelectedId(e.target.value)}
          className="w-full max-w-md rounded-lg bg-surface-container-low border border-outline-variant px-3 py-2.5 text-sm text-on-surface focus:outline-none focus:border-primary"
        >
          <option value="">-- Choose a system --</option>
          {assessLoading ? (
            <option disabled>Loading...</option>
          ) : (
            assessments.map((a) => (
              <option key={a.id} value={a.id}>
                {a.system_name} ({a.tier})
              </option>
            ))
          )}
        </select>
      </div>

      {/* Selected System Info */}
      {selectedAssessment && (
        <div className="rounded-lg bg-surface-container p-5">
          <div className="flex items-center gap-4">
            <div className="flex items-center justify-center w-12 h-12 rounded-lg bg-surface-container-high">
              <Icon name="hub" className="text-2xl text-on-surface" />
            </div>
            <div className="flex-1">
              <h3 className="font-headline font-bold text-lg text-on-surface">{selectedAssessment.system_name}</h3>
              <div className="flex items-center gap-3 mt-1">
                <span className={`inline-block rounded px-2 py-0.5 text-xs font-medium uppercase ${getBadge(selectedAssessment.tier)}`}>
                  {selectedAssessment.tier}
                </span>
                <span className="text-xs text-on-surface-variant">
                  BIA Score: <span className="font-bold text-on-surface">{selectedAssessment.bia_score}</span>
                </span>
                <span className="text-xs text-on-surface-variant">
                  Code: <span className="font-mono">{selectedAssessment.system_code}</span>
                </span>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Dependencies Table */}
      {selectedId && (
        <div className="rounded-lg bg-surface-container p-5">
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-headline font-bold text-lg text-on-surface">Linked Dependencies</h3>
            <button
              type="button"
              onClick={() => setShowModal(true)}
              className="inline-flex items-center gap-2 rounded-lg bg-primary px-3.5 py-2 text-sm font-medium text-on-primary transition-colors hover:bg-primary/90"
            >
              <Icon name="add" className="text-lg" />
              Add Dependency
            </button>
          </div>

          {depsLoading ? (
            <div className="space-y-2">
              {[1, 2, 3].map((i) => (
                <div key={i} className="h-10 rounded bg-surface-container-high animate-pulse" />
              ))}
            </div>
          ) : deps.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-on-surface-variant">
              <Icon name="link_off" className="text-4xl mb-2 opacity-50" />
              <p className="text-sm">No dependencies linked yet</p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-outline-variant">
                    <th className="text-left py-2.5 px-3 text-[0.6875rem] uppercase tracking-wider text-on-surface-variant font-medium">Asset</th>
                    <th className="text-left py-2.5 px-3 text-[0.6875rem] uppercase tracking-wider text-on-surface-variant font-medium">Type</th>
                    <th className="text-left py-2.5 px-3 text-[0.6875rem] uppercase tracking-wider text-on-surface-variant font-medium">Criticality</th>
                  </tr>
                </thead>
                <tbody>
                  {deps.map((dep) => (
                    <tr key={dep.id} className="border-b border-outline-variant/30 hover:bg-surface-container-high/40 transition-colors">
                      <td className="py-2.5 px-3 text-on-surface font-medium">
                        {assetNameMap.get(dep.asset_id) || dep.asset_id}
                      </td>
                      <td className="py-2.5 px-3">
                        <span className={`inline-block rounded px-2 py-0.5 text-xs font-medium ${DEP_TYPE_COLORS[dep.dependency_type] || 'bg-surface-container-high text-on-surface-variant'}`}>
                          {dep.dependency_type.replace(/_/g, ' ')}
                        </span>
                      </td>
                      <td className="py-2.5 px-3">
                        <span className={`inline-block rounded px-2 py-0.5 text-xs font-medium uppercase ${CRITICALITY_COLORS[(dep.criticality || 'medium').toLowerCase()] || CRITICALITY_COLORS.medium}`}>
                          {dep.criticality || 'medium'}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {/* Add Dependency Modal */}
      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="w-full max-w-md rounded-xl bg-surface-container p-6 shadow-2xl">
            <div className="flex items-center justify-between mb-5">
              <h3 className="font-headline font-bold text-lg text-on-surface">Add Dependency</h3>
              <button
                type="button"
                onClick={() => setShowModal(false)}
                className="rounded-lg p-1 hover:bg-surface-container-high transition-colors"
              >
                <Icon name="close" className="text-xl text-on-surface-variant" />
              </button>
            </div>

            <div className="space-y-4">
              <div>
                <label className="block text-[0.6875rem] uppercase tracking-wider text-on-surface-variant mb-1.5">Asset</label>
                <select
                  value={newDep.asset_id}
                  onChange={(e) => setNewDep((p) => ({ ...p, asset_id: e.target.value }))}
                  className="w-full rounded-lg bg-surface-container-low border border-outline-variant px-3 py-2 text-sm text-on-surface focus:outline-none focus:border-primary"
                >
                  <option value="">-- Select asset --</option>
                  {assets.map((a) => (
                    <option key={a.id} value={a.id}>{a.name || a.id}</option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-[0.6875rem] uppercase tracking-wider text-on-surface-variant mb-1.5">Dependency Type</label>
                <select
                  value={newDep.dependency_type}
                  onChange={(e) => setNewDep((p) => ({ ...p, dependency_type: e.target.value }))}
                  className="w-full rounded-lg bg-surface-container-low border border-outline-variant px-3 py-2 text-sm text-on-surface focus:outline-none focus:border-primary"
                >
                  <option value="runs_on">Runs On</option>
                  <option value="depends_on">Depends On</option>
                  <option value="backed_by">Backed By</option>
                </select>
              </div>

              <div>
                <label className="block text-[0.6875rem] uppercase tracking-wider text-on-surface-variant mb-1.5">Criticality</label>
                <select
                  value={newDep.criticality}
                  onChange={(e) => setNewDep((p) => ({ ...p, criticality: e.target.value }))}
                  className="w-full rounded-lg bg-surface-container-low border border-outline-variant px-3 py-2 text-sm text-on-surface focus:outline-none focus:border-primary"
                >
                  <option value="critical">Critical</option>
                  <option value="high">High</option>
                  <option value="medium">Medium</option>
                  <option value="low">Low</option>
                </select>
              </div>
            </div>

            <div className="flex items-center justify-end gap-3 mt-6">
              <button
                type="button"
                onClick={() => setShowModal(false)}
                className="rounded-lg bg-surface-container-high px-4 py-2 text-sm font-medium text-on-surface hover:bg-surface-container-highest transition-colors"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleAddDependency}
                disabled={!newDep.asset_id || createDep.isPending}
                className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-on-primary hover:bg-primary/90 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
              >
                <Icon name="add_link" className="text-lg" />
                {createDep.isPending ? 'Adding...' : 'Add'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default memo(DependencyMap)
