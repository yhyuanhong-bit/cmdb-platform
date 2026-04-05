import { useState } from 'react'
import { useCreateAdapter } from '../hooks/useIntegration'

interface Props {
  open: boolean
  onClose: () => void
}

const initial = {
  name: '',
  type: 'rest',
  direction: 'inbound',
  endpoint: '',
  enabled: true,
}

export default function CreateAdapterModal({ open, onClose }: Props) {
  const [formData, setFormData] = useState({ ...initial })
  const mutation = useCreateAdapter()

  if (!open) return null

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-[#1a1f2e] p-6 rounded-xl w-[28rem] space-y-4 max-h-[90vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-bold text-white">Create Adapter</h3>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Name *</label>
          <input value={formData.name} onChange={e => setFormData(p => ({ ...p, name: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="Adapter name" />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Type</label>
          <select value={formData.type} onChange={e => setFormData(p => ({ ...p, type: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="dify">Dify</option>
            <option value="rest">REST</option>
            <option value="grpc">gRPC</option>
            <option value="mqtt">MQTT</option>
          </select>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Direction</label>
          <select value={formData.direction} onChange={e => setFormData(p => ({ ...p, direction: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="inbound">Inbound</option>
            <option value="outbound">Outbound</option>
            <option value="bidirectional">Bidirectional</option>
          </select>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Endpoint</label>
          <input value={formData.endpoint} onChange={e => setFormData(p => ({ ...p, endpoint: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="https://..." />
        </div>

        <div className="flex items-center gap-2">
          <input type="checkbox" checked={formData.enabled} onChange={e => setFormData(p => ({ ...p, enabled: e.target.checked }))}
            className="rounded border-gray-700" />
          <label className="text-sm text-gray-400">Enabled</label>
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
