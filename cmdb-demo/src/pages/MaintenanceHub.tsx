import { useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import Icon from '../components/Icon'
import StatusBadge from '../components/StatusBadge'
import { useWorkOrders } from '../hooks/useMaintenance'
import type { WorkOrder } from '../lib/api/maintenance'

/* ------------------------------------------------------------------ */
/*  Types (local view models)                                          */
/* ------------------------------------------------------------------ */

interface MaintenanceTask {
  id: string
  woId: string
  description: string
  asset: string
  priority: 'Critical' | 'High' | 'Medium' | 'Low'
  scheduledDate: string
  assignedTo: string
  status: string
}

interface MaintenanceRecord {
  id: string
  date: string
  asset: string
  type: string
  technician: string
  duration: string
  outcome: string
}

/* ------------------------------------------------------------------ */
/*  Helpers: map WorkOrder -> local view models                        */
/* ------------------------------------------------------------------ */

function mapPriority(p: string): 'Critical' | 'High' | 'Medium' | 'Low' {
  const map: Record<string, 'Critical' | 'High' | 'Medium' | 'Low'> = {
    CRITICAL: 'Critical', HIGH: 'High', MEDIUM: 'Medium', LOW: 'Low',
  }
  return map[p?.toUpperCase()] ?? 'Medium'
}

function toTask(wo: WorkOrder): MaintenanceTask {
  return {
    id: wo.code,
    woId: wo.id,
    description: wo.title,
    asset: wo.description?.split(' ')[0] ?? '',
    priority: mapPriority(wo.priority),
    scheduledDate: wo.scheduled_start?.slice(0, 10) ?? '',
    assignedTo: wo.assignee_id ?? '',
    status: wo.status?.replace(/_/g, ' ') ?? 'Pending',
  }
}

function toRecord(wo: WorkOrder): MaintenanceRecord {
  return {
    id: wo.code,
    date: (wo.actual_end ?? wo.scheduled_end)?.slice(0, 10) ?? '',
    asset: wo.description?.split(' ')[0] ?? '',
    type: wo.type ?? '',
    technician: wo.assignee_id ?? '',
    duration: wo.actual_start && wo.actual_end
      ? `${((new Date(wo.actual_end).getTime() - new Date(wo.actual_start).getTime()) / 3600000).toFixed(1)}h`
      : '-',
    outcome: wo.status.toLowerCase() === 'completed' ? 'Success' : wo.status ?? '',
  }
}

const priorityColors: Record<string, string> = {
  Critical: 'bg-error-container text-on-error-container',
  High: 'bg-[#92400e] text-[#fbbf24]',
  Medium: 'bg-[#1e3a5f] text-primary',
  Low: 'bg-surface-container-highest text-on-surface-variant',
}

interface CalendarBlock {
  day: string
  date: number
  slots: { label: string; color: string; time: string }[]
}

const weekData: CalendarBlock[] = [
  {
    day: 'Mon',
    date: 23,
    slots: [
      { label: 'HVAC Inspection', color: 'bg-[#1e3a5f]', time: '09:00-11:00' },
    ],
  },
  {
    day: 'Tue',
    date: 24,
    slots: [
      { label: 'Network Audit', color: 'bg-[#92400e]', time: '14:00-18:00' },
    ],
  },
  {
    day: 'Wed',
    date: 25,
    slots: [
      { label: 'FW Failover Test', color: 'bg-[#064e3b]', time: '06:00-08:00' },
      { label: 'PDU Calibration', color: 'bg-[#1e3a5f]', time: '13:00-15:00' },
    ],
  },
  {
    day: 'Thu',
    date: 26,
    slots: [
      { label: 'Cooling Repair', color: 'bg-error-container', time: '08:00-16:00' },
    ],
  },
  {
    day: 'Fri',
    date: 27,
    slots: [],
  },
  {
    day: 'Sat',
    date: 28,
    slots: [
      { label: 'UPS Battery Swap', color: 'bg-error-container', time: '02:00-06:00' },
    ],
  },
  {
    day: 'Sun',
    date: 29,
    slots: [
      { label: 'Firmware Update', color: 'bg-[#92400e]', time: '01:00-04:00' },
    ],
  },
]

function buildSummaryCards(workOrders: WorkOrder[]) {
  const scheduled = workOrders.filter((wo) => wo.status === 'SCHEDULED' || wo.status === 'PENDING').length
  const inProgress = workOrders.filter((wo) => wo.status === 'IN_PROGRESS').length
  const overdue = workOrders.filter((wo) => {
    if (wo.status === 'COMPLETED' || wo.status === 'CANCELLED') return false
    return wo.scheduled_end && new Date(wo.scheduled_end) < new Date()
  }).length
  const completed = workOrders.filter((wo) => wo.status === 'COMPLETED').length
  return [
    { label: 'Scheduled', labelKey: 'maintenance_schedule.scheduled_tasks', value: String(scheduled ?? 18), icon: 'calendar_month', color: 'text-primary' },
    { label: 'In Progress', labelKey: 'maintenance_schedule.in_progress', value: String(inProgress ?? 5), icon: 'pending_actions', color: 'text-[#fbbf24]' },
    { label: 'Overdue', labelKey: 'maintenance_schedule.overdue', value: String(overdue ?? 2), icon: 'warning', color: 'text-error' },
    { label: 'Completed', labelKey: 'maintenance_schedule.completed_this_month', value: String(completed ?? 34), icon: 'task_alt', color: 'text-[#34d399]' },
    { label: 'Total Records', labelKey: 'maintenance_records.total_records', value: String(workOrders.length ?? 0), icon: 'folder_open', color: 'text-on-surface-variant' },
  ]
}

/* ------------------------------------------------------------------ */
/*  Schedule View                                                      */
/* ------------------------------------------------------------------ */

function ScheduleView({
  search,
  navigate,
  t,
  tasks,
}: {
  search: string
  navigate: ReturnType<typeof useNavigate>
  t: ReturnType<typeof useTranslation>['t']
  tasks: MaintenanceTask[]
}) {
  const filteredTasks = tasks.filter(
    (task) => !search || task.description.toLowerCase().includes(search.toLowerCase()) || task.id.toLowerCase().includes(search.toLowerCase()),
  )

  return (
    <>
      {/* Weekly Calendar View */}
      <div className="mb-6 bg-surface-container rounded overflow-hidden">
        <div className="flex items-center justify-between bg-surface-container-low px-4 py-3">
          <h2 className="font-headline text-sm font-semibold text-on-surface">
            {t('maintenance_schedule.week_of')} March 23 - 29, 2026
          </h2>
          <div className="flex items-center gap-2">
            <button className="p-1.5 rounded bg-surface-container-high hover:bg-surface-container-highest transition-colors">
              <Icon name="chevron_left" className="text-[18px] text-on-surface-variant" />
            </button>
            <button className="px-3 py-1 rounded bg-surface-container-high text-xs text-on-surface-variant hover:bg-surface-container-highest transition-colors">
              {t('common.today')}
            </button>
            <button className="p-1.5 rounded bg-surface-container-high hover:bg-surface-container-highest transition-colors">
              <Icon name="chevron_right" className="text-[18px] text-on-surface-variant" />
            </button>
          </div>
        </div>
        <div className="grid grid-cols-7 gap-px bg-surface-container-low">
          {weekData.map((day) => (
            <div
              key={day.day}
              className={`bg-surface-container p-3 min-h-[120px] ${
                day.date === 28 ? 'bg-surface-container-high' : ''
              }`}
            >
              <div className="mb-2 flex items-center justify-between">
                <span className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant">
                  {day.day}
                </span>
                <span
                  className={`text-sm font-semibold ${
                    day.date === 28
                      ? 'flex h-6 w-6 items-center justify-center rounded-full bg-on-primary-container text-white text-xs'
                      : 'text-on-surface'
                  }`}
                >
                  {day.date}
                </span>
              </div>
              <div className="flex flex-col gap-1.5">
                {day.slots.map((slot, i) => (
                  <div
                    key={i}
                    className={`${slot.color} rounded px-2 py-1.5 cursor-pointer hover:brightness-125 transition-all`}
                  >
                    <p className="text-[0.625rem] font-semibold text-white truncate">
                      {slot.label}
                    </p>
                    <p className="text-[0.5625rem] text-white/70">{slot.time}</p>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Upcoming Tasks Table */}
      <div className="bg-surface-container rounded overflow-hidden">
        <div className="grid grid-cols-[110px_1fr_120px_90px_120px_140px_120px] items-center gap-2 bg-surface-container-low px-4 py-3 text-[0.6875rem] font-semibold uppercase tracking-wider text-on-surface-variant">
          <span>{t('maintenance_schedule.table_task_id')}</span>
          <span>{t('maintenance_schedule.table_description')}</span>
          <span>{t('maintenance_schedule.table_asset')}</span>
          <span>{t('maintenance_schedule.table_priority')}</span>
          <span>{t('maintenance_schedule.table_scheduled')}</span>
          <span>{t('maintenance_schedule.table_assigned_to')}</span>
          <span>{t('maintenance_schedule.table_status')}</span>
        </div>
        {filteredTasks.map((task, i) => (
          <div
            key={task.id}
            onClick={() => navigate('/maintenance/task/' + task.woId)}
            className={`grid grid-cols-[110px_1fr_120px_90px_120px_140px_120px] items-center gap-2 px-4 py-3 text-sm transition-colors hover:bg-surface-container-high cursor-pointer ${
              i % 2 === 1 ? 'bg-surface-container-low/40' : ''
            }`}
          >
            <span className="font-mono text-primary text-xs font-semibold">
              {task.id}
            </span>
            <span className="text-on-surface truncate">{task.description}</span>
            <span className="font-mono text-on-surface-variant text-xs">
              {task.asset}
            </span>
            <span>
              <span
                className={`px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${priorityColors[task.priority]}`}
              >
                {task.priority}
              </span>
            </span>
            <span className="text-on-surface-variant text-xs">{task.scheduledDate}</span>
            <span className="text-on-surface-variant text-xs">{task.assignedTo}</span>
            <span>
              <StatusBadge status={task.status} />
            </span>
          </div>
        ))}
      </div>
    </>
  )
}

/* ------------------------------------------------------------------ */
/*  Records View                                                       */
/* ------------------------------------------------------------------ */

function RecordsView({ search, t, records }: { search: string; t: ReturnType<typeof useTranslation>['t']; records: MaintenanceRecord[] }) {
  const filteredRecords = records.filter(
    (r) => !search || r.type.toLowerCase().includes(search.toLowerCase()) || r.id.toLowerCase().includes(search.toLowerCase()),
  )

  return (
    <div className="bg-surface-container rounded overflow-hidden">
      <div className="grid grid-cols-[110px_100px_120px_1fr_140px_80px_100px] items-center gap-2 bg-surface-container-low px-4 py-3 text-[0.6875rem] font-semibold uppercase tracking-wider text-on-surface-variant">
        <span>{t('maintenance_records.table_record_id')}</span>
        <span>{t('maintenance_records.table_date')}</span>
        <span>{t('maintenance_records.table_asset')}</span>
        <span>{t('maintenance_records.table_type')}</span>
        <span>{t('maintenance_records.table_technician')}</span>
        <span>{t('maintenance_records.table_duration')}</span>
        <span>{t('maintenance_records.table_outcome')}</span>
      </div>
      {filteredRecords.map((record, i) => (
        <div
          key={record.id}
          className={`grid grid-cols-[110px_100px_120px_1fr_140px_80px_100px] items-center gap-2 px-4 py-3 text-sm transition-colors hover:bg-surface-container-high cursor-pointer ${
            i % 2 === 1 ? 'bg-surface-container-low/40' : ''
          }`}
        >
          <span className="font-mono text-primary text-xs font-semibold">
            {record.id}
          </span>
          <span className="text-on-surface-variant text-xs">{record.date}</span>
          <span className="font-mono text-on-surface-variant text-xs">
            {record.asset}
          </span>
          <span className="text-on-surface text-xs">{record.type}</span>
          <span className="text-on-surface-variant text-xs">{record.technician}</span>
          <span className="font-mono text-on-surface text-xs">{record.duration}</span>
          <span>
            <StatusBadge status={record.outcome} />
          </span>
        </div>
      ))}
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  Main Page                                                          */
/* ------------------------------------------------------------------ */

export default function MaintenanceHub() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { data: woResponse, isLoading, error } = useWorkOrders()
  const workOrders: WorkOrder[] = woResponse?.data ?? []

  const tasks = useMemo(() =>
    workOrders.filter((wo) => wo.status.toLowerCase() !== 'completed').map(toTask),
    [workOrders],
  )
  const records = useMemo(() =>
    workOrders.filter((wo) => wo.status.toLowerCase() === 'completed').map(toRecord),
    [workOrders],
  )

  const [viewMode, setViewMode] = useState<'schedule' | 'records'>('schedule')
  const [search, setSearch] = useState('')
  const [dateFrom, setDateFrom] = useState('2026-03-01')
  const [dateTo, setDateTo] = useState('2026-03-28')
  const [typeFilter, setTypeFilter] = useState('All Types')
  const [statusFilter, setStatusFilter] = useState('All Status')

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
        <p className="text-error text-sm">Failed to load maintenance data</p>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-surface p-6 font-body text-on-surface">
      {/* Header */}
      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="font-headline text-2xl font-bold tracking-tight text-on-surface">
            {t('maintenance_schedule.title_zh')} / {t('maintenance_schedule.title')}
          </h1>
          <p className="mt-1 text-sm text-on-surface-variant">
            {t('maintenance_schedule.subtitle')}
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={() => navigate('/maintenance/dispatch')}
            className="flex items-center gap-1.5 bg-surface-container-high px-4 py-2.5 text-sm font-medium text-on-surface rounded hover:bg-surface-container-highest transition-all"
          >
            <Icon name="group" className="text-[18px]" />
            任務調度
          </button>
          <button
            onClick={() => navigate('/maintenance/workorder')}
            className="flex items-center gap-1.5 bg-surface-container-high px-4 py-2.5 text-sm font-medium text-on-surface rounded hover:bg-surface-container-highest transition-all"
          >
            <Icon name="assignment" className="text-[18px]" />
            {t('maintenance_schedule.workorder_management') ?? '\u5de5\u55ae\u7ba1\u7406'}
          </button>
          <button
            onClick={() => navigate('/maintenance/add')}
            className="flex items-center gap-1.5 bg-on-primary-container px-4 py-2.5 text-sm font-semibold text-white rounded hover:brightness-110 transition-all"
          >
            <Icon name="add" className="text-[18px]" />
            {t('maintenance_schedule.create_maintenance_window')}
          </button>
        </div>
      </div>

      {/* Summary Cards */}
      <div className="mb-6 grid grid-cols-5 gap-4">
        {buildSummaryCards(workOrders).map((card) => (
          <div
            key={card.labelKey}
            className={`bg-surface-container rounded p-4 flex items-center gap-4 ${
              card.label === 'Overdue' ? 'ring-1 ring-error/30' : ''
            }`}
          >
            <div className="flex h-11 w-11 items-center justify-center rounded bg-surface-container-high">
              <Icon name={card.icon} className={`text-[24px] ${card.color}`} />
            </div>
            <div>
              <p className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant">
                {t(card.labelKey)}
              </p>
              <p className={`font-headline text-2xl font-bold ${card.color}`}>
                {card.value}
              </p>
            </div>
          </div>
        ))}
      </div>

      {/* View Toggle + Filter Bar */}
      <div className="mb-4 flex flex-wrap items-center gap-3">
        {/* View Toggle */}
        <div className="flex bg-surface-container-low rounded overflow-hidden">
          <button
            onClick={() => setViewMode('schedule')}
            className={`flex items-center gap-1.5 px-3 py-2 text-xs font-semibold transition-colors ${
              viewMode === 'schedule'
                ? 'bg-on-primary-container text-white'
                : 'text-on-surface-variant hover:bg-surface-container-high'
            }`}
          >
            <Icon name="calendar_month" className="text-[16px]" />
            {t('maintenance_schedule.view_schedule') ?? '\u6392\u7a0b'}
          </button>
          <button
            onClick={() => setViewMode('records')}
            className={`flex items-center gap-1.5 px-3 py-2 text-xs font-semibold transition-colors ${
              viewMode === 'records'
                ? 'bg-on-primary-container text-white'
                : 'text-on-surface-variant hover:bg-surface-container-high'
            }`}
          >
            <Icon name="history" className="text-[16px]" />
            {t('maintenance_schedule.view_records') ?? '\u8a18\u9304'}
          </button>
        </div>

        {/* Search */}
        <div className="relative flex-1 min-w-[220px]">
          <Icon
            name="search"
            className="absolute left-3 top-1/2 -translate-y-1/2 text-on-surface-variant text-[20px]"
          />
          <input
            type="text"
            placeholder={t('maintenance_schedule.search_placeholder')}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full bg-surface-container-low py-2.5 pl-10 pr-4 text-sm text-on-surface placeholder:text-on-surface-variant/50 rounded focus:outline-none focus:ring-1 focus:ring-primary/40"
          />
        </div>

        {/* Date Range */}
        <div className="flex items-center gap-2">
          <label className="text-xs text-on-surface-variant">{t('common.from')}</label>
          <input
            type="date"
            value={dateFrom}
            onChange={(e) => setDateFrom(e.target.value)}
            className="bg-surface-container-low py-2 px-3 text-sm text-on-surface rounded focus:outline-none focus:ring-1 focus:ring-primary/40"
          />
        </div>
        <div className="flex items-center gap-2">
          <label className="text-xs text-on-surface-variant">{t('common.to')}</label>
          <input
            type="date"
            value={dateTo}
            onChange={(e) => setDateTo(e.target.value)}
            className="bg-surface-container-low py-2 px-3 text-sm text-on-surface rounded focus:outline-none focus:ring-1 focus:ring-primary/40"
          />
        </div>

        {/* Type Filter */}
        <select
          value={typeFilter}
          onChange={(e) => setTypeFilter(e.target.value)}
          className="bg-surface-container-low py-2.5 px-3 text-sm text-on-surface rounded appearance-none cursor-pointer focus:outline-none focus:ring-1 focus:ring-primary/40"
        >
          <option>{t('maintenance_records.all_types') ?? 'All Types'}</option>
          <option>Firmware Update</option>
          <option>Disk Replacement</option>
          <option>Preventive Maintenance</option>
          <option>Emergency Repair</option>
        </select>

        {/* Status Filter */}
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="bg-surface-container-low py-2.5 px-3 text-sm text-on-surface rounded appearance-none cursor-pointer focus:outline-none focus:ring-1 focus:ring-primary/40"
        >
          <option>{t('assets.all_status') ?? 'All Status'}</option>
          <option>Pending</option>
          <option>In Progress</option>
          <option>Completed</option>
          <option>Overdue</option>
        </select>
      </div>

      {/* Content Area */}
      {viewMode === 'schedule' && (
        <ScheduleView search={search} navigate={navigate} t={t} tasks={tasks.filter(task => {
          if (statusFilter !== 'All Status' && task.status.toLowerCase() !== statusFilter.toLowerCase()) return false
          return true
        })} />
      )}

      {viewMode === 'records' && (
        <RecordsView search={search} t={t} records={records.filter(r => {
          if (typeFilter !== 'All Types' && r.type !== typeFilter) return false
          if (statusFilter !== 'All Status' && r.outcome.toLowerCase() !== statusFilter.toLowerCase()) return false
          return true
        })} />
      )}

      {/* Pagination */}
      <div className="mt-4 flex items-center justify-between text-sm text-on-surface-variant">
        <span>
          {viewMode === 'schedule'
            ? t('maintenance_schedule.showing_upcoming_tasks', { count: tasks.length })
            : `${t('common.showing')} 1-${records.length} ${t('common.of')} ${records.length} ${t('common.records')}`}
        </span>
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
            {viewMode === 'records' ? '156' : '3'}
          </button>
          <button className="px-3 py-1.5 rounded bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest transition-colors">
            <Icon name="chevron_right" className="text-[18px]" />
          </button>
        </div>
      </div>
    </div>
  )
}
