import { memo, useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useBIAScoringRules, useUpdateBIAScoringRule } from '../../hooks/useBIA'

/* ──────────────────────────────────────────────
   Local types
   ────────────────────────────────────────────── */

interface BIARule {
  id: string
  tier_name: string
  tier_level: number
  display_name: string
  description: string
  icon?: string
  min_score: number
  max_score: number
  rto_threshold?: number
  rpo_threshold?: number
  enabled?: boolean
  threshold?: number
  action?: string
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

function ScoringRules() {
  const { t } = useTranslation()
  const { data: rulesResp, isLoading } = useBIAScoringRules()
  const updateRule = useUpdateBIAScoringRule()
  const rules: BIARule[] = (rulesResp as ApiListResponse<BIARule>)?.data || []

  const [selectedRuleId, setSelectedRuleId] = useState<string>('')
  const [editData, setEditData] = useState<Partial<BIARule>>({})
  const [dirty, setDirty] = useState(false)

  // Auto-select first rule
  useEffect(() => {
    if (!selectedRuleId && rules.length > 0) {
      setSelectedRuleId(rules[0].id)
    }
  }, [rules, selectedRuleId])

  // Sync edit data when selection changes
  useEffect(() => {
    const rule = rules.find((r) => r.id === selectedRuleId)
    if (rule) {
      setEditData({ ...rule })
      setDirty(false)
    }
  }, [selectedRuleId, rules])

  const selectedRule = rules.find((r) => r.id === selectedRuleId)

  function handleFieldChange(field: string, value: string | number) {
    setEditData((prev) => ({ ...prev, [field]: value }))
    setDirty(true)
  }

  function handleSave() {
    if (!selectedRuleId) return
    updateRule.mutate(
      { id: selectedRuleId, data: editData },
      { onSuccess: () => setDirty(false) }
    )
  }

  function handleCancel() {
    if (selectedRule) {
      setEditData({ ...selectedRule })
      setDirty(false)
    }
  }

  return (
    <div className="space-y-6">
      {/* Breadcrumb + Header */}
      <div>
        <div className="flex items-center gap-1.5 text-[0.6875rem] uppercase tracking-wider text-on-surface-variant mb-2">
          <Link to="/bia" className="hover:text-on-surface transition-colors">{t('bia_rules.breadcrumb_bia')}</Link>
          <Icon name="chevron_right" className="text-base" />
          <span className="text-on-surface">{t('bia_rules.page_title')}</span>
        </div>
        <h1 className="font-headline font-bold text-2xl text-on-surface">{t('bia_rules.page_title')}</h1>
      </div>

      {isLoading ? (
        <div className="rounded-lg bg-surface-container p-5 h-96 animate-pulse" />
      ) : (
        <div className="grid grid-cols-1 gap-5 lg:grid-cols-[320px_1fr]">
          {/* Left Panel: Rule List */}
          <div className="rounded-lg bg-surface-container p-4">
            <h3 className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant font-medium mb-3">
              {t('bia_rules.panel_tier_rules')}
            </h3>
            <div className="space-y-1">
              {rules.map((rule) => (
                <button
                  key={rule.id}
                  type="button"
                  onClick={() => setSelectedRuleId(rule.id)}
                  className={`w-full flex items-center gap-3 rounded-lg px-3 py-3 text-left transition-colors ${
                    selectedRuleId === rule.id
                      ? 'bg-primary/15 text-on-surface'
                      : 'hover:bg-surface-container-high text-on-surface-variant'
                  }`}
                >
                  <Icon name={rule.icon || 'tune'} className="text-xl" />
                  <div className="flex-1 min-w-0">
                    <div className="font-medium text-sm truncate">{rule.display_name}</div>
                    <div className="flex items-center gap-2 mt-0.5">
                      <span className={`inline-block rounded px-1.5 py-0.5 text-[0.625rem] font-medium uppercase ${getBadge(rule.tier_name)}`}>
                        {rule.tier_name}
                      </span>
                      <span className="text-[0.625rem] text-on-surface-variant">
                        {rule.min_score}–{rule.max_score}
                      </span>
                    </div>
                  </div>
                  {selectedRuleId === rule.id && (
                    <Icon name="chevron_right" className="text-base text-on-surface-variant" />
                  )}
                </button>
              ))}
            </div>
          </div>

          {/* Right Panel: Rule Detail / Edit */}
          <div className="rounded-lg bg-surface-container p-5">
            {!selectedRule ? (
              <div className="flex items-center justify-center h-64 text-on-surface-variant">
                {t('bia_rules.select_rule_hint')}
              </div>
            ) : (
              <div className="space-y-5">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <Icon name={selectedRule.icon || 'tune'} className="text-2xl text-on-surface" />
                    <div>
                      <h3 className="font-headline font-bold text-lg text-on-surface">{selectedRule.display_name}</h3>
                      <span className={`inline-block rounded px-2 py-0.5 text-xs font-medium uppercase mt-1 ${getBadge(selectedRule.tier_name)}`}>
                        {selectedRule.tier_name}
                      </span>
                    </div>
                  </div>
                  <span className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant">
                    {t('bia_rules.label_level')} {selectedRule.tier_level}
                  </span>
                </div>

                {/* Editable fields */}
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <div>
                    <label className="block text-[0.6875rem] uppercase tracking-wider text-on-surface-variant mb-1.5">{t('bia_rules.label_display_name')}</label>
                    <input
                      type="text"
                      value={editData.display_name ?? ''}
                      onChange={(e) => handleFieldChange('display_name', e.target.value)}
                      className="w-full rounded-lg bg-surface-container-low border border-outline-variant px-3 py-2 text-sm text-on-surface focus:outline-none focus:border-primary"
                    />
                  </div>
                  <div>
                    <label className="block text-[0.6875rem] uppercase tracking-wider text-on-surface-variant mb-1.5">{t('bia_rules.label_description')}</label>
                    <input
                      type="text"
                      value={editData.description ?? ''}
                      onChange={(e) => handleFieldChange('description', e.target.value)}
                      className="w-full rounded-lg bg-surface-container-low border border-outline-variant px-3 py-2 text-sm text-on-surface focus:outline-none focus:border-primary"
                    />
                  </div>
                  <div>
                    <label className="block text-[0.6875rem] uppercase tracking-wider text-on-surface-variant mb-1.5">{t('bia_rules.label_min_score')}</label>
                    <input
                      type="number"
                      value={editData.min_score ?? ''}
                      onChange={(e) => handleFieldChange('min_score', parseInt(e.target.value) || 0)}
                      className="w-full rounded-lg bg-surface-container-low border border-outline-variant px-3 py-2 text-sm text-on-surface focus:outline-none focus:border-primary"
                    />
                  </div>
                  <div>
                    <label className="block text-[0.6875rem] uppercase tracking-wider text-on-surface-variant mb-1.5">{t('bia_rules.label_max_score')}</label>
                    <input
                      type="number"
                      value={editData.max_score ?? ''}
                      onChange={(e) => handleFieldChange('max_score', parseInt(e.target.value) || 0)}
                      className="w-full rounded-lg bg-surface-container-low border border-outline-variant px-3 py-2 text-sm text-on-surface focus:outline-none focus:border-primary"
                    />
                  </div>
                  <div>
                    <label className="block text-[0.6875rem] uppercase tracking-wider text-on-surface-variant mb-1.5">{t('bia_rules.label_rto_threshold')}</label>
                    <input
                      type="number"
                      step="0.5"
                      value={editData.rto_threshold ?? ''}
                      onChange={(e) => handleFieldChange('rto_threshold', parseFloat(e.target.value) || 0)}
                      className="w-full rounded-lg bg-surface-container-low border border-outline-variant px-3 py-2 text-sm text-on-surface focus:outline-none focus:border-primary"
                    />
                  </div>
                  <div>
                    <label className="block text-[0.6875rem] uppercase tracking-wider text-on-surface-variant mb-1.5">{t('bia_rules.label_rpo_threshold')}</label>
                    <input
                      type="number"
                      step="1"
                      value={editData.rpo_threshold ?? ''}
                      onChange={(e) => handleFieldChange('rpo_threshold', parseFloat(e.target.value) || 0)}
                      className="w-full rounded-lg bg-surface-container-low border border-outline-variant px-3 py-2 text-sm text-on-surface focus:outline-none focus:border-primary"
                    />
                  </div>
                </div>

                {/* Save / Cancel */}
                <div className="flex items-center gap-3 pt-2">
                  <button
                    type="button"
                    onClick={handleSave}
                    disabled={!dirty || updateRule.isPending}
                    className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-on-primary transition-colors hover:bg-primary/90 disabled:opacity-40 disabled:cursor-not-allowed"
                  >
                    <Icon name="save" className="text-lg" />
                    {updateRule.isPending ? t('bia_rules.btn_saving') : t('bia_rules.btn_save')}
                  </button>
                  <button
                    type="button"
                    onClick={handleCancel}
                    disabled={!dirty}
                    className="inline-flex items-center gap-2 rounded-lg bg-surface-container-high px-4 py-2 text-sm font-medium text-on-surface transition-colors hover:bg-surface-container-highest disabled:opacity-40 disabled:cursor-not-allowed"
                  >
                    {t('bia_rules.btn_cancel')}
                  </button>
                  {updateRule.isSuccess && !dirty && (
                    <span className="flex items-center gap-1 text-xs text-[#34d399]">
                      <Icon name="check_circle" className="text-sm" /> {t('bia_rules.saved_indicator')}
                    </span>
                  )}
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

export default memo(ScoringRules)
