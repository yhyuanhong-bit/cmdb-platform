import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useCreateWebhook } from '../hooks/useIntegration'
import { Modal } from './ui/Modal'

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

function validateUrl(value: string): 'empty' | 'invalid' | null {
  if (!value) return 'empty'
  try {
    const parsed = new URL(value)
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return 'invalid'
    return null
  } catch {
    return 'invalid'
  }
}

export default function CreateWebhookModal({ open, onClose }: Props) {
  const { t } = useTranslation()
  const [formData, setFormData] = useState({ ...initial })
  const [urlTouched, setUrlTouched] = useState(false)
  const mutation = useCreateWebhook()

  const urlError = validateUrl(formData.url)
  const showUrlError = urlTouched && urlError === 'invalid'
  const canSubmit = !mutation.isPending && !!formData.name && urlError === null

  return (
    <Modal open={open} onOpenChange={(next) => { if (!next) onClose() }}>
      <Modal.Header title={t('webhook_modal.title')} onClose={onClose} />
      <Modal.Body>
        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('webhook_modal.name_label')} *</label>
          <input value={formData.name} onChange={e => setFormData(p => ({ ...p, name: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('webhook_modal.name_placeholder')} />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('webhook_modal.url_label')} *</label>
          <input value={formData.url}
            onChange={e => setFormData(p => ({ ...p, url: e.target.value }))}
            onBlur={() => setUrlTouched(true)}
            aria-invalid={showUrlError}
            aria-describedby={showUrlError ? 'webhook-url-error' : undefined}
            className={`w-full p-2 bg-[#0d1117] rounded border ${showUrlError ? 'border-red-500' : 'border-gray-700'} text-white text-sm`}
            placeholder={t('webhook_modal.url_placeholder')} />
          {showUrlError && (
            <p id="webhook-url-error" className="mt-1 text-xs text-red-400">{t('webhook_modal.url_invalid')}</p>
          )}
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
      </Modal.Body>
      <Modal.Footer>
        <button onClick={onClose} className="px-4 py-2 rounded bg-gray-700 text-white text-sm">{t('webhook_modal.btn_cancel')}</button>
        <button
          onClick={() => {
            setUrlTouched(true)
            if (!canSubmit) return
            mutation.mutate(
              { ...formData, events: formData.events.split(',').map(s => s.trim()).filter(Boolean) },
              { onSuccess: () => { onClose(); setFormData({ ...initial }); setUrlTouched(false) } }
            )
          }}
          disabled={!canSubmit}
          className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50">
          {mutation.isPending ? t('webhook_modal.btn_creating') : t('webhook_modal.btn_create')}
        </button>
      </Modal.Footer>
    </Modal>
  )
}
