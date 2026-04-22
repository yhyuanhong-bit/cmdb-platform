import { toast } from 'sonner'
import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { useCreateRack, useRootLocations, useLocationChildren } from '../hooks/useTopology'
import type { Location } from '../lib/api/topology'

import { useLocationContext } from '../contexts/LocationContext'

export default function AddNewRack() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const createRack = useCreateRack()
  const { path } = useLocationContext()

  const [activeTab, setActiveTab] = useState<'basic' | 'environment'>('basic')
  const [rackName, setRackName] = useState('Rack-A015')
  const [rowLabel, setRowLabel] = useState('A')
  const [uCount, setUCount] = useState('42')
  const [maxPower, setMaxPower] = useState('12.5')

  // Location cascade state
  const [selectedTerritoryId, setSelectedTerritoryId] = useState('')
  const [selectedRegionId, setSelectedRegionId] = useState('')
  const [selectedCityId, setSelectedCityId] = useState('')
  const [selectedCampusId, setSelectedCampusId] = useState('')

  // API-driven location data
  const { data: countriesResp } = useRootLocations()
  const countries = countriesResp?.data || []
  const { data: regionsResp } = useLocationChildren(selectedTerritoryId)
  const regions = regionsResp?.data || []
  const { data: citiesResp } = useLocationChildren(selectedRegionId)
  const cities = citiesResp?.data || []
  const { data: campusesResp } = useLocationChildren(selectedCityId)
  const campuses = campusesResp?.data || []

  // Pre-fill from LocationContext if available
  useEffect(() => {
    if (path.territory?.id && !selectedTerritoryId) setSelectedTerritoryId(path.territory.id)
    if (path.region?.id && !selectedRegionId) setSelectedRegionId(path.region.id)
    if (path.city?.id && !selectedCityId) setSelectedCityId(path.city.id)
    if (path.campus?.id && !selectedCampusId) setSelectedCampusId(path.campus.id)
  }, [path])

  // Cascade clear: when parent changes, clear children
  useEffect(() => { setSelectedRegionId(''); setSelectedCityId(''); setSelectedCampusId('') }, [selectedTerritoryId])
  useEffect(() => { setSelectedCityId(''); setSelectedCampusId('') }, [selectedRegionId])
  useEffect(() => { setSelectedCampusId('') }, [selectedCityId])

  // Dynamic rack preview (new rack = all empty)
  const totalU = parseInt(uCount) || 42
  const rackSlots = Array.from({ length: totalU }, (_, i) => ({ u: totalU - i, occupied: false }))

  // Selected location names for breadcrumb
  const selectedCountry = countries.find((c: Location) => c.id === selectedTerritoryId)
  const selectedRegion = regions.find((r: Location) => r.id === selectedRegionId)
  const selectedCity = cities.find((c: Location) => c.id === selectedCityId)

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
              <div>
                <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
                  {t('add_new_rack.label_row')}
                </label>
                <select
                  value={rowLabel}
                  onChange={(e) => setRowLabel(e.target.value)}
                  className="w-full bg-surface-container-low rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40 appearance-none"
                >
                  {Array.from({ length: 26 }, (_, i) => String.fromCharCode(65 + i)).map((letter) => (
                    <option key={letter} value={letter}>{letter}</option>
                  ))}
                </select>
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
              <span className="text-sm text-on-surface">{selectedCountry?.name_en || selectedCountry?.name || '\u2014'}</span>
              <span className="material-symbols-outlined text-on-surface-variant text-[16px]">chevron_right</span>
              <span className="text-sm text-on-surface">{selectedRegion?.name_en || selectedRegion?.name || '\u2014'}</span>
              <span className="material-symbols-outlined text-on-surface-variant text-[16px]">chevron_right</span>
              <span className="text-sm text-primary">{selectedCity?.name_en || selectedCity?.name || '\u2014'}</span>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
              {/* Territory */}
              <div>
                <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
                  {t('locations.level_territory')}
                </label>
                <select value={selectedTerritoryId} onChange={e => setSelectedTerritoryId(e.target.value)}
                  className="w-full bg-surface-container-low rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40 appearance-none">
                  <option value="">{t('add_new_rack.select_territory')}</option>
                  {(countries as Location[]).map((c) => <option key={c.id} value={c.id}>{c.name_en || c.name}</option>)}
                </select>
              </div>
              {/* Region */}
              <div>
                <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
                  {t('locations.level_region')}
                </label>
                <select value={selectedRegionId} onChange={e => setSelectedRegionId(e.target.value)}
                  disabled={!selectedTerritoryId}
                  className="w-full bg-surface-container-low rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40 appearance-none disabled:opacity-40">
                  <option value="">{t('add_new_rack.select_region')}</option>
                  {(regions as Location[]).map((r) => <option key={r.id} value={r.id}>{r.name_en || r.name}</option>)}
                </select>
              </div>
              {/* City */}
              <div>
                <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
                  {t('locations.level_city')}
                </label>
                <select value={selectedCityId} onChange={e => setSelectedCityId(e.target.value)}
                  disabled={!selectedRegionId}
                  className="w-full bg-surface-container-low rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40 appearance-none disabled:opacity-40">
                  <option value="">{t('add_new_rack.select_city')}</option>
                  {(cities as Location[]).map((c) => <option key={c.id} value={c.id}>{c.name_en || c.name}</option>)}
                </select>
              </div>
              {/* Campus */}
              <div>
                <label className="text-[0.6875rem] uppercase tracking-[0.05rem] text-on-surface-variant font-label block mb-1.5">
                  {t('locations.level_campus')}
                </label>
                <select value={selectedCampusId} onChange={e => setSelectedCampusId(e.target.value)}
                  disabled={!selectedCityId}
                  className="w-full bg-surface-container-low rounded-lg px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40 appearance-none disabled:opacity-40">
                  <option value="">{t('add_new_rack.select_campus')}</option>
                  {(campuses as Location[]).map((c) => <option key={c.id} value={c.id}>{c.name_en || c.name}</option>)}
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
              disabled={createRack.isPending || !selectedCampusId || !rackName.trim()}
              onClick={() => {
                createRack.mutate(
                  {
                    name: rackName,
                    location_id: selectedCampusId,
                    row_label: rowLabel,
                    total_u: parseInt(uCount, 10) || 42,
                    used_u: 0,
                    power_capacity_kw: parseFloat(maxPower) || 0,
                    power_current_kw: 0,
                    status: 'active',
                    tags: [],
                  },
                  {
                    onSuccess: () => navigate('/racks'),
                    onError: (err: Error) => {
                      if ((err as import('../lib/api/client').ApiRequestError)?.code === 'DUPLICATE') {
                        toast.error('A rack with this name already exists in this location')
                      } else {
                        toast.error(t('add_new_rack.error_create_failed'))
                      }
                    },
                  },
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
              <span className="text-xs font-semibold text-[#fbbf24]">0U</span>
            </div>
            <div className="flex items-center justify-between bg-surface-container rounded-lg px-3 py-2">
              <span className="text-xs text-on-surface-variant">{t('add_new_rack.summary_available')}</span>
              <span className="text-xs font-semibold text-[#34d399]">{uCount}U</span>
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
