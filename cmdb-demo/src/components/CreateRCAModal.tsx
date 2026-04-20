import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useCreateRCA } from '../hooks/usePrediction'
import { Modal } from './ui/Modal'

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
  const { t } = useTranslation()
  const [formData, setFormData] = useState({ ...initial })
  const mutation = useCreateRCA()

  return (
    <Modal open={open} onOpenChange={(next) => { if (!next) onClose() }}>
      <Modal.Header title={t('rca_modal.title')} onClose={onClose} />
      <Modal.Body>
        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('rca_modal.incident_id_label')} *</label>
          <input value={formData.incident_id} onChange={e => setFormData(p => ({ ...p, incident_id: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('rca_modal.incident_id_placeholder')} />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('rca_modal.model_label')}</label>
          <select value={formData.model_name} onChange={e => setFormData(p => ({ ...p, model_name: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="Dify RCA Analyzer">Dify RCA Analyzer</option>
            <option value="Local Failure Predictor">Local Failure Predictor</option>
          </select>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('rca_modal.context_label')}</label>
          <textarea value={formData.context} onChange={e => setFormData(p => ({ ...p, context: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm h-24 resize-none" placeholder={t('rca_modal.context_placeholder')} />
        </div>
      </Modal.Body>
      <Modal.Footer>
        <button onClick={onClose} className="px-4 py-2 rounded bg-gray-700 text-white text-sm">{t('rca_modal.btn_cancel')}</button>
        <button
          onClick={() => mutation.mutate(formData, { onSuccess: () => { onClose(); setFormData({ ...initial }) } })}
          disabled={mutation.isPending || !formData.incident_id}
          className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50">
          {mutation.isPending ? t('rca_modal.btn_running') : t('rca_modal.btn_run')}
        </button>
      </Modal.Footer>
    </Modal>
  )
}
