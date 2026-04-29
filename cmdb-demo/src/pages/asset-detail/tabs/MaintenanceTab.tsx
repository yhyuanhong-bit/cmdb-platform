import { toast } from 'sonner'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useWorkOrders } from '../../../hooks/useMaintenance'
import type { WorkOrder } from '../../../lib/api/maintenance'

type MaintenanceEvent = {
  id: number
  date: string
  type: string
  description: string
  technician: string
  status: 'COMPLETED' | 'SCHEDULED'
  accent: string
}

const maintEvents: MaintenanceEvent[] = [
  { id: 1, date: '2024-03-15', type: 'Firmware Update', description: 'Patch v4.0.2 Security Hardening', technician: 'Alex J.', status: 'COMPLETED', accent: '#34d399' },
  { id: 2, date: '2024-03-28', type: 'Hardware Inspection', description: 'Routine check', technician: 'Marcus K.', status: 'SCHEDULED', accent: '#fbbf24' },
  { id: 3, date: '2024-02-12', type: 'Emergency PSU Swap', description: 'Power failure recovery', technician: 'Sarah L.', status: 'COMPLETED', accent: '#34d399' },
  { id: 4, date: '2024-01-05', type: 'Routine Database Optimization', description: 'Post-migration check', technician: 'Alex J.', status: 'COMPLETED', accent: '#34d399' },
  { id: 5, date: '2023-12-18', type: 'Thermal Paste Re-application', description: 'Maintenance re-adherence, high-temp warning', technician: 'Marcus K.', status: 'COMPLETED', accent: '#34d399' },
]

const OPEN_STATUSES = ['submitted', 'approved', 'in_progress']

function isOpen(status?: string): boolean {
  return !!status && OPEN_STATUSES.includes(status.toLowerCase())
}

function statusBadgeClass(status?: string): string {
  const s = (status ?? '').toLowerCase()
  if (s === 'completed' || s === 'verified') return 'bg-[#064e3b] text-[#34d399]'
  if (s === 'in_progress' || s === 'approved') return 'bg-[#92400e] text-[#fbbf24]'
  if (s === 'rejected') return 'bg-error-container text-on-error-container'
  return 'bg-surface-container-highest text-on-surface-variant'
}

export default function MaintenanceTab({ assetId }: { assetId?: string }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const woQuery = useWorkOrders(assetId ? { asset_id: assetId } : undefined)
  const workOrders: WorkOrder[] = woQuery.data?.data ?? []
  const openWorkOrders = workOrders.filter((wo) => isOpen(wo.status))

  return (
    <div className="space-y-6">
      {/* Title row */}
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
        <h2 className="font-headline font-bold text-lg tracking-tight text-on-surface">
          {t('asset_maint_history.title')}
        </h2>
        <div className="flex gap-3">
          <button onClick={() => toast.info('Coming Soon')} className="bg-surface-container-high px-5 py-2.5 rounded-lg text-xs font-semibold tracking-wider text-on-surface-variant uppercase hover:bg-surface-container-highest transition-colors">
            <span className="flex items-center gap-2">
              <span className="material-symbols-outlined text-[16px]">filter_list</span>
              {t('asset_maint_history.btn_filter_logs')}
            </span>
          </button>
          <button onClick={() => navigate('/maintenance/add')} className="bg-on-primary-container text-white px-5 py-2.5 rounded-lg text-xs font-semibold tracking-wider uppercase hover:brightness-110 transition-all">
            <span className="flex items-center gap-2">
              <span className="material-symbols-outlined text-[16px]">add</span>
              {t('asset_maint_history.btn_schedule_entry')}
            </span>
          </button>
        </div>
      </div>

      {/* Open Work Orders for this asset (real data) */}
      <div className="bg-surface-container rounded-2xl p-5">
        <div className="flex items-center justify-between mb-3">
          <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
            Open Work Orders
          </h3>
          <span className="text-xs text-on-surface-variant">
            {woQuery.isLoading ? '…' : `${openWorkOrders.length} open / ${workOrders.length} total`}
          </span>
        </div>
        {woQuery.isLoading ? (
          <div className="flex items-center justify-center py-6">
            <div className="animate-spin rounded-full h-5 w-5 border-2 border-primary border-t-transparent" />
          </div>
        ) : openWorkOrders.length === 0 ? (
          <p className="text-sm text-on-surface-variant italic">No open work orders for this asset.</p>
        ) : (
          <ul className="divide-y divide-surface-container-high">
            {openWorkOrders.map((wo) => (
              <li key={wo.id} className="py-3 first:pt-0 last:pb-0">
                <button
                  type="button"
                  onClick={() => navigate(`/maintenance/task/${wo.id}`)}
                  className="w-full text-left flex items-center justify-between gap-4 hover:bg-surface-container-high/40 rounded px-2 py-1.5 transition-colors"
                >
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-xs text-primary hover:underline">#{wo.code}</span>
                      <span className="text-sm font-medium text-on-surface truncate">{wo.title}</span>
                    </div>
                    <p className="text-xs text-on-surface-variant mt-0.5">
                      {wo.type} · priority: {wo.priority}
                      {wo.scheduled_start && ` · due ${wo.scheduled_start.slice(0, 10)}`}
                    </p>
                  </div>
                  <span className={`shrink-0 inline-flex items-center px-2.5 py-1 rounded text-[0.625rem] font-semibold tracking-wider uppercase ${statusBadgeClass(wo.status)}`}>
                    {wo.status}
                  </span>
                  <span className="material-symbols-outlined text-[18px] text-on-surface-variant shrink-0">chevron_right</span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>

      {/* Table */}
      <div className="bg-surface-container rounded-2xl overflow-hidden">
        <div className="grid grid-cols-[2.5fr_2fr_1.5fr_1fr] gap-4 px-6 py-4 bg-surface-container-low">
          <span className="text-[0.625rem] font-semibold tracking-widest text-on-surface-variant uppercase">{t('asset_maint_history.table_event_date')}</span>
          <span className="text-[0.625rem] font-semibold tracking-widest text-on-surface-variant uppercase">{t('asset_maint_history.table_type_of_work')}</span>
          <span className="text-[0.625rem] font-semibold tracking-widest text-on-surface-variant uppercase">{t('asset_maint_history.table_technician')}</span>
          <span className="text-[0.625rem] font-semibold tracking-widest text-on-surface-variant uppercase text-right">{t('asset_maint_history.table_workflow_status')}</span>
        </div>
        {maintEvents.map((evt) => (
          <div
            key={evt.id}
            className="grid grid-cols-[2.5fr_2fr_1.5fr_1fr] gap-4 px-6 py-5 items-center hover:bg-surface-container-high/50 transition-colors relative"
          >
            <div className="absolute left-0 top-2 bottom-2 w-[3px] rounded-r-full" style={{ backgroundColor: evt.accent }} />
            <div className="pl-2">
              <p className="text-on-surface text-sm font-medium">{evt.date}</p>
              <p className="text-on-surface-variant text-xs mt-0.5">{evt.description}</p>
            </div>
            <div><p className="text-on-surface text-sm font-medium">{evt.type}</p></div>
            <div><p className="text-on-surface text-sm">{evt.technician}</p></div>
            <div className="flex justify-end">
              <span
                className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-[0.625rem] font-semibold tracking-wider uppercase ${
                  evt.status === 'COMPLETED'
                    ? 'bg-[#064e3b] text-[#34d399]'
                    : 'bg-[#92400e] text-[#fbbf24]'
                }`}
              >
                <span className="w-1.5 h-1.5 rounded-full" style={{ backgroundColor: evt.status === 'COMPLETED' ? '#34d399' : '#fbbf24' }} />
                {evt.status}
              </span>
            </div>
          </div>
        ))}
      </div>

      {/* View all + Pagination */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <p className="text-xs text-on-surface-variant tracking-wider uppercase">
            {t('asset_maint_history.pagination_showing')}
          </p>
          <button
            onClick={() => navigate('/maintenance')}
            className="flex items-center gap-1 text-xs font-semibold text-primary hover:underline"
          >
            {t('asset_detail.btn_view_all_records')}
            <span className="material-symbols-outlined text-[14px]">arrow_forward</span>
          </button>
        </div>
        <div className="flex gap-2">
          <button className="bg-surface-container-high w-9 h-9 rounded-lg flex items-center justify-center hover:bg-surface-container-highest transition-colors text-on-surface-variant">
            <span className="material-symbols-outlined text-[18px]">chevron_left</span>
          </button>
          <button className="bg-surface-container-high w-9 h-9 rounded-lg flex items-center justify-center hover:bg-surface-container-highest transition-colors text-on-surface-variant">
            <span className="material-symbols-outlined text-[18px]">chevron_right</span>
          </button>
        </div>
      </div>

      {/* AI Insight Card */}
      <div className="bg-surface-container rounded-2xl p-6 flex items-start gap-4 max-w-xl ml-auto">
        <div className="w-10 h-10 rounded-xl bg-primary/10 flex items-center justify-center flex-shrink-0">
          <span className="material-symbols-outlined text-primary text-xl">auto_awesome</span>
        </div>
        <div>
          <p className="text-[0.625rem] font-semibold tracking-widest text-primary uppercase mb-1">{t('asset_maint_history.ai_insight_label')}</p>
          <p className="text-on-surface-variant text-sm leading-relaxed">
            {t('asset_maint_history.ai_insight_text')}
          </p>
        </div>
      </div>
    </div>
  )
}
