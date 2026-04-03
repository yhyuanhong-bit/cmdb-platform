import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { useCreateRack } from '../hooks/useTopology'
import { useLocationContext } from '../contexts/LocationContext'

const rackSlots = Array.from({ length: 42 }, (_, i) => ({
  u: 42 - i,
  occupied: [1, 2, 3, 8, 9, 10, 15, 16, 22, 23, 24, 25, 30, 31, 38, 39, 40].includes(42 - i),
}))

export default function AddNewRack() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const createRack = useCreateRack()
  const { path } = useLocationContext()
  const locationId = path.idc?.id ?? path.campus?.id ?? ""

  const [activeTab, setActiveTab] = useState<'basic' | 'environment'>('basic')
  const [rackId, setRackId] = useState('RK-TPC01-4B-016')
  const [rackName, setRackName] = useState('Rack-A015')
  const [country] = useState('台灣 (Taiwan)')
  const [region] = useState('北部 (North)')
  const [city] = useState('台北 (Taipei)')
  const [dataCenter, setDataCenter] = useState('TPC-01 Main DC')
  const [room, setRoom] = useState('Room 4B - High Dense')
  const [uCount, setUCount] = useState('42')
  const [maxPower, setMaxPower] = useState('12.5')

  return (
    <div className="min-h-screen bg-surface p-6 font-body text-on-surface">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 mb-6">
        <button
          onClick={() => navigate('/assets')}
          className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label hover:text-primary transition-colors"
        >
          {t('add_new_rack.breadcrumb_assets')}
        </button>
        <span className="material-symbols-outlined text-on-surface-variant text-[16px]">chevron_right</span>
        <button
          onClick={() => navigate('/racks')}
          className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label hover:text-primary transition-colors"
        >
          {t('add_new_rack.breadcrumb_rack_management')}
        </button>
        <span className="material-symbols-outlined text-on-surface-variant text-[16px]">chevron_right</span>
        <span className="text-[0.6875rem] uppercase tracking-[0.05rem] text-primary font-label">{t('add_new_rack.breadcrumb_add_new_rack')}</span>
      </div>

      {/* Title */}
      <div className="mb-2">
        <h1 className="font-headline font-bold text-2xl text-on-surface">{t('add_new_rack.title_zh')}</h1>
        <p className="text-on-surface-variant text-sm mt-1">{t('add_new_rack.subtitle')}</p>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 mb-6 mt-5">
        {(['basic', 'environment'] as const).map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-4 py-2 rounded-lg text-sm font-semibold transition-colors ${
              activeTab === tab
                ? 'bg-surface-container-high text-on-surface'
                : 'text-on-surface-variant hover:bg-surface-container'
            }`}
          >
            {tab === 'basic' ? t('add_new_rack.tab_basic') : t('add_new_rack.tab_environment')}
          </button>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left Panel - Form */}
        <div className="lg:col-span-2 flex flex-col gap-6">
          {/* Basic Configuration */}
          <div className="bg-surface-container rounded-lg p-6">
            <h2 className="font-headline font-bold text-lg text-on-surface mb-1">{t('add_new_rack.section_basic_config_zh')}</h2>
            <p className="text-on-surface-variant text-xs mb-5">{t('add_new_rack.section_basic_config')}</p>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
              <div>
                <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
                  {t('add_new_rack.label_rack_id_zh')} <span className="normal-case">({t('add_new_rack.label_rack_id')})</span>
                </label>
                <div className="relative">
                  <input
                    type="text"
                    value={rackId}
                    onChange={(e) => setRackId(e.target.value)}
                    className="w-full bg-surface-container-low rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40"
                  />
                  <span className="absolute right-3 top-1/2 -translate-y-1/2 text-[0.625rem] text-on-surface-variant/60 uppercase tracking-wider">
                    {t('add_new_rack.auto')}
                  </span>
                </div>
              </div>
              <div>
                <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
                  {t('add_new_rack.label_rack_name_zh')} <span className="normal-case">({t('add_new_rack.label_rack_name')})</span>
                </label>
                <input
                  type="text"
                  value={rackName}
                  onChange={(e) => setRackName(e.target.value)}
                  placeholder={t('add_new_rack.placeholder_rack_name')}
                  className="w-full bg-surface-container-low rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-on-surface-variant/50 outline-none focus:ring-1 focus:ring-primary/40"
                />
              </div>
            </div>
          </div>

          {/* Location Hierarchy */}
          <div className="bg-surface-container rounded-lg p-6">
            <h2 className="font-headline font-bold text-lg text-on-surface mb-1">{t('add_new_rack.section_location_zh')}</h2>
            <p className="text-on-surface-variant text-xs mb-5">{t('add_new_rack.section_location')}</p>

            {/* Visual Hierarchy Path */}
            <div className="flex items-center gap-2 mb-5 bg-surface-container-low rounded-lg px-4 py-3">
              <span className="material-symbols-outlined text-primary text-[18px]">public</span>
              <span className="text-sm text-on-surface">{country}</span>
              <span className="material-symbols-outlined text-on-surface-variant text-[16px]">chevron_right</span>
              <span className="text-sm text-on-surface">{region}</span>
              <span className="material-symbols-outlined text-on-surface-variant text-[16px]">chevron_right</span>
              <span className="text-sm text-primary">{city}</span>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
              <div>
                <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
                  {t('add_new_rack.label_data_center_zh')} <span className="normal-case">({t('add_new_rack.label_data_center')})</span>
                </label>
                <select
                  value={dataCenter}
                  onChange={(e) => setDataCenter(e.target.value)}
                  className="w-full bg-surface-container-low rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40 appearance-none"
                >
                  <option value="TPC-01 Main DC">TPC-01 Main DC</option>
                  <option value="TPC-02 Backup DC">TPC-02 Backup DC</option>
                  <option value="KHH-01 South DC">KHH-01 South DC</option>
                </select>
              </div>
              <div>
                <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
                  {t('add_new_rack.label_tech_park_zh')}
                </label>
                <select
                  value={room}
                  onChange={(e) => setRoom(e.target.value)}
                  className="w-full bg-surface-container-low rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40 appearance-none"
                >
                  <option value="Room 4B - High Dense">Room 4B - High Dense</option>
                  <option value="Room 3A - Standard">Room 3A - Standard</option>
                  <option value="Room 5C - Cold Aisle">Room 5C - Cold Aisle</option>
                </select>
              </div>
            </div>
          </div>

          {/* Specifications */}
          <div className="bg-surface-container rounded-lg p-6">
            <h2 className="font-headline font-bold text-lg text-on-surface mb-1">{t('add_new_rack.section_specs_zh')}</h2>
            <p className="text-on-surface-variant text-xs mb-5">{t('add_new_rack.section_specs')}</p>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
              <div>
                <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
                  {t('add_new_rack.label_u_count')}
                </label>
                <div className="relative">
                  <input
                    type="number"
                    value={uCount}
                    onChange={(e) => setUCount(e.target.value)}
                    className="w-full bg-surface-container-low rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40"
                  />
                  <span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-on-surface-variant">U</span>
                </div>
              </div>
              <div>
                <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
                  {t('add_new_rack.label_max_power_zh')} <span className="normal-case">({t('add_new_rack.label_max_power')})</span>
                </label>
                <div className="relative">
                  <input
                    type="number"
                    step="0.1"
                    value={maxPower}
                    onChange={(e) => setMaxPower(e.target.value)}
                    className="w-full bg-surface-container-low rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40"
                  />
                  <span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-on-surface-variant">kW</span>
                </div>
              </div>
            </div>
          </div>

          {/* Submit */}
          <div className="flex items-center justify-end gap-3">
            <button
              onClick={() => navigate('/racks')}
              className="px-6 py-2.5 rounded-lg bg-surface-container text-on-surface-variant text-sm font-semibold hover:bg-surface-container-high transition-colors"
            >
              {t('add_new_rack.btn_cancel')}
            </button>
            <button
              disabled={createRack.isPending}
              onClick={() => {
                createRack.mutate(
                  {
                    name: rackName,
                    location_id: locationId,
                    row_label: rackId,
                    total_u: parseInt(uCount, 10) || 42,
                    used_u: 0,
                    power_capacity_kw: parseFloat(maxPower) || 0,
                    power_current_kw: 0,
                    status: 'OPERATIONAL',
                    tags: [],
                  },
                  { onSuccess: () => navigate('/racks') },
                )
              }}
              className="px-6 py-2.5 rounded-lg bg-on-primary-container text-white text-sm font-semibold hover:bg-on-primary-container/90 transition-colors disabled:opacity-50"
            >
              {createRack.isPending ? t('common.saving') ?? 'Saving...' : t('add_new_rack.btn_create_rack')}
            </button>
          </div>
        </div>

        {/* Right Panel - Rack Preview */}
        <div className="bg-surface-container rounded-lg p-6">
          <h2 className="font-headline font-bold text-lg text-on-surface mb-1">{t('add_new_rack.section_preview_zh')}</h2>
          <p className="text-on-surface-variant text-xs mb-4">{t('add_new_rack.section_preview')}</p>

          {/* Rack Diagram */}
          <div className="bg-surface-container-low rounded-lg p-3">
            {/* Rack frame top */}
            <div className="bg-surface-container-highest rounded-t-lg h-3" />

            {/* U slots */}
            <div className="flex flex-col">
              {rackSlots.map((slot) => (
                <div key={slot.u} className="flex items-center h-[14px]">
                  <span className="w-7 text-[0.5625rem] text-on-surface-variant/60 text-right pr-1.5 shrink-0 font-mono">
                    {slot.u}
                  </span>
                  <div
                    className={`flex-1 mx-0.5 h-[12px] rounded-[2px] ${
                      slot.occupied
                        ? 'bg-on-primary-container/40'
                        : 'bg-surface-container'
                    }`}
                  />
                </div>
              ))}
            </div>

            {/* Rack frame bottom */}
            <div className="bg-surface-container-highest rounded-b-lg h-3" />
          </div>

          {/* Legend */}
          <div className="flex items-center gap-4 mt-4">
            <div className="flex items-center gap-2">
              <span className="w-3 h-3 rounded-sm bg-on-primary-container/40" />
              <span className="text-[0.625rem] text-on-surface-variant">{t('add_new_rack.legend_occupied')}</span>
            </div>
            <div className="flex items-center gap-2">
              <span className="w-3 h-3 rounded-sm bg-surface-container" />
              <span className="text-[0.625rem] text-on-surface-variant">{t('add_new_rack.legend_available')}</span>
            </div>
          </div>

          {/* Summary */}
          <div className="mt-5 flex flex-col gap-2">
            <div className="flex items-center justify-between bg-surface-container rounded-lg px-3 py-2">
              <span className="text-xs text-on-surface-variant">{t('add_new_rack.summary_total_u')}</span>
              <span className="text-xs font-semibold text-on-surface">{uCount}U</span>
            </div>
            <div className="flex items-center justify-between bg-surface-container rounded-lg px-3 py-2">
              <span className="text-xs text-on-surface-variant">{t('add_new_rack.summary_occupied')}</span>
              <span className="text-xs font-semibold text-[#fbbf24]">17U</span>
            </div>
            <div className="flex items-center justify-between bg-surface-container rounded-lg px-3 py-2">
              <span className="text-xs text-on-surface-variant">{t('add_new_rack.summary_available')}</span>
              <span className="text-xs font-semibold text-[#34d399]">25U</span>
            </div>
            <div className="flex items-center justify-between bg-surface-container rounded-lg px-3 py-2">
              <span className="text-xs text-on-surface-variant">{t('add_new_rack.summary_max_power')}</span>
              <span className="text-xs font-semibold text-on-surface">{maxPower} kW</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
