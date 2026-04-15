import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useCreateAsset } from '../hooks/useAssets'

interface Props {
  open: boolean
  onClose: () => void
}

const initial = {
  asset_tag: '',
  name: '',
  type: 'server',
  sub_type: '',
  status: 'operational',
  bia_level: 'normal',
  vendor: '',
  model: '',
  serial_number: '',
  property_number: '',
  control_number: '',
  bmc_ip: '',
  bmc_type: '',
  bmc_firmware: '',
}

export default function CreateAssetModal({ open, onClose }: Props) {
  const { t } = useTranslation()
  const [formData, setFormData] = useState({ ...initial })
  const mutation = useCreateAsset()

  if (!open) return null

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-[#1a1f2e] p-6 rounded-xl w-[28rem] space-y-4 max-h-[90vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-bold text-white">{t('asset_modal.title')}</h3>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('asset_modal.asset_tag_label')} *</label>
          <input value={formData.asset_tag} onChange={e => setFormData(p => ({ ...p, asset_tag: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('asset_modal.asset_tag_placeholder')} />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('asset_modal.name_label')} *</label>
          <input value={formData.name} onChange={e => setFormData(p => ({ ...p, name: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('asset_modal.name_placeholder')} />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('asset_modal.type_label')}</label>
          <select value={formData.type} onChange={e => setFormData(p => ({ ...p, type: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="server">{t('asset_modal.type_server')}</option>
            <option value="network">{t('asset_modal.type_network')}</option>
            <option value="storage">{t('asset_modal.type_storage')}</option>
            <option value="power">{t('asset_modal.type_power')}</option>
          </select>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('asset_modal.sub_type_label')}</label>
          <input value={formData.sub_type} onChange={e => setFormData(p => ({ ...p, sub_type: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('asset_modal.sub_type_placeholder')} />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('asset_modal.status_label')}</label>
          <select value={formData.status} onChange={e => setFormData(p => ({ ...p, status: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="operational">{t('asset_modal.status_operational')}</option>
            <option value="maintenance">{t('asset_modal.status_maintenance')}</option>
            <option value="deployed">{t('asset_modal.status_deployed')}</option>
            <option value="inventoried">{t('asset_modal.status_inventoried')}</option>
          </select>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('asset_modal.bia_level_label')}</label>
          <select value={formData.bia_level} onChange={e => setFormData(p => ({ ...p, bia_level: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="critical">{t('asset_modal.bia_critical')}</option>
            <option value="important">{t('asset_modal.bia_important')}</option>
            <option value="normal">{t('asset_modal.bia_normal')}</option>
          </select>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('asset_modal.vendor_label')}</label>
          <input value={formData.vendor} onChange={e => setFormData(p => ({ ...p, vendor: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('asset_modal.vendor_placeholder')} />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('asset_modal.model_label')}</label>
          <input value={formData.model} onChange={e => setFormData(p => ({ ...p, model: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('asset_modal.model_placeholder')} />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('asset_modal.serial_number_label')}</label>
          <input value={formData.serial_number} onChange={e => setFormData(p => ({ ...p, serial_number: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('asset_modal.serial_number_placeholder')} />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('asset_modal.field_property_number')}</label>
          <input value={formData.property_number} onChange={e => setFormData(p => ({ ...p, property_number: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
            placeholder={t('asset_modal.placeholder_property')} />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('asset_modal.field_control_number')}</label>
          <input value={formData.control_number} onChange={e => setFormData(p => ({ ...p, control_number: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
            placeholder={t('asset_modal.placeholder_control')} />
        </div>

        {/* BMC Management */}
        <div className="border-t border-gray-700 pt-4">
          <p className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-3">
            {t('asset_detail.bmc_section')}
          </p>

          <div className="space-y-3">
            <div>
              <label className="block text-sm text-gray-400 mb-1">{t('asset_detail.bmc_ip')}</label>
              <input
                value={formData.bmc_ip}
                onChange={e => setFormData(p => ({ ...p, bmc_ip: e.target.value }))}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
                placeholder="10.0.100.5"
              />
            </div>

            <div>
              <label className="block text-sm text-gray-400 mb-1">{t('asset_detail.bmc_type')}</label>
              <select
                value={formData.bmc_type}
                onChange={e => setFormData(p => ({ ...p, bmc_type: e.target.value }))}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
              >
                <option value="">—</option>
                <option value="ilo">iLO</option>
                <option value="idrac">iDRAC</option>
                <option value="ipmi">IPMI</option>
                <option value="ami">AMI</option>
                <option value="other">Other</option>
              </select>
            </div>

            <div>
              <label className="block text-sm text-gray-400 mb-1">{t('asset_detail.bmc_firmware')}</label>
              <input
                value={formData.bmc_firmware}
                onChange={e => setFormData(p => ({ ...p, bmc_firmware: e.target.value }))}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
                placeholder="iLO 5 v2.72"
              />
            </div>
          </div>
        </div>

        <div className="flex gap-2 justify-end pt-2">
          <button onClick={onClose} className="px-4 py-2 rounded bg-gray-700 text-white text-sm">{t('asset_modal.btn_cancel')}</button>
          <button
            onClick={() => mutation.mutate(formData, {
              onSuccess: () => { onClose(); setFormData({ ...initial }) },
              onError: (err: unknown) => {
                const code = err !== null && typeof err === 'object' && 'code' in err ? (err as { code: unknown }).code : undefined
                if (code === 'DUPLICATE') {
                  toast.error('An asset with this asset tag already exists')
                } else {
                  toast.error('Failed to create asset')
                }
              },
            })}
            disabled={mutation.isPending || !formData.asset_tag || !formData.name}
            className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50">
            {mutation.isPending ? t('asset_modal.btn_creating') : t('asset_modal.btn_create')}
          </button>
        </div>
      </div>
    </div>
  )
}
