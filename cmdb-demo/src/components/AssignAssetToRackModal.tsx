import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAssets } from '../hooks/useAssets'
import { useCreateRackSlot } from '../hooks/useTopology'

interface Props {
  open: boolean
  onClose: () => void
  rackId: string
  totalU: number
}

const initial = {
  asset_id: '',
  start_u: 1,
  end_u: 1,
  side: 'front',
}

export default function AssignAssetToRackModal({ open, onClose, rackId, totalU }: Props) {
  const { t } = useTranslation()
  const [formData, setFormData] = useState({ ...initial })
  const { data: assetsResp } = useAssets()
  const assets: any[] = (assetsResp as any)?.data ?? assetsResp ?? []
  const createRackSlot = useCreateRackSlot()

  if (!open) return null

  const set = (key: string, value: string | number) =>
    setFormData(p => ({ ...p, [key]: value }))

  function handleSubmit() {
    createRackSlot.mutate(
      {
        rackId,
        data: {
          asset_id: formData.asset_id,
          start_u: Number(formData.start_u),
          end_u: Number(formData.end_u),
          side: formData.side,
        },
      },
      {
        onSuccess: () => {
          onClose()
          setFormData({ ...initial })
        },
      }
    )
  }

  const isPending = createRackSlot.isPending
  const isValid = !!formData.asset_id && formData.start_u >= 1 && formData.end_u >= formData.start_u && formData.end_u <= totalU

  return (
    <div
      className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      onClick={onClose}
    >
      <div
        className="bg-[#1a1f2e] p-6 rounded-xl w-[28rem] space-y-4 max-h-[90vh] overflow-y-auto"
        onClick={e => e.stopPropagation()}
      >
        <h3 className="text-lg font-bold text-white">
          {t('rack_detail.assign_asset_title')}
        </h3>

        {/* Asset */}
        <div>
          <label className="block text-sm text-gray-400 mb-1">
            {t('rack_detail.field_asset')} *
          </label>
          <select
            value={formData.asset_id}
            onChange={e => set('asset_id', e.target.value)}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
          >
            <option value="">— {t('rack_detail.field_asset')} —</option>
            {assets.map((a: any) => (
              <option key={a.id} value={a.id}>
                {a.name ?? a.asset_tag ?? a.id}
              </option>
            ))}
          </select>
        </div>

        {/* Start U */}
        <div>
          <label className="block text-sm text-gray-400 mb-1">
            {t('rack_detail.field_start_u')} *
          </label>
          <input
            type="number"
            min={1}
            max={totalU}
            value={formData.start_u}
            onChange={e => {
              const val = Math.max(1, Math.min(totalU, Number(e.target.value)))
              set('start_u', val)
              if (formData.end_u < val) set('end_u', val)
            }}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
          />
        </div>

        {/* End U */}
        <div>
          <label className="block text-sm text-gray-400 mb-1">
            {t('rack_detail.field_end_u')} *
          </label>
          <input
            type="number"
            min={formData.start_u}
            max={totalU}
            value={formData.end_u}
            onChange={e => {
              const val = Math.max(formData.start_u, Math.min(totalU, Number(e.target.value)))
              set('end_u', val)
            }}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
          />
        </div>

        {/* Side */}
        <div>
          <label className="block text-sm text-gray-400 mb-1">
            {t('rack_detail.field_side')}
          </label>
          <select
            value={formData.side}
            onChange={e => set('side', e.target.value)}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
          >
            <option value="front">{t('rack_detail.side_front')}</option>
            <option value="rear">{t('rack_detail.side_rear')}</option>
          </select>
        </div>

        <div className="flex gap-2 justify-end pt-2">
          <button
            onClick={onClose}
            className="px-4 py-2 rounded bg-gray-700 text-white text-sm"
          >
            {t('credential_modal.btn_cancel')}
          </button>
          <button
            onClick={handleSubmit}
            disabled={isPending || !isValid}
            className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50"
          >
            {isPending ? t('rack_detail.btn_assigning') : t('rack_detail.btn_assign')}
          </button>
        </div>
      </div>
    </div>
  )
}
