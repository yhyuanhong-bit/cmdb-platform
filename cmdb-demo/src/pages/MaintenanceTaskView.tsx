import { toast } from 'sonner'
import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import Icon from '../components/Icon'
import StatusBadge from '../components/StatusBadge'
import { useWorkOrder, useUpdateWorkOrder, useTransitionWorkOrder, useWorkOrderLogs, useWorkOrderComments, useCreateWorkOrderComment } from '../hooks/useMaintenance'

// Local comment type with fields used in this view
interface WorkOrderComment {
  id?: string
  content?: string
  text?: string
  author_name?: string
  created_at?: string
}


type TimelineEvent = {
  id?: string
  label?: string
  timestamp?: string
  color?: string
  textColor?: string
  action?: string
  from_status?: string
  to_status?: string
  created_at?: string
  comment?: string
  performed_by?: string
  performed_at?: string
  note?: string
}

const taskSteps = [
  'Execute standard hot-swap procedure for battery modules UPS-BAT-01 and UPS-BAT-02',
  'Perform automated redundancy testing via IronGrid telemetry suite',
  'Verify telemetry synchronization across all monitored endpoints',
  'Manual log confirmation and sign-off by supervising engineer',
]

const timeline = [
  {
    label: 'Redundancy test initiated',
    timestamp: '2026-03-28 09:42',
    color: 'bg-on-primary-container',
    textColor: 'text-primary',
  },
  {
    label: 'Parts arrived at DC-1 receiving dock',
    timestamp: '2026-03-27 14:15',
    color: 'bg-on-primary-container',
    textColor: 'text-primary',
  },
  {
    label: 'Task Assigned to Sarah Jenkins',
    timestamp: '2026-03-26 08:30',
    color: 'bg-surface-container-highest',
    textColor: 'text-on-surface-variant',
  },
]

const associatedAssets = [
  {
    id: 'UPS-BAT-01',
    location: 'Rack A01, U-Pos 14',
    health: 60,
    healthColor: 'text-[#fbbf24]',
    barColor: 'bg-[#fbbf24]',
  },
  {
    id: 'UPS-BAT-02',
    location: 'Rack A01, U-Pos 15',
    health: 89,
    healthColor: 'text-[#34d399]',
    barColor: 'bg-[#34d399]',
  },
]

const envMetrics = [
  { labelKey: 'maintenance_task.ambient_temp', value: '22.4°C', icon: 'thermostat', color: 'text-primary' },
  { labelKey: 'rack_visualization.humidity', value: '44%', icon: 'humidity_mid', color: 'text-[#34d399]' },
  { labelKey: 'maintenance_task.current_load', value: '14.2kW', icon: 'bolt', color: 'text-[#fbbf24]' },
]

export default function MaintenanceTaskView() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { id: taskId } = useParams<{ id: string }>()
  const { data: woResponse, isLoading, error } = useWorkOrder(taskId ?? '')
  const workOrder = woResponse?.data
  const orderId = workOrder?.id ?? taskId ?? ''
  const transitionWO = useTransitionWorkOrder()
  const { data: logsData } = useWorkOrderLogs(taskId ?? '')
  const { data: commentsData } = useWorkOrderComments(orderId)
  const comments = commentsData?.data?.comments ?? []
  const createComment = useCreateWorkOrderComment()
  const [comment, setComment] = useState('')
  const updateWO = useUpdateWorkOrder()
  const [showEditModal, setShowEditModal] = useState(false)
  const [editData, setEditData] = useState({ title: '', description: '', priority: '' })

  const handleCommentSubmit = () => {
    if (!comment.trim()) return
    createComment.mutate({ orderId, data: { text: comment } }, {
      onSuccess: () => setComment('')
    })
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-primary border-t-transparent" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center py-20 gap-3">
        <span className="material-symbols-outlined text-error text-4xl">error</span>
        <p className="text-error text-sm">Failed to load task details</p>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-surface p-6 font-body text-on-surface">
      {/* Header */}
      <div className="mb-6">
        <div className="flex items-center gap-2 mb-2">
          <button
            onClick={() => navigate('/maintenance')}
            className="flex items-center gap-1 text-sm text-on-surface-variant hover:text-primary transition-colors"
          >
            <Icon name="arrow_back" className="text-[18px]" />
            {t('maintenance_task.back_to_schedule')}
          </button>
        </div>
        <div className="flex items-start justify-between">
          <div>
            <div className="flex items-center gap-3 mb-1">
              <h1 className="font-headline text-2xl font-bold tracking-tight text-on-surface">
                {workOrder?.title ?? 'UPS System Battery Replacement (Rack A01)'}
              </h1>
              <StatusBadge status={workOrder?.status ?? 'In_Progress'} />
            </div>
            <div className="flex items-center gap-4 text-sm text-on-surface-variant">
              <span className="flex items-center gap-1.5">
                <Icon name="tag" className="text-[16px]" />
                <span className="font-mono text-primary">#{workOrder?.code ?? 'MT-2026-8842'}</span>
              </span>
              <span className="flex items-center gap-1.5">
                <Icon name="calendar_today" className="text-[16px]" />
                {t('maintenance_task.due')} {workOrder?.scheduled_end?.slice(0, 10) ?? 'March 28, 2026'}
              </span>
              <span className="flex items-center gap-1.5">
                <Icon name="person" className="text-[16px]" />
                {t('maintenance_task.assigned')}: {workOrder?.assignee_id ?? 'Sarah Jenkins'}
              </span>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => navigate('/maintenance/workorder')}
              className="flex items-center gap-1.5 bg-surface-container-high px-4 py-2.5 text-sm font-medium text-on-surface rounded hover:bg-surface-container-highest transition-all"
            >
              <Icon name="assignment" className="text-[18px]" />
              建立工單
            </button>
            <button
              onClick={() => navigate('/maintenance/dispatch')}
              className="flex items-center gap-1.5 bg-surface-container-high px-4 py-2.5 text-sm font-medium text-on-surface rounded hover:bg-surface-container-highest transition-all"
            >
              <Icon name="group" className="text-[18px]" />
              調度人員
            </button>
            <button
              onClick={() => {
                const status = workOrder?.status?.toLowerCase()
                if (status && status !== 'submitted' && status !== 'rejected') {
                  toast.error(t('work_order.edit_blocked'))
                  return
                }
                setEditData({
                  title: workOrder?.title ?? '',
                  description: workOrder?.description ?? '',
                  priority: workOrder?.priority ?? 'medium',
                })
                setShowEditModal(true)
              }}
              className="flex items-center gap-1.5 bg-surface-container-high px-4 py-2.5 text-sm font-medium text-on-surface rounded hover:bg-surface-container-highest transition-all"
            >
              <Icon name="edit" className="text-[18px]" />
              {t('maintenance_task.edit_task')}
            </button>
            {(() => {
              // The state machine only allows in_progress -> completed.
              // Only render the Complete button when the order is in that state;
              // for other states show a disabled hint button so users know why.
              const status = workOrder?.status?.toLowerCase() ?? ''
              const canComplete = status === 'in_progress'
              const tooltipKey = !workOrder
                ? 'maintenance_task.complete_disabled_loading'
                : status === 'completed' || status === 'verified'
                  ? 'maintenance_task.complete_disabled_already_completed'
                  : status === 'submitted' || status === 'rejected'
                    ? 'maintenance_task.complete_disabled_not_started'
                    : status === 'approved'
                      ? 'maintenance_task.complete_disabled_not_in_progress'
                      : 'maintenance_task.complete_disabled_invalid_state'
              return (
                <button
                  onClick={() => {
                    if (!canComplete || !taskId) return
                    transitionWO.mutate(
                      { id: taskId, data: { status: 'completed', comment: 'Task completed' } },
                      {
                        onError: (err: unknown) => {
                          const status = (err as { status?: number })?.status
                          const code = (err as { code?: string })?.code
                          if (status === 403 || code === 'FORBIDDEN') {
                            toast.error(t('maintenance_task.complete_disabled_already_completed'))
                          } else {
                            toast.error(t('maintenance_task.complete_failed'))
                          }
                        },
                      },
                    )
                  }}
                  disabled={!canComplete || transitionWO.isPending}
                  title={canComplete ? '' : t(tooltipKey)}
                  aria-disabled={!canComplete || transitionWO.isPending}
                  className="flex items-center gap-1.5 bg-on-primary-container px-4 py-2.5 text-sm font-semibold text-white rounded hover:brightness-110 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <Icon name="check_circle" className="text-[18px]" />
                  {transitionWO.isPending ? t('maintenance_task.completing') : t('maintenance_task.complete_action')}
                </button>
              )
            })()}
          </div>
        </div>
      </div>

      {/* Main Content Grid */}
      <div className="grid grid-cols-[1fr_360px] gap-4">
        {/* Left Column */}
        <div className="flex flex-col gap-4">
          {/* Maintenance Description */}
          <div className="bg-surface-container rounded p-5">
            <h2 className="font-headline text-sm font-semibold text-on-surface mb-4 flex items-center gap-2">
              <Icon name="description" className="text-[18px] text-primary" />
              {t('maintenance_task.maintenance_description')}
            </h2>
            <div className="flex flex-col gap-3">
              {taskSteps.map((step, i) => (
                <div key={i} className="flex gap-3">
                  <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded bg-surface-container-high text-xs font-semibold text-primary">
                    {i + 1}
                  </div>
                  <p className="text-sm text-on-surface-variant leading-relaxed pt-0.5">
                    {step}
                  </p>
                </div>
              ))}
            </div>
          </div>

          {/* Associated Assets */}
          <div className="bg-surface-container rounded p-5">
            <h2 className="font-headline text-sm font-semibold text-on-surface mb-4 flex items-center gap-2">
              <Icon name="devices" className="text-[18px] text-primary" />
              {t('maintenance_task.associated_assets')}
            </h2>
            <div className="grid grid-cols-2 gap-3">
              {associatedAssets.map((asset) => (
                <div
                  key={asset.id}
                  className="bg-surface-container-low rounded p-4 hover:bg-surface-container-high transition-colors cursor-pointer"
                >
                  <div className="flex items-center justify-between mb-2">
                    <span
                      className="font-mono text-primary text-sm font-semibold cursor-pointer hover:underline"
                      onClick={() => navigate(`/assets/${asset.id}`)}
                    >
                      {asset.id}
                    </span>
                    <Icon name="battery_charging_full" className="text-[20px] text-on-surface-variant" />
                  </div>
                  <p className="text-xs text-on-surface-variant mb-3">{asset.location}</p>
                  <div className="flex items-center justify-between mb-1.5">
                    <span className="text-[0.6875rem] text-on-surface-variant">{t('maintenance_task.health')}</span>
                    <span className={`text-sm font-semibold ${asset.healthColor}`}>
                      {asset.health}%
                    </span>
                  </div>
                  <div className="h-1.5 w-full rounded-full bg-surface-container-highest">
                    <div
                      className={`h-full rounded-full ${asset.barColor}`}
                      style={{ width: `${asset.health}%` }}
                    />
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Comment Input */}
          <div className="bg-surface-container rounded p-5">
            <h2 className="font-headline text-sm font-semibold text-on-surface mb-3 flex items-center gap-2">
              <Icon name="chat" className="text-[18px] text-primary" />
              {t('maintenance_task.add_comment')}
            </h2>
            <textarea
              value={comment}
              onChange={(e) => setComment(e.target.value)}
              placeholder={t('maintenance_task.comment_placeholder')}
              rows={3}
              className="w-full bg-surface-container-low rounded p-3 text-sm text-on-surface placeholder:text-on-surface-variant/50 resize-none focus:outline-none focus:ring-1 focus:ring-primary/40"
            />
            <div className="mt-3 flex justify-end">
              <button onClick={handleCommentSubmit} disabled={createComment.isPending} className="flex items-center gap-1.5 bg-on-primary-container px-4 py-2 text-sm font-semibold text-white rounded hover:brightness-110 transition-all disabled:opacity-50">
                <Icon name="send" className="text-[16px]" />
                {createComment.isPending ? '...' : t('maintenance_task.post_update')}
              </button>
            </div>
            {comments.length > 0 && (
              <div className="mt-4">
                {comments.map((c: WorkOrderComment) => (
                  <div key={c.id} className="border-t border-surface-container-high py-3">
                    <div className="flex justify-between text-xs text-on-surface-variant">
                      <span className="font-semibold">{c.author_name ?? 'System'}</span>
                      <span>{c.created_at ? new Date(c.created_at).toLocaleString() : ""}</span>
                    </div>
                    <p className="text-sm text-on-surface mt-1">{c.text}</p>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Right Column */}
        <div className="flex flex-col gap-4">
          {/* Progress & Notes Timeline */}
          <div className="bg-surface-container rounded p-5">
            <h2 className="font-headline text-sm font-semibold text-on-surface mb-4 flex items-center gap-2">
              <Icon name="timeline" className="text-[18px] text-primary" />
              {t('maintenance_task.progress_notes')}
            </h2>
            <div className="relative flex flex-col gap-0">
              {(logsData?.data && logsData.data.length > 0 ? logsData.data : timeline).map((event: TimelineEvent, i: number, arr: TimelineEvent[]) => (
                <div key={event.id ?? `${event.label}-${i}`} className="flex gap-3 pb-5 last:pb-0">
                  <div className="relative flex flex-col items-center">
                    <div className={`h-3 w-3 rounded-full ${event.color ?? 'bg-on-primary-container'} shrink-0 mt-0.5`} />
                    {i < arr.length - 1 && (
                      <div className="w-px flex-1 bg-surface-container-highest mt-1" />
                    )}
                  </div>
                  <div>
                    <p className={`text-sm font-medium ${event.textColor ?? 'text-primary'}`}>
                      {event.label ?? `${event.action}${event.from_status ? ` (${event.from_status} → ${event.to_status})` : ''}`}
                    </p>
                    <p className="text-[0.6875rem] text-on-surface-variant mt-0.5">
                      {event.timestamp ?? event.created_at?.slice(0, 16).replace('T', ' ')}
                    </p>
                    {event.comment && (
                      <p className="text-xs text-on-surface-variant mt-0.5 italic">{event.comment}</p>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Environmental Context */}
          <div className="bg-surface-container rounded p-5">
            <h2 className="font-headline text-sm font-semibold text-on-surface mb-4 flex items-center gap-2">
              <Icon name="monitoring" className="text-[18px] text-primary" />
              {t('maintenance_task.environmental_context')}
            </h2>
            <div className="flex flex-col gap-3">
              {envMetrics.map((metric) => (
                <div
                  key={metric.labelKey}
                  className="flex items-center justify-between bg-surface-container-low rounded p-3"
                >
                  <div className="flex items-center gap-2">
                    <Icon name={metric.icon} className={`text-[20px] ${metric.color}`} />
                    <span className="text-xs text-on-surface-variant">{t(metric.labelKey)}</span>
                  </div>
                  <span className={`font-mono text-sm font-semibold ${metric.color}`}>
                    {metric.value}
                  </span>
                </div>
              ))}
            </div>
          </div>

          {/* Task Metadata */}
          <div className="bg-surface-container rounded p-5">
            <h2 className="font-headline text-sm font-semibold text-on-surface mb-4 flex items-center gap-2">
              <Icon name="info" className="text-[18px] text-primary" />
              {t('maintenance_task.task_details')}
            </h2>
            <div className="flex flex-col gap-2.5 text-sm">
              {[
                { label: t('maintenance_task.task_id'), value: `#${workOrder?.code ?? 'MT-2026-8842'}` },
                { label: t('common.priority'), value: workOrder?.priority ?? 'Critical' },
                { label: t('common.type'), value: workOrder?.type ?? t('maintenance_task.type_corrective_maintenance') },
                { label: t('maintenance_task.created'), value: workOrder?.scheduled_start?.slice(0, 16).replace('T', ' ') ?? '2026-03-25 16:20' },
                { label: t('maintenance_task.last_updated'), value: workOrder?.actual_start?.slice(0, 16).replace('T', ' ') ?? '2026-03-28 09:42' },
                { label: t('maintenance_task.estimated_duration'), value: '4 hours' },
              ].map((row) => (
                <div key={row.label} className="flex items-center justify-between">
                  <span className="text-on-surface-variant text-xs">{row.label}</span>
                  <span className="text-on-surface text-xs font-medium">{row.value}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* Edit Modal */}
      {showEditModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowEditModal(false)}>
          <div className="bg-surface-container p-6 rounded-xl w-[480px] space-y-4" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-bold text-on-surface">{t('maintenance_task.edit_task')}</h3>
            <div className="space-y-3">
              <div>
                <label className="text-xs text-on-surface-variant uppercase tracking-wider block mb-1">{t('common.title')}</label>
                <input
                  value={editData.title}
                  onChange={e => setEditData(p => ({ ...p, title: e.target.value }))}
                  className="w-full p-2.5 bg-surface-container-low rounded-lg border border-surface-container-highest text-on-surface text-sm"
                />
              </div>
              <div>
                <label className="text-xs text-on-surface-variant uppercase tracking-wider block mb-1">{t('common.description')}</label>
                <textarea
                  value={editData.description}
                  onChange={e => setEditData(p => ({ ...p, description: e.target.value }))}
                  rows={3}
                  className="w-full p-2.5 bg-surface-container-low rounded-lg border border-surface-container-highest text-on-surface text-sm"
                />
              </div>
              <div>
                <label className="text-xs text-on-surface-variant uppercase tracking-wider block mb-1">{t('common.priority')}</label>
                <select
                  value={editData.priority}
                  onChange={e => setEditData(p => ({ ...p, priority: e.target.value }))}
                  className="w-full p-2.5 bg-surface-container-low rounded-lg border border-surface-container-highest text-on-surface text-sm"
                >
                  <option value="critical">{t('common.critical')}</option>
                  <option value="high">{t('common.high')}</option>
                  <option value="medium">{t('common.medium')}</option>
                  <option value="low">{t('common.low')}</option>
                </select>
              </div>
            </div>
            <div className="flex gap-2 justify-end pt-2">
              <button onClick={() => setShowEditModal(false)} className="px-4 py-2 rounded-lg bg-surface-container-high text-on-surface-variant text-sm">
                {t('common.cancel')}
              </button>
              <button
                onClick={() => {
                  if (!taskId) return
                  updateWO.mutate({ id: taskId, data: editData }, {
                    onSuccess: () => {
                      setShowEditModal(false)
                      toast.success(t('work_order.updated_success'))
                    }
                  })
                }}
                disabled={updateWO.isPending}
                className="px-4 py-2 rounded-lg bg-on-primary-container text-white text-sm font-semibold disabled:opacity-50"
              >
                {updateWO.isPending ? t('common.saving') : t('common.save_changes')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
