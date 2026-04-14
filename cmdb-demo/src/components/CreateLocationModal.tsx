import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useCreateLocation } from '../hooks/useTopology'

interface LocationPreset {
  slug: string
  name: string
  nameEn: string
  flag: string
  lat: number
  lng: number
}

const LOCATION_PRESETS: LocationPreset[] = [
  { slug: 'tw', name: '台灣', nameEn: 'Taiwan', flag: '🇹🇼', lat: 23.5, lng: 121.0 },
  { slug: 'china', name: '中國', nameEn: 'China', flag: '🇨🇳', lat: 35.0, lng: 105.0 },
  { slug: 'japan', name: '日本', nameEn: 'Japan', flag: '🇯🇵', lat: 36.0, lng: 138.0 },
  { slug: 'singapore', name: '新加坡', nameEn: 'Singapore', flag: '🇸🇬', lat: 1.35, lng: 103.8 },
  { slug: 'us', name: '美國', nameEn: 'United States', flag: '🇺🇸', lat: 39.8, lng: -98.5 },
  { slug: 'uk', name: '英國', nameEn: 'United Kingdom', flag: '🇬🇧', lat: 54.0, lng: -2.0 },
  { slug: 'de', name: '德國', nameEn: 'Germany', flag: '🇩🇪', lat: 51.2, lng: 10.4 },
  { slug: 'fr', name: '法國', nameEn: 'France', flag: '🇫🇷', lat: 46.2, lng: 2.2 },
  { slug: 'kr', name: '韓國', nameEn: 'South Korea', flag: '🇰🇷', lat: 35.9, lng: 127.8 },
  { slug: 'au', name: '澳洲', nameEn: 'Australia', flag: '🇦🇺', lat: -25.3, lng: 133.8 },
  { slug: 'in', name: '印度', nameEn: 'India', flag: '🇮🇳', lat: 20.6, lng: 79.0 },
  { slug: 'br', name: '巴西', nameEn: 'Brazil', flag: '🇧🇷', lat: -14.2, lng: -51.9 },
  { slug: 'ca', name: '加拿大', nameEn: 'Canada', flag: '🇨🇦', lat: 56.1, lng: -106.3 },
  { slug: 'mx', name: '墨西哥', nameEn: 'Mexico', flag: '🇲🇽', lat: 23.6, lng: -102.6 },
  { slug: 'th', name: '泰國', nameEn: 'Thailand', flag: '🇹🇭', lat: 15.9, lng: 100.9 },
  { slug: 'vn', name: '越南', nameEn: 'Vietnam', flag: '🇻🇳', lat: 14.1, lng: 108.3 },
  { slug: 'id', name: '印尼', nameEn: 'Indonesia', flag: '🇮🇩', lat: -0.8, lng: 113.9 },
  { slug: 'my', name: '馬來西亞', nameEn: 'Malaysia', flag: '🇲🇾', lat: 4.2, lng: 101.9 },
  { slug: 'ph', name: '菲律賓', nameEn: 'Philippines', flag: '🇵🇭', lat: 12.9, lng: 121.8 },
  { slug: 'hk', name: '香港', nameEn: 'Hong Kong', flag: '🇭🇰', lat: 22.3, lng: 114.2 },
]

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
  const [customMode, setCustomMode] = useState(false)
  const mutation = useCreateLocation()

  // Reset customMode when level changes away from territory
  useEffect(() => {
    if (formData.level !== 'territory') {
      setCustomMode(false)
    }
  }, [formData.level])

  if (!open) return null

  const handleNameEnChange = (val: string) => {
    setFormData(p => ({
      ...p,
      name_en: val,
      slug: val.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, ''),
    }))
  }

  const handlePresetChange = (value: string) => {
    if (value === '__custom__') {
      setCustomMode(true)
      setFormData(p => ({ ...p, slug: '', name: '', name_en: '', latitude: '', longitude: '' }))
      return
    }
    const preset = LOCATION_PRESETS.find(p => p.slug === value)
    if (preset) {
      setCustomMode(false)
      setFormData(p => ({
        ...p,
        slug: preset.slug,
        name: preset.name,
        name_en: preset.nameEn,
        latitude: String(preset.lat),
        longitude: String(preset.lng),
      }))
    }
  }

  const handleSubmit = () => {
    const { latitude, longitude, ...rest } = formData
    const metadata: Record<string, unknown> = {}
    if (latitude) metadata.latitude = parseFloat(latitude)
    if (longitude) metadata.longitude = parseFloat(longitude)
    const payload = { ...rest, metadata: Object.keys(metadata).length > 0 ? metadata : undefined }
    mutation.mutate(payload as any, {
      onSuccess: () => {
        onClose()
        setFormData({ ...initial })
        setCustomMode(false)
      },
      onError: (err: any) => {
        if (err?.code === 'DUPLICATE') {
          toast.error('A location with this slug already exists')
        } else {
          toast.error('Failed to create location')
        }
      },
    })
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div className="bg-[#1a1f2e] p-6 rounded-xl w-[28rem] space-y-4 max-h-[90vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-bold text-white">{t('location_modal.title')}</h3>

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

        {/* Country/region preset selector — only for territory level */}
        {formData.level === 'territory' && !customMode ? (
          <div>
            <label className="block text-sm text-gray-400 mb-1">{t('location_modal.country_label', 'Country / Region')}</label>
            <select
              value={formData.slug || ''}
              onChange={e => handlePresetChange(e.target.value)}
              className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
            >
              <option value="">{t('location_modal.select_country', 'Select country/region...')}</option>
              {LOCATION_PRESETS.map(preset => (
                <option key={preset.slug} value={preset.slug}>
                  {preset.flag} {preset.name} ({preset.nameEn})
                </option>
              ))}
              <option value="__custom__">✏️ {t('location_modal.custom_location', 'Custom (manual input)')}</option>
            </select>
          </div>
        ) : (
          <div>
            <div className="flex items-center justify-between mb-1">
              <label className="block text-sm text-gray-400">{t('location_modal.slug_label')}</label>
              {formData.level === 'territory' && customMode && (
                <button
                  type="button"
                  onClick={() => setCustomMode(false)}
                  className="text-xs text-blue-400 hover:underline"
                >
                  {t('location_modal.back_to_list', '← Back to list')}
                </button>
              )}
            </div>
            <input
              value={formData.slug}
              onChange={e => setFormData(p => ({ ...p, slug: e.target.value }))}
              className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm"
              placeholder={t('location_modal.slug_placeholder')}
            />
          </div>
        )}

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
            onClick={handleSubmit}
            disabled={mutation.isPending || !formData.name}
            className="px-4 py-2 rounded bg-blue-600 text-white text-sm disabled:opacity-50">
            {mutation.isPending ? t('location_modal.btn_creating') : t('location_modal.btn_create')}
          </button>
        </div>
      </div>
    </div>
  )
}
