import { toast } from 'sonner'
import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import Icon from '../../components/Icon'
import StatusBadge from '../../components/StatusBadge'
import { useAsset, useUpdateAsset, useDeleteAsset } from '../../hooks/useAssets'
import { useBIAImpact } from '../../hooks/useBIA'
import { useRootLocations, useLocationDescendants } from '../../hooks/useTopology'
import OverviewTab from './tabs/OverviewTab'
import HealthTab from './tabs/HealthTab'
import UsageTab from './tabs/UsageTab'
import MaintenanceTab from './tabs/MaintenanceTab'

const assetDefaults = {
  description: '',
  tags: [] as string[],
  warranty: { status: 'Unknown', expiry: '-' },
  uptime: { days: 0, hours: 0, minutes: 0 },
  mtbf: '-',
  cpu: '-',
  memory: '-',
  formFactor: '-',
  storage: '-',
  network: '-',
  os: '-',
  facility: '-',
  room: '-',
  rackId: '-',
  uPosition: '-',
  inventoryNo: '-',
  poRef: '-',
  purchaseDate: '-',
  cost: '-',
  depreciation: '-',
  bookValue: '-',
  primaryIp: '-',
  mgmtIp: '-',
  domain: '-',
  vlan: '-',
}

const tabs = [
  { key: 'overview', label: '\u6982\u89bd', icon: 'dashboard' },
  { key: 'health', label: '\u5065\u5eb7\u76e3\u63a7', icon: 'monitor_heart' },
  { key: 'usage', label: '\u4f7f\u7528\u5206\u6790', icon: 'analytics' },
  { key: 'maintenance', label: '\u7dad\u8b77\u6b77\u53f2', icon: 'build' },
] as const

type TabKey = (typeof tabs)[number]['key']

export default function AssetDetailUnified() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { assetId } = useParams<{ assetId: string }>()
  const [activeTab, setActiveTab] = useState<TabKey>('overview')
  const [editing, setEditing] = useState(false)
  const [editData, setEditData] = useState<Record<string, string>>({})
  const updateAsset = useUpdateAsset()
  const deleteAsset = useDeleteAsset()

  const rootLocQ = useRootLocations()
  const firstTerritoryId = rootLocQ.data?.data?.[0]?.id ?? ''
  const descQ = useLocationDescendants(firstTerritoryId)
  const allLocations = (descQ.data?.data ?? []).filter((l: any) =>
    ['room', 'module', 'idc', 'campus'].includes(l.level)
  )

  const tabLabels: Record<string, string> = {
    overview: t('asset_detail.tab_overview'),
    health: t('asset_detail.tab_health'),
    usage: t('asset_detail.tab_usage'),
    maintenance: t('asset_detail.tab_maintenance'),
  }

  const assetQ = useAsset(assetId ?? '')
  const apiAsset = assetQ.data?.data

  const { data: impactResp } = useBIAImpact(assetId || '')
  const impactedSystems: any[] = (impactResp as any)?.data || []

  const attr = (key: string): string | undefined => {
    const v = apiAsset?.attributes?.[key]
    return v != null ? String(v) : undefined
  }

  const osValue = (() => {
    if (attr('os')) return attr('os') as string
    const osType = attr('os_type')
    const osVer = attr('os_version')
    if (osType && osVer) return `${osType} ${osVer}`
    return osType ?? osVer ?? assetDefaults.os
  })()

  const primaryIpValue =
    (apiAsset as any)?.ip_address as string | undefined ??
    attr('ip_address') ??
    attr('primary_ip') ??
    assetDefaults.primaryIp

  const cpuValue = attr('cpu_arch') ?? attr('cpu') ?? assetDefaults.cpu

  const memoryValue = (() => {
    const gb = attr('memory_gb')
    if (gb) return `${gb} GB`
    return attr('memory') ?? assetDefaults.memory
  })()

  const storageValue = (() => {
    const tb = attr('storage_tb')
    if (tb) return `${tb} TB`
    return attr('storage') ?? assetDefaults.storage
  })()

  const asset = {
    ...assetDefaults,
    id: apiAsset?.asset_tag ?? assetId ?? '-',
    status: apiAsset?.status ?? 'Unknown',
    biaLevel: apiAsset?.bia_level ?? '-',
    model: apiAsset ? `${apiAsset.vendor} ${apiAsset.model}` : '-',
    serial: apiAsset?.serial_number ?? '-',
    tags: apiAsset?.tags ?? [],
    description: attr('description') ?? assetDefaults.description,
    cpu: cpuValue,
    memory: memoryValue,
    formFactor: attr('form_factor') ?? assetDefaults.formFactor,
    storage: storageValue,
    network: attr('network') ?? assetDefaults.network,
    os: osValue,
    primaryIp: primaryIpValue,
    mgmtIp: attr('mgmt_ip') ?? assetDefaults.mgmtIp,
    domain: attr('domain') ?? assetDefaults.domain,
    vlan: attr('vlan') ?? assetDefaults.vlan,
  }

  if (assetQ.isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-sky-400 border-t-transparent" />
      </div>
    )
  }

  if (assetQ.error) {
    return (
      <div className="p-6">
        <div className="rounded-lg bg-red-900/20 p-4 text-red-300 text-sm">
          Failed to load asset detail.{' '}
          <button onClick={() => assetQ.refetch()} className="underline">Retry</button>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-surface font-body text-on-surface">
      {/* Back Button */}
      <div className="px-8 pt-6 pb-1">
        <button
          onClick={() => navigate('/assets')}
          className="flex items-center gap-1 text-sm text-on-surface-variant hover:text-primary transition-colors"
        >
          <span className="material-symbols-outlined text-[18px]">arrow_back</span>
          {t('asset_detail.back_to_assets')}
        </button>
      </div>

      {/* Header */}
      <div className="px-8 pt-5 pb-4">
        <div className="flex items-center gap-2 mb-3">
          {asset.tags.map((tag) => (
            <span
              key={tag}
              className="px-2.5 py-1 rounded text-[0.625rem] font-semibold uppercase tracking-wider bg-surface-container-high text-on-surface-variant"
            >
              {tag}
            </span>
          ))}
        </div>

        <div className="flex items-start justify-between gap-6">
          <div className="flex items-center gap-4">
            <h1 className="font-headline font-bold text-3xl text-on-surface leading-tight">
              {asset.id}
            </h1>
            <StatusBadge status={asset.status} />
          </div>
          <div className="flex items-center gap-3 shrink-0 pt-1">
            <button onClick={() => {
              setEditing(true)
              setEditData({
                name: apiAsset?.name || '',
                status: apiAsset?.status || '',
                vendor: apiAsset?.vendor || '',
                model: apiAsset?.model || '',
                bia_level: apiAsset?.bia_level || '',
                serial_number: apiAsset?.serial_number || '',
                ip_address: (apiAsset as any)?.ip_address || '',
                location_id: (apiAsset as any)?.location_id || '',
                tags: (apiAsset?.tags || []).join(', '),
                property_number: (apiAsset as any)?.property_number || '',
                control_number: (apiAsset as any)?.control_number || '',
              })
            }} className="flex items-center gap-2 px-5 py-2.5 rounded-lg bg-on-primary-container text-[#ffffff] text-[0.75rem] font-semibold uppercase tracking-wider hover:opacity-90 transition-opacity">
              <span className="material-symbols-outlined text-[18px]">edit</span>
              {t('asset_detail.btn_edit_asset')}
            </button>
            <button onClick={() => {
              if (confirm(t('asset_detail.confirm_delete'))) {
                deleteAsset.mutate(assetId!, { onSuccess: () => navigate('/assets') })
              }
            }} className="flex items-center gap-2 px-5 py-2.5 rounded-lg bg-red-500/20 text-red-400 text-[0.75rem] font-semibold uppercase tracking-wider hover:bg-red-500/30 transition-colors">
              <span className="material-symbols-outlined text-[18px]">delete</span>
              {deleteAsset.isPending ? t('asset_detail.btn_deleting') : t('asset_detail.btn_delete')}
            </button>
            <button
              onClick={() => navigate('/audit')}
              className="flex items-center gap-2 px-5 py-2.5 rounded-lg bg-surface-container-high text-on-surface-variant text-[0.75rem] font-semibold uppercase tracking-wider hover:bg-surface-container-highest transition-colors"
            >
              <span className="material-symbols-outlined text-[18px]">history</span>
              {t('asset_detail.btn_audit_log')}
            </button>
          </div>
        </div>

        <p className="mt-1.5 text-sm text-on-surface-variant max-w-2xl leading-relaxed">
          {asset.description}
        </p>
      </div>

      {/* Tabs */}
      <div className="px-8 flex items-center gap-1 border-b border-outline-variant/20 mb-6">
        {tabs.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`flex items-center gap-1.5 px-4 py-3 text-[0.75rem] font-semibold uppercase tracking-wider transition-colors border-b-2 ${
              activeTab === tab.key
                ? 'border-primary text-primary'
                : 'border-transparent text-on-surface-variant hover:text-on-surface hover:border-outline-variant/30'
            }`}
          >
            <Icon name={tab.icon} className="text-[18px]" />
            {tabLabels[tab.key] ?? tab.label}
          </button>
        ))}
      </div>

      {/* Inline Edit Panel */}
      {editing && (
        <div className="px-8 py-4">
          <div className="bg-surface-container rounded-lg p-5 space-y-4">
            <h3 className="font-headline text-sm font-bold text-on-surface uppercase tracking-wider">{t('asset_detail.edit_title')}</h3>
            <div className="grid grid-cols-3 gap-4">
              {[
                { key: 'name', label: t('asset_detail.field_name') },
                { key: 'vendor', label: t('asset_detail.field_vendor') },
                { key: 'model', label: t('asset_detail.field_model') },
                { key: 'serial_number', label: t('asset_detail.field_serial_number') },
                { key: 'ip_address', label: t('asset_detail.field_ip_address'), placeholder: '192.168.1.100' },
                { key: 'property_number', label: t('asset_detail.field_property_number'), placeholder: 'P-2025-0001' },
                { key: 'control_number', label: t('asset_detail.field_control_number'), placeholder: 'CTRL-TW-A-0001' },
              ].map(({ key, label, placeholder }) => (
                <div key={key} className="flex flex-col gap-1">
                  <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">{label}</label>
                  <input
                    value={editData[key] ?? ''}
                    onChange={e => setEditData(p => ({ ...p, [key]: e.target.value }))}
                    placeholder={placeholder}
                    className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm"
                  />
                </div>
              ))}
              {/* Status */}
              <div className="flex flex-col gap-1">
                <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">{t('asset_detail.field_status')}</label>
                <select
                  value={editData.status ?? ''}
                  onChange={e => setEditData(p => ({ ...p, status: e.target.value }))}
                  className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm"
                >
                  <option value=""></option>
                  <option value="inventoried">{t('asset_detail.status_inventoried')}</option>
                  <option value="operational">{t('asset_detail.status_operational')}</option>
                  <option value="deployed">{t('asset_detail.status_deployed')}</option>
                  <option value="maintenance">{t('asset_detail.status_maintenance')}</option>
                  <option value="retired">{t('asset_detail.status_retired')}</option>
                </select>
              </div>
              {/* BIA Level */}
              <div className="flex flex-col gap-1">
                <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">{t('asset_detail.field_bia_level')}</label>
                <select
                  value={editData.bia_level ?? ''}
                  onChange={e => setEditData(p => ({ ...p, bia_level: e.target.value }))}
                  className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm"
                >
                  <option value=""></option>
                  <option value="critical">{t('asset_detail.bia_critical')}</option>
                  <option value="important">{t('asset_detail.bia_important')}</option>
                  <option value="normal">{t('asset_detail.bia_normal')}</option>
                  <option value="minor">{t('asset_detail.bia_minor')}</option>
                </select>
              </div>
              {/* Location */}
              <div className="flex flex-col gap-1">
                <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">{t('asset_detail.field_location')}</label>
                <select
                  value={editData.location_id ?? ''}
                  onChange={e => setEditData(p => ({ ...p, location_id: e.target.value }))}
                  className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm"
                >
                  <option value="">{t('asset_detail.select_location')}</option>
                  {allLocations.map((loc: any) => (
                    <option key={loc.id} value={loc.id}>{loc.name}</option>
                  ))}
                </select>
              </div>
              {/* Tags */}
              <div className="flex flex-col gap-1">
                <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
                  {t('asset_detail.field_tags')}
                  <span className="ml-1 normal-case text-[0.6rem] opacity-60">({t('asset_detail.tags_hint')})</span>
                </label>
                <input
                  value={editData.tags ?? ''}
                  onChange={e => setEditData(p => ({ ...p, tags: e.target.value }))}
                  placeholder="production, tier-1"
                  className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm"
                />
              </div>
            </div>
            <div className="flex gap-2 mt-4">
              <button onClick={() => {
                const payload: Record<string, any> = { ...editData }
                if (typeof payload.tags === 'string') {
                  payload.tags = payload.tags.split(',').map((s: string) => s.trim()).filter(Boolean)
                }
                Object.keys(payload).forEach(k => { if (payload[k] === '') delete payload[k] })
                updateAsset.mutate({ id: assetId!, data: payload }, {
                  onSuccess: (resp: any) => {
                    setEditing(false)
                    if (resp?.meta?.change_order_id) {
                      toast.success(`Critical asset change recorded. Audit order: ${resp.meta.change_order_id}`)
                    }
                  }
                })
              }} disabled={updateAsset.isPending}
                className="px-4 py-2 rounded bg-blue-600 text-white text-sm">
                {updateAsset.isPending ? t('asset_detail.edit_saving') : t('asset_detail.edit_save')}
              </button>
              <button onClick={() => setEditing(false)}
                className="px-4 py-2 rounded bg-gray-700 text-white text-sm">{t('asset_detail.edit_cancel')}</button>
            </div>
          </div>
        </div>
      )}

      {/* Tab Content */}
      <div className="px-8 pb-10">
        {activeTab === 'overview' && <OverviewTab asset={asset} assetId={assetId} impactedSystems={impactedSystems} />}
        {activeTab === 'health' && <HealthTab assetId={assetId} />}
        {activeTab === 'usage' && <UsageTab />}
        {activeTab === 'maintenance' && <MaintenanceTab />}
      </div>
    </div>
  )
}
