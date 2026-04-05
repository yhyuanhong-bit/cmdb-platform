import { useState } from 'react'
import { useCreateRCA } from '../hooks/usePrediction'

interface Props {
  open: boolean
  onClose: () => void
}

const initial = {
  incident_id: '',
  model_name: 'Dify RCA Analyzer',
  context: '',
}

export default function CreateRCAModal({ open, onClose }: Props) {
  const [formData, setFormData] = useState({ ...initial })
  const mutation = useCreateRCA()

  if (!open) return null

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-[#1a1f2e] p-6 rounded-xl w-[28rem] space-y-4 max-h-[90vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-bold text-white">New RCA Analysis</h3>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Incident ID *</label>
          <input value={formData.incident_id} onChange={e => setFormData(p => ({ ...p, incident_id: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="UUID of the incident" />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Model</label>
          <select value={formData.model_name} onChange={e => setFormData(p => ({ ...p, model_name: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="Dify RCA Analyzer">Dify RCA Analyzer</option>
            <option value="Local Failure Predictor">Local Failure Predictor</option>
          </select>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">Context (optional)</label>
          <textarea value={formData.context} onChange={e => setFormData(p => ({ ...p, context: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm h-24 resize-none" placeholder="Additional context for the analysis..." />
        </div>

        <div className="flex gap-2 justify-end pt-2">
          <button onClick={onClose} className="px-4 py-2 rounded bg-gray-700 text-white text-sm">Cancel</button>
          <button
            onClick={() => mutation.mutate(formData, { onSuccess: () => { onClose(); setFormData({ ...initial }) } })}
            disabled={mutation.isPending || !formData.incident_id}
            className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50">
            {mutation.isPending ? 'Running...' : 'Run Analysis'}
          </button>
        </div>
      </div>
    </div>
  )
}
