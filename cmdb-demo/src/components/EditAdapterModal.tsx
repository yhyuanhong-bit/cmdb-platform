import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useUpdateAdapter } from '../hooks/useIntegration'
import type { AdapterConfig, UpdateAdapterInput } from '../lib/api/integration'

interface Props {
  adapter: AdapterConfig | null
  onClose: () => void
}

export default function EditAdapterModal({ adapter, onClose }: Props) {
  const { t } = useTranslation()
  const mutation = useUpdateAdapter()
  const [name, setName] = useState('')
  const [endpoint, setEndpoint] = useState('')
  const [enabled, setEnabled] = useState(true)

  useEffect(() => {
    if (adapter) {
      setName(adapter.name)
      setEndpoint(adapter.endpoint || '')
      setEnabled(adapter.enabled)
    }
  }, [adapter])

  if (!adapter) return null

  const handleSave = () => {
    const patch: UpdateAdapterInput = {}
    if (name !== adapter.name) patch.name = name
    if (endpoint !== (adapter.endpoint || '')) patch.endpoint = endpoint
    if (enabled !== adapter.enabled) patch.enabled = enabled
    if (Object.keys(patch).length === 0) { onClose(); return }
    mutation.mutate({ id: adapter.id, data: patch }, { onSuccess: onClose })
  }

  const inputCls = 'w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm'
  const labelCls = 'block text-sm text-gray-400 mb-1'
  const readOnlyCls = 'w-full p-2 bg-[#0d1117]/50 rounded border border-gray-800 text-gray-500 text-sm cursor-not-allowed'

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-[#1a1f2e] p-6 rounded-xl w-[28rem] space-y-4 max-h-[90vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-bold text-white">{t('edit_adapter_modal.title')}</h3>

        <div>
          <label className={labelCls}>{t('adapter_modal.name_label')} *</label>
          <input value={name} onChange={e => setName(e.target.value)} className={inputCls} />
        </div>

        <div>
          <label className={labelCls}>{t('adapter_modal.type_label')}</label>
          <input value={adapter.type} readOnly className={readOnlyCls} />
          <p className="mt-1 text-xs text-gray-500">{t('edit_adapter_modal.type_locked_hint')}</p>
        </div>

        <div>
          <label className={labelCls}>{t('adapter_modal.direction_label')}</label>
          <input value={adapter.direction} readOnly className={readOnlyCls} />
        </div>

        <div>
          <label className={labelCls}>{t('adapter_modal.endpoint_label')}</label>
          <input value={endpoint} onChange={e => setEndpoint(e.target.value)} className={inputCls} placeholder="https://..." />
        </div>

        <div className="flex items-center gap-2">
          <input type="checkbox" checked={enabled} onChange={e => setEnabled(e.target.checked)} className="rounded border-gray-700" />
          <label className="text-sm text-gray-400">{t('adapter_modal.enabled_label')}</label>
        </div>

        <p className="text-xs text-gray-500">{t('edit_adapter_modal.config_hint')}</p>

        <div className="flex gap-2 justify-end pt-2">
          <button onClick={onClose} className="px-4 py-2 rounded bg-gray-700 text-white text-sm">{t('adapter_modal.btn_cancel')}</button>
          <button onClick={handleSave} disabled={mutation.isPending || !name}
            className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50">
            {mutation.isPending ? t('edit_adapter_modal.btn_saving') : t('edit_adapter_modal.btn_save')}
          </button>
        </div>
      </div>
    </div>
  )
}
