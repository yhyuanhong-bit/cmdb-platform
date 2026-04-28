import { useState, useEffect, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'
import { usePredictionsByAsset, useVerifyRCA, useFailureDistribution } from '../../hooks/usePrediction'
import { useAssets } from '../../hooks/useAssets'
import { useAuthStore } from '../../stores/authStore'
import { Icon, RulBar, type AdvisorMessage } from './shared'

const CATEGORY_COLOR: Record<string, string> = {
  Mechanical: 'bg-error',
  Electrical: 'bg-tertiary',
  Thermal: 'bg-primary',
  Software: 'bg-secondary',
}

export function PredictionOverviewTab() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [currentPage, setCurrentPage] = useState(1)
  const [selectedAssetId, setSelectedAssetId] = useState('')
  const { data: assetsData } = useAssets({ page_size: '1' })
  const firstAssetId = assetsData?.data?.[0]?.id || ''
  useEffect(() => {
    if (firstAssetId && !selectedAssetId) setSelectedAssetId(firstAssetId)
  }, [firstAssetId])
  const verifyRCA = useVerifyRCA()
  // Audit C-H7 (2026-04-28): the previous `'current-user'` literal made
  // the audit trail anonymous; verifier identity now flows from the
  // authenticated user. Falls back to '' if somehow not signed in so
  // the mutation surfaces a clear backend error rather than silently
  // attributing to a hardcoded string.
  const currentUser = useAuthStore((s) => s.user)
  const verifierIdentity = currentUser?.username || currentUser?.id || ''

  const { data: predictionsResponse } = usePredictionsByAsset(selectedAssetId)
  const { data: failDistData } = useFailureDistribution()
  const predictions = predictionsResponse?.data ?? []

  const rawDist = failDistData?.distribution ?? []
  const totalCount = rawDist.reduce((s, d) => s + d.count, 0)
  const failureDist = rawDist.map((d) => ({
    label: d.category,
    pct: totalCount > 0 ? Math.round((d.count / totalCount) * 100) : 0,
    color: CATEGORY_COLOR[d.category] ?? 'bg-on-surface-variant',
  }))

  const SEVERITY_MAP: Record<string, { color: string; bg: string }> = {
    critical: { color: 'text-error', bg: 'bg-error-container' },
    high: { color: 'text-tertiary', bg: 'bg-tertiary-container' },
    medium: { color: 'text-primary', bg: 'bg-primary-container' },
    low: { color: 'text-secondary', bg: 'bg-secondary-container' },
  }

  const advisorMessages = useMemo<AdvisorMessage[]>(() => {
    if (predictions.length === 0) {
      return [{ text: t('predictive_hub.ai_advisor_empty'), time: '' }]
    }
    const rankWeight: Record<string, number> = { critical: 3, high: 2, medium: 1, low: 0 }
    const ranked = [...predictions].sort((a, b) => {
      const wa = rankWeight[(a.severity ?? 'medium').toLowerCase()] ?? 1
      const wb = rankWeight[(b.severity ?? 'medium').toLowerCase()] ?? 1
      return wb - wa
    })
    return ranked.slice(0, 4).map((p) => {
      const horizon = p.expires_at
        ? t('predictive_hub.ai_advisor_horizon_days', {
            days: Math.max(0, Math.round((new Date(p.expires_at).getTime() - Date.now()) / (1000 * 60 * 60 * 24))),
          })
        : t('predictive_hub.ai_advisor_horizon_unknown')
      const text = t('predictive_hub.ai_advisor_message', {
        asset: p.ci_id,
        type: p.prediction_type,
        severity: (p.severity ?? 'medium').toUpperCase(),
        horizon,
      })
      const time = p.created_at
        ? new Date(p.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
        : ''
      return { text, time }
    })
  }, [predictions, t])

  // Map API predictions to ASSETS shape, fall back to empty array
  const ASSETS = predictions.map((p) => {
    const sevKey = (p.severity ?? 'medium').toLowerCase()
    const sevStyle = SEVERITY_MAP[sevKey] ?? SEVERITY_MAP.medium
    const daysUntilExpiry = p.expires_at ? Math.max(0, Math.round((new Date(p.expires_at).getTime() - Date.now()) / (1000 * 60 * 60 * 24))) : 45
    return {
      name: p.ci_id,
      type: p.prediction_type,
      failureDate: p.expires_at ? new Date(p.expires_at).toISOString().split('T')[0] : '—',
      rulDays: daysUntilExpiry,
      rulMax: 90,
      severity: (p.severity ?? 'MEDIUM').toUpperCase(),
      severityColor: sevStyle.color,
      severityBg: sevStyle.bg,
    }
  })

  return (
    <div className="space-y-6">
      {/* Failure Distribution mini chart */}
      <div className="bg-surface-container rounded-xl p-5">
        <div className="flex items-center gap-2 mb-3">
          <div className="bg-surface-container-high rounded-lg p-2">
            <Icon name="pie_chart" className="text-primary text-xl" />
          </div>
          <span className="text-xs text-on-surface-variant font-label uppercase tracking-widest">
            {t('predictive.failure_distribution')}
          </span>
        </div>
        <div className="flex gap-1 h-3 rounded-full overflow-hidden mt-1">
          {failureDist.length > 0 ? failureDist.map((d) => (
            <div key={d.label} className={`${d.color} transition-all`} style={{ width: `${d.pct}%` }} />
          )) : <div className="flex-1 bg-surface-container-low rounded-full" />}
        </div>
        <div className="flex flex-wrap items-center gap-x-6 gap-y-1 mt-3">
          {failureDist.map((d) => (
            <div key={d.label} className="flex items-center gap-1.5">
              <div className={`w-2 h-2 rounded-full ${d.color}`} />
              <span className="text-[10px] text-on-surface-variant font-label">{d.label} {d.pct}%</span>
            </div>
          ))}
          <button
            onClick={() => navigate('/monitoring')}
            className="ml-auto flex items-center gap-1 text-xs text-primary font-label hover:underline"
          >
            {t('predictive_hub.view_monitoring')}
            <Icon name="arrow_forward" className="text-sm" />
          </button>
        </div>
      </div>

      {/* Assets table */}
      <div className="bg-surface-container rounded-xl">
        <div className="px-6 py-4 flex items-center justify-between">
          <div>
            <h2 className="font-headline text-lg font-bold text-on-surface">
              {t('predictive.assets_requiring_attention_zh')}
            </h2>
            <p className="text-xs text-on-surface-variant font-label tracking-widest uppercase mt-0.5">
              {t('predictive.assets_requiring_attention')}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <button className="bg-surface-container-high hover:bg-surface-container-highest px-3 py-1.5 rounded-lg text-xs font-label text-on-surface-variant flex items-center gap-1.5 transition-colors">
              <Icon name="filter_list" className="text-base" />
              {t('common.filter')}
            </button>
            <button className="bg-surface-container-high hover:bg-surface-container-highest px-3 py-1.5 rounded-lg text-xs font-label text-on-surface-variant flex items-center gap-1.5 transition-colors">
              <Icon name="download" className="text-base" />
              {t('common.export')}
            </button>
          </div>
        </div>

        <div className="grid grid-cols-[1.5fr_1fr_2fr_1fr_1fr] gap-4 px-6 py-3 bg-surface-container-low text-[10px] text-on-surface-variant font-label uppercase tracking-widest">
          <span>{t('predictive.table_asset_name')}</span>
          <span>{t('predictive.table_failure_date')}</span>
          <span>{t('predictive.table_rul_indicator')}</span>
          <span>{t('predictive.table_severity')}</span>
          <span className="text-right">{t('predictive.table_action')}</span>
        </div>

        {ASSETS.map((a) => (
          <div key={a.name} className="grid grid-cols-[1.5fr_1fr_2fr_1fr_1fr] gap-4 px-6 py-4 items-center hover:bg-surface-container-high transition-colors">
            <div>
              <p className="font-headline text-sm font-bold text-on-surface">{a.name}</p>
              <p className="text-[10px] text-on-surface-variant font-label mt-0.5">{a.type}</p>
            </div>
            <span className="text-sm text-on-surface tabular-nums">{a.failureDate}</span>
            <RulBar days={a.rulDays} max={a.rulMax} />
            <span className={`inline-flex items-center justify-center text-[10px] font-label font-bold tracking-widest px-3 py-1 rounded-lg ${a.severityBg} ${a.severityColor} w-fit`}>
              {a.severity}
            </span>
            <div className="text-right flex items-center gap-2 justify-end">
              <button onClick={() => verifyRCA.mutate({ id: a.name, data: { verified_by: verifierIdentity } })}
                className="text-xs px-2 py-1 rounded bg-green-500/20 text-green-400 hover:bg-green-500/30">
                {verifyRCA.isPending ? '...' : t('predictive_hub.btn_verify')}
              </button>
              <button className="text-xs text-primary font-label hover:underline flex items-center gap-1">
                {t('predictive.view_details_zh')}
                <Icon name="arrow_forward" className="text-sm" />
              </button>
            </div>
          </div>
        ))}

        <div className="px-6 py-4 flex items-center justify-between">
          <span className="text-xs text-on-surface-variant font-label">
            {t('predictive.showing_assets', { shown: 4, total: 42 })}
          </span>
          <div className="flex items-center gap-1">
            <button
              className="bg-surface-container-high hover:bg-surface-container-highest w-8 h-8 rounded-lg flex items-center justify-center transition-colors disabled:opacity-30"
              disabled={currentPage === 1}
              onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
            >
              <Icon name="chevron_left" className="text-base text-on-surface-variant" />
            </button>
            {[1, 2, 3].map((p) => (
              <button
                key={p}
                onClick={() => setCurrentPage(p)}
                className={`w-8 h-8 rounded-lg flex items-center justify-center text-xs font-label transition-colors ${
                  p === currentPage
                    ? 'bg-primary text-on-primary font-bold'
                    : 'bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest'
                }`}
              >
                {p}
              </button>
            ))}
            <span className="text-on-surface-variant text-xs px-1">...</span>
            <button
              className="bg-surface-container-high hover:bg-surface-container-highest w-8 h-8 rounded-lg flex items-center justify-center transition-colors"
              onClick={() => setCurrentPage((p) => p + 1)}
            >
              <Icon name="chevron_right" className="text-base text-on-surface-variant" />
            </button>
          </div>
        </div>
      </div>

      {/* AI Advisor panel */}
      <div className="bg-surface-container rounded-xl p-5">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <Icon name="smart_toy" className="text-primary text-xl" />
            <h3 className="font-headline text-base font-bold text-on-surface">
              {t('predictive.ai_maintenance_advisor')}
            </h3>
          </div>
          <span className="text-[10px] text-on-surface-variant font-label tracking-widest uppercase">
            {t('predictive.ai_version')}
          </span>
        </div>
        <div className="bg-surface-container-low rounded-xl p-4 flex flex-col gap-3 max-h-[320px] overflow-y-auto">
          {advisorMessages.map((msg, i) => (
            <div key={i} className="flex justify-start">
              <div className="max-w-[85%] rounded-xl px-4 py-3 bg-surface-container text-on-surface">
                <div className="flex items-center gap-1.5 mb-1.5">
                  <Icon name="smart_toy" className="text-primary text-sm" />
                  <span className="text-[10px] text-primary font-label font-bold tracking-widest uppercase">
                    {t('predictive.ai_advisor')}
                  </span>
                </div>
                <p className="text-sm leading-relaxed">{msg.text}</p>
                {msg.time && (
                  <p className="text-[10px] text-on-surface-variant mt-2 text-right tabular-nums">{msg.time}</p>
                )}
              </div>
            </div>
          ))}
        </div>
        <div className="mt-3 flex items-center gap-2">
          <div className="flex-1 bg-surface-container-low rounded-xl px-4 py-2.5 flex items-center gap-2">
            <Icon name="chat" className="text-on-surface-variant text-lg" />
            <span className="text-sm text-on-surface-variant">{t('predictive.ai_input_placeholder')}</span>
          </div>
          <button className="bg-primary rounded-xl p-2.5 hover:opacity-90 transition-opacity">
            <Icon name="send" className="text-on-primary text-lg" />
          </button>
        </div>
      </div>
    </div>
  )
}
