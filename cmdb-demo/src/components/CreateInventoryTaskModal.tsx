import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useCreateInventoryTask } from '../hooks/useInventory'
import { Modal } from './ui/Modal'

interface Props {
  open: boolean
  onClose: () => void
}

const initial = {
  name: '',
  method: 'barcode',
  planned_date: '',
  assigned_to: '',
  scope_location_id: '',
}

export default function CreateInventoryTaskModal({ open, onClose }: Props) {
  const { t } = useTranslation()
  const [formData, setFormData] = useState({ ...initial })
  const mutation = useCreateInventoryTask()

  return (
    <Modal open={open} onOpenChange={(next) => { if (!next) onClose() }}>
      <Modal.Header title={t('inventory_task_modal.title')} onClose={onClose} />
      <Modal.Body>
        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('inventory_task_modal.name_label')} *</label>
          <input value={formData.name} onChange={e => setFormData(p => ({ ...p, name: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('inventory_task_modal.name_placeholder')} />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('inventory_task_modal.method_label')}</label>
          <select value={formData.method} onChange={e => setFormData(p => ({ ...p, method: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="barcode">{t('inventory_task_modal.method_barcode')}</option>
            <option value="rfid">{t('inventory_task_modal.method_rfid')}</option>
            <option value="manual">{t('inventory_task_modal.method_manual')}</option>
          </select>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('inventory_task_modal.planned_date_label')}</label>
          <input type="date" value={formData.planned_date} onChange={e => setFormData(p => ({ ...p, planned_date: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('inventory_task_modal.assigned_to_label')}</label>
          <input value={formData.assigned_to} onChange={e => setFormData(p => ({ ...p, assigned_to: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('inventory_task_modal.assigned_to_placeholder')} />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('inventory_task_modal.scope_location_label')}</label>
          <input value={formData.scope_location_id} onChange={e => setFormData(p => ({ ...p, scope_location_id: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('inventory_task_modal.scope_location_placeholder')} />
        </div>
      </Modal.Body>
      <Modal.Footer>
        <button onClick={onClose} className="px-4 py-2 rounded bg-gray-700 text-white text-sm">{t('inventory_task_modal.btn_cancel')}</button>
        <button
          onClick={() => mutation.mutate(formData, { onSuccess: () => { onClose(); setFormData({ ...initial }) } })}
          disabled={mutation.isPending || !formData.name}
          className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50">
          {mutation.isPending ? t('inventory_task_modal.btn_creating') : t('inventory_task_modal.btn_create')}
        </button>
      </Modal.Footer>
    </Modal>
  )
}
