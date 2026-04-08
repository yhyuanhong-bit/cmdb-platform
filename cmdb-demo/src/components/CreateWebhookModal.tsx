import { useState } from 'react'
import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation()
  const [formData, setFormData] = useState({ ...initial })
  const mutation = useCreateWebhook()

  if (!open) return null

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-[#1a1f2e] p-6 rounded-xl w-[28rem] space-y-4 max-h-[90vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-bold text-white">{t('webhook_modal.title')}</h3>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('webhook_modal.name_label')} *</label>
          <input value={formData.name} onChange={e => setFormData(p => ({ ...p, name: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('webhook_modal.name_placeholder')} />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('webhook_modal.url_label')} *</label>
          <input value={formData.url} onChange={e => setFormData(p => ({ ...p, url: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('webhook_modal.url_placeholder')} />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('webhook_modal.events_label')}</label>
          <input value={formData.events} onChange={e => setFormData(p => ({ ...p, events: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('webhook_modal.events_placeholder')} />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('webhook_modal.secret_label')}</label>
          <input value={formData.secret} onChange={e => setFormData(p => ({ ...p, secret: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('webhook_modal.secret_placeholder')} />
        </div>

        <div className="flex items-center gap-2">
          <input type="checkbox" checked={formData.enabled} onChange={e => setFormData(p => ({ ...p, enabled: e.target.checked }))}
            className="rounded border-gray-700" />
          <label className="text-sm text-gray-400">{t('webhook_modal.enabled_label')}</label>
        </div>

        <div className="flex gap-2 justify-end pt-2">
          <button onClick={onClose} className="px-4 py-2 rounded bg-gray-700 text-white text-sm">{t('webhook_modal.btn_cancel')}</button>
          <button
            onClick={() => mutation.mutate(
              { ...formData, events: formData.events.split(',').map(s => s.trim()).filter(Boolean) },
              { onSuccess: () => { onClose(); setFormData({ ...initial }) } }
            )}
            disabled={mutation.isPending || !formData.name || !formData.url}
            className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50">
            {mutation.isPending ? t('webhook_modal.btn_creating') : t('webhook_modal.btn_create')}
          </button>
        </div>
      </div>
    </div>
  )
}
