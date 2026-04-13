import { useState } from 'react'
import { useTranslation } from 'react-i18next'
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
  queries: '',
  pull_interval: '300',
}

export default function CreateAdapterModal({ open, onClose }: Props) {
  const { t } = useTranslation()
  const [formData, setFormData] = useState({ ...initial })
  const mutation = useCreateAdapter()

  if (!open) return null

  const isInboundRest = formData.type === 'rest' && formData.direction === 'inbound'

  const handleCreate = () => {
    const payload: Record<string, unknown> = {
      name: formData.name,
      type: formData.type,
      direction: formData.direction,
      endpoint: formData.endpoint,
      enabled: formData.enabled,
    }
    if (isInboundRest && formData.queries.trim()) {
      payload.config = {
        queries: formData.queries.split('\n').map(q => q.trim()).filter(Boolean),
        pull_interval_seconds: parseInt(formData.pull_interval, 10),
      }
    }
    mutation.mutate(payload as any, {
      onSuccess: () => { onClose(); setFormData({ ...initial }) },
    })
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-[#1a1f2e] p-6 rounded-xl w-[28rem] space-y-4 max-h-[90vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-bold text-white">{t('adapter_modal.title')}</h3>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('adapter_modal.name_label')} *</label>
          <input value={formData.name} onChange={e => setFormData(p => ({ ...p, name: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('adapter_modal.name_placeholder')} />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('adapter_modal.type_label')}</label>
          <select value={formData.type} onChange={e => setFormData(p => ({ ...p, type: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="dify">Dify</option>
            <option value="rest">REST</option>
            <option value="grpc">gRPC</option>
            <option value="mqtt">MQTT</option>
          </select>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('adapter_modal.direction_label')}</label>
          <select value={formData.direction} onChange={e => setFormData(p => ({ ...p, direction: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="inbound">{t('adapter_modal.direction_inbound')}</option>
            <option value="outbound">{t('adapter_modal.direction_outbound')}</option>
            <option value="bidirectional">{t('adapter_modal.direction_bidirectional')}</option>
          </select>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('adapter_modal.endpoint_label')}</label>
          <input value={formData.endpoint} onChange={e => setFormData(p => ({ ...p, endpoint: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="https://..." />
        </div>

        {isInboundRest && (
          <>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Metric Queries (one per line)</label>
              <textarea
                value={formData.queries}
                onChange={e => setFormData(p => ({ ...p, queries: e.target.value }))}
                rows={4}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm font-mono"
                placeholder={"node_cpu_seconds_total\nnode_memory_MemAvailable_bytes\npower_kw"}
              />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Pull Interval</label>
              <select value={formData.pull_interval} onChange={e => setFormData(p => ({ ...p, pull_interval: e.target.value }))}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
                <option value="60">1 minute</option>
                <option value="300">5 minutes (default)</option>
                <option value="900">15 minutes</option>
                <option value="1800">30 minutes</option>
                <option value="3600">1 hour</option>
              </select>
            </div>
          </>
        )}

        <div className="flex items-center gap-2">
          <input type="checkbox" checked={formData.enabled} onChange={e => setFormData(p => ({ ...p, enabled: e.target.checked }))}
            className="rounded border-gray-700" />
          <label className="text-sm text-gray-400">{t('adapter_modal.enabled_label')}</label>
        </div>

        <div className="flex gap-2 justify-end pt-2">
          <button onClick={onClose} className="px-4 py-2 rounded bg-gray-700 text-white text-sm">{t('adapter_modal.btn_cancel')}</button>
          <button onClick={handleCreate} disabled={mutation.isPending || !formData.name}
            className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50">
            {mutation.isPending ? t('adapter_modal.btn_creating') : t('adapter_modal.btn_create')}
          </button>
        </div>
      </div>
    </div>
  )
}
