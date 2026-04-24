import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import SectionLabel from './SectionLabel'
import { useServicesForAsset } from '../../../hooks/useServices'

const tierStyle: Record<string, string> = {
  platinum: 'bg-error-container text-on-error-container',
  gold: 'bg-[#92400e] text-[#fbbf24]',
  silver: 'bg-[#1e3a5f] text-on-primary-container',
  bronze: 'bg-surface-container-highest text-on-surface-variant',
}

const statusStyle: Record<string, string> = {
  active: 'bg-emerald-500/20 text-emerald-400',
  degraded: 'bg-amber-500/20 text-amber-400',
  incident: 'bg-red-500/20 text-red-400',
  retired: 'bg-surface-container-highest text-on-surface-variant',
}

export default function ServicesPanel({ assetId }: { assetId?: string }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { data, isLoading, error } = useServicesForAsset(assetId)
  const services = data?.data ?? []

  if (isLoading) {
    return (
      <div className="bg-surface-container rounded-lg p-5">
        <SectionLabel>{t('asset_detail.section_business_services')}</SectionLabel>
        <div className="py-4 flex justify-center">
          <div className="animate-spin rounded-full h-5 w-5 border-2 border-sky-400 border-t-transparent" />
        </div>
      </div>
    )
  }

  if (error) {
    // Services is a Wave 2 entity; on older deployments the endpoint may 404.
    // We silently hide the panel rather than surface a scary red error.
    return null
  }

  if (services.length === 0) {
    return (
      <div className="bg-surface-container rounded-lg p-5">
        <SectionLabel>{t('asset_detail.section_business_services')}</SectionLabel>
        <p className="text-xs text-on-surface-variant">
          {t('asset_detail.services_empty')}
        </p>
      </div>
    )
  }

  return (
    <div className="bg-surface-container rounded-lg p-5">
      <div className="flex items-center justify-between mb-3">
        <h3 className="font-label text-[0.6875rem] uppercase tracking-[0.08em] text-on-surface-variant">
          {t('asset_detail.section_business_services')}
        </h3>
        <span className="text-xs text-on-surface-variant">
          {t('asset_detail.services_count', { count: services.length })}
        </span>
      </div>
      <ul className="space-y-2">
        {services.map((svc) => (
          <li key={svc.id}>
            <button
              type="button"
              onClick={() => navigate(`/services/${svc.id}`)}
              className="w-full flex items-center justify-between rounded-lg bg-surface-container-low p-3 hover:bg-surface-container-high transition-colors text-left"
              aria-label={`Open service ${svc.name}`}
            >
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <p className="text-sm font-semibold text-primary truncate">{svc.name}</p>
                  {svc.is_critical && (
                    <span className="px-1.5 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider bg-error-container text-on-error-container">
                      {t('asset_detail.services_critical')}
                    </span>
                  )}
                </div>
                <p className="text-xs text-on-surface-variant font-mono">
                  {svc.code} · {svc.role}
                </p>
              </div>
              <div className="flex items-center gap-2 shrink-0">
                <span className={`px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${tierStyle[svc.tier] ?? tierStyle.bronze}`}>
                  {svc.tier}
                </span>
                <span className={`px-2 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${statusStyle[svc.status] ?? statusStyle.retired}`}>
                  {svc.status}
                </span>
                <span className="material-symbols-outlined text-[18px] text-on-surface-variant">chevron_right</span>
              </div>
            </button>
          </li>
        ))}
      </ul>
    </div>
  )
}
