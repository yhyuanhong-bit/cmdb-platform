import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { useCreateWorkOrder } from '../hooks/useMaintenance'

const assignees = [
  { id: 1, name: 'Chen Wei', nameCn: '陳偉', initials: 'CW' },
  { id: 2, name: 'Elena Rossi', nameCn: '', initials: 'ER' },
]

export default function AddMaintenanceTask() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const createWorkOrder = useCreateWorkOrder()

  const [assetName, setAssetName] = useState('')
  const [taskType, setTaskType] = useState('repair')
  const [priority, setPriority] = useState<'low' | 'medium' | 'high'>('low')
  const [scheduledDate, setScheduledDate] = useState('')
  const [scheduledTime, setScheduledTime] = useState('')
  const [selectedAssignees, setSelectedAssignees] = useState<number[]>([])
  const [description, setDescription] = useState('')

  const toggleAssignee = (id: number) => {
    setSelectedAssignees((prev) =>
      prev.includes(id) ? prev.filter((a) => a !== id) : [...prev, id]
    )
  }

  return (
    <div className="min-h-screen bg-surface p-6 font-body text-on-surface flex flex-col">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 mb-6">
        <button
          onClick={() => navigate('/maintenance')}
          className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label hover:text-primary transition-colors"
        >
          {t('add_maintenance_task.breadcrumb_maintenance')}
        </button>
        <span className="material-symbols-outlined text-on-surface-variant text-[16px]">chevron_right</span>
        <span className="text-[0.6875rem] uppercase tracking-[0.05rem] text-primary font-label">
          {t('add_maintenance_task.breadcrumb_new_entry')} 20-ABCE
        </span>
      </div>

      {/* Title Row */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="font-headline font-bold text-2xl text-on-surface">{t('add_maintenance_task.title_zh')}</h1>
          <p className="text-on-surface-variant text-sm mt-1">{t('add_maintenance_task.title')}</p>
        </div>
        <span className="px-3 py-1.5 rounded text-[0.6875rem] font-semibold uppercase tracking-wider bg-[#064e3b] text-[#34d399]">
          {t('add_maintenance_task.active_session')}
        </span>
      </div>

      {/* Form */}
      <div className="bg-surface-container rounded-lg p-6 flex-1">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-x-8 gap-y-5">
          {/* Asset Name */}
          <div>
            <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
              {t('add_maintenance_task.label_asset_name_zh')} <span className="normal-case">({t('add_maintenance_task.label_asset_name')})</span>
            </label>
            <div className="relative">
              <input
                type="text"
                value={assetName}
                onChange={(e) => setAssetName(e.target.value)}
                placeholder={t('add_maintenance_task.placeholder_search_asset')}
                className="w-full bg-surface-container-low rounded-lg pl-10 pr-4 py-2.5 text-sm text-on-surface placeholder:text-on-surface-variant/50 outline-none focus:ring-1 focus:ring-primary/40"
              />
              <span className="material-symbols-outlined text-on-surface-variant text-[18px] absolute left-3 top-1/2 -translate-y-1/2">
                search
              </span>
            </div>
          </div>

          {/* Task Type */}
          <div>
            <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
              {t('add_maintenance_task.label_task_type_zh')} <span className="normal-case">({t('add_maintenance_task.label_task_type')})</span>
            </label>
            <select
              value={taskType}
              onChange={(e) => setTaskType(e.target.value)}
              className="w-full bg-surface-container-low rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40 appearance-none"
            >
              <option value="repair">{t('add_maintenance_task.option_repair')}</option>
              <option value="inspection">{t('add_maintenance_task.option_inspection')}</option>
              <option value="replacement">{t('add_maintenance_task.option_replacement')}</option>
              <option value="upgrade">{t('add_maintenance_task.option_upgrade')}</option>
            </select>
          </div>

          {/* Priority Level */}
          <div>
            <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
              {t('add_maintenance_task.label_priority_zh')} <span className="normal-case">({t('add_maintenance_task.label_priority')})</span>
            </label>
            <div className="flex gap-2">
              {([
                { key: 'low', label: 'Medium', color: 'bg-[#064e3b] text-[#34d399]', activeRing: 'ring-[#34d399]/50' },
                { key: 'medium', label: 'Medium', color: 'bg-[#92400e] text-[#fbbf24]', activeRing: 'ring-[#fbbf24]/50' },
                { key: 'high', label: t('add_maintenance_task.priority_high'), color: 'bg-error-container text-on-error-container', activeRing: 'ring-error/50' },
              ] as const).map((p) => (
                <button
                  key={p.key}
                  onClick={() => setPriority(p.key)}
                  className={`flex-1 py-2.5 rounded-lg text-sm font-semibold transition-all ${p.color} ${
                    priority === p.key ? `ring-2 ${p.activeRing}` : 'opacity-50'
                  }`}
                >
                  {p.label}
                </button>
              ))}
            </div>
          </div>

          {/* Scheduled Date & Time */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
                {t('add_maintenance_task.label_scheduled_date_zh')} <span className="normal-case">({t('add_maintenance_task.label_scheduled_date')})</span>
              </label>
              <input
                type="date"
                value={scheduledDate}
                onChange={(e) => setScheduledDate(e.target.value)}
                className="w-full bg-surface-container-low rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40 [color-scheme:dark]"
              />
            </div>
            <div>
              <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
                {t('add_maintenance_task.label_scheduled_time_zh')} <span className="normal-case">({t('add_maintenance_task.label_scheduled_time')})</span>
              </label>
              <input
                type="time"
                value={scheduledTime}
                onChange={(e) => setScheduledTime(e.target.value)}
                className="w-full bg-surface-container-low rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40 [color-scheme:dark]"
              />
            </div>
          </div>

          {/* Assigned To */}
          <div>
            <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
              {t('add_maintenance_task.label_assigned_to_zh')} <span className="normal-case">({t('add_maintenance_task.label_assigned_to')})</span>
            </label>
            <div className="flex gap-3">
              {assignees.map((a) => (
                <button
                  key={a.id}
                  onClick={() => toggleAssignee(a.id)}
                  className={`flex items-center gap-3 px-4 py-2.5 rounded-lg transition-all ${
                    selectedAssignees.includes(a.id)
                      ? 'bg-on-primary-container/20 ring-1 ring-on-primary-container/40'
                      : 'bg-surface-container-low hover:bg-surface-container-high'
                  }`}
                >
                  <div className="w-8 h-8 rounded-full bg-surface-container-high flex items-center justify-center text-[0.6875rem] font-semibold text-on-surface-variant">
                    {a.initials}
                  </div>
                  <div className="text-left">
                    <p className="text-sm text-on-surface">{a.name}</p>
                    {a.nameCn && <p className="text-xs text-on-surface-variant">{a.nameCn}</p>}
                  </div>
                </button>
              ))}
            </div>
          </div>

          {/* Description */}
          <div className="lg:col-span-2">
            <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
              {t('add_maintenance_task.label_description_zh')} <span className="normal-case">({t('add_maintenance_task.label_description')})</span>
            </label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={4}
              placeholder={t('add_maintenance_task.placeholder_description')}
              className="w-full bg-surface-container-low rounded-lg px-4 py-3 text-sm text-on-surface placeholder:text-on-surface-variant/50 outline-none focus:ring-1 focus:ring-primary/40 resize-none"
            />
          </div>
        </div>

        {/* Action Buttons */}
        <div className="flex items-center justify-end gap-3 mt-8">
          <button
            onClick={() => navigate('/maintenance')}
            className="px-6 py-2.5 rounded-lg bg-surface-container-low text-on-surface-variant text-sm font-semibold hover:bg-surface-container-high transition-colors"
          >
            {t('add_maintenance_task.btn_cancel')}
          </button>
          <button
            disabled={createWorkOrder.isPending}
            onClick={() => {
              const scheduled = scheduledDate && scheduledTime
                ? new Date(`${scheduledDate}T${scheduledTime}`).toISOString()
                : new Date().toISOString()
              createWorkOrder.mutate(
                {
                  title: `${taskType} - ${assetName}`,
                  type: taskType,
                  priority: priority.toUpperCase(),
                  description,
                  scheduled_start: scheduled,
                  scheduled_end: scheduled,
                },
                { onSuccess: () => navigate('/maintenance') },
              )
            }}
            className="px-6 py-2.5 rounded-lg bg-on-primary-container text-white text-sm font-semibold hover:bg-on-primary-container/90 transition-colors disabled:opacity-50"
          >
            {createWorkOrder.isPending ? t('common.saving') ?? 'Saving...' : t('add_maintenance_task.btn_create_task')}
          </button>
        </div>
      </div>

      {/* Footer */}
      <div className="flex items-center justify-between mt-4 px-2">
        <div className="flex items-center gap-4">
          <span className="text-[0.625rem] uppercase tracking-widest text-on-surface-variant/60">{t('add_maintenance_task.footer_descriptor')}</span>
          <span className="text-[0.625rem] uppercase tracking-widest text-on-surface-variant/60">{t('add_maintenance_task.footer_auto_log')}</span>
        </div>
        <span className="text-[0.625rem] uppercase tracking-widest text-on-surface-variant/60">{t('add_maintenance_task.footer_version')}</span>
      </div>
    </div>
  )
}
