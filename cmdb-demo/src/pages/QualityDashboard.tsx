import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQualityDashboard, useQualityRules, useTriggerQualityScan, useCreateQualityRule } from '../hooks/useQuality'

function scoreColor(score: number) {
  if (score >= 80) return 'text-[#34d399]'
  if (score >= 60) return 'text-[#fbbf24]'
  return 'text-error'
}

function scoreBgColor(score: number) {
  if (score >= 80) return 'bg-[#34d399]'
  if (score >= 60) return 'bg-[#fbbf24]'
  return 'bg-error'
}

interface DimensionScore {
  dimension: string
  score: number
  weight: number
}

interface WorstAsset {
  asset_name: string
  asset_tag: string
  total_score: number
  issues: number
}

interface QualityRule {
  id: string
  dimension: string
  field_name: string
  rule_type: string
  weight: number
  enabled: boolean
}

interface DashboardData {
  total_score: number
  dimensions: DimensionScore[]
  worst_assets: WorstAsset[]
}

const dimensionIcons: Record<string, string> = {
  completeness: 'checklist',
  accuracy: 'target',
  timeliness: 'schedule',
  consistency: 'sync',
}

const dimensionLabels: Record<string, string> = {
  completeness: 'Completeness',
  accuracy: 'Accuracy',
  timeliness: 'Timeliness',
  consistency: 'Consistency',
}

export default function QualityDashboard() {
  const { t } = useTranslation()
  const { data: dashResp, isLoading: dashLoading } = useQualityDashboard()
  const { data: rulesResp, isLoading: rulesLoading } = useQualityRules()
  const triggerScan = useTriggerQualityScan()
  const createRule = useCreateQualityRule()

  const dashboard: DashboardData | null = dashResp?.data ?? null
  const rules: QualityRule[] = rulesResp?.data ?? []

  const [showAddRule, setShowAddRule] = useState(false)
  const [newRule, setNewRule] = useState({
    dimension: 'completeness',
    field_name: '',
    rule_type: 'not_empty',
    weight: 1,
  })

  const handleAddRule = () => {
    createRule.mutate(newRule, {
      onSuccess: () => {
        setShowAddRule(false)
        setNewRule({ dimension: 'completeness', field_name: '', rule_type: 'not_empty', weight: 1 })
      },
    })
  }

  const totalScore = dashboard?.total_score ?? 0
  const dimensions = dashboard?.dimensions ?? []
  const worstAssets = dashboard?.worst_assets ?? []

  // Build donut gradient
  const filledDeg = (totalScore / 100) * 360
  const donutGradient = `conic-gradient(${totalScore >= 80 ? '#34d399' : totalScore >= 60 ? '#fbbf24' : 'oklch(var(--er))'} 0deg ${filledDeg}deg, oklch(var(--b3)) ${filledDeg}deg 360deg)`

  if (dashLoading || rulesLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="flex flex-col items-center gap-3">
          <div className="w-8 h-8 border-2 border-primary/30 border-t-primary rounded-full animate-spin" />
          <span className="text-sm text-on-surface-variant">{t('common.loading')}</span>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="font-headline text-2xl font-bold text-on-surface tracking-tight">
            {t('quality.title')}
          </h1>
          <p className="text-sm text-on-surface-variant mt-1">{t('quality.subtitle')}</p>
        </div>
        <button
          onClick={() => triggerScan.mutate()}
          disabled={triggerScan.isPending}
          className="flex items-center gap-2 px-5 py-2.5 rounded-lg bg-on-primary-container text-white text-[0.75rem] font-semibold uppercase tracking-wider hover:opacity-90 transition-opacity disabled:opacity-50"
        >
          <span className="material-symbols-outlined text-[18px]">
            {triggerScan.isPending ? 'hourglass_empty' : 'play_arrow'}
          </span>
          {triggerScan.isPending ? t('quality.scanning') : t('quality.run_scan')}
        </button>
      </div>

      {/* Row 1: Score donut + dimension cards */}
      {!dashboard ? (
        <div className="bg-surface-container rounded-lg p-10 text-center">
          <span className="material-symbols-outlined text-5xl text-on-surface-variant mb-3 block">verified</span>
          <h2 className="font-headline text-lg font-bold text-on-surface mb-2">{t('quality.no_data_title')}</h2>
          <p className="text-sm text-on-surface-variant">{t('quality.no_data_desc')}</p>
        </div>
      ) : (
        <>
          <div className="grid grid-cols-12 gap-5">
            {/* Donut */}
            <div className="col-span-12 lg:col-span-4 bg-surface-container rounded-lg p-5 flex flex-col items-center justify-center">
              <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant mb-4">
                {t('quality.total_score')}
              </h3>
              <div
                className="relative mx-auto h-44 w-44 rounded-full"
                style={{ background: donutGradient }}
              >
                <div className="absolute inset-5 flex flex-col items-center justify-center rounded-full bg-surface-container">
                  <span className={`font-headline text-3xl font-bold ${scoreColor(totalScore)}`}>
                    {totalScore}
                  </span>
                  <span className="text-xs text-on-surface-variant">/100</span>
                </div>
              </div>
            </div>

            {/* 4 Dimension Cards */}
            <div className="col-span-12 lg:col-span-8 grid grid-cols-2 gap-4">
              {['completeness', 'accuracy', 'timeliness', 'consistency'].map((dim) => {
                const d = dimensions.find((x) => x.dimension === dim)
                const score = d?.score ?? 0
                return (
                  <div key={dim} className="bg-surface-container rounded-lg p-5">
                    <div className="flex items-center gap-2 mb-3">
                      <span className="material-symbols-outlined text-primary text-xl">
                        {dimensionIcons[dim]}
                      </span>
                      <span className="font-headline text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                        {dimensionLabels[dim]}
                      </span>
                    </div>
                    <div className="flex items-end gap-2 mb-2">
                      <span className={`font-headline text-2xl font-bold ${scoreColor(score)}`}>
                        {score}
                      </span>
                      <span className="text-xs text-on-surface-variant mb-1">/100</span>
                    </div>
                    <div className="w-full h-2 rounded-full bg-surface-container-highest overflow-hidden">
                      <div
                        className={`h-full rounded-full transition-all duration-500 ${scoreBgColor(score)}`}
                        style={{ width: `${score}%` }}
                      />
                    </div>
                  </div>
                )
              })}
            </div>
          </div>

          {/* Row 2: Worst Assets */}
          {worstAssets.length > 0 && (
            <div className="bg-surface-container rounded-lg p-5">
              <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant mb-4">
                {t('quality.worst_assets')}
              </h3>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-outline-variant/15">
                      <th className="text-left py-2 px-3 font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
                        {t('quality.col_asset_name')}
                      </th>
                      <th className="text-left py-2 px-3 font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
                        {t('quality.col_asset_tag')}
                      </th>
                      <th className="text-right py-2 px-3 font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
                        {t('quality.col_score')}
                      </th>
                      <th className="text-right py-2 px-3 font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
                        {t('quality.col_issues')}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {worstAssets.slice(0, 10).map((a, i) => (
                      <tr key={i} className="border-b border-outline-variant/10 hover:bg-surface-container-low transition-colors">
                        <td className="py-2.5 px-3 text-on-surface font-semibold">{a.asset_name}</td>
                        <td className="py-2.5 px-3 text-on-surface-variant font-mono text-xs">{a.asset_tag}</td>
                        <td className={`py-2.5 px-3 text-right font-bold ${scoreColor(a.total_score)}`}>{a.total_score}</td>
                        <td className="py-2.5 px-3 text-right text-on-surface-variant">{a.issues}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </>
      )}

      {/* Row 3: Quality Rules */}
      <div className="bg-surface-container rounded-lg p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
            {t('quality.rules_title')}
          </h3>
          <button
            onClick={() => setShowAddRule(!showAddRule)}
            className="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-surface-container-high text-xs font-semibold text-on-surface hover:bg-surface-container-highest transition-colors"
          >
            <span className="material-symbols-outlined text-[16px]">add</span>
            {t('quality.add_rule')}
          </button>
        </div>

        {/* Add Rule Form */}
        {showAddRule && (
          <div className="mb-4 p-4 bg-surface-container-low rounded-lg space-y-3">
            <div className="grid grid-cols-4 gap-3">
              <div className="flex flex-col gap-1">
                <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
                  Dimension
                </label>
                <select
                  value={newRule.dimension}
                  onChange={(e) => setNewRule({ ...newRule, dimension: e.target.value })}
                  className="bg-surface-container-high text-on-surface text-sm rounded px-3 py-2 border border-outline-variant/20"
                >
                  <option value="completeness">Completeness</option>
                  <option value="accuracy">Accuracy</option>
                  <option value="timeliness">Timeliness</option>
                  <option value="consistency">Consistency</option>
                </select>
              </div>
              <div className="flex flex-col gap-1">
                <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
                  Field Name
                </label>
                <input
                  value={newRule.field_name}
                  onChange={(e) => setNewRule({ ...newRule, field_name: e.target.value })}
                  placeholder="e.g. serial_number"
                  className="bg-surface-container-high text-on-surface text-sm rounded px-3 py-2 border border-outline-variant/20"
                />
              </div>
              <div className="flex flex-col gap-1">
                <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
                  Rule Type
                </label>
                <select
                  value={newRule.rule_type}
                  onChange={(e) => setNewRule({ ...newRule, rule_type: e.target.value })}
                  className="bg-surface-container-high text-on-surface text-sm rounded px-3 py-2 border border-outline-variant/20"
                >
                  <option value="not_empty">Not Empty</option>
                  <option value="regex">Regex</option>
                  <option value="enum_check">Enum Check</option>
                  <option value="freshness">Freshness</option>
                  <option value="cross_field">Cross Field</option>
                </select>
              </div>
              <div className="flex flex-col gap-1">
                <label className="font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
                  Weight
                </label>
                <input
                  type="number"
                  min={1}
                  max={10}
                  value={newRule.weight}
                  onChange={(e) => setNewRule({ ...newRule, weight: Number(e.target.value) })}
                  className="bg-surface-container-high text-on-surface text-sm rounded px-3 py-2 border border-outline-variant/20"
                />
              </div>
            </div>
            <div className="flex gap-2">
              <button
                onClick={handleAddRule}
                disabled={!newRule.field_name || createRule.isPending}
                className="px-4 py-2 rounded-lg bg-on-primary-container text-white text-xs font-semibold uppercase tracking-wider hover:opacity-90 disabled:opacity-50"
              >
                {t('common.save')}
              </button>
              <button
                onClick={() => setShowAddRule(false)}
                className="px-4 py-2 rounded-lg bg-surface-container-high text-on-surface text-xs font-semibold uppercase tracking-wider hover:bg-surface-container-highest"
              >
                {t('common.cancel')}
              </button>
            </div>
          </div>
        )}

        {rules.length === 0 ? (
          <p className="text-sm text-on-surface-variant py-4 text-center">{t('quality.no_rules')}</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-outline-variant/15">
                  <th className="text-left py-2 px-3 font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
                    Dimension
                  </th>
                  <th className="text-left py-2 px-3 font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
                    Field
                  </th>
                  <th className="text-left py-2 px-3 font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
                    Rule Type
                  </th>
                  <th className="text-right py-2 px-3 font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
                    Weight
                  </th>
                  <th className="text-center py-2 px-3 font-label text-[0.625rem] uppercase tracking-[0.06em] text-on-surface-variant">
                    Status
                  </th>
                </tr>
              </thead>
              <tbody>
                {rules.map((rule) => (
                  <tr key={rule.id} className="border-b border-outline-variant/10 hover:bg-surface-container-low transition-colors">
                    <td className="py-2.5 px-3">
                      <span className="flex items-center gap-2">
                        <span className="material-symbols-outlined text-primary text-[16px]">
                          {dimensionIcons[rule.dimension] ?? 'rule'}
                        </span>
                        <span className="text-on-surface capitalize">{rule.dimension}</span>
                      </span>
                    </td>
                    <td className="py-2.5 px-3 font-mono text-xs text-on-surface-variant">{rule.field_name}</td>
                    <td className="py-2.5 px-3 text-on-surface-variant">{rule.rule_type}</td>
                    <td className="py-2.5 px-3 text-right text-on-surface font-semibold">{rule.weight}</td>
                    <td className="py-2.5 px-3 text-center">
                      <span className={`inline-block px-2.5 py-0.5 rounded text-[0.625rem] font-semibold uppercase tracking-wider ${
                        rule.enabled
                          ? 'bg-[#065f46] text-[#34d399]'
                          : 'bg-surface-container-highest text-on-surface-variant'
                      }`}>
                        {rule.enabled ? 'Active' : 'Disabled'}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
