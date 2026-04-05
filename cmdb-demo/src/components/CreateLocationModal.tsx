import { useState } from 'react'
import { useCreateLocation } from '../hooks/useTopology'

interface Props {
  open: boolean
  onClose: () => void
}

const initial = {
  name: '',
  name_en: '',
  slug: '',
  level: 'country',
  parent_id: '',
  status: 'active',
}

export default function CreateLocationModal({ open, onClose }: Props) {
  const [formData, setFormData] = useState({ ...initial })
  const mutation = useCreateLocation()

  if (!open) return null

  const handleNameEnChange = (val: string) => {
    setFormData(p => ({
      ...p,
      name_en: val,
      slug: val.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, ''),
    }))
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-[#1a1f2e] p-6 rounded-xl w-[28rem] space-y-4 max-h-[90vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-bold text-white">Create Location</h3>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Name *</label>
          <input value={formData.name} onChange={e => setFormData(p => ({ ...p, name: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="Location name" />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Name (English)</label>
          <input value={formData.name_en} onChange={e => handleNameEnChange(e.target.value)}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="English name" />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Slug</label>
          <input value={formData.slug} onChange={e => setFormData(p => ({ ...p, slug: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="auto-derived-from-name-en" />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Level</label>
          <select value={formData.level} onChange={e => setFormData(p => ({ ...p, level: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="country">Country</option>
            <option value="region">Region</option>
            <option value="city">City</option>
            <option value="campus">Campus</option>
          </select>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Parent ID (optional)</label>
          <input value={formData.parent_id} onChange={e => setFormData(p => ({ ...p, parent_id: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="Parent location UUID" />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Status</label>
          <select value={formData.status} onChange={e => setFormData(p => ({ ...p, status: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="active">Active</option>
            <option value="inactive">Inactive</option>
          </select>
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
