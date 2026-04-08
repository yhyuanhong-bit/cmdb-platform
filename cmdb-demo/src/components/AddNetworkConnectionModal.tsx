import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useCreateNetworkConnection } from '../hooks/useTopology'
import { useAssets } from '../hooks/useAssets'

interface Props {
  open: boolean
  onClose: () => void
  rackId: string
}

const initial = {
  port: '',
  deviceType: 'internal' as 'internal' | 'external',
  assetId: '',
  externalDevice: '',
  speed: '1GbE',
  status: 'UP',
  vlans: '',
  connectionType: 'network',
}

export default function AddNetworkConnectionModal({ open, onClose, rackId }: Props) {
  const { t } = useTranslation()
  const [formData, setFormData] = useState({ ...initial })
  const mutation = useCreateNetworkConnection()
  const { data: assetsData } = useAssets()
  const assets = (assetsData as any)?.data ?? []

  if (!open) return null

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const payload = {
      source_port: formData.port,
      speed: formData.speed,
      status: formData.status,
      vlans: formData.vlans.split(',').map(v => parseInt(v.trim())).filter(n => !isNaN(n)),
      connection_type: formData.connectionType,
      ...(formData.deviceType === 'internal'
        ? { connected_asset_id: formData.assetId }
        : { external_device: formData.externalDevice }),
    }
    mutation.mutate({ rackId, data: payload }, {
      onSuccess: () => {
        setFormData({ ...initial })
        onClose()
      },
    })
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-[#1a1f2e] p-6 rounded-xl w-[32rem] space-y-4 max-h-[90vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-bold text-white">{t('rack_detail.add_connection_title')}</h3>

        <form onSubmit={handleSubmit} className="space-y-4">
          {/* Port */}
          <div>
            <label className="block text-sm text-gray-400 mb-1">{t('rack_detail.field_port')}</label>
            <input
              value={formData.port}
              onChange={e => setFormData(p => ({ ...p, port: e.target.value }))}
              className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
              placeholder="Eth1/1"
              required
            />
          </div>

          {/* Device Type toggle */}
          <div>
            <label className="block text-sm text-gray-400 mb-2">{t('rack_detail.field_device_type')}</label>
            <div className="flex gap-4">
              <label className="flex items-center gap-2 text-sm text-gray-300 cursor-pointer">
                <input
                  type="radio"
                  name="deviceType"
                  value="internal"
                  checked={formData.deviceType === 'internal'}
                  onChange={() => setFormData(p => ({ ...p, deviceType: 'internal' }))}
                  className="accent-primary"
                />
                {t('rack_detail.option_internal_asset')}
              </label>
              <label className="flex items-center gap-2 text-sm text-gray-300 cursor-pointer">
                <input
                  type="radio"
                  name="deviceType"
                  value="external"
                  checked={formData.deviceType === 'external'}
                  onChange={() => setFormData(p => ({ ...p, deviceType: 'external' }))}
                  className="accent-primary"
                />
                {t('rack_detail.option_external_device')}
              </label>
            </div>
          </div>

          {/* Internal Asset dropdown */}
          {formData.deviceType === 'internal' && (
            <div>
              <label className="block text-sm text-gray-400 mb-1">{t('rack_detail.option_internal_asset')}</label>
              <select
                value={formData.assetId}
                onChange={e => setFormData(p => ({ ...p, assetId: e.target.value }))}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
              >
                <option value="">— Select Asset —</option>
                {assets.map((a: any) => (
                  <option key={a.id} value={a.id}>{a.name ?? a.asset_tag}</option>
                ))}
              </select>
            </div>
          )}

          {/* External Device text input */}
          {formData.deviceType === 'external' && (
            <div>
              <label className="block text-sm text-gray-400 mb-1">{t('rack_detail.field_external_device')}</label>
              <input
                value={formData.externalDevice}
                onChange={e => setFormData(p => ({ ...p, externalDevice: e.target.value }))}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
                placeholder="switch-core-01"
              />
            </div>
          )}

          {/* Speed */}
          <div>
            <label className="block text-sm text-gray-400 mb-1">{t('rack_detail.field_speed')}</label>
            <select
              value={formData.speed}
              onChange={e => setFormData(p => ({ ...p, speed: e.target.value }))}
              className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
            >
              <option value="1GbE">1GbE</option>
              <option value="10GbE">10GbE</option>
              <option value="25GbE">25GbE</option>
              <option value="100GbE">100GbE</option>
            </select>
          </div>

          {/* Status */}
          <div>
            <label className="block text-sm text-gray-400 mb-1">{t('rack_detail.table_status')}</label>
            <select
              value={formData.status}
              onChange={e => setFormData(p => ({ ...p, status: e.target.value }))}
              className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
            >
              <option value="UP">UP</option>
              <option value="DOWN">DOWN</option>
            </select>
          </div>

          {/* VLANs */}
          <div>
            <label className="block text-sm text-gray-400 mb-1">
              {t('rack_detail.field_vlans')}
              <span className="ml-2 text-xs text-gray-500">({t('rack_detail.vlans_hint')})</span>
            </label>
            <input
              value={formData.vlans}
              onChange={e => setFormData(p => ({ ...p, vlans: e.target.value }))}
              className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
              placeholder="100,200,300"
            />
          </div>

          {/* Connection Type */}
          <div>
            <label className="block text-sm text-gray-400 mb-1">{t('rack_detail.field_conn_type')}</label>
            <select
              value={formData.connectionType}
              onChange={e => setFormData(p => ({ ...p, connectionType: e.target.value }))}
              className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
            >
              <option value="network">{t('rack_detail.conn_network')}</option>
              <option value="power">{t('rack_detail.conn_power')}</option>
              <option value="management">{t('rack_detail.conn_management')}</option>
            </select>
          </div>

          {mutation.isError && (
            <p className="text-red-400 text-sm">Failed to create connection. Please try again.</p>
          )}

          <div className="flex gap-3 pt-2">
            <button
              type="submit"
              disabled={mutation.isPending}
              className="flex-1 py-2 rounded bg-primary text-on-primary text-sm font-semibold hover:bg-primary/90 disabled:opacity-50 transition-colors"
            >
              {mutation.isPending ? '...' : t('rack_detail.btn_add_connection')}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="flex-1 py-2 rounded bg-gray-700 text-white text-sm font-semibold hover:bg-gray-600 transition-colors"
            >
              {t('common.cancel')}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
