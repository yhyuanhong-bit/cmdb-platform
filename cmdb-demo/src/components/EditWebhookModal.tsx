import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useUpdateWebhook } from '../hooks/useIntegration'
import type { UpdateWebhookInput, WebhookSubscription } from '../lib/api/integration'
import { Modal } from './ui/Modal'

interface Props {
  webhook: WebhookSubscription | null
  onClose: () => void
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

export default function EditWebhookModal({ webhook, onClose }: Props) {
  const { t } = useTranslation()
  const mutation = useUpdateWebhook()
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [events, setEvents] = useState('')
  const [enabled, setEnabled] = useState(true)
  const [rotateSecret, setRotateSecret] = useState(false)
  const [newSecret, setNewSecret] = useState('')
  const [urlTouched, setUrlTouched] = useState(false)

  useEffect(() => {
    if (webhook) {
      setName(webhook.name)
      setUrl(webhook.url)
      setEvents((webhook.events || []).join(', '))
      setEnabled(webhook.enabled)
      setRotateSecret(false)
      setNewSecret('')
      setUrlTouched(false)
    }
  }, [webhook])

  const urlError = validateUrl(url)
  const showUrlError = urlTouched && urlError === 'invalid'
  const canSubmit = !mutation.isPending && !!name && urlError === null

  const handleSave = () => {
    if (!webhook) return
    setUrlTouched(true)
    if (!canSubmit) return
    const patch: UpdateWebhookInput = {}
    if (name !== webhook.name) patch.name = name
    if (url !== webhook.url) patch.url = url
    const nextEvents = events.split(',').map(s => s.trim()).filter(Boolean)
    const prevEvents = webhook.events || []
    if (nextEvents.length !== prevEvents.length || nextEvents.some((e, i) => e !== prevEvents[i])) {
      patch.events = nextEvents
    }
    if (enabled !== webhook.enabled) patch.enabled = enabled
    if (rotateSecret && newSecret) patch.secret = newSecret
    if (Object.keys(patch).length === 0) { onClose(); return }
    mutation.mutate({ id: webhook.id, data: patch }, { onSuccess: onClose })
  }

  const inputCls = 'w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm'
  const labelCls = 'block text-sm text-gray-400 mb-1'

  return (
    <Modal
      open={webhook !== null}
      onOpenChange={(next) => { if (!next) onClose() }}
    >
      <Modal.Header title={t('edit_webhook_modal.title')} onClose={onClose} />
      <Modal.Body>
        <div>
          <label className={labelCls}>{t('webhook_modal.name_label')} *</label>
          <input value={name} onChange={e => setName(e.target.value)} className={inputCls} />
        </div>

        <div>
          <label className={labelCls}>{t('webhook_modal.url_label')} *</label>
          <input value={url}
            onChange={e => setUrl(e.target.value)}
            onBlur={() => setUrlTouched(true)}
            aria-invalid={showUrlError}
            aria-describedby={showUrlError ? 'edit-webhook-url-error' : undefined}
            className={`w-full p-2 bg-[#0d1117] rounded border ${showUrlError ? 'border-red-500' : 'border-gray-700'} text-white text-sm`} />
          {showUrlError && (
            <p id="edit-webhook-url-error" className="mt-1 text-xs text-red-400">{t('webhook_modal.url_invalid')}</p>
          )}
        </div>

        <div>
          <label className={labelCls}>{t('webhook_modal.events_label')}</label>
          <input value={events} onChange={e => setEvents(e.target.value)} className={inputCls} placeholder={t('webhook_modal.events_placeholder')} />
        </div>

        <div className="flex items-center gap-2">
          <input id="rotate-secret" type="checkbox" checked={rotateSecret} onChange={e => setRotateSecret(e.target.checked)} className="rounded border-gray-700" />
          <label htmlFor="rotate-secret" className="text-sm text-gray-400">{t('edit_webhook_modal.rotate_secret_label')}</label>
        </div>

        {rotateSecret && (
          <div>
            <label className={labelCls}>{t('edit_webhook_modal.new_secret_label')}</label>
            <input type="password" value={newSecret} onChange={e => setNewSecret(e.target.value)}
              className={inputCls} placeholder={t('edit_webhook_modal.new_secret_placeholder')} />
            <p className="mt-1 text-xs text-amber-400">{t('edit_webhook_modal.rotate_secret_warning')}</p>
          </div>
        )}

        <div className="flex items-center gap-2">
          <input id="edit-webhook-enabled" type="checkbox" checked={enabled} onChange={e => setEnabled(e.target.checked)} className="rounded border-gray-700" />
          <label htmlFor="edit-webhook-enabled" className="text-sm text-gray-400">{t('webhook_modal.enabled_label')}</label>
        </div>
      </Modal.Body>
      <Modal.Footer>
        <button onClick={onClose} className="px-4 py-2 rounded bg-gray-700 text-white text-sm">{t('webhook_modal.btn_cancel')}</button>
        <button onClick={handleSave} disabled={!canSubmit}
          className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50">
          {mutation.isPending ? t('edit_webhook_modal.btn_saving') : t('edit_webhook_modal.btn_save')}
        </button>
      </Modal.Footer>
    </Modal>
  )
}
