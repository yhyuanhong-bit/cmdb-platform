import { useState, useMemo } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import Icon from '../../components/Icon'
import BIAComplianceIcon from '../../components/BIAComplianceIcon'
import CreateAssessmentModal from '../../components/CreateAssessmentModal'
import { useBIAScoringRules, useBIAAssessments, useBIAStats } from '../../hooks/useBIA'

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

const NAV_ITEMS = [
  { icon: 'dashboard',  label: 'BIA Overview',     path: '/bia' },
  { icon: 'grade',      label: 'System Grading',   path: '/bia/grading' },
  { icon: 'timer',      label: 'RTO/RPO Matrices', path: '/bia/rto-rpo' },
  { icon: 'rule',       label: 'Scoring Rules',    path: '/bia/rules' },
  { icon: 'device_hub', label: 'Dependency Map',   path: '/bia/dependencies' },
]

/* ──────────────────────────────────────────────
   Fallback / seed data (used when API is unavailable)
   ────────────────────────────────────────────── */

const SEED_RULES = [
  { id: '1', tier_name: 'critical',  tier_level: 1, display_name: 'Tier 1 - CRITICAL',  description: 'Core payment systems, building monitoring - downtime causes major financial/safety impact', color: '#ff6b6b', icon: 'error', min_score: 85, max_score: 100, rto_threshold: 4, rpo_threshold: 15 },
  { id: '2', tier_name: 'important', tier_level: 2, display_name: 'Tier 2 - IMPORTANT', description: 'Core system groups (CRM, ERP) - downtime impacts business efficiency', color: '#ffa94d', icon: 'warning', min_score: 60, max_score: 84, rto_threshold: 12, rpo_threshold: 60 },
  { id: '3', tier_name: 'normal',    tier_level: 3, display_name: 'Tier 3 - NORMAL',    description: 'General business systems - downtime has workarounds available', color: '#9ecaff', icon: 'info', min_score: 30, max_score: 59, rto_threshold: 24, rpo_threshold: 240 },
  { id: '4', tier_name: 'minor',     tier_level: 4, display_name: 'Tier 4 - MINOR',     description: 'Test/sandbox environments - downtime has no business impact', color: '#8e9196', icon: 'expand_circle_down', min_score: 0, max_score: 29, rto_threshold: 72, rpo_threshold: null },
]

const SEED_ASSESSMENTS = [
  { id: '1', system_name: 'Core Payment Gateway', system_code: 'SYS-PROD-PAY-001', owner: 'David Yun', bia_score: 98, tier: 'critical',  rto_hours: 4, rpo_minutes: 15,   data_compliance: true,  asset_compliance: true,  audit_compliance: true,  description: 'Processes all online payment transactions' },
  { id: '2', system_name: 'CRM Core',             system_code: 'SYS-PROD-CRM-001', owner: 'Lin Sheng', bia_score: 85, tier: 'important', rto_hours: 12, rpo_minutes: 120, data_compliance: true,  asset_compliance: true,  audit_compliance: false, description: 'Customer data and service history' },
  { id: '3', system_name: 'Admin Panel',           system_code: 'SYS-CORP-ADM-001', owner: 'Wang Zhi', bia_score: 62, tier: 'normal',    rto_hours: 24, rpo_minutes: 240, data_compliance: true,  asset_compliance: false, audit_compliance: false, description: 'Internal process management' },
  { id: '4', system_name: 'QA Sandbox',            system_code: 'SYS-TEST-QA-001',  owner: null,       bia_score: 15, tier: 'minor',     rto_hours: 72, rpo_minutes: null, data_compliance: false, asset_compliance: false, audit_compliance: false, description: 'QA testing environment' },
]

const SEED_STATS = {
  total: 4,
  by_tier: { critical: 1, important: 1, normal: 1, minor: 1 },
  avg_compliance: 58,
  data_compliant: 3,
  asset_compliant: 2,
  audit_compliant: 1,
}

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
  const navigate = useNavigate()
  const location = useLocation()
  const [modalOpen, setModalOpen] = useState(false)
  const [tierFilter, setTierFilter] = useState<string | null>(null)

  // Data hooks (fall back to seed data when API unavailable)
  const rulesQuery = useBIAScoringRules()
  const assessmentsQuery = useBIAAssessments()
  const statsQuery = useBIAStats()

  const rules: any[] = rulesQuery.data || SEED_RULES
  const rawAssessments: any[] = (assessmentsQuery.data as any)?.items || (assessmentsQuery.data as any) || SEED_ASSESSMENTS
  const stats: any = statsQuery.data || SEED_STATS

  const assessments = useMemo(() => {
    if (!tierFilter) return rawAssessments
    return rawAssessments.filter((a: any) => a.tier === tierFilter)
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
          <h1 className="font-headline text-xl font-bold text-on-surface">BIA Modeler</h1>
        </div>
        <div className="flex items-center gap-2">
          <button className="flex items-center gap-1.5 px-3 py-1.5 rounded bg-surface-container text-on-surface-variant text-xs hover:bg-surface-container-high transition-colors">
            <span className="material-symbols-outlined text-sm">download</span>
            Export
          </button>
          <button className="flex items-center gap-1.5 px-3 py-1.5 rounded bg-surface-container text-on-surface-variant text-xs hover:bg-surface-container-high transition-colors">
            <span className="material-symbols-outlined text-sm">description</span>
            Report
          </button>
        </div>
      </div>

      {/* Main grid: Left Nav + Content */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[240px_1fr]">
        {/* Left Nav Panel */}
        <div className="space-y-1">
          {NAV_ITEMS.map((item) => {
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
                  {item.label}
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
              Run New Analysis
            </span>
          </button>

          <button
            onClick={() => navigate('/audit')}
            className="w-full flex items-center gap-3 px-4 py-2.5 rounded-lg text-sm text-on-surface-variant hover:bg-surface-container-low transition-colors"
          >
            <Icon name="history" className="text-[20px]" />
            <span className="font-headline tracking-tight font-semibold text-[0.75rem]">
              Audit Logs
            </span>
          </button>

          <button
            onClick={() => alert('Coming Soon')}
            className="w-full flex items-center gap-3 px-4 py-2.5 rounded-lg text-sm text-on-surface-variant hover:bg-surface-container-low transition-colors"
          >
            <Icon name="download" className="text-[20px]" />
            <span className="font-headline tracking-tight font-semibold text-[0.75rem]">
              Export
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
                  BIA Tier Rules
                </h3>
              </div>
              <div className="space-y-2">
                {rules.map((rule: any) => {
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
                  System Overview
                </h3>
              </div>
              <DonutChart segments={donutSegments} />
              <div className="mt-3 text-center">
                <p className="font-headline text-3xl font-bold text-on-surface">
                  {stats.total}
                </p>
                <p className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant">
                  Total Monitored
                </p>
              </div>
              <div className="mt-2 text-center">
                <p className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant">
                  Avg Compliance
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
                  Asset Dependencies
                </h3>
              </div>
              <div className="space-y-3">
                <div className="rounded-lg bg-surface-container-low p-4 text-center">
                  <p className="font-headline text-3xl font-bold text-on-surface">
                    {stats.total_dependencies ?? 8}
                  </p>
                  <p className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant mt-1">
                    Linked CIs
                  </p>
                </div>
                <div className="space-y-2">
                  {[
                    { label: 'Data Compliant', count: stats.data_compliant, icon: 'storage' },
                    { label: 'Asset Compliant', count: stats.asset_compliant, icon: 'dns' },
                    { label: 'Audit Compliant', count: stats.audit_compliant, icon: 'verified_user' },
                  ].map((item) => (
                    <div
                      key={item.label}
                      className="flex items-center justify-between rounded-lg bg-surface-container-low px-3 py-2"
                    >
                      <div className="flex items-center gap-2">
                        <Icon name={item.icon} className="text-on-surface-variant text-lg" />
                        <span className="text-xs text-on-surface-variant">{item.label}</span>
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
                  BIA Assessment Matrix
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
                    Business System
                  </th>
                  <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                    Owner
                  </th>
                  <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                    Tier
                  </th>
                  <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                    BIA Score
                  </th>
                  <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                    RTO (hrs)
                  </th>
                  <th className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                    RPO (min)
                  </th>
                  <th className="px-5 py-3 text-center text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                    Asset
                  </th>
                  <th className="px-5 py-3 text-center text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                    Data
                  </th>
                  <th className="px-5 py-3 text-center text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                    Audit
                  </th>
                </tr>
              </thead>
              <tbody>
                {assessments.map((a: any) => {
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
                      No assessments found
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
