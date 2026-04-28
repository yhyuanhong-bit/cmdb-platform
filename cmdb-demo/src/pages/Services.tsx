import { memo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useServices, useCreateService } from '../hooks/useServices'
import type { Service } from '../lib/api/services'
import EmptyState from '../components/EmptyState'

function Icon({ name, className = '' }: { name: string; className?: string }) {
  return <span className={`material-symbols-outlined ${className}`}>{name}</span>
}

/** Map tier → color-scheme pair for the table + cards. Keeping this out
 * of the JSX tree lets the same set of tier styles serve both the list
 * and ServiceDetail without duplicating. */
const TIER_STYLES: Record<Service['tier'], { badge: string; text: string }> = {
  critical: { badge: 'bg-error-container', text: 'text-error' },
  important: { badge: 'bg-warning-container', text: 'text-warning' },
  normal: { badge: 'bg-primary-container', text: 'text-primary' },
  low: { badge: 'bg-surface-container-high', text: 'text-on-surface-variant' },
  minor: { badge: 'bg-surface-container-high', text: 'text-on-surface-variant' },
}

const STATUS_STYLES: Record<Service['status'], string> = {
  active: 'text-success',
  deprecated: 'text-warning',
  decommissioned: 'text-on-surface-variant line-through',
}

type StatusFilter = Service['status'] | 'all'

const Services = memo(function Services() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [tierFilter, setTierFilter] = useState<Service['tier'] | ''>('')
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('active')
  const [showCreate, setShowCreate] = useState(false)

  const { data, isLoading, error } = useServices({
    tier: tierFilter || undefined,
    status: statusFilter === 'all' ? undefined : statusFilter,
    page: 1,
    page_size: 100,
  })

  const services = data?.data ?? []

  return (
    <div className="min-h-screen bg-surface p-6 font-body text-on-surface">
      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="font-headline text-3xl font-bold tracking-tight text-on-surface">
            {t('services.title')}
          </h1>
          <p className="mt-1 text-sm text-on-surface-variant">
            {t('services.subtitle')}
          </p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="bg-primary hover:opacity-90 text-on-primary px-4 py-2 rounded-xl text-sm font-label font-bold flex items-center gap-2 transition-opacity"
        >
          <Icon name="add" className="text-lg" />
          {t('services.create')}
        </button>
      </div>

      {/* Tier filter bar. Each button toggles a single tier; clicking the
          active filter again returns to the unfiltered view. */}
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <button
          onClick={() => setTierFilter('')}
          className={`px-3 py-1.5 rounded-lg text-xs font-label transition-colors ${
            tierFilter === ''
              ? 'bg-primary text-on-primary'
              : 'bg-surface-container text-on-surface-variant hover:bg-surface-container-high'
          }`}
        >
          {t('services.filter_all')}
        </button>
        {(['critical', 'important', 'normal', 'low'] as const).map((tier) => (
          <button
            key={tier}
            onClick={() => setTierFilter(tierFilter === tier ? '' : tier)}
            className={`px-3 py-1.5 rounded-lg text-xs font-label transition-colors ${
              tierFilter === tier
                ? 'bg-primary text-on-primary'
                : 'bg-surface-container text-on-surface-variant hover:bg-surface-container-high'
            }`}
          >
            {t(`services.tier_${tier}`)}
          </button>
        ))}

        <span className="ml-auto inline-flex items-center gap-2">
          <label htmlFor="services-status-filter" className="text-[10px] uppercase tracking-widest text-on-surface-variant font-label">
            {t('services.col_status')}
          </label>
          <select
            id="services-status-filter"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as StatusFilter)}
            className="px-3 py-1.5 rounded-lg text-xs font-label bg-surface-container text-on-surface border border-outline-variant focus:outline-none focus:ring-1 focus:ring-primary"
          >
            <option value="active">{t('services.status_active')}</option>
            <option value="deprecated">{t('services.status_deprecated')}</option>
            <option value="decommissioned">{t('services.status_decommissioned')}</option>
            <option value="all">{t('services.status_filter_all')}</option>
          </select>
        </span>
      </div>

      {isLoading && (
        <div className="flex items-center justify-center py-20">
          <div className="animate-spin rounded-full h-8 w-8 border-2 border-primary border-t-transparent" />
        </div>
      )}

      {error && (
        <EmptyState
          icon="error"
          title={t('common.error')}
          description={String(error)}
          tone="warning"
        />
      )}

      {!isLoading && !error && services.length === 0 && (
        <EmptyState
          icon="layers"
          title={t('services.empty_title')}
          description={t('services.empty_description')}
          tone="info"
        />
      )}

      {services.length > 0 && (
        <div className="bg-surface-container rounded-xl overflow-hidden">
          <table className="w-full">
            <thead>
              <tr className="border-b border-outline-variant/20 text-[10px] uppercase tracking-widest text-on-surface-variant">
                <th className="py-3 px-4 text-left font-label">{t('services.col_code')}</th>
                <th className="py-3 px-4 text-left font-label">{t('services.col_name')}</th>
                <th className="py-3 px-4 text-left font-label">{t('services.col_tier')}</th>
                <th className="py-3 px-4 text-left font-label">{t('services.col_status')}</th>
                <th className="py-3 px-4 text-left font-label">{t('services.col_owner')}</th>
                <th className="py-3 px-4 text-right font-label">{t('services.col_updated')}</th>
              </tr>
            </thead>
            <tbody>
              {services.map((svc) => {
                const tierStyle = TIER_STYLES[svc.tier]
                return (
                  <tr
                    key={svc.id}
                    onClick={() => navigate(`/services/${svc.id}`)}
                    className="border-b border-outline-variant/10 hover:bg-surface-container-high cursor-pointer transition-colors"
                  >
                    <td className="py-3 px-4 font-mono text-xs text-on-surface">{svc.code}</td>
                    <td className="py-3 px-4 text-sm text-on-surface font-medium">{svc.name}</td>
                    <td className="py-3 px-4">
                      <span
                        className={`inline-flex items-center gap-1 px-2 py-1 rounded text-[10px] font-label tracking-widest uppercase ${tierStyle.badge} ${tierStyle.text}`}
                      >
                        {t(`services.tier_${svc.tier}`)}
                      </span>
                    </td>
                    <td className={`py-3 px-4 text-xs ${STATUS_STYLES[svc.status]}`}>
                      {t(`services.status_${svc.status}`)}
                    </td>
                    <td className="py-3 px-4 text-xs text-on-surface-variant">
                      {svc.owner_team ?? '—'}
                    </td>
                    <td className="py-3 px-4 text-right text-xs text-on-surface-variant">
                      {new Date(svc.updated_at).toLocaleDateString()}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      {showCreate && <CreateServiceModal onClose={() => setShowCreate(false)} />}
    </div>
  )
})

export default Services

// ---------------------------------------------------------------------------
// Create modal — minimal form covering the 3 required fields + tier. Owner
// team + description are editable from the detail page later.
// ---------------------------------------------------------------------------

function CreateServiceModal({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const create = useCreateService()
  const [code, setCode] = useState('')
  const [name, setName] = useState('')
  const [tier, setTier] = useState<Service['tier']>('normal')
  const [error, setError] = useState('')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    // Mirror the backend Q1 regex so the user gets a fast error, not
    // a round-trip to the API.
    if (!/^[A-Z][A-Z0-9_-]{1,63}$/.test(code)) {
      setError(t('services.error_invalid_code'))
      return
    }
    try {
      const res = await create.mutateAsync({ code, name, tier })
      onClose()
      if (res.data?.id) navigate(`/services/${res.data.id}`)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err)
      setError(msg.includes('409') ? t('services.error_duplicate_code') : msg)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <form
        onSubmit={handleSubmit}
        className="bg-surface-container w-full max-w-md rounded-2xl p-6 shadow-xl"
      >
        <h2 className="font-headline text-xl font-bold mb-4 text-on-surface">
          {t('services.create')}
        </h2>

        <label className="block text-xs font-label text-on-surface-variant mb-1">
          {t('services.field_code')}
        </label>
        <input
          type="text"
          value={code}
          onChange={(e) => setCode(e.target.value.toUpperCase())}
          placeholder="ORDER-API"
          className="w-full mb-3 px-3 py-2 rounded-lg bg-surface text-on-surface font-mono text-sm border border-outline-variant focus:outline-none focus:ring-1 focus:ring-primary"
          required
        />

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
          {t('services.field_tier')}
        </label>
        <select
          value={tier}
          onChange={(e) => setTier(e.target.value as Service['tier'])}
          className="w-full mb-3 px-3 py-2 rounded-lg bg-surface text-on-surface text-sm border border-outline-variant focus:outline-none focus:ring-1 focus:ring-primary"
        >
          <option value="critical">{t('services.tier_critical')}</option>
          <option value="important">{t('services.tier_important')}</option>
          <option value="normal">{t('services.tier_normal')}</option>
          <option value="low">{t('services.tier_low')}</option>
        </select>

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
            disabled={create.isPending}
            className="bg-primary text-on-primary px-4 py-2 rounded-lg text-sm font-bold disabled:opacity-50"
          >
            {create.isPending ? t('common.saving') : t('common.create')}
          </button>
        </div>
      </form>
    </div>
  )
}
