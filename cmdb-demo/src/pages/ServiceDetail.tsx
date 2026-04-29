import { memo, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  useService,
  useServiceAssets,
  useServiceHealth,
  useRemoveServiceAsset,
  useUpdateService,
  useDeleteService,
  useAddServiceAsset,
} from '../hooks/useServices'
import { useAssets } from '../hooks/useAssets'
import type {
  Service,
  ServiceAssetMember,
  UpdateServiceRequest,
} from '../lib/api/services'

function Icon({ name, className = '' }: { name: string; className?: string }) {
  return <span className={`material-symbols-outlined ${className}`}>{name}</span>
}

const HEALTH_COLOR: Record<string, string> = {
  healthy: 'bg-success-container text-success',
  degraded: 'bg-error-container text-error',
  unknown: 'bg-surface-container-high text-on-surface-variant',
}

// Roles taken verbatim from internal/domain/service/model.go (Q3 sign-off
// locks the 7-value set; additions require a spec revision). Keeping this
// list local mirrors the backend enum without coupling to the OpenAPI gen.
const ASSET_ROLES: ServiceAssetMember['role'][] = [
  'primary',
  'replica',
  'cache',
  'proxy',
  'storage',
  'dependency',
  'component',
]

const TIER_OPTIONS: Service['tier'][] = ['critical', 'important', 'normal', 'low', 'minor']
const STATUS_OPTIONS: Service['status'][] = ['active', 'deprecated', 'decommissioned']

const ServiceDetail = memo(function ServiceDetail() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()

  const serviceQ = useService(id)
  const assetsQ = useServiceAssets(id)
  const healthQ = useServiceHealth(id)
  const removeAsset = useRemoveServiceAsset(id ?? '')

  const [showEdit, setShowEdit] = useState(false)
  const [showAddAsset, setShowAddAsset] = useState(false)
  const [showDelete, setShowDelete] = useState(false)

  const service = serviceQ.data?.data
  const assets = assetsQ.data?.data ?? []
  const health = healthQ.data?.data

  if (serviceQ.isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-primary border-t-transparent" />
      </div>
    )
  }

  if (!service) {
    return (
      <div className="min-h-screen bg-surface p-6">
        <p className="text-on-surface-variant">{t('services.not_found')}</p>
        <button onClick={() => navigate('/services')} className="text-primary mt-2 text-sm">
          ← {t('services.back_to_list')}
        </button>
      </div>
    )
  }

  const healthStatus = health?.status ?? 'unknown'
  const healthColor = HEALTH_COLOR[healthStatus]

  return (
    <div className="min-h-screen bg-surface p-6 font-body text-on-surface">
      {/* Breadcrumb */}
      <button
        onClick={() => navigate('/services')}
        className="text-xs text-on-surface-variant font-label flex items-center gap-1 mb-3 hover:text-primary transition-colors"
      >
        <Icon name="arrow_back" className="text-sm" />
        {t('services.back_to_list')}
      </button>

      {/* Header */}
      <div className="flex items-start justify-between mb-6 gap-4">
        <div className="min-w-0">
          <div className="flex items-center gap-3 mb-1 flex-wrap">
            <h1 className="font-headline text-3xl font-bold tracking-tight text-on-surface">
              {service.name}
            </h1>
            <span
              className={`inline-flex items-center gap-1 px-2 py-1 rounded text-[10px] font-label tracking-widest uppercase bg-primary-container text-primary`}
            >
              {service.tier}
            </span>
            <span
              className={`inline-flex items-center gap-1 px-2 py-1 rounded text-[10px] font-label tracking-widest uppercase ${healthColor}`}
            >
              <Icon
                name={
                  healthStatus === 'healthy' ? 'check_circle' : healthStatus === 'degraded' ? 'error' : 'help'
                }
                className="text-xs"
              />
              {t(`services.health_${healthStatus}`)}
            </span>
          </div>
          <p className="font-mono text-xs text-on-surface-variant">{service.code}</p>
          {service.description && (
            <p className="mt-2 text-sm text-on-surface-variant max-w-2xl">{service.description}</p>
          )}
        </div>
        {/* Actions — Edit + Delete (Delete is destructive, parked on the right) */}
        <div className="flex items-center gap-2 shrink-0">
          <button
            type="button"
            onClick={() => setShowEdit(true)}
            className="bg-surface-container-high hover:bg-surface-container-highest text-on-surface px-3 py-2 rounded-lg text-xs font-label font-bold flex items-center gap-1.5 transition-colors"
          >
            <Icon name="edit" className="text-base" />
            {t('services.edit')}
          </button>
          <button
            type="button"
            onClick={() => setShowDelete(true)}
            className="bg-error-container hover:opacity-90 text-on-error-container px-3 py-2 rounded-lg text-xs font-label font-bold flex items-center gap-1.5 transition-opacity"
          >
            <Icon name="delete" className="text-base" />
            {t('services.delete')}
          </button>
        </div>
      </div>

      {/* Metadata grid */}
      <div className="grid grid-cols-4 gap-4 mb-6">
        <Stat label={t('services.field_status')} value={service.status} />
        <Stat label={t('services.field_owner_team')} value={service.owner_team ?? '—'} />
        <Stat
          label={t('services.critical_health')}
          value={
            health
              ? `${health.critical_total - health.critical_unhealthy}/${health.critical_total}`
              : '—'
          }
        />
        <Stat label={t('services.total_assets')} value={String(assets.length)} />
      </div>

      {/* Assets table */}
      <div className="bg-surface-container rounded-xl overflow-hidden">
        <div className="flex items-center justify-between px-4 py-3 border-b border-outline-variant/20">
          <h2 className="font-headline text-sm font-bold uppercase tracking-widest text-on-surface-variant">
            {t('services.member_assets')}
          </h2>
          <button
            type="button"
            onClick={() => setShowAddAsset(true)}
            className="bg-primary hover:opacity-90 text-on-primary px-3 py-1.5 rounded-lg text-xs font-label font-bold flex items-center gap-1.5 transition-opacity"
          >
            <Icon name="add_link" className="text-base" />
            {t('services.add_asset')}
          </button>
        </div>
        {assets.length === 0 ? (
          <div className="p-6 text-sm text-on-surface-variant text-center">
            {t('services.no_assets_yet')}
          </div>
        ) : (
          <table className="w-full">
            <thead>
              <tr className="text-[10px] uppercase tracking-widest text-on-surface-variant">
                <th className="py-2 px-4 text-left font-label">{t('services.col_asset_tag')}</th>
                <th className="py-2 px-4 text-left font-label">{t('services.col_asset_name')}</th>
                <th className="py-2 px-4 text-left font-label">{t('services.col_role')}</th>
                <th className="py-2 px-4 text-left font-label">{t('services.col_critical')}</th>
                <th className="py-2 px-4 text-left font-label">{t('services.col_asset_status')}</th>
                <th className="py-2 px-4 text-right font-label"></th>
              </tr>
            </thead>
            <tbody>
              {assets.map((a) => (
                <tr
                  key={a.asset_id}
                  className="border-t border-outline-variant/10 hover:bg-surface-container-high transition-colors"
                >
                  <td className="py-2 px-4 font-mono text-xs">
                    <button
                      onClick={() => navigate(`/assets/${a.asset_id}`)}
                      className="text-primary hover:underline"
                    >
                      {a.asset_tag ?? a.asset_id.slice(0, 8)}
                    </button>
                  </td>
                  <td className="py-2 px-4 text-sm">{a.asset_name ?? '—'}</td>
                  <td className="py-2 px-4 text-xs uppercase tracking-widest text-on-surface-variant">
                    {a.role}
                  </td>
                  <td className="py-2 px-4 text-xs">
                    {a.is_critical ? (
                      <span className="text-error font-bold">{t('services.critical_yes')}</span>
                    ) : (
                      <span className="text-on-surface-variant">{t('services.critical_no')}</span>
                    )}
                  </td>
                  <td className="py-2 px-4 text-xs text-on-surface-variant">{a.asset_status ?? '—'}</td>
                  <td className="py-2 px-4 text-right">
                    <button
                      onClick={() => removeAsset.mutate(a.asset_id)}
                      disabled={removeAsset.isPending}
                      className="text-error hover:opacity-80 text-xs disabled:opacity-40"
                      title={t('services.remove_asset')}
                    >
                      <Icon name="link_off" className="text-base" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Modals — rendered conditionally so they unmount completely on close,
          which resets local form state. */}
      {showEdit && id && (
        <EditServiceModal
          serviceId={id}
          service={service}
          onClose={() => setShowEdit(false)}
        />
      )}
      {showAddAsset && id && (
        <AddAssetModal
          serviceId={id}
          existingAssetIds={assets.map((a) => a.asset_id)}
          onClose={() => setShowAddAsset(false)}
        />
      )}
      {showDelete && id && (
        <DeleteServiceDialog
          service={service}
          onClose={() => setShowDelete(false)}
          onDeleted={() => navigate('/services')}
        />
      )}
    </div>
  )
})

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-surface-container rounded-xl p-4">
      <p className="text-[10px] uppercase tracking-widest text-on-surface-variant mb-1">{label}</p>
      <p className="text-base font-headline font-bold text-on-surface">{value}</p>
    </div>
  )
}

export default ServiceDetail

// ---------------------------------------------------------------------------
// Edit Service modal — partial update of metadata fields. Only fields that
// changed from the snapshot are sent so concurrent edits to other fields
// from a different tab don't get clobbered.
// ---------------------------------------------------------------------------

interface EditServiceModalProps {
  serviceId: string
  service: Service
  onClose: () => void
}

function EditServiceModal({ serviceId, service, onClose }: EditServiceModalProps) {
  const { t } = useTranslation()
  const update = useUpdateService(serviceId)

  const [name, setName] = useState(service.name)
  const [description, setDescription] = useState(service.description ?? '')
  const [tier, setTier] = useState<Service['tier']>(service.tier)
  const [ownerTeam, setOwnerTeam] = useState(service.owner_team ?? '')
  const [status, setStatus] = useState<Service['status']>(service.status)
  const [tagsInput, setTagsInput] = useState((service.tags ?? []).join(', '))
  const [error, setError] = useState('')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    // Build a minimal patch — only send fields the user actually changed.
    const patch: UpdateServiceRequest = {}
    if (name.trim() !== service.name) patch.name = name.trim()
    if (description.trim() !== (service.description ?? '')) patch.description = description.trim()
    if (tier !== service.tier) patch.tier = tier
    if (ownerTeam.trim() !== (service.owner_team ?? '')) patch.owner_team = ownerTeam.trim()
    if (status !== service.status) patch.status = status

    const newTags = tagsInput
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean)
    const oldTags = service.tags ?? []
    const tagsChanged =
      newTags.length !== oldTags.length || newTags.some((tag, i) => tag !== oldTags[i])
    if (tagsChanged) patch.tags = newTags

    if (Object.keys(patch).length === 0) {
      onClose()
      return
    }

    try {
      await update.mutateAsync(patch)
      toast.success(t('services.toast_updated'))
      onClose()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err)
      setError(msg)
      toast.error(msg)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <form
        onSubmit={handleSubmit}
        className="bg-surface-container w-full max-w-md rounded-2xl p-6 shadow-xl max-h-[90vh] overflow-y-auto"
      >
        <h2 className="font-headline text-xl font-bold mb-4 text-on-surface">
          {t('services.edit')}
        </h2>

        <label className="block text-xs font-label text-on-surface-variant mb-1">
          {t('services.field_name')}
        </label>
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="w-full mb-3 px-3 py-2 rounded-lg bg-surface text-on-surface text-sm border border-outline-variant focus:outline-none focus:ring-1 focus:ring-primary"
          required
        />

        <label className="block text-xs font-label text-on-surface-variant mb-1">
          {t('services.field_description')}
        </label>
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          rows={2}
          className="w-full mb-3 px-3 py-2 rounded-lg bg-surface text-on-surface text-sm border border-outline-variant focus:outline-none focus:ring-1 focus:ring-primary resize-y"
        />

        <label className="block text-xs font-label text-on-surface-variant mb-1">
          {t('services.field_tier')}
        </label>
        <select
          value={tier}
          onChange={(e) => setTier(e.target.value as Service['tier'])}
          className="w-full mb-3 px-3 py-2 rounded-lg bg-surface text-on-surface text-sm border border-outline-variant focus:outline-none focus:ring-1 focus:ring-primary"
        >
          {TIER_OPTIONS.map((opt) => (
            <option key={opt} value={opt}>
              {t(`services.tier_${opt}`)}
            </option>
          ))}
        </select>

        <label className="block text-xs font-label text-on-surface-variant mb-1">
          {t('services.field_owner_team')}
        </label>
        <input
          type="text"
          value={ownerTeam}
          onChange={(e) => setOwnerTeam(e.target.value)}
          className="w-full mb-3 px-3 py-2 rounded-lg bg-surface text-on-surface text-sm border border-outline-variant focus:outline-none focus:ring-1 focus:ring-primary"
        />

        <label className="block text-xs font-label text-on-surface-variant mb-1">
          {t('services.field_status')}
        </label>
        <select
          value={status}
          onChange={(e) => setStatus(e.target.value as Service['status'])}
          className="w-full mb-3 px-3 py-2 rounded-lg bg-surface text-on-surface text-sm border border-outline-variant focus:outline-none focus:ring-1 focus:ring-primary"
        >
          {STATUS_OPTIONS.map((opt) => (
            <option key={opt} value={opt}>
              {t(`services.status_${opt}`)}
            </option>
          ))}
        </select>

        <label className="block text-xs font-label text-on-surface-variant mb-1">
          {t('services.field_tags')}
          <span className="text-on-surface-variant/60 normal-case ml-1">
            ({t('services.field_tags_hint')})
          </span>
        </label>
        <input
          type="text"
          value={tagsInput}
          onChange={(e) => setTagsInput(e.target.value)}
          placeholder="payments, customer-facing"
          className="w-full mb-3 px-3 py-2 rounded-lg bg-surface text-on-surface text-sm border border-outline-variant focus:outline-none focus:ring-1 focus:ring-primary"
        />

        {error && (
          <div className="mb-3 rounded-lg bg-error-container text-on-error-container text-sm p-3">
            {error}
          </div>
        )}

        <div className="flex justify-end gap-2">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 rounded-lg text-on-surface-variant hover:bg-surface-container-high text-sm"
          >
            {t('common.cancel')}
          </button>
          <button
            type="submit"
            disabled={update.isPending}
            className="bg-primary text-on-primary px-4 py-2 rounded-lg text-sm font-bold disabled:opacity-50"
          >
            {update.isPending ? t('common.saving') : t('common.save')}
          </button>
        </div>
      </form>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Add Asset modal — picker is a search-as-you-type combo over the global
// /assets list. We hide assets already attached to keep the picker uncluttered.
// ---------------------------------------------------------------------------

interface AddAssetModalProps {
  serviceId: string
  existingAssetIds: string[]
  onClose: () => void
}

function AddAssetModal({ serviceId, existingAssetIds, onClose }: AddAssetModalProps) {
  const { t } = useTranslation()
  const addAsset = useAddServiceAsset(serviceId)

  const [search, setSearch] = useState('')
  const [selectedAssetId, setSelectedAssetId] = useState('')
  const [role, setRole] = useState<ServiceAssetMember['role']>('primary')
  const [isCritical, setIsCritical] = useState(false)
  const [error, setError] = useState('')

  // Cap the picker query — operators rarely need to scroll past 50 candidates,
  // and the asset list endpoint is shared with bulk pages where 100+ rows hurt.
  const assetsQ = useAssets({ page_size: '50', ...(search.trim() ? { search: search.trim() } : {}) })
  const allAssets = assetsQ.data?.data ?? []
  const existingSet = useMemo(() => new Set(existingAssetIds), [existingAssetIds])
  const candidates = allAssets.filter((a) => !existingSet.has(a.id))

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    if (!selectedAssetId) {
      setError(t('services.error_pick_asset'))
      return
    }
    try {
      await addAsset.mutateAsync({
        asset_id: selectedAssetId,
        role,
        is_critical: isCritical,
      })
      toast.success(t('services.toast_asset_added'))
      onClose()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err)
      const friendly = msg.includes('409') ? t('services.error_asset_already_member') : msg
      setError(friendly)
      toast.error(friendly)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <form
        onSubmit={handleSubmit}
        className="bg-surface-container w-full max-w-lg rounded-2xl p-6 shadow-xl max-h-[90vh] overflow-y-auto"
      >
        <h2 className="font-headline text-xl font-bold mb-4 text-on-surface">
          {t('services.add_asset')}
        </h2>

        <label className="block text-xs font-label text-on-surface-variant mb-1">
          {t('services.search_assets')}
        </label>
        <input
          type="text"
          value={search}
          onChange={(e) => {
            setSearch(e.target.value)
            // Clear the picked asset whenever the search query changes; the
            // selection may no longer be in the visible list.
            setSelectedAssetId('')
          }}
          placeholder={t('services.search_assets_placeholder')}
          className="w-full mb-2 px-3 py-2 rounded-lg bg-surface text-on-surface text-sm border border-outline-variant focus:outline-none focus:ring-1 focus:ring-primary"
        />

        <div className="mb-3 max-h-48 overflow-y-auto rounded-lg border border-outline-variant/40 bg-surface">
          {assetsQ.isLoading ? (
            <div className="p-4 text-center text-xs text-on-surface-variant">
              {t('common.loading')}
            </div>
          ) : candidates.length === 0 ? (
            <div className="p-4 text-center text-xs text-on-surface-variant">
              {t('services.no_assets_to_add')}
            </div>
          ) : (
            <ul role="listbox" aria-label={t('services.search_assets')}>
              {candidates.map((a) => {
                const isSelected = a.id === selectedAssetId
                return (
                  <li key={a.id}>
                    <button
                      type="button"
                      role="option"
                      aria-selected={isSelected}
                      onClick={() => setSelectedAssetId(a.id)}
                      className={`w-full text-left px-3 py-2 text-sm border-b border-outline-variant/10 last:border-b-0 transition-colors ${
                        isSelected
                          ? 'bg-primary-container text-on-primary-container'
                          : 'hover:bg-surface-container-high'
                      }`}
                    >
                      <div className="flex items-center justify-between gap-2">
                        <span className="font-mono text-xs">{a.asset_tag}</span>
                        <span className="text-[10px] uppercase tracking-widest text-on-surface-variant">
                          {a.type}
                        </span>
                      </div>
                      <div className="text-xs text-on-surface-variant truncate">{a.name}</div>
                    </button>
                  </li>
                )
              })}
            </ul>
          )}
        </div>

        <label className="block text-xs font-label text-on-surface-variant mb-1">
          {t('services.field_role')}
        </label>
        <select
          value={role}
          onChange={(e) => setRole(e.target.value as ServiceAssetMember['role'])}
          className="w-full mb-3 px-3 py-2 rounded-lg bg-surface text-on-surface text-sm border border-outline-variant focus:outline-none focus:ring-1 focus:ring-primary"
        >
          {ASSET_ROLES.map((r) => (
            <option key={r} value={r}>
              {t(`services.role_${r}`)}
            </option>
          ))}
        </select>

        <label className="flex items-center gap-2 mb-3 text-sm text-on-surface cursor-pointer select-none">
          <input
            type="checkbox"
            checked={isCritical}
            onChange={(e) => setIsCritical(e.target.checked)}
            className="rounded border-outline-variant text-primary focus:ring-1 focus:ring-primary"
          />
          {t('services.field_is_critical')}
        </label>

        {error && (
          <div className="mb-3 rounded-lg bg-error-container text-on-error-container text-sm p-3">
            {error}
          </div>
        )}

        <div className="flex justify-end gap-2">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 rounded-lg text-on-surface-variant hover:bg-surface-container-high text-sm"
          >
            {t('common.cancel')}
          </button>
          <button
            type="submit"
            disabled={addAsset.isPending || !selectedAssetId}
            className="bg-primary text-on-primary px-4 py-2 rounded-lg text-sm font-bold disabled:opacity-50"
          >
            {addAsset.isPending ? t('common.saving') : t('services.add_asset')}
          </button>
        </div>
      </form>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Delete confirmation — requires typing the service code to prevent fat-finger
// loss of a critical mapping. Modeled on GitHub's "type the repo name" pattern.
// ---------------------------------------------------------------------------

interface DeleteServiceDialogProps {
  service: Service
  onClose: () => void
  onDeleted: () => void
}

function DeleteServiceDialog({ service, onClose, onDeleted }: DeleteServiceDialogProps) {
  const { t } = useTranslation()
  const del = useDeleteService()
  const [confirmCode, setConfirmCode] = useState('')
  const [error, setError] = useState('')

  const matches = confirmCode === service.code

  const handleDelete = async () => {
    if (!matches) return
    setError('')
    try {
      await del.mutateAsync(service.id)
      toast.success(t('services.toast_deleted'))
      onDeleted()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err)
      setError(msg)
      toast.error(msg)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="delete-service-title"
    >
      <div className="bg-surface-container w-full max-w-md rounded-2xl p-6 shadow-xl">
        <h2 id="delete-service-title" className="font-headline text-xl font-bold mb-2 text-error">
          {t('services.delete_confirm_title')}
        </h2>
        <p className="text-sm text-on-surface-variant mb-4">
          {t('services.delete_confirm_description', { code: service.code })}
        </p>

        <label className="block text-xs font-label text-on-surface-variant mb-1">
          {t('services.delete_confirm_prompt', { code: service.code })}
        </label>
        <input
          type="text"
          value={confirmCode}
          onChange={(e) => setConfirmCode(e.target.value)}
          autoFocus
          autoComplete="off"
          className="w-full mb-3 px-3 py-2 rounded-lg bg-surface text-on-surface font-mono text-sm border border-outline-variant focus:outline-none focus:ring-1 focus:ring-error"
        />

        {error && (
          <div className="mb-3 rounded-lg bg-error-container text-on-error-container text-sm p-3">
            {error}
          </div>
        )}

        <div className="flex justify-end gap-2">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 rounded-lg text-on-surface-variant hover:bg-surface-container-high text-sm"
          >
            {t('common.cancel')}
          </button>
          <button
            type="button"
            onClick={handleDelete}
            disabled={!matches || del.isPending}
            className="bg-error text-on-error px-4 py-2 rounded-lg text-sm font-bold disabled:opacity-40"
          >
            {del.isPending ? t('common.deleting') : t('services.delete')}
          </button>
        </div>
      </div>
    </div>
  )
}
