import { useState } from 'react'
import { useCreateAsset } from '../hooks/useAssets'

interface Props {
  open: boolean
  onClose: () => void
}

const initial = {
  asset_tag: '',
  name: '',
  type: 'server',
  sub_type: '',
  status: 'operational',
  bia_level: 'normal',
  vendor: '',
  model: '',
  serial_number: '',
}

export default function CreateAssetModal({ open, onClose }: Props) {
  const [formData, setFormData] = useState({ ...initial })
  const mutation = useCreateAsset()

  if (!open) return null

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-[#1a1f2e] p-6 rounded-xl w-[28rem] space-y-4 max-h-[90vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-bold text-white">Create Asset</h3>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Asset Tag *</label>
          <input value={formData.asset_tag} onChange={e => setFormData(p => ({ ...p, asset_tag: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="e.g. SRV-001" />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Name *</label>
          <input value={formData.name} onChange={e => setFormData(p => ({ ...p, name: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="Asset name" />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Type</label>
          <select value={formData.type} onChange={e => setFormData(p => ({ ...p, type: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="server">Server</option>
            <option value="network">Network</option>
            <option value="storage">Storage</option>
            <option value="power">Power</option>
          </select>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Sub Type</label>
          <input value={formData.sub_type} onChange={e => setFormData(p => ({ ...p, sub_type: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="e.g. rack-mount" />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Status</label>
          <select value={formData.status} onChange={e => setFormData(p => ({ ...p, status: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="operational">Operational</option>
            <option value="maintenance">Maintenance</option>
            <option value="deployed">Deployed</option>
            <option value="inventoried">Inventoried</option>
          </select>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">BIA Level</label>
          <select value={formData.bia_level} onChange={e => setFormData(p => ({ ...p, bia_level: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="critical">Critical</option>
            <option value="important">Important</option>
            <option value="normal">Normal</option>
          </select>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Vendor</label>
          <input value={formData.vendor} onChange={e => setFormData(p => ({ ...p, vendor: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="e.g. Dell" />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Model</label>
          <input value={formData.model} onChange={e => setFormData(p => ({ ...p, model: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="e.g. PowerEdge R750" />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Serial Number</label>
          <input value={formData.serial_number} onChange={e => setFormData(p => ({ ...p, serial_number: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="Serial number" />
        </div>

        <div className="flex gap-2 justify-end pt-2">
          <button onClick={onClose} className="px-4 py-2 rounded bg-gray-700 text-white text-sm">Cancel</button>
          <button
            onClick={() => mutation.mutate(formData, { onSuccess: () => { onClose(); setFormData({ ...initial }) } })}
            disabled={mutation.isPending || !formData.asset_tag || !formData.name}
            className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50">
            {mutation.isPending ? 'Creating...' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  )
}
