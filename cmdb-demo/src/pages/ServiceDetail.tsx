import { memo } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  useService,
  useServiceAssets,
  useServiceHealth,
  useRemoveServiceAsset,
} from '../hooks/useServices'

function Icon({ name, className = '' }: { name: string; className?: string }) {
  return <span className={`material-symbols-outlined ${className}`}>{name}</span>
}

const HEALTH_COLOR: Record<string, string> = {
  healthy: 'bg-success-container text-success',
  degraded: 'bg-error-container text-error',
  unknown: 'bg-surface-container-high text-on-surface-variant',
}

const ServiceDetail = memo(function ServiceDetail() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()

  const serviceQ = useService(id)
  const assetsQ = useServiceAssets(id)
  const healthQ = useServiceHealth(id)
  const removeAsset = useRemoveServiceAsset(id ?? '')

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
      <div className="flex items-start justify-between mb-6">
        <div>
          <div className="flex items-center gap-3 mb-1">
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
                      className="text-error hover:opacity-80 text-xs"
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
