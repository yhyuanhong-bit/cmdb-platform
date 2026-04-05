import { useState } from 'react'
import { useCreateWebhook } from '../hooks/useIntegration'

interface Props {
  open: boolean
  onClose: () => void
}

const initial = {
  name: '',
  url: '',
  events: '',
  secret: '',
  enabled: true,
}

export default function CreateWebhookModal({ open, onClose }: Props) {
  const [formData, setFormData] = useState({ ...initial })
  const mutation = useCreateWebhook()

  if (!open) return null

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-[#1a1f2e] p-6 rounded-xl w-[28rem] space-y-4 max-h-[90vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-bold text-white">Create Webhook</h3>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Name *</label>
          <input value={formData.name} onChange={e => setFormData(p => ({ ...p, name: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="Webhook name" />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">URL *</label>
          <input value={formData.url} onChange={e => setFormData(p => ({ ...p, url: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="https://example.com/webhook" />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Events (comma separated)</label>
          <input value={formData.events} onChange={e => setFormData(p => ({ ...p, events: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="asset.created,alert.fired" />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Secret (optional)</label>
          <input value={formData.secret} onChange={e => setFormData(p => ({ ...p, secret: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="Signing secret" />
        </div>

        <div className="flex items-center gap-2">
          <input type="checkbox" checked={formData.enabled} onChange={e => setFormData(p => ({ ...p, enabled: e.target.checked }))}
            className="rounded border-gray-700" />
          <label className="text-sm text-gray-400">Enabled</label>
        </div>

        <div className="flex gap-2 justify-end pt-2">
          <button onClick={onClose} className="px-4 py-2 rounded bg-gray-700 text-white text-sm">Cancel</button>
          <button
            onClick={() => mutation.mutate(
              { ...formData, events: formData.events.split(',').map(s => s.trim()).filter(Boolean) },
              { onSuccess: () => { onClose(); setFormData({ ...initial }) } }
            )}
            disabled={mutation.isPending || !formData.name || !formData.url}
            className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50">
            {mutation.isPending ? 'Creating...' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  )
}
