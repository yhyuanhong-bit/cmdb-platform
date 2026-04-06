import { memo, useMemo } from 'react'
import { Link } from 'react-router-dom'
import { useBIAStats, useBIAAssessments } from '../../hooks/useBIA'

const TIER_COLORS: Record<string, { bg: string; text: string; badge: string }> = {
  critical:  { bg: 'bg-[#dc2626]', text: 'text-[#fca5a5]', badge: 'bg-[#7f1d1d] text-[#fca5a5]' },
  important: { bg: 'bg-[#f59e0b]', text: 'text-[#fde68a]', badge: 'bg-[#78350f] text-[#fde68a]' },
  normal:    { bg: 'bg-[#3b82f6]', text: 'text-[#93c5fd]', badge: 'bg-[#1e3a5f] text-[#93c5fd]' },
  low:       { bg: 'bg-[#6b7280]', text: 'text-[#d1d5db]', badge: 'bg-[#374151] text-[#d1d5db]' },
}

function getTierStyle(tier: string) {
  return TIER_COLORS[tier.toLowerCase()] || TIER_COLORS.low
}

function Icon({ name, className = '' }: { name: string; className?: string }) {
  return <span className={`material-symbols-outlined ${className}`}>{name}</span>
}

function SystemGrading() {
  const { data: statsResp, isLoading: statsLoading } = useBIAStats()
  const { data: assessResp, isLoading: assessLoading } = useBIAAssessments()

  const stats = statsResp?.data
  const assessments = useMemo(() => {
    const list = assessResp?.data || []
    return [...list].sort((a, b) => b.bia_score - a.bia_score)
  }, [assessResp])

  const total = stats?.total || 0
  const byTier = stats?.by_tier || {}

  const statCards = [
    { label: 'Total Systems', value: total, icon: 'dns', color: 'text-primary' },
    { label: 'Critical', value: byTier['critical'] || 0, icon: 'error', color: 'text-[#dc2626]' },
    { label: 'Important', value: byTier['important'] || 0, icon: 'warning', color: 'text-[#f59e0b]' },
    { label: 'Normal', value: byTier['normal'] || 0, icon: 'info', color: 'text-[#3b82f6]' },
  ]

  const isLoading = statsLoading || assessLoading

  return (
    <div className="space-y-6">
      {/* Breadcrumb + Header */}
      <div>
        <div className="flex items-center gap-1.5 text-[0.6875rem] uppercase tracking-wider text-on-surface-variant mb-2">
          <Link to="/bia" className="hover:text-on-surface transition-colors">BIA</Link>
          <Icon name="chevron_right" className="text-base" />
          <span className="text-on-surface">System Grading</span>
        </div>
        <h1 className="font-headline font-bold text-2xl text-on-surface">System Grading</h1>
      </div>

      {/* Stats Row */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {statCards.map((card) => (
          <div key={card.label} className="rounded-lg bg-surface-container p-5">
            <div className="flex items-center justify-between mb-3">
              <span className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant">{card.label}</span>
              <Icon name={card.icon} className={`text-xl ${card.color}`} />
            </div>
            <div className="text-3xl font-bold text-on-surface">
              {isLoading ? <span className="inline-block w-10 h-8 rounded bg-surface-container-high animate-pulse" /> : card.value}
            </div>
          </div>
        ))}
      </div>

      {/* Score Distribution */}
      <div className="rounded-lg bg-surface-container p-5">
        <h2 className="font-headline font-bold text-lg text-on-surface mb-4">Score Distribution by Tier</h2>
        {isLoading ? (
          <div className="space-y-3">
            {[1, 2, 3].map((i) => (
              <div key={i} className="h-6 rounded bg-surface-container-high animate-pulse" />
            ))}
          </div>
        ) : (
          <div className="space-y-3">
            {Object.entries(byTier).map(([tier, count]) => (
              <div key={tier} className="flex items-center gap-3">
                <span className="w-20 text-xs text-on-surface-variant uppercase">{tier}</span>
                <div className="flex-1 h-6 rounded bg-surface-container-low">
                  <div
                    className={`h-6 rounded ${getTierStyle(tier).bg}`}
                    style={{ width: `${total > 0 ? (count / total) * 100 : 0}%` }}
                  />
                </div>
                <span className="w-8 text-sm font-bold text-on-surface">{count}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Full Assessment Table */}
      <div className="rounded-lg bg-surface-container p-5">
        <h2 className="font-headline font-bold text-lg text-on-surface mb-4">All Assessments</h2>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-outline-variant">
                <th className="text-left py-3 px-3 text-[0.6875rem] uppercase tracking-wider text-on-surface-variant font-medium">System</th>
                <th className="text-left py-3 px-3 text-[0.6875rem] uppercase tracking-wider text-on-surface-variant font-medium">Code</th>
                <th className="text-left py-3 px-3 text-[0.6875rem] uppercase tracking-wider text-on-surface-variant font-medium">Tier</th>
                <th className="text-right py-3 px-3 text-[0.6875rem] uppercase tracking-wider text-on-surface-variant font-medium">BIA Score</th>
                <th className="text-right py-3 px-3 text-[0.6875rem] uppercase tracking-wider text-on-surface-variant font-medium">RTO (hrs)</th>
                <th className="text-right py-3 px-3 text-[0.6875rem] uppercase tracking-wider text-on-surface-variant font-medium">RPO (min)</th>
                <th className="text-left py-3 px-3 text-[0.6875rem] uppercase tracking-wider text-on-surface-variant font-medium">Owner</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                Array.from({ length: 5 }).map((_, i) => (
                  <tr key={i} className="border-b border-outline-variant/30">
                    {Array.from({ length: 7 }).map((_, j) => (
                      <td key={j} className="py-3 px-3">
                        <div className="h-4 rounded bg-surface-container-high animate-pulse" />
                      </td>
                    ))}
                  </tr>
                ))
              ) : assessments.length === 0 ? (
                <tr>
                  <td colSpan={7} className="py-12 text-center text-on-surface-variant">
                    No assessments found
                  </td>
                </tr>
              ) : (
                assessments.map((a) => {
                  const style = getTierStyle(a.tier)
                  return (
                    <tr key={a.id} className="border-b border-outline-variant/30 hover:bg-surface-container-high/40 transition-colors">
                      <td className="py-3 px-3 text-on-surface font-medium">{a.system_name}</td>
                      <td className="py-3 px-3 text-on-surface-variant font-mono text-xs">{a.system_code}</td>
                      <td className="py-3 px-3">
                        <span className={`inline-block rounded px-2 py-0.5 text-xs font-medium uppercase ${style.badge}`}>
                          {a.tier}
                        </span>
                      </td>
                      <td className="py-3 px-3 text-right font-bold text-on-surface">{a.bia_score}</td>
                      <td className="py-3 px-3 text-right text-on-surface-variant">{a.rto_hours ?? '—'}</td>
                      <td className="py-3 px-3 text-right text-on-surface-variant">{a.rpo_minutes ?? '—'}</td>
                      <td className="py-3 px-3 text-on-surface-variant">{a.owner ?? '—'}</td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}

export default memo(SystemGrading)
