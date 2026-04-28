import { toast } from 'sonner'
import { useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useQueryClient } from '@tanstack/react-query'
import Icon from '../components/Icon'
import StatCard from '../components/StatCard'
import { useWorkOrders, useTransitionWorkOrder, useDeleteWorkOrder } from '../hooks/useMaintenance'
import { maintenanceApi } from '../lib/api/maintenance'
import type { WorkOrder as ApiWorkOrder } from '../lib/api/maintenance'

/* ------------------------------------------------------------------ */
/*  Types (local view model)                                           */
/* ------------------------------------------------------------------ */

type DisplayStatus = 'SUBMITTED' | 'APPROVED' | 'REJECTED' | 'IN_PROGRESS' | 'COMPLETED' | 'VERIFIED'

interface WorkOrderItem {
  id: string
  apiId: string
  title: string
  status: DisplayStatus
  requestor: { zh: string; en: string; avatar: string }
  ciName: string
  assetId?: string
  reason: string
  createdAt: string
  priority: string
  sla_deadline?: string
  sla_breached?: boolean
  sla_warning_sent?: boolean
}

/* ------------------------------------------------------------------ */
/*  Mapping helpers                                                    */
/* ------------------------------------------------------------------ */

const STATUS_MAP: Record<string, DisplayStatus> = {
  submitted: 'SUBMITTED',
  approved: 'APPROVED',
  rejected: 'REJECTED',
  in_progress: 'IN_PROGRESS',
  completed: 'COMPLETED',
  verified: 'VERIFIED',
}

function mapStatus(s: string): DisplayStatus {
  return STATUS_MAP[s?.toLowerCase()] ?? 'SUBMITTED'
}

// SLA fields are server-side extensions not yet in the generated OpenAPI schema.
// asset_id is in the CreateWorkOrder request shape but missing from the
// WorkOrder response schema — widen the type here so the list can deep-link.
type ApiWorkOrderWithSLA = ApiWorkOrder & {
  sla_deadline?: string
  sla_breached?: boolean
  sla_warning_sent?: boolean
  asset_id?: string
}

function toWorkOrderItem(wo: ApiWorkOrderWithSLA): WorkOrderItem {
  const initials = wo.title.slice(0, 2).toUpperCase()
  return {
    id: wo.code || wo.id,
    apiId: wo.id,
    title: wo.title,
    status: mapStatus(wo.status),
    requestor: { zh: wo.assignee_id ?? '', en: wo.assignee_id ?? '', avatar: initials },
    ciName: wo.description?.split(' ')[0] ?? '',
    assetId: wo.asset_id ?? undefined,
    reason: wo.description ?? '',
    createdAt: wo.scheduled_start?.slice(0, 16).replace('T', ' ') ?? '',
    priority: wo.priority ?? 'MEDIUM',
    sla_deadline: wo.sla_deadline,
    sla_breached: wo.sla_breached,
    sla_warning_sent: wo.sla_warning_sent,
  }
}

const FILTER_TABS = [
  { key: 'all', labelKey: 'work_order.filter_all', icon: 'list_alt' },
  { key: 'review', labelKey: 'work_order.filter_review', icon: 'rate_review' },
  { key: 'approve', labelKey: 'work_order.filter_approve', icon: 'approval' },
  { key: 'sort', labelKey: 'work_order.filter_sort', icon: 'filter_list' },
]

const STATUS_COLORS: Record<string, string> = {
  SUBMITTED: 'bg-[#1e3a5f] text-primary',
  APPROVED: 'bg-[#064e3b] text-[#34d399]',
  REJECTED: 'bg-error-container text-on-error-container',
  IN_PROGRESS: 'bg-[#92400e] text-[#fbbf24]',
  COMPLETED: 'bg-surface-container-highest text-on-surface-variant',
  VERIFIED: 'bg-[#064e3b] text-[#34d399]',
}

/* ------------------------------------------------------------------ */
/*  Sub-components                                                     */
/* ------------------------------------------------------------------ */

function OrderStatusBadge({ status }: { status: string }) {
  const colors = STATUS_COLORS[status] ?? STATUS_COLORS.SUBMITTED
  return (
    <span className={`inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-[0.6875rem] font-semibold uppercase tracking-wider ${colors}`}>
      <span className="w-1.5 h-1.5 rounded-full bg-current" />
      {status}
    </span>
  )
}

function AvatarBadge({ initials }: { initials: string }) {
  return (
    <span className="inline-flex items-center justify-center w-8 h-8 rounded-full bg-primary/20 text-primary text-xs font-bold shrink-0">
      {initials}
    </span>
  )
}

function WorkOrderCard({
  order,
  isSelected,
  onSelect,
  onApprove,
  onReject,
  onTransition,
  onDelete,
}: {
  order: WorkOrderItem
  isSelected: boolean
  onSelect: () => void
  onApprove?: (id: string) => void
  onReject?: (id: string) => void
  onTransition?: (id: string, status: string) => void
  onDelete?: (id: string) => void
}) {
  const { t } = useTranslation()
  const cardNavigate = useNavigate()
  const displayStatus = order.status

  return (
    <button
      type="button"
      onClick={onSelect}
      className={`w-full text-left rounded-xl p-5 transition-colors duration-150 ${
        isSelected
          ? 'bg-surface-container-high ring-1 ring-primary/30'
          : 'bg-surface-container hover:bg-surface-container-high'
      }`}
    >
      {/* Top row */}
      <div className="flex items-start justify-between gap-4 mb-3">
        <div className="flex-1 min-w-0">
          <h3 className="font-headline font-semibold text-on-surface truncate">{order.title}</h3>
          <span className="text-xs text-on-surface-variant font-mono">{order.id}</span>
        </div>
        <div className="flex items-center gap-2">
          {order.sla_deadline && (
            <span className={`text-[0.6rem] px-1.5 py-0.5 rounded ${
              order.sla_breached ? 'bg-error-container text-on-error-container' :
              order.sla_warning_sent ? 'bg-[#92400e] text-[#fbbf24]' :
              'bg-surface-container-highest text-on-surface-variant'
            }`}>
              SLA: {new Date(order.sla_deadline).toLocaleString()}
            </span>
          )}
          <OrderStatusBadge status={displayStatus} />
        </div>
      </div>

      {/* Detail grid */}
      <div className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm mb-4">
        <div className="flex items-center gap-2">
          <AvatarBadge initials={order.requestor.avatar} />
          <div className="min-w-0">
            <div className="text-on-surface truncate">{order.requestor.zh}</div>
            <div className="text-xs text-on-surface-variant">{order.requestor.en}</div>
          </div>
        </div>
        <div>
          <span className="text-[0.625rem] uppercase tracking-wider text-on-surface-variant block">{t('work_order.label_ci_name')}</span>
          {order.assetId ? (
            <span className="cursor-pointer text-primary hover:underline font-mono text-xs" onClick={(e) => { e.stopPropagation(); cardNavigate(`/assets/${order.assetId}`); }}>{order.ciName}</span>
          ) : (
            <span className="text-on-surface font-mono text-xs">{order.ciName}</span>
          )}
        </div>
        <div>
          <span className="text-[0.625rem] uppercase tracking-wider text-on-surface-variant block">{t('work_order.label_reason')}</span>
          <span className="text-on-surface text-xs">{order.reason}</span>
        </div>
        <div>
          <span className="text-[0.625rem] uppercase tracking-wider text-on-surface-variant block">{t('work_order.label_created')}</span>
          <span className="text-on-surface font-mono text-xs">{order.createdAt}</span>
        </div>
      </div>

      {/* Actions row */}
      <div className="flex items-center gap-3">
        <span
          className="inline-flex items-center gap-1 text-xs text-primary cursor-pointer hover:underline"
          onClick={(e) => { e.stopPropagation(); cardNavigate('/maintenance/task'); }}
        >
          <Icon name="open_in_new" className="text-[16px]" />
          查看任務
        </span>
        {displayStatus === 'SUBMITTED' && (
          <>
            <span
              className="inline-flex items-center gap-1.5 px-4 py-1.5 rounded-lg machined-gradient text-on-primary text-xs font-bold cursor-pointer"
              onClick={(e) => { e.stopPropagation(); onApprove?.(order.apiId); }}
            >
              <Icon name="check_circle" className="text-[16px]" />
              {t('work_order.btn_approve')}
            </span>
            <span
              className="inline-flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-error text-white text-xs font-bold cursor-pointer"
              onClick={(e) => { e.stopPropagation(); onReject?.(order.apiId); }}
            >
              <Icon name="block" className="text-[16px]" />
              {t('work_order.btn_reject')}
            </span>
            <span onClick={(e) => { e.stopPropagation(); toast.info('Coming Soon'); }} className="inline-flex items-center gap-1 text-xs text-primary cursor-pointer hover:underline">
              <Icon name="history" className="text-[16px]" />
              {t('work_order.btn_history')}
            </span>
          </>
        )}
        {displayStatus === 'APPROVED' && (
          <span className="inline-flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-[#064e3b] text-[#34d399] text-xs font-bold cursor-pointer">
            <Icon name="check_circle" className="text-[16px]" />
            {t('work_order.status_processed')}
          </span>
        )}
        {displayStatus === 'IN_PROGRESS' && (
          <span className="inline-flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-[#92400e] text-[#fbbf24] text-xs font-bold">
            <Icon name="sync" className="text-[16px]" />
            {t('work_order.stat_change')}
          </span>
        )}
        {displayStatus === 'COMPLETED' && (
          <>
            <span className="inline-flex items-center gap-1 text-xs text-on-surface-variant">
              <Icon name="check" className="text-[16px]" />
              {t('work_order.status_completed')}
            </span>
            <span
              className="text-xs text-[#34d399] font-semibold cursor-pointer hover:underline"
              onClick={(e) => { e.stopPropagation(); onTransition?.(order.apiId, 'verified'); }}
            >
              {t('work_order.btn_verify')}
            </span>
          </>
        )}
        {displayStatus === 'VERIFIED' && (
          <span className="inline-flex items-center gap-1 text-xs text-[#34d399]">
            <Icon name="verified" className="text-[16px]" />
            {t('work_order.btn_verify')}
          </span>
        )}
        {displayStatus === 'REJECTED' && (
          <>
            <span className="inline-flex items-center gap-1 text-xs text-error">
              <Icon name="block" className="text-[16px]" />
              {t('work_order.status_rejected')}
            </span>
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation()
                onTransition?.(order.apiId, 'submitted')
              }}
              className="text-xs text-primary font-semibold hover:underline"
            >
              {t('work_order.btn_resubmit')}
            </button>
          </>
        )}
        {(displayStatus === 'SUBMITTED' || displayStatus === 'REJECTED') && onDelete && (
          <span
            className="inline-flex items-center gap-1 text-xs text-error cursor-pointer hover:underline ml-auto"
            onClick={(e) => {
              e.stopPropagation()
              if (confirm(t('work_order.confirm_delete', { id: order.id }))) {
                onDelete(order.apiId)
              }
            }}
          >
            <Icon name="delete" className="text-[16px]" />
            {t('work_order.btn_delete')}
          </span>
        )}
      </div>
    </button>
  )
}

function AiPanel({ order }: { order: WorkOrderItem | null }) {
  const { t } = useTranslation()
  if (!order) {
    return (
      <div className="bg-surface-container rounded-xl p-6 flex flex-col items-center justify-center h-full gap-3 text-center">
        <Icon name="smart_toy" className="text-[40px] text-primary/40" />
        <p className="text-sm text-on-surface-variant">
          {t('work_order.ai_panel_empty')}
        </p>
      </div>
    )
  }

  return (
    <div className="bg-surface-container rounded-xl p-6 flex flex-col gap-5">
      {/* Header */}
      <div className="flex items-center gap-3">
        <div className="w-9 h-9 rounded-lg machined-gradient flex items-center justify-center shrink-0">
          <Icon name="smart_toy" className="text-[20px] text-on-primary" />
        </div>
        <div>
          <h3 className="font-headline font-bold text-on-surface text-sm">{t('work_order.ai_panel_title')}</h3>
          <span className="text-[0.625rem] text-on-surface-variant uppercase tracking-wider">{t('work_order.ai_panel_subtitle')}</span>
        </div>
      </div>

      {/* Analysis card */}
      <div className="bg-surface-container-low rounded-lg p-4">
        <div className="flex items-center gap-2 mb-3">
          <Icon name="analytics" className="text-[16px] text-primary" />
          <span className="text-xs font-semibold text-primary uppercase tracking-wider">{t('work_order.ai_analysis_label')}</span>
        </div>
        <p className="text-sm text-on-surface leading-relaxed">
          {t('work_order.ai_analysis_text', { id: order.id.split('-').pop() })}
        </p>
      </div>

      {/* Recommendations */}
      <div className="flex flex-col gap-3">
        <div className="flex items-start gap-3 text-sm">
          <Icon name="check_circle" className="text-[18px] text-[#34d399] mt-0.5 shrink-0" />
          <div>
            <span className="text-on-surface font-medium">{t('work_order.ai_check_dependency')}</span>
            <p className="text-xs text-on-surface-variant mt-0.5">{t('work_order.ai_check_dependency_detail')}</p>
          </div>
        </div>
        <div className="flex items-start gap-3 text-sm">
          <Icon name="check_circle" className="text-[18px] text-[#34d399] mt-0.5 shrink-0" />
          <div>
            <span className="text-on-surface font-medium">{t('work_order.ai_check_window')}</span>
            <p className="text-xs text-on-surface-variant mt-0.5">{t('work_order.ai_check_window_detail')}</p>
          </div>
        </div>
        <div className="flex items-start gap-3 text-sm">
          <Icon name="warning" className="text-[18px] text-[#fbbf24] mt-0.5 shrink-0" />
          <div>
            <span className="text-on-surface font-medium">{t('work_order.ai_warning_backup')}</span>
            <p className="text-xs text-on-surface-variant mt-0.5">{t('work_order.ai_warning_backup_detail')}</p>
          </div>
        </div>
        <div className="flex items-start gap-3 text-sm">
          <Icon name="info" className="text-[18px] text-primary mt-0.5 shrink-0" />
          <div>
            <span className="text-on-surface font-medium">{t('work_order.ai_info_similar')}</span>
            <p className="text-xs text-on-surface-variant mt-0.5">{t('work_order.ai_info_similar_detail')}</p>
          </div>
        </div>
      </div>

      {/* Risk score */}
      <div className="bg-surface-container-low rounded-lg p-4">
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs text-on-surface-variant uppercase tracking-wider">{t('work_order.ai_risk_score')}</span>
          <span className="text-sm font-bold text-[#fbbf24]">MEDIUM</span>
        </div>
        <div className="w-full h-1.5 rounded-full bg-surface-container-highest">
          <div className="h-full w-[45%] rounded-full bg-[#fbbf24]" />
        </div>
      </div>

      {/* Action button */}
      <button
        type="button"
        onClick={() => toast.info('AI Review: Coming Soon')}
        className="w-full py-3 rounded-lg machined-gradient text-on-primary text-sm font-bold flex items-center justify-center gap-2 cursor-pointer hover:opacity-90 transition-opacity"
      >
        <Icon name="auto_fix_high" className="text-[18px]" />
        {t('work_order.btn_execute_auto_review')}
      </button>
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Main page                                                          */
/* ------------------------------------------------------------------ */

export default function WorkOrder() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [activeTab, setActiveTab] = useState('all')
  const [selectedOrderId, setSelectedOrderId] = useState<string>('')
  const [currentPage, setCurrentPage] = useState(1)
  const { data: woResponse, isLoading, error } = useWorkOrders({ page: String(currentPage), page_size: '20' })
  const transitionWO = useTransitionWorkOrder()
  const deleteWO = useDeleteWorkOrder()
  const apiOrders: ApiWorkOrder[] = woResponse?.data ?? []
  const WORK_ORDERS = useMemo(() => apiOrders.map(toWorkOrderItem), [apiOrders])
  const [showApprovalModal, setShowApprovalModal] = useState(false)
  const [approvalAction, setApprovalAction] = useState<'approved' | 'rejected'>('approved')
  const [approvalComment, setApprovalComment] = useState('')
  const [approvalTargetId, setApprovalTargetId] = useState<string>('')
  const stats = useMemo(() => {
    const all = apiOrders ?? []
    return {
      submitted: all.filter(o => o.status === 'submitted').length,
      inProgress: all.filter(o => ['approved', 'in_progress'].includes(o.status)).length,
      completed: all.filter(o => o.status === 'completed').length,
      overdue: all.filter(o => (o as ApiWorkOrderWithSLA).sla_breached).length,
    }
  }, [apiOrders])
  const totalPages = woResponse?.pagination?.total_pages ?? 4
  const totalItems = woResponse?.pagination?.total ?? 64

  // Auto-select first order
  const effectiveSelectedId = selectedOrderId || WORK_ORDERS[0]?.id || ''
  const selectedOrder = WORK_ORDERS.find((o) => o.id === effectiveSelectedId) ?? null

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
        <p className="text-error text-sm">Failed to load work orders</p>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-6 p-6 min-h-screen font-body text-on-surface">
      {/* ---- Breadcrumb ---- */}
      <nav className="flex items-center gap-2 text-[0.6875rem] uppercase tracking-[0.08em]">
        <span className="text-on-surface-variant cursor-pointer hover:text-primary" onClick={() => navigate('/maintenance')}>{t('work_order.breadcrumb_maintenance')}</span>
        <Icon name="chevron_right" className="text-[14px] text-on-surface-variant" />
        <span className="text-primary font-semibold">{t('work_order.breadcrumb_work_order_approval')}</span>
      </nav>

      {/* ---- Header ---- */}
      <div className="flex items-end justify-between gap-4 flex-wrap">
        <div>
          <h1 className="font-headline font-bold text-2xl text-on-surface">
            {t('work_order.title_zh')} <span className="text-on-surface-variant font-normal text-lg">({t('work_order.title')})</span>
          </h1>
          <p className="text-sm text-on-surface-variant mt-1">{t('work_order.subtitle')}</p>
        </div>
        <button
          type="button"
          onClick={() => navigate('/maintenance/add')}
          className="inline-flex items-center gap-2 px-5 py-2.5 rounded-lg machined-gradient text-on-primary text-sm font-bold hover:opacity-90 transition-opacity shrink-0"
        >
          <Icon name="add" className="text-[18px]" />
          {t('work_order.btn_new_change_request')}
        </button>
      </div>

      {/* ---- Stats row ---- */}
      <div className="grid grid-cols-5 gap-4">
        <StatCard icon="pending_actions" label={t('work_order.stat_wait')} value={String(stats.submitted).padStart(2, '0')} sub={`${stats.submitted} pending`} subColor="text-primary" />
        <StatCard icon="sync" label={t('work_order.stat_change')} value={String(stats.inProgress).padStart(2, '0')} sub={`${stats.inProgress} active`} subColor="text-[#fbbf24]" />
        <StatCard icon="build" label={t('work_order.stat_repair')} value={String(stats.overdue).padStart(2, '0')} sub={`${stats.overdue} overdue`} subColor="text-error" />
        <StatCard icon="task_alt" label={t('work_order.stat_done')} value={String(stats.completed).padStart(2, '0')} sub={`${apiOrders.length} total`} subColor="text-[#34d399]" />

        {/* System Health card */}
        <div className="bg-surface-container-low rounded-lg p-5 flex flex-col justify-between gap-2">
          <div className="flex items-center justify-between">
            <span className="font-label text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant">
              {t('work_order.stat_system_health')}
            </span>
            <span className="w-2.5 h-2.5 rounded-full bg-[#34d399] animate-pulse" />
          </div>
          <div>
            <p className="text-sm text-on-surface font-medium">{t('work_order.system_health_normal')}</p>
            <p className="text-[0.625rem] uppercase tracking-wider text-[#34d399] font-semibold mt-1">
              {t('work_order.system_health_pulse')}
            </p>
          </div>
        </div>
      </div>

      {/* ---- Filter tabs ---- */}
      <div className="flex items-center gap-1 bg-surface-container-low rounded-xl p-1">
        {FILTER_TABS.map((tab) => (
          <button
            key={tab.key}
            type="button"
            onClick={() => setActiveTab(tab.key)}
            className={`inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
              activeTab === tab.key
                ? 'bg-surface-container-high text-on-surface'
                : 'text-on-surface-variant hover:text-on-surface'
            }`}
          >
            <Icon name={tab.icon} className="text-[18px]" />
            {t(tab.labelKey)}
          </button>
        ))}
      </div>

      {/* ---- Main content: list + AI panel ---- */}
      <div className="grid grid-cols-[1fr_380px] gap-5 flex-1">
        {/* Work order list */}
        <div className="flex flex-col gap-3">
          {WORK_ORDERS.filter((order) => {
            if (activeTab === 'all') return true
            if (activeTab === 'review') return order.status === 'SUBMITTED'
            if (activeTab === 'approve') return order.status === 'APPROVED' || order.status === 'IN_PROGRESS'
            return true
          }).map((order) => (
            <WorkOrderCard
              key={order.id}
              order={order}
              isSelected={order.id === effectiveSelectedId}
              onSelect={() => setSelectedOrderId(order.id)}
              onApprove={(id) => {
                setApprovalTargetId(id)
                setApprovalAction('approved')
                setShowApprovalModal(true)
              }}
              onReject={(id) => {
                setApprovalTargetId(id)
                setApprovalAction('rejected')
                setShowApprovalModal(true)
              }}
              onTransition={(id, status) => {
                transitionWO.mutate({ id, data: { status, comment: 'Verified' } })
              }}
              onDelete={(id) => {
                deleteWO.mutate(id, {
                  onSuccess: () => toast.success(t('work_order.delete_success')),
                })
              }}
            />
          ))}

          {/* Pagination */}
          <div className="flex items-center justify-between mt-2 text-sm text-on-surface-variant">
            <span>
              {t('work_order.pagination_page', { current: currentPage, total: totalPages, items: totalItems })}
            </span>
            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                className="inline-flex items-center justify-center w-8 h-8 rounded-lg bg-surface-container hover:bg-surface-container-high transition-colors disabled:opacity-40"
                disabled={currentPage === 1}
              >
                <Icon name="chevron_left" className="text-[18px]" />
              </button>
              {Array.from({ length: Math.min(totalPages, 7) }, (_, i) => i + 1).map((page) => (
                <button
                  key={page}
                  type="button"
                  onClick={() => setCurrentPage(page)}
                  className={`inline-flex items-center justify-center w-8 h-8 rounded-lg text-xs font-semibold transition-colors ${
                    page === currentPage
                      ? 'machined-gradient text-on-primary'
                      : 'bg-surface-container text-on-surface-variant hover:bg-surface-container-high'
                  }`}
                >
                  {page}
                </button>
              ))}
              <button
                type="button"
                onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
                className="inline-flex items-center justify-center w-8 h-8 rounded-lg bg-surface-container hover:bg-surface-container-high transition-colors disabled:opacity-40"
                disabled={currentPage === totalPages}
              >
                <Icon name="chevron_right" className="text-[18px]" />
              </button>
            </div>
          </div>
        </div>

        {/* AI suggestion panel */}
        <div className="sticky top-6 self-start">
          <AiPanel order={selectedOrder} />
        </div>
      </div>

      {/* Approval / Reject modal */}
      {showApprovalModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setShowApprovalModal(false)}>
          <div className="bg-surface-container p-6 rounded-xl w-96 space-y-4" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-bold text-on-surface">
              {approvalAction === 'approved' ? t('work_order.approve_title') : t('work_order.reject_title')}
            </h3>
            <textarea
              value={approvalComment}
              onChange={e => setApprovalComment(e.target.value)}
              placeholder={approvalAction === 'approved' ? t('work_order.approve_reason_placeholder') : t('work_order.reject_reason_placeholder')}
              className="w-full p-3 bg-surface-container-low rounded-lg border border-surface-container-highest text-on-surface text-sm min-h-[100px]"
              required
            />
            <div className="flex gap-2 justify-end">
              <button onClick={() => { setShowApprovalModal(false); setApprovalComment('') }}
                className="px-4 py-2 rounded bg-surface-container-high text-on-surface-variant">
                {t('common.cancel')}
              </button>
              <button
                onClick={async () => {
                  if (!approvalComment.trim()) { toast.error(t('work_order.comment_required')); return }
                  if (!approvalTargetId) return
                  // Re-validate latest server state to catch concurrent approvals
                  // between modal-open and confirm-click.
                  try {
                    const fresh = await queryClient.fetchQuery({
                      queryKey: ['workOrders', approvalTargetId],
                      queryFn: () => maintenanceApi.getById(approvalTargetId),
                      staleTime: 0,
                    })
                    const currentStatus = fresh?.data?.status?.toLowerCase()
                    if (currentStatus !== 'submitted') {
                      toast.error(t('work_order.already_decided', { status: currentStatus ?? 'unknown' }))
                      setShowApprovalModal(false)
                      setApprovalComment('')
                      queryClient.invalidateQueries({ queryKey: ['workOrders'] })
                      return
                    }
                  } catch {
                    toast.error(t('work_order.refresh_failed'))
                    return
                  }
                  transitionWO.mutate(
                    { id: approvalTargetId, data: { status: approvalAction, comment: approvalComment } },
                    {
                      onSuccess: () => {
                        setShowApprovalModal(false)
                        setApprovalComment('')
                        toast.success(approvalAction === 'approved' ? t('work_order.approved_success') : t('work_order.rejected_success'))
                      },
                      onError: (err: unknown) => {
                        const apiError = err as { status?: number; code?: string; message?: string }
                        if (apiError.status === 403 || apiError.code === 'FORBIDDEN') {
                          toast.error(t('work_order.already_decided', { status: 'changed' }))
                          queryClient.invalidateQueries({ queryKey: ['workOrders'] })
                          setShowApprovalModal(false)
                          setApprovalComment('')
                        } else {
                          toast.error(apiError.message || t('work_order.transition_failed'))
                        }
                      },
                    }
                  )
                }}
                disabled={transitionWO.isPending}
                className={`px-4 py-2 rounded text-white ${approvalAction === 'approved' ? 'bg-[#064e3b]' : 'bg-error'} disabled:opacity-50`}
              >
                {transitionWO.isPending ? t('common.saving') : (approvalAction === 'approved' ? t('work_order.btn_approve') : t('work_order.btn_reject'))}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
