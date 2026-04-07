import { useState } from 'react'
import Icon from './Icon'
import {
  useScanTargets,
  useDeleteScanTarget,
  useTriggerScan,
  useDiscoveryTasks,
  useTestCollector,
} from '../hooks/useScanTargets'
import CreateScanTargetModal from './CreateScanTargetModal'

/* ------------------------------------------------------------------ */
/*  Type helpers                                                        */
/* ------------------------------------------------------------------ */

interface ScanTarget {
  id: string
  name: string
  collector_type: string
  cidrs: string[]
  credential_id: string
  mode: string
  tenant_id?: string
}

interface DiscoveryTask {
  id: string
  task_type: string
  status: string
  started_at?: string
  completed_at?: string
  result?: Record<string, any>
}

/* ------------------------------------------------------------------ */
/*  Icon mapping                                                        */
/* ------------------------------------------------------------------ */

const typeIcon: Record<string, string> = {
  snmp: 'router',
  ssh:  'terminal',
  ipmi: 'developer_board',
}

const typeBg: Record<string, string> = {
  snmp: 'bg-[#064e3b]',
  ssh:  'bg-[#1a365d]',
  ipmi: 'bg-[#92400e]',
}

/* ------------------------------------------------------------------ */
/*  Status color for scan history                                       */
/* ------------------------------------------------------------------ */

const taskStatusStyle: Record<string, string> = {
  completed: 'bg-emerald-500/20 text-emerald-400',
  running:   'bg-blue-500/20 text-blue-400',
  pending:   'bg-gray-500/20 text-gray-400',
  failed:    'bg-red-500/20 text-red-400',
}

function TaskStatusBadge({ status }: { status: string }) {
  const cls = taskStatusStyle[status] ?? taskStatusStyle.pending
  return (
    <span className={`inline-block px-2.5 py-1 rounded text-[0.6875rem] font-semibold uppercase tracking-wider ${cls}`}>
      {status}
    </span>
  )
}

/* ------------------------------------------------------------------ */
/*  Stat summary from task result                                       */
/* ------------------------------------------------------------------ */

function TaskStatsSummary({ result }: { result?: Record<string, any> }) {
  if (!result) return <span className="text-xs text-on-surface-variant">—</span>
  const parts: string[] = []
  if (result.discovered != null)  parts.push(`${result.discovered} discovered`)
  if (result.approved   != null)  parts.push(`${result.approved} approved`)
  if (result.failed     != null)  parts.push(`${result.failed} failed`)
  if (parts.length === 0) return <span className="text-xs text-on-surface-variant">—</span>
  return <span className="text-xs text-on-surface-variant">{parts.join(' · ')}</span>
}

/* ------------------------------------------------------------------ */
/*  Date formatting                                                     */
/* ------------------------------------------------------------------ */

function fmt(ts?: string) {
  if (!ts) return '—'
  return new Date(ts).toLocaleString(undefined, {
    month: 'short', day: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

/* ------------------------------------------------------------------ */
/*  Component                                                           */
/* ------------------------------------------------------------------ */

export default function ScanManagementTab() {
  const [modalOpen, setModalOpen]     = useState(false)
  const [editingTarget, setEditingTarget] = useState<ScanTarget | null>(null)

  const { data: targetsData, isLoading: targetsLoading } = useScanTargets()
  const { data: tasksData,   isLoading: tasksLoading }   = useDiscoveryTasks()

  const deleteMutation  = useDeleteScanTarget()
  const triggerMutation = useTriggerScan()
  const testMutation    = useTestCollector()

  const targets: ScanTarget[] = (targetsData as any)?.data ?? []
  const tasks: DiscoveryTask[] = (tasksData as any)?.data ?? []

  function handleEdit(t: ScanTarget) {
    setEditingTarget(t)
    setModalOpen(true)
  }

  function handleAdd() {
    setEditingTarget(null)
    setModalOpen(true)
  }

  function handleDelete(id: string) {
    if (confirm('Delete this scan target?')) {
      deleteMutation.mutate(id)
    }
  }

  function handleScanNow(id: string) {
    triggerMutation.mutate(id)
  }

  function handleTest(target: ScanTarget) {
    testMutation.mutate({
      name: target.collector_type,
      data: {
        credential_id: target.credential_id,
        endpoint: target.cidrs?.[0] ?? '',
      },
    })
  }

  return (
    <div className="px-8 pb-8 space-y-6">
      {/* ============================================================ */}
      {/*  Scan Targets                                                 */}
      {/* ============================================================ */}
      <div>
        <div className="flex items-center justify-between mb-3">
          <h2 className="font-headline font-bold text-lg text-on-surface flex items-center gap-2">
            <Icon name="radar" className="text-[20px] text-primary" />
            Scan Targets
          </h2>
          <button
            onClick={handleAdd}
            className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-primary text-on-primary text-sm font-semibold hover:opacity-90 transition-opacity"
          >
            <Icon name="add" className="text-[18px]" />
            Add Target
          </button>
        </div>

        <div className="bg-surface-container rounded-lg overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-surface-container-high text-on-surface-variant text-[0.6875rem] uppercase tracking-wider">
                <th className="px-4 py-3 text-left font-semibold">Name</th>
                <th className="px-4 py-3 text-left font-semibold">Type</th>
                <th className="px-4 py-3 text-left font-semibold">CIDRs</th>
                <th className="px-4 py-3 text-left font-semibold">Credential</th>
                <th className="px-4 py-3 text-left font-semibold">Mode</th>
                <th className="px-4 py-3 text-right font-semibold">Actions</th>
              </tr>
            </thead>
            <tbody>
              {targetsLoading && (
                <tr>
                  <td colSpan={6} className="py-10 text-center">
                    <div className="inline-block animate-spin rounded-full h-6 w-6 border-2 border-sky-400 border-t-transparent" />
                  </td>
                </tr>
              )}
              {!targetsLoading && targets.length === 0 && (
                <tr>
                  <td colSpan={6} className="py-10 text-center text-on-surface-variant text-sm">
                    No scan targets configured. Click "Add Target" to get started.
                  </td>
                </tr>
              )}
              {targets.map(t => {
                const icon = typeIcon[t.collector_type] ?? 'devices'
                const bg   = typeBg[t.collector_type]  ?? 'bg-surface-container-high'
                return (
                  <tr
                    key={t.id}
                    className="bg-surface-container hover:bg-surface-container-high transition-colors border-t border-surface-container-high"
                  >
                    <td className="px-4 py-3 font-medium text-on-surface">{t.name}</td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <span className={`inline-flex items-center justify-center w-7 h-7 rounded-md ${bg}`}>
                          <Icon name={icon} className="text-[16px] text-on-surface" />
                        </span>
                        <span className="text-on-surface-variant uppercase text-xs font-semibold">
                          {t.collector_type}
                        </span>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex flex-wrap gap-1">
                        {(t.cidrs ?? []).slice(0, 3).map(cidr => (
                          <span
                            key={cidr}
                            className="font-mono text-xs bg-surface-container-high px-1.5 py-0.5 rounded text-on-surface-variant"
                          >
                            {cidr}
                          </span>
                        ))}
                        {(t.cidrs ?? []).length > 3 && (
                          <span className="text-xs text-on-surface-variant">
                            +{t.cidrs.length - 3} more
                          </span>
                        )}
                        {(!t.cidrs || t.cidrs.length === 0) && (
                          <span className="text-xs text-on-surface-variant">—</span>
                        )}
                      </div>
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-on-surface-variant">
                      {t.credential_id ? t.credential_id.slice(0, 8) + '…' : '—'}
                    </td>
                    <td className="px-4 py-3">
                      <span className="text-xs text-on-surface-variant capitalize">{t.mode}</span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center justify-end gap-1">
                        <button
                          onClick={() => handleScanNow(t.id)}
                          disabled={triggerMutation.isPending}
                          className="p-1.5 rounded-md hover:bg-primary/20 transition-colors"
                          title="Scan Now"
                        >
                          <Icon name="play_arrow" className="text-[18px] text-primary" />
                        </button>
                        <button
                          onClick={() => handleTest(t)}
                          disabled={testMutation.isPending}
                          className="p-1.5 rounded-md hover:bg-blue-500/20 transition-colors"
                          title="Test Connection"
                        >
                          <Icon name="lan" className="text-[18px] text-blue-400" />
                        </button>
                        <button
                          onClick={() => handleEdit(t)}
                          className="p-1.5 rounded-md hover:bg-surface-container-highest transition-colors"
                          title="Edit"
                        >
                          <Icon name="edit" className="text-[18px] text-on-surface-variant" />
                        </button>
                        <button
                          onClick={() => handleDelete(t.id)}
                          disabled={deleteMutation.isPending}
                          className="p-1.5 rounded-md hover:bg-error-container/40 transition-colors"
                          title="Delete"
                        >
                          <Icon name="delete" className="text-[18px] text-error" />
                        </button>
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      </div>

      {/* ============================================================ */}
      {/*  Scan History                                                 */}
      {/* ============================================================ */}
      <div>
        <h2 className="font-headline font-bold text-lg text-on-surface flex items-center gap-2 mb-3">
          <Icon name="history" className="text-[20px] text-primary" />
          Scan History
        </h2>

        <div className="bg-surface-container rounded-lg overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-surface-container-high text-on-surface-variant text-[0.6875rem] uppercase tracking-wider">
                <th className="px-4 py-3 text-left font-semibold">Type</th>
                <th className="px-4 py-3 text-left font-semibold">Status</th>
                <th className="px-4 py-3 text-left font-semibold">Stats</th>
                <th className="px-4 py-3 text-left font-semibold">Started</th>
                <th className="px-4 py-3 text-left font-semibold">Completed</th>
              </tr>
            </thead>
            <tbody>
              {tasksLoading && (
                <tr>
                  <td colSpan={5} className="py-10 text-center">
                    <div className="inline-block animate-spin rounded-full h-6 w-6 border-2 border-sky-400 border-t-transparent" />
                  </td>
                </tr>
              )}
              {!tasksLoading && tasks.length === 0 && (
                <tr>
                  <td colSpan={5} className="py-10 text-center text-on-surface-variant text-sm">
                    No scan history yet.
                  </td>
                </tr>
              )}
              {tasks.map(task => (
                <tr
                  key={task.id}
                  className="bg-surface-container hover:bg-surface-container-high transition-colors border-t border-surface-container-high"
                >
                  <td className="px-4 py-3 text-on-surface-variant capitalize text-xs font-semibold uppercase tracking-wider">
                    {task.task_type}
                  </td>
                  <td className="px-4 py-3">
                    <TaskStatusBadge status={task.status} />
                  </td>
                  <td className="px-4 py-3">
                    <TaskStatsSummary result={task.result} />
                  </td>
                  <td className="px-4 py-3 text-xs text-on-surface-variant">{fmt(task.started_at)}</td>
                  <td className="px-4 py-3 text-xs text-on-surface-variant">{fmt(task.completed_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* ============================================================ */}
      {/*  Modal                                                        */}
      {/* ============================================================ */}
      <CreateScanTargetModal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        editing={editingTarget}
      />
    </div>
  )
}
