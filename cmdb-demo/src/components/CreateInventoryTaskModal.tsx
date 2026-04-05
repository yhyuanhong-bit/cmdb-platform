import { useState } from 'react'
import { useCreateInventoryTask } from '../hooks/useInventory'

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
  const [formData, setFormData] = useState({ ...initial })
  const mutation = useCreateInventoryTask()

  if (!open) return null

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-[#1a1f2e] p-6 rounded-xl w-[28rem] space-y-4 max-h-[90vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-bold text-white">Create Inventory Task</h3>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Name *</label>
          <input value={formData.name} onChange={e => setFormData(p => ({ ...p, name: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="Task name" />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Method</label>
          <select value={formData.method} onChange={e => setFormData(p => ({ ...p, method: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="barcode">Barcode</option>
            <option value="rfid">RFID</option>
            <option value="manual">Manual</option>
          </select>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Planned Date</label>
          <input type="date" value={formData.planned_date} onChange={e => setFormData(p => ({ ...p, planned_date: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Assigned To</label>
          <input value={formData.assigned_to} onChange={e => setFormData(p => ({ ...p, assigned_to: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="User ID or name" />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Scope Location ID</label>
          <input value={formData.scope_location_id} onChange={e => setFormData(p => ({ ...p, scope_location_id: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="UUID (optional)" />
        </div>

        <div className="flex gap-2 justify-end pt-2">
          <button onClick={onClose} className="px-4 py-2 rounded bg-gray-700 text-white text-sm">Cancel</button>
          <button
            onClick={() => mutation.mutate(formData, { onSuccess: () => { onClose(); setFormData({ ...initial }) } })}
            disabled={mutation.isPending || !formData.name}
            className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50">
            {mutation.isPending ? 'Creating...' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  )
}
