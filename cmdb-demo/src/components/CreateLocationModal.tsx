import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useCreateLocation } from '../hooks/useTopology'

interface Props {
  open: boolean
  onClose: () => void
}

const initial = {
  name: '',
  name_en: '',
  slug: '',
  level: 'territory',
  parent_id: '',
  status: 'active',
  latitude: '',
  longitude: '',
}

export default function CreateLocationModal({ open, onClose }: Props) {
  const { t } = useTranslation()
  const [formData, setFormData] = useState({ ...initial })
  const mutation = useCreateLocation()

  if (!open) return null

  const handleNameEnChange = (val: string) => {
    setFormData(p => ({
      ...p,
      name_en: val,
      slug: val.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, ''),
    }))
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-[#1a1f2e] p-6 rounded-xl w-[28rem] space-y-4 max-h-[90vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-bold text-white">{t('location_modal.title')}</h3>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('location_modal.name_label')} *</label>
          <input value={formData.name} onChange={e => setFormData(p => ({ ...p, name: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('location_modal.name_placeholder')} />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('location_modal.name_en_label')}</label>
          <input value={formData.name_en} onChange={e => handleNameEnChange(e.target.value)}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('location_modal.name_en_placeholder')} />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('location_modal.slug_label')}</label>
          <input value={formData.slug} onChange={e => setFormData(p => ({ ...p, slug: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('location_modal.slug_placeholder')} />
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('location_modal.level_label')}</label>
          <select value={formData.level} onChange={e => setFormData(p => ({ ...p, level: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="territory">{t('location_modal.level_territory')}</option>
            <option value="region">{t('location_modal.level_region')}</option>
            <option value="city">{t('location_modal.level_city')}</option>
            <option value="campus">{t('location_modal.level_campus')}</option>
          </select>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('location_modal.parent_id_label')}</label>
          <input value={formData.parent_id} onChange={e => setFormData(p => ({ ...p, parent_id: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder={t('location_modal.parent_id_placeholder')} />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="block text-sm text-gray-400 mb-1">{t('location_modal.latitude')}</label>
            <input value={formData.latitude} onChange={e => setFormData(p => ({ ...p, latitude: e.target.value }))}
              type="number" step="any"
              className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="25.03" />
          </div>
          <div>
            <label className="block text-sm text-gray-400 mb-1">{t('location_modal.longitude')}</label>
            <input value={formData.longitude} onChange={e => setFormData(p => ({ ...p, longitude: e.target.value }))}
              type="number" step="any"
              className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" placeholder="121.56" />
          </div>
        </div>

        <div>
          <label className="block text-sm text-gray-400 mb-1">{t('location_modal.status_label')}</label>
          <select value={formData.status} onChange={e => setFormData(p => ({ ...p, status: e.target.value }))}
            className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
            <option value="active">{t('location_modal.status_active')}</option>
            <option value="inactive">{t('location_modal.status_inactive')}</option>
          </select>
        </div>

        <div className="flex gap-2 justify-end pt-2">
          <button onClick={onClose} className="px-4 py-2 rounded bg-gray-700 text-white text-sm">{t('location_modal.btn_cancel')}</button>
          <button
            onClick={() => {
              const { latitude, longitude, ...rest } = formData
              const metadata: Record<string, unknown> = {}
              if (latitude) metadata.latitude = parseFloat(latitude)
              if (longitude) metadata.longitude = parseFloat(longitude)
              const payload = { ...rest, metadata: Object.keys(metadata).length > 0 ? metadata : undefined }
              mutation.mutate(payload as any, {
                onSuccess: () => { onClose(); setFormData({ ...initial }) },
                onError: (err: any) => {
                  if (err?.code === 'DUPLICATE') {
                    toast.error('A location with this slug already exists')
                  } else {
                    toast.error('Failed to create location')
                  }
                },
              })
            }}
            disabled={mutation.isPending || !formData.name}
            className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50">
            {mutation.isPending ? t('location_modal.btn_creating') : t('location_modal.btn_create')}
          </button>
        </div>
      </div>
    </div>
  )
}
