import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useCreateBIAAssessment } from '../hooks/useBIA'
import { Modal } from './ui/Modal'

interface Props {
  open: boolean
  onClose: () => void
}

const initial = {
  system_name: '',
  system_code: '',
  owner: '',
  bia_score: 50,
  tier: 'normal',
  rto_hours: 24,
  rpo_minutes: 240,
  description: '',
}

export default function CreateAssessmentModal({ open, onClose }: Props) {
  const { t } = useTranslation()
  const [formData, setFormData] = useState({ ...initial })
  const mutation = useCreateBIAAssessment()

  const inputCls =
    'w-full p-2 bg-surface-container-lowest rounded border border-outline-variant/30 text-on-surface text-sm focus:border-primary focus:outline-none'
  const labelCls = 'block text-sm text-on-surface-variant mb-1'

  return (
    <Modal
      open={open}
      onOpenChange={(next) => { if (!next) onClose() }}
      panelClassName="bg-surface-container text-on-surface"
    >
      <Modal.Header
        title={
          <span className="font-headline">{t('assessment_modal.title')}</span>
        }
        onClose={onClose}
      />
      <Modal.Body>
        <div>
          <label className={labelCls}>{t('assessment_modal.system_name_label')} *</label>
          <input
            value={formData.system_name}
            onChange={(e) =>
              setFormData((p) => ({ ...p, system_name: e.target.value }))
            }
            className={inputCls}
            placeholder={t('assessment_modal.system_name_placeholder')}
          />
        </div>

        <div>
          <label className={labelCls}>{t('assessment_modal.system_code_label')} *</label>
          <input
            value={formData.system_code}
            onChange={(e) =>
              setFormData((p) => ({ ...p, system_code: e.target.value }))
            }
            className={inputCls}
            placeholder={t('assessment_modal.system_code_placeholder')}
          />
        </div>

        <div>
          <label className={labelCls}>{t('assessment_modal.owner_label')}</label>
          <input
            value={formData.owner}
            onChange={(e) =>
              setFormData((p) => ({ ...p, owner: e.target.value }))
            }
            className={inputCls}
            placeholder={t('assessment_modal.owner_placeholder')}
          />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className={labelCls}>{t('assessment_modal.bia_score_label')}</label>
            <input
              type="number"
              min={0}
              max={100}
              value={formData.bia_score}
              onChange={(e) =>
                setFormData((p) => ({
                  ...p,
                  bia_score: parseInt(e.target.value) || 0,
                }))
              }
              className={inputCls}
            />
          </div>
          <div>
            <label className={labelCls}>{t('assessment_modal.tier_label')}</label>
            <select
              value={formData.tier}
              onChange={(e) =>
                setFormData((p) => ({ ...p, tier: e.target.value }))
              }
              className={inputCls}
            >
              <option value="critical">{t('assessment_modal.tier_critical')}</option>
              <option value="important">{t('assessment_modal.tier_important')}</option>
              <option value="normal">{t('assessment_modal.tier_normal')}</option>
              <option value="minor">{t('assessment_modal.tier_minor')}</option>
            </select>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className={labelCls}>{t('assessment_modal.rto_label')}</label>
            <input
              type="number"
              min={0}
              value={formData.rto_hours}
              onChange={(e) =>
                setFormData((p) => ({
                  ...p,
                  rto_hours: parseFloat(e.target.value) || 0,
                }))
              }
              className={inputCls}
            />
          </div>
          <div>
            <label className={labelCls}>{t('assessment_modal.rpo_label')}</label>
            <input
              type="number"
              min={0}
              value={formData.rpo_minutes}
              onChange={(e) =>
                setFormData((p) => ({
                  ...p,
                  rpo_minutes: parseFloat(e.target.value) || 0,
                }))
              }
              className={inputCls}
            />
          </div>
        </div>

        <div>
          <label className={labelCls}>{t('assessment_modal.description_label')}</label>
          <textarea
            value={formData.description}
            onChange={(e) =>
              setFormData((p) => ({ ...p, description: e.target.value }))
            }
            className={`${inputCls} h-20 resize-none`}
            placeholder={t('assessment_modal.description_placeholder')}
          />
        </div>
      </Modal.Body>
      <Modal.Footer>
        <button
          onClick={onClose}
          className="px-4 py-2 rounded bg-surface-container-high text-on-surface text-sm hover:bg-surface-container-highest transition-colors"
        >
          {t('assessment_modal.btn_cancel')}
        </button>
        <button
          onClick={() =>
            mutation.mutate(formData, {
              onSuccess: () => {
                onClose()
                setFormData({ ...initial })
              },
            })
          }
          disabled={
            mutation.isPending ||
            !formData.system_name ||
            !formData.system_code
          }
          className="px-4 py-2 rounded machined-gradient text-on-primary text-sm font-semibold disabled:opacity-50"
        >
          {mutation.isPending ? t('assessment_modal.btn_running') : t('assessment_modal.btn_run')}
        </button>
      </Modal.Footer>
    </Modal>
  )
}
