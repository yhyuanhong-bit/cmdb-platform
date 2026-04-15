import { memo, useMemo } from 'react'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useBIAAssessments, useBIAScoringRules } from '../../hooks/useBIA'

/* ──────────────────────────────────────────────
   Local types
   ────────────────────────────────────────────── */

interface BIAAssessment {
  id: string
  system_name: string
  tier: string
  rto_hours?: number | null
  rpo_minutes?: number | null
}

interface BIARule {
  id: string
  tier_name: string
  rto_threshold?: number | null
  rpo_threshold?: number | null
}

interface ApiListResponse<T> {
  data?: T[]
}



const TIER_BADGE: Record<string, string> = {
  critical:  'bg-[#7f1d1d] text-[#fca5a5]',
  important: 'bg-[#78350f] text-[#fde68a]',
  normal:    'bg-[#1e3a5f] text-[#93c5fd]',
  low:       'bg-[#374151] text-[#d1d5db]',
}

function Icon({ name, className = '' }: { name: string; className?: string }) {
  return <span className={`material-symbols-outlined ${className}`}>{name}</span>
}

function getBadge(tier: string) {
  return TIER_BADGE[tier.toLowerCase()] || TIER_BADGE.low
}

function ComplianceMatrix({
  titleKey,
  subtitleKey,
  assessments,
  rules,
  mode,
}: {
  titleKey: string
  subtitleKey: string
  assessments: BIAAssessment[]
  rules: BIARule[]
  mode: 'rto' | 'rpo'
}) {
  const { t } = useTranslation()
  const sorted = useMemo(() => {
    return [...assessments].sort((a, b) => {
      const aVal = mode === 'rto' ? (a.rto_hours ?? Infinity) : (a.rpo_minutes ?? Infinity)
      const bVal = mode === 'rto' ? (b.rto_hours ?? Infinity) : (b.rpo_minutes ?? Infinity)
      return aVal - bVal
    })
  }, [assessments, mode])

  const valueLabel = mode === 'rto' ? t('bia_rto_rpo.col_rto_hrs') : t('bia_rto_rpo.col_rpo_min')

  return (
    <div className="rounded-lg bg-surface-container p-5">
      <div className="mb-4">
        <h3 className="font-headline font-bold text-lg text-on-surface">{t(titleKey)}</h3>
        <p className="text-xs text-on-surface-variant mt-1">{t(subtitleKey)}</p>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-outline-variant">
              <th className="text-left py-2.5 px-3 text-[0.6875rem] uppercase tracking-wider text-on-surface-variant font-medium">{t('bia_rto_rpo.col_system')}</th>
              <th className="text-left py-2.5 px-3 text-[0.6875rem] uppercase tracking-wider text-on-surface-variant font-medium">{t('bia_rto_rpo.col_tier')}</th>
              <th className="text-right py-2.5 px-3 text-[0.6875rem] uppercase tracking-wider text-on-surface-variant font-medium">{valueLabel}</th>
              <th className="text-right py-2.5 px-3 text-[0.6875rem] uppercase tracking-wider text-on-surface-variant font-medium">{t('bia_rto_rpo.col_threshold')}</th>
              <th className="text-center py-2.5 px-3 text-[0.6875rem] uppercase tracking-wider text-on-surface-variant font-medium">{t('bia_rto_rpo.col_status')}</th>
            </tr>
          </thead>
          <tbody>
            {sorted.length === 0 ? (
              <tr>
                <td colSpan={5} className="py-10 text-center text-on-surface-variant">{t('bia_rto_rpo.no_data')}</td>
              </tr>
            ) : (
              sorted.map((a) => {
                const rule = rules.find((r) => r.tier_name.toLowerCase() === a.tier.toLowerCase())
                const value = mode === 'rto' ? a.rto_hours : a.rpo_minutes
                const threshold = mode === 'rto' ? rule?.rto_threshold : rule?.rpo_threshold
                const ok = threshold != null && value != null && value <= threshold
                const exceeded = threshold != null && value != null && value > threshold
                return (
                  <tr key={a.id} className="border-b border-outline-variant/30 hover:bg-surface-container-high/40 transition-colors">
                    <td className="py-2.5 px-3 text-on-surface font-medium">{a.system_name}</td>
                    <td className="py-2.5 px-3">
                      <span className={`inline-block rounded px-2 py-0.5 text-xs font-medium uppercase ${getBadge(a.tier)}`}>
                        {a.tier}
                      </span>
                    </td>
                    <td className="py-2.5 px-3 text-right text-on-surface font-mono">
                      {value ?? '—'}
                    </td>
                    <td className="py-2.5 px-3 text-right text-on-surface-variant font-mono">
                      {threshold ?? '—'}
                    </td>
                    <td className="py-2.5 px-3 text-center">
                      {value == null ? (
                        <span className="text-on-surface-variant">—</span>
                      ) : ok ? (
                        <Icon name="check_circle" className="text-[#34d399] text-lg" />
                      ) : exceeded ? (
                        <Icon name="cancel" className="text-[#f87171] text-lg" />
                      ) : (
                        <span className="text-on-surface-variant">—</span>
                      )}
                    </td>
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function RtoRpoMatrices() {
  const { t } = useTranslation()
  const { data: assessResp, isLoading: assessLoading } = useBIAAssessments()
  const { data: rulesResp, isLoading: rulesLoading } = useBIAScoringRules()

  const assessments: BIAAssessment[] = (assessResp as ApiListResponse<BIAAssessment>)?.data || []
  const rules: BIARule[] = (rulesResp as ApiListResponse<BIARule>)?.data || []
  const isLoading = assessLoading || rulesLoading

  return (
    <div className="space-y-6">
      {/* Breadcrumb + Header */}
      <div>
        <div className="flex items-center gap-1.5 text-[0.6875rem] uppercase tracking-wider text-on-surface-variant mb-2">
          <Link to="/bia" className="hover:text-on-surface transition-colors">{t('bia_rto_rpo.breadcrumb_bia')}</Link>
          <Icon name="chevron_right" className="text-base" />
          <span className="text-on-surface">{t('bia_rto_rpo.page_title')}</span>
        </div>
        <h1 className="font-headline font-bold text-2xl text-on-surface">{t('bia_rto_rpo.page_title')}</h1>
      </div>

      {isLoading ? (
        <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
          {[1, 2].map((i) => (
            <div key={i} className="rounded-lg bg-surface-container p-5 h-64 animate-pulse" />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
          <ComplianceMatrix
            titleKey="bia_rto_rpo.rto_title"
            subtitleKey="bia_rto_rpo.rto_subtitle"
            assessments={assessments}
            rules={rules}
            mode="rto"
          />
          <ComplianceMatrix
            titleKey="bia_rto_rpo.rpo_title"
            subtitleKey="bia_rto_rpo.rpo_subtitle"
            assessments={assessments}
            rules={rules}
            mode="rpo"
          />
        </div>
      )}
    </div>
  )
}

export default memo(RtoRpoMatrices)
