import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useCreateScanTarget, useUpdateScanTarget } from '../hooks/useScanTargets'
import { useCredentials } from '../hooks/useCredentials'

interface Credential {
  id: string
  name: string
  type: string
  cred_type?: string
}

interface ScanTarget {
  id: string
  name: string
  collector_type: string
  cidrs: string[]
  credential_id: string
  mode: string
  tenant_id?: string
}

interface Props {
  open: boolean
  onClose: () => void
  editing?: ScanTarget | null
}

const CREDENTIAL_TYPE_MAP: Record<string, string[]> = {
  snmp:  ['snmp_v2c', 'snmp_v3'],
  ssh:   ['ssh_password', 'ssh_key'],
  ipmi:  ['ipmi'],
}

const initial = {
  name: '',
  collector_type: 'snmp',
  cidrs: '',
  credential_id: '',
  mode: 'auto',
}

export default function CreateScanTargetModal({ open, onClose, editing }: Props) {
  const { t } = useTranslation()
  const tenantId = 'a0000000-0000-0000-0000-000000000001'
  const [formData, setFormData] = useState({ ...initial })

  const createMutation = useCreateScanTarget()
  const updateMutation = useUpdateScanTarget()
  const { data: credsData } = useCredentials()

  const credentials: Credential[] = (credsData as { data?: Credential[] } | undefined)?.data ?? []

  /* Populate form when editing */
  useEffect(() => {
    if (editing) {
      setFormData({
        name:           editing.name,
        collector_type: editing.collector_type,
        cidrs:          Array.isArray(editing.cidrs) ? editing.cidrs.join('\n') : '',
        credential_id:  editing.credential_id,
        mode:           editing.mode,
      })
    } else {
      setFormData({ ...initial })
    }
  }, [editing, open])

  if (!open) return null

  const compatibleTypes = CREDENTIAL_TYPE_MAP[formData.collector_type] ?? []
  const filteredCreds = credentials.filter(c => c.cred_type !== undefined && compatibleTypes.includes(c.cred_type))

  const isEditing = Boolean(editing)
  const isPending = createMutation.isPending || updateMutation.isPending

  function handleSubmit() {
    const payload = {
      name:           formData.name,
      collector_type: formData.collector_type,
      cidrs:          formData.cidrs.split('\n').map(s => s.trim()).filter(Boolean),
      credential_id:  formData.credential_id,
      mode:           formData.mode,
      tenant_id:      tenantId,
    }

    if (isEditing && editing) {
      updateMutation.mutate(
        { id: editing.id, data: payload },
        {
          onSuccess: () => {
            onClose()
            setFormData({ ...initial })
          },
        }
      )
    } else {
      createMutation.mutate(payload as any, {
        onSuccess: () => {
          onClose()
          setFormData({ ...initial })
        },
      })
    }
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div
        className="bg-[#1a1f2e] p-6 rounded-xl w-[30rem] space-y-4 max-h-[90vh] overflow-y-auto"
        onClick={e => e.stopPropagation()}
      >
        <h3 className="text-lg font-bold text-white">
          {isEditing ? t('scan_target_modal.title_edit') : t('scan_target_modal.title_create')}
        </h3>

        {/* Name */}
        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('scan_target_modal.label_name')} *</label>
          <input
            value={formData.name}
            onChange={e => setFormData(p => ({ ...p, name: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
            placeholder={t('scan_target_modal.placeholder_name')}
          />
        </div>

        {/* Collector Type */}
        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('scan_target_modal.label_collector_type')}</label>
          <select
            value={formData.collector_type}
            onChange={e => setFormData(p => ({ ...p, collector_type: e.target.value, credential_id: '' }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
          >
            <option value="snmp">SNMP</option>
            <option value="ssh">SSH</option>
            <option value="ipmi">IPMI</option>
          </select>
        </div>

        {/* CIDRs */}
        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('scan_target_modal.label_cidrs')}</label>
          <textarea
            value={formData.cidrs}
            onChange={e => setFormData(p => ({ ...p, cidrs: e.target.value }))}
            rows={4}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm font-mono resize-y"
            placeholder={t('scan_target_modal.placeholder_cidrs')}
          />
        </div>

        {/* Credential */}
        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('scan_target_modal.label_credential')}</label>
          <select
            value={formData.credential_id}
            onChange={e => setFormData(p => ({ ...p, credential_id: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
          >
            <option value="">{t('scan_target_modal.option_select_credential')}</option>
            {filteredCreds.map((c: Credential) => (
              <option key={c.id} value={c.id}>
                {c.name} ({c.cred_type})
              </option>
            ))}
          </select>
          {filteredCreds.length === 0 && (
            <p className="text-xs text-gray-500 mt-1">
              {t('scan_target_modal.no_credentials', { type: formData.collector_type.toUpperCase() })}
            </p>
          )}
        </div>

        {/* Mode */}
        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('scan_target_modal.label_mode')}</label>
          <select
            value={formData.mode}
            onChange={e => setFormData(p => ({ ...p, mode: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
          >
            <option value="auto">{t('scan_target_modal.mode_auto')}</option>
            <option value="review">{t('scan_target_modal.mode_review')}</option>
            <option value="smart">{t('scan_target_modal.mode_smart')}</option>
          </select>
        </div>

        {/* Buttons */}
        <div className="flex gap-2 justify-end pt-2">
          <button
            onClick={onClose}
            className="px-4 py-2 rounded bg-gray-700 text-white text-sm hover:bg-gray-600 transition-colors"
          >
            {t('scan_target_modal.btn_cancel')}
          </button>
          <button
            onClick={handleSubmit}
            disabled={isPending || !formData.name}
            className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50 hover:bg-blue-500 transition-colors"
          >
            {isPending
              ? (isEditing ? t('scan_target_modal.btn_saving') : t('scan_target_modal.btn_creating'))
              : (isEditing ? t('scan_target_modal.btn_update') : t('scan_target_modal.btn_create'))}
          </button>
        </div>
      </div>
    </div>
  )
}
