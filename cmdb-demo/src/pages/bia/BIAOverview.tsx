import { toast } from 'sonner'
import { useState, useMemo } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import Icon from '../../components/Icon'
import BIAComplianceIcon from '../../components/BIAComplianceIcon'
import CreateAssessmentModal from '../../components/CreateAssessmentModal'
import { useBIAScoringRules, useBIAAssessments, useBIAStats } from '../../hooks/useBIA'
import { SEED_RULES, SEED_ASSESSMENTS, SEED_STATS } from '../../data/fallbacks/bia'

/* ──────────────────────────────────────────────
   Local types
   ────────────────────────────────────────────── */

interface BIARule {
  id: string
  tier_name: string
  tier_level: number
  display_name: string
  description: string
  color: string
  icon: string
  min_score: number
  max_score: number
  rto_threshold: number | null
  rpo_threshold: number | null
}

interface BIAAssessment {
  id: string
  system_name: string
  system_code: string
  owner: string | null
  bia_score: number
  tier: string
  rto_hours: number
  rpo_minutes: number | null
  data_compliance: boolean
  asset_compliance: boolean
  audit_compliance: boolean
  description: string
}

interface BIAStats {
  total: number
  by_tier: Record<string, number>
  avg_compliance: number
  data_compliant: number
  asset_compliant: number
  audit_compliant: number
  total_dependencies?: number
}

interface ApiListResponse<T> {
  data?: T[]
  items?: T[]
}

interface ApiDataResponse<T> {
  data?: T
}



/* ──────────────────────────────────────────────
   Constants
   ────────────────────────────────────────────── */

const TIER_COLORS: Record<string, { bg: string; text: string; icon: string }> = {
  critical:  { bg: 'bg-error-container',            text: 'text-on-error-container', icon: 'error' },
  important: { bg: 'bg-[#92400e]',                  text: 'text-[#fbbf24]',          icon: 'warning' },
  normal:    { bg: 'bg-[#1e3a5f]',                  text: 'text-on-primary-container', icon: 'info' },
  minor:     { bg: 'bg-surface-container-highest',   text: 'text-on-surface-variant', icon: 'expand_circle_down' },
}

const DONUT_COLORS: Record<string, string> = {
  critical:  '#ff6b6b',
  important: '#ffa94d',
  normal:    '#9ecaff',
  minor:     '#8e9196',
}

const NAV_ITEM_KEYS = [
  { icon: 'dashboard',  labelKey: 'bia_overview.nav_overview',    path: '/bia' },
  { icon: 'grade',      labelKey: 'bia_overview.nav_grading',     path: '/bia/grading' },
  { icon: 'timer',      labelKey: 'bia_overview.nav_rto_rpo',     path: '/bia/rto-rpo' },
  { icon: 'rule',       labelKey: 'bia_overview.nav_rules',       path: '/bia/rules' },
  { icon: 'device_hub', labelKey: 'bia_overview.nav_dependencies', path: '/bia/dependencies' },
]

/* ──────────────────────────────────────────────
   Donut Chart (SVG)
   ────────────────────────────────────────────── */

function DonutChart({ segments }: { segments: { label: string; value: number; color: string }[] }) {
  const total = segments.reduce((s, seg) => s + seg.value, 0)
  if (total === 0) return null

  const radius = 40
  const circumference = 2 * Math.PI * radius
  let offset = 0

  return (
    <svg viewBox="0 0 100 100" className="w-28 h-28 mx-auto">
      {segments.map((seg) => {
        const pct = seg.value / total
        const dash = pct * circumference
        const gap = circumference - dash
        const currentOffset = offset
        offset += dash
        return (
          <circle
            key={seg.label}
            cx="50" cy="50" r={radius}
            fill="none"
            stroke={seg.color}
            strokeWidth="12"
            strokeDasharray={`${dash} ${gap}`}
            strokeDashoffset={-currentOffset}
            className="transition-all duration-500"
          />
        )
      })}
    </svg>
  )
}

/* ──────────────────────────────────────────────
   Score arrow indicator
   ────────────────────────────────────────────── */

function BiaScoreArrow({ score }: { score: number }) {
  if (score >= 85) return <span className="material-symbols-outlined text-sm text-[#34d399] ml-1">arrow_upward</span>
  if (score >= 60) return <span className="material-symbols-outlined text-sm text-[#fbbf24] ml-1">arrow_forward</span>
  return <span className="material-symbols-outlined text-sm text-error ml-1">arrow_downward</span>
}

/* ──────────────────────────────────────────────
   Main Component
   ────────────────────────────────────────────── */

export default function BIAOverview() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const [modalOpen, setModalOpen] = useState(false)
  const [tierFilter, setTierFilter] = useState<string | null>(null)

  // Data hooks (fall back to seed data when API unavailable)
  const rulesQuery = useBIAScoringRules()
  const assessmentsQuery = useBIAAssessments()
  const statsQuery = useBIAStats()

  const rules: BIARule[] = (rulesQuery.data as ApiListResponse<BIARule>)?.data || SEED_RULES
  const rawAssessments: BIAAssessment[] = (assessmentsQuery.data as ApiListResponse<BIAAssessment>)?.data || (assessmentsQuery.data as ApiListResponse<BIAAssessment>)?.items || SEED_ASSESSMENTS
  const stats: BIAStats = (statsQuery.data as ApiDataResponse<BIAStats>)?.data ?? SEED_STATS

  const assessments = useMemo(() => {
    if (!tierFilter) return rawAssessments
    return rawAssessments.filter((a) => a.tier === tierFilter)
  }, [rawAssessments, tierFilter])

  // Donut segments
  const donutSegments = useMemo(() => {
    const byTier = stats.by_tier || {}
    return Object.entries(byTier).map(([tier, count]) => ({
      label: tier,
      value: count as number,
      color: DONUT_COLORS[tier] || '#8e9196',
    }))
  }, [stats])

  return (
    <div className="space-y-5">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <span className="material-symbols-outlined text-primary text-2xl">assessment</span>
          <h1 className="font-headline text-xl font-bold text-on-surface">{t('bia_overview.page_title')}</h1>
        </div>
        <div className="flex items-center gap-2">
          <button className="flex items-center gap-1.5 px-3 py-1.5 rounded bg-surface-container text-on-surface-variant text-xs hover:bg-surface-container-high transition-colors">
            <span className="material-symbols-outlined text-sm">download</span>
            {t('bia_overview.btn_export')}
          </button>
          <button className="flex items-center gap-1.5 px-3 py-1.5 rounded bg-surface-container text-on-surface-variant text-xs hover:bg-surface-container-high transition-colors">
            <span className="material-symbols-outlined text-sm">description</span>
            {t('bia_overview.btn_report')}
          </button>
        </div>
      </div>

      {/* Main grid: Left Nav + Content */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[240px_1fr]">
        {/* Left Nav Panel */}
        <div className="space-y-1">
          {NAV_ITEM_KEYS.map((item) => {
            const active = location.pathname === item.path
            return (
              <button
                key={item.path}
                onClick={() => navigate(item.path)}
                className={`w-full flex items-center gap-3 px-4 py-2.5 rounded-lg text-sm transition-colors ${
                  active
                    ? 'bg-surface-container text-primary'
                    : 'text-on-surface-variant hover:bg-surface-container-low'
                }`}
              >
                <Icon name={item.icon} className="text-[20px]" />
                <span className="font-headline tracking-tight font-semibold text-[0.75rem]">
                  {t(item.labelKey)}
                </span>
              </button>
            )
          })}

          <div className="my-4 border-t border-outline-variant/20" />

          {/* Run New Analysis */}
          <button
            onClick={() => setModalOpen(true)}
            className="w-full flex items-center gap-3 px-4 py-2.5 rounded-lg text-sm machined-gradient text-on-primary font-semibold"
          >
            <Icon name="play_arrow" className="text-[20px]" />
            <span className="font-headline tracking-tight font-semibold text-[0.75rem]">
              {t('bia_overview.btn_run_analysis')}
            </span>
          </button>

          <button
            onClick={() => navigate('/audit')}
            className="w-full flex items-center gap-3 px-4 py-2.5 rounded-lg text-sm text-on-surface-variant hover:bg-surface-container-low transition-colors"
          >
            <Icon name="history" className="text-[20px]" />
            <span className="font-headline tracking-tight font-semibold text-[0.75rem]">
              {t('bia_overview.btn_audit_logs')}
            </span>
          </button>

          <button
            onClick={() => toast.info('Coming Soon')}
            className="w-full flex items-center gap-3 px-4 py-2.5 rounded-lg text-sm text-on-surface-variant hover:bg-surface-container-low transition-colors"
          >
            <Icon name="download" className="text-[20px]" />
            <span className="font-headline tracking-tight font-semibold text-[0.75rem]">
              {t('bia_overview.btn_export')}
            </span>
          </button>
        </div>

        {/* Right Content */}
        <div className="space-y-5">
          {/* Row 1: 3 stat cards */}
          <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
            {/* Tier Rules Card */}
            <div className="rounded-lg bg-surface-container p-5">
              <div className="mb-4 flex items-center gap-2">
                <Icon name="tune" className="text-primary text-xl" />
                <h3 className="font-headline text-[0.6875rem] font-semibold uppercase tracking-wider text-on-surface-variant">
                  {t('bia_overview.card_tier_rules')}
                </h3>
              </div>
              <div className="space-y-2">
                {rules.map((rule) => {
                  const colors = TIER_COLORS[rule.tier_name] || TIER_COLORS.minor
                  return (
                    <div
                      key={rule.id}
                      className="flex items-center gap-3 rounded-lg bg-surface-container-low p-3"
                    >
                      <Icon name={rule.icon || colors.icon} className={`${colors.text} text-xl`} />
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-semibold text-on-surface truncate">
                          {rule.display_name}
                        </p>
                        <p className="text-xs text-on-surface-variant truncate">
                          {rule.description}
                        </p>
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>

            {/* System Overview Card */}
            <div className="rounded-lg bg-surface-container p-5">
              <div className="mb-4 flex items-center gap-2">
                <Icon name="donut_large" className="text-primary text-xl" />
                <h3 className="font-headline text-[0.6875rem] font-semibold uppercase tracking-wider text-on-surface-variant">
                  {t('bia_overview.card_system_overview')}
                </h3>
              </div>
              <DonutChart segments={donutSegments} />
              <div className="mt-3 text-center">
                <p className="font-headline text-3xl font-bold text-on-surface">
                  {stats.total}
                </p>
                <p className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant">
                  {t('bia_overview.total_monitored')}
                </p>
              </div>
              <div className="mt-2 text-center">
                <p className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant">
                  {t('bia_overview.avg_compliance')}
                </p>
                <p className="font-headline text-xl font-bold text-[#34d399]">
                  {stats.avg_compliance}%
                </p>
              </div>
              {/* Tier legend */}
              <div className="mt-3 grid grid-cols-2 gap-1">
                {Object.entries(stats.by_tier || {}).map(([tier, count]) => (
                  <div key={tier} className="flex items-center gap-1.5">
                    <span
                      className="w-2.5 h-2.5 rounded-full"
                      style={{ backgroundColor: DONUT_COLORS[tier] || '#8e9196' }}
                    />
                    <span className="text-xs text-on-surface-variant capitalize">
                      {tier}: {count as number}
                    </span>
                  </div>
                ))}
              </div>
            </div>

            {/* Asset Dependency Card */}
            <div className="rounded-lg bg-surface-container p-5">
              <div className="mb-4 flex items-center gap-2">
                <Icon name="device_hub" className="text-primary text-xl" />
                <h3 className="font-headline text-[0.6875rem] font-semibold uppercase tracking-wider text-on-surface-variant">
                  {t('bia_overview.card_asset_deps')}
                </h3>
              </div>
              <div className="space-y-3">
                <div className="rounded-lg bg-surface-container-low p-4 text-center">
                  <p className="font-headline text-3xl font-bold text-on-surface">
                    {stats.total_dependencies ?? 8}
                  </p>
                  <p className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant mt-1">
                    {t('bia_overview.linked_cis')}
                  </p>
                </div>
                <div className="space-y-2">
                  {[
                    { labelKey: 'bia_overview.data_compliant', count: stats.data_compliant, icon: 'storage' },
                    { labelKey: 'bia_overview.asset_compliant', count: stats.asset_compliant, icon: 'dns' },
                    { labelKey: 'bia_overview.audit_compliant', count: stats.audit_compliant, icon: 'verified_user' },
                  ].map((item) => (
                    <div
                      key={item.labelKey}
                      className="flex items-center justify-between rounded-lg bg-surface-container-low px-3 py-2"
                    >
                      <div className="flex items-center gap-2">
                        <Icon name={item.icon} className="text-on-surface-variant text-lg" />
                        <span className="text-xs text-on-surface-variant">{t(item.labelKey)}</span>
                      </div>
                      <span className="text-sm font-semibold text-on-surface">
                        {item.count}/{stats.total}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </div>

          {/* Row 2: Assessment Matrix Table */}
          <div className="rounded-lg bg-surface-container overflow-x-auto">
            <div className="flex items-center justify-between p-5 pb-3">
              <div className="flex items-center gap-2">
                <Icon name="assessment" className="text-primary text-xl" />
                <h3 className="font-headline text-[0.6875rem] font-semibold uppercase tracking-wider text-on-surface-variant">
                  {t('bia_overview.table_title')}
                </h3>
              </div>
              <div className="flex items-center gap-2">
                {Object.entries(TIER_COLORS).map(([tier, colors]) => (
                  <button
                    key={tier}
                    onClick={() => setTierFilter(tierFilter === tier ? null : tier)}
                    className={`px-2.5 py-1 rounded text-[0.6875rem] font-semibold uppercase transition-colors ${
                      tierFilter === tier
                        ? `${colors.bg} ${colors.text}`
                        : 'bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest'
                    }`}
                  >
                    {tier}
                  </button>
                ))}
              </div>
            </div>
            <table className="w-full">
              <thead>
                <tr className="bg-surface-container-high">
                  <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                    {t('bia_overview.col_business_system')}
                  </th>
                  <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                    {t('bia_overview.col_owner')}
                  </th>
                  <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                    {t('bia_overview.col_tier')}
                  </th>
                  <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                    {t('bia_overview.col_bia_score')}
                  </th>
                  <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                    {t('bia_overview.col_rto_hrs')}
                  </th>
                  <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                    {t('bia_overview.col_rpo_min')}
                  </th>
                  <th className="px-5 py-3 text-center text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                    {t('bia_overview.col_asset')}
                  </th>
                  <th className="px-5 py-3 text-center text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                    {t('bia_overview.col_data')}
                  </th>
                  <th className="px-5 py-3 text-center text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                    {t('bia_overview.col_audit')}
                  </th>
                </tr>
              </thead>
              <tbody>
                {assessments.map((a) => {
                  const tierColor = TIER_COLORS[a.tier] || TIER_COLORS.minor
                  return (
                    <tr
                      key={a.id}
                      className="hover:bg-surface-container-low cursor-pointer transition-colors border-t border-outline-variant/10"
                    >
                      <td className="px-5 py-3.5">
                        <p className="text-sm font-semibold text-on-surface">
                          {a.system_name}
                        </p>
                        <p className="text-xs text-on-surface-variant">
                          {a.system_code}
                        </p>
                      </td>
                      <td className="px-5 py-3.5 text-sm text-on-surface">
                        {a.owner || '\u2014'}
                      </td>
                      <td className="px-5 py-3.5">
                        <span
                          className={`inline-block px-2.5 py-1 rounded text-[0.6875rem] font-semibold uppercase ${tierColor.bg} ${tierColor.text}`}
                        >
                          {a.tier}
                        </span>
                      </td>
                      <td className="px-5 py-3.5">
                        <span className="font-headline text-lg font-bold text-on-surface">
                          {a.bia_score}
                        </span>
                        <BiaScoreArrow score={a.bia_score} />
                      </td>
                      <td className="px-5 py-3.5 text-sm text-on-surface">
                        {a.rto_hours}
                      </td>
                      <td className="px-5 py-3.5 text-sm text-on-surface">
                        {a.rpo_minutes ?? 'N/A'}
                      </td>
                      <td className="px-5 py-3.5 text-center">
                        <BIAComplianceIcon ok={a.asset_compliance} />
                      </td>
                      <td className="px-5 py-3.5 text-center">
                        <BIAComplianceIcon ok={a.data_compliance} />
                      </td>
                      <td className="px-5 py-3.5 text-center">
                        <BIAComplianceIcon ok={a.audit_compliance} />
                      </td>
                    </tr>
                  )
                })}
                {assessments.length === 0 && (
                  <tr>
                    <td colSpan={9} className="px-5 py-8 text-center text-sm text-on-surface-variant">
                      {t('bia_overview.no_assessments')}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      {/* Create Assessment Modal */}
      <CreateAssessmentModal open={modalOpen} onClose={() => setModalOpen(false)} />
    </div>
  )
}
