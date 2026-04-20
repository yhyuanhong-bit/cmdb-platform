import { toast } from 'sonner'
import { useTranslation } from "react-i18next";
import Icon from "../../components/Icon";
import { useUpdateAlertRule, useCreateAlertRule, useDeleteAlertRule } from "../../hooks/useMonitoring";
import { Section, Toggle, type AlertRule, type NewRuleDraft, type EditRuleDraft } from "./shared";

interface AlertRulesSectionProps {
  rules: AlertRule[];
  setRules: React.Dispatch<React.SetStateAction<AlertRule[]>>;
  toggleRule: (id: string) => void;
  editingRuleId: string | null;
  setEditingRuleId: (value: string | null) => void;
  editDraft: EditRuleDraft | null;
  setEditDraft: React.Dispatch<React.SetStateAction<EditRuleDraft | null>>;
  showAddRule: boolean;
  setShowAddRule: (value: boolean) => void;
  newRule: NewRuleDraft;
  setNewRule: React.Dispatch<React.SetStateAction<NewRuleDraft>>;
  updateAlertRuleMutate: ReturnType<typeof useUpdateAlertRule>['mutate'];
  createAlertRuleMutate: ReturnType<typeof useCreateAlertRule>['mutate'];
  deleteAlertRuleMutate: ReturnType<typeof useDeleteAlertRule>['mutate'];
}

export function AlertRulesSection({
  rules,
  setRules,
  toggleRule,
  editingRuleId,
  setEditingRuleId,
  editDraft,
  setEditDraft,
  showAddRule,
  setShowAddRule,
  newRule,
  setNewRule,
  updateAlertRuleMutate,
  createAlertRuleMutate,
  deleteAlertRuleMutate,
}: AlertRulesSectionProps) {
  const { t } = useTranslation();

  return (
    <Section title={t('sensors.alert_rule_configuration')} icon="rule">
      <div className="space-y-3">
        {rules.map((rule) => (
          <div key={rule.id}>
            <div
              className={`flex items-center gap-4 rounded-lg bg-surface-container-low p-4 transition-opacity ${!rule.enabled ? "opacity-50" : ""}`}
            >
              <Toggle
                enabled={rule.enabled}
                onToggle={() => toggleRule(rule.id)}
              />
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <p className="text-sm font-semibold text-on-surface">
                    {rule.name}
                  </p>
                  <span className="font-mono text-[10px] text-on-surface-variant">
                    {rule.id}
                  </span>
                </div>
                <div className="mt-1 flex flex-wrap items-center gap-x-4 gap-y-1">
                  <span className="flex items-center gap-1 text-xs text-on-surface-variant">
                    <Icon name="filter_alt" className="text-sm text-primary" />
                    {rule.condition}
                  </span>
                  <span className="flex items-center gap-1 text-xs text-on-surface-variant">
                    <Icon
                      name="notifications_active"
                      className="text-sm text-[#fbbf24]"
                    />
                    {rule.action}
                  </span>
                </div>
              </div>
              <button
                type="button"
                onClick={() => {
                  if (editingRuleId === rule.id) {
                    setEditingRuleId(null);
                    setEditDraft(null);
                  } else {
                    setEditingRuleId(rule.id);
                    setEditDraft({ name: rule.name, condition: rule.condition, action: rule.action });
                  }
                }}
                className="shrink-0 rounded p-1.5 text-on-surface-variant transition-colors hover:bg-surface-container-high hover:text-primary"
                title={t('sensors.edit_rule')}
              >
                <Icon name="edit" className="text-lg" />
              </button>
              <button
                type="button"
                onClick={() => {
                  if (confirm(t('sensors.delete_rule_confirm', { name: rule.name }))) {
                    setRules(prev => prev.filter(r => r.id !== rule.id));
                    deleteAlertRuleMutate(rule.id);
                  }
                }}
                className="shrink-0 rounded p-1.5 text-on-surface-variant transition-colors hover:bg-surface-container-high hover:text-error"
                title={t('sensors.delete_rule')}
              >
                <Icon name="delete" className="text-lg" />
              </button>
            </div>
            {editingRuleId === rule.id && editDraft && (
              <div className="mt-1 rounded-lg bg-surface-container-low border border-primary/20 p-4 space-y-3">
                <div className="grid grid-cols-2 gap-3">
                  <input
                    value={editDraft.name}
                    onChange={e => setEditDraft(p => p ? { ...p, name: e.target.value } : p)}
                    placeholder={t('sensors.rule_name_placeholder')}
                    className="p-2 bg-surface-container rounded-lg text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40"
                  />
                  <input
                    value={editDraft.condition}
                    onChange={e => setEditDraft(p => p ? { ...p, condition: e.target.value } : p)}
                    placeholder={t('sensors.condition_placeholder')}
                    className="p-2 bg-surface-container rounded-lg text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40"
                  />
                  <input
                    value={editDraft.action}
                    onChange={e => setEditDraft(p => p ? { ...p, action: e.target.value } : p)}
                    placeholder={t('sensors.action_placeholder')}
                    className="col-span-2 p-2 bg-surface-container rounded-lg text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40"
                  />
                </div>
                <div className="flex gap-2 justify-end">
                  <button
                    onClick={() => { setEditingRuleId(null); setEditDraft(null); }}
                    className="px-3 py-1.5 rounded-lg bg-surface-container-high text-xs text-on-surface-variant"
                  >
                    {t('common.cancel')}
                  </button>
                  <button
                    onClick={() => {
                      setRules(prev => prev.map(r => r.id === rule.id ? { ...r, name: editDraft.name, condition: editDraft.condition, action: editDraft.action } : r));
                      updateAlertRuleMutate({ id: rule.id, data: { name: editDraft.name } });
                      setEditingRuleId(null);
                      setEditDraft(null);
                      toast.success(t('sensors.rule_updated'));
                    }}
                    disabled={!editDraft.name}
                    className="px-3 py-1.5 rounded-lg bg-primary text-on-primary-container text-xs font-semibold disabled:opacity-40"
                  >
                    {t('common.save')}
                  </button>
                </div>
              </div>
            )}
          </div>
        ))}
      </div>
      <button
        type="button"
        onClick={() => setShowAddRule(!showAddRule)}
        className="mt-4 flex items-center gap-2 rounded-lg bg-surface-container-low px-4 py-2.5 text-sm font-semibold text-primary transition-colors hover:bg-surface-container-high"
      >
        <Icon name="add" className="text-lg" />
        {t('sensors.add_new_rule')}
      </button>
      {showAddRule && (
        <div className="mt-3 rounded-lg bg-surface-container-low p-4 space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <input placeholder={t('sensors.rule_name_placeholder')} value={newRule.name}
              onChange={e => setNewRule(p => ({...p, name: e.target.value}))}
              className="p-2 bg-surface-container rounded-lg text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40" />
            <select value={newRule.metric_name}
              onChange={e => setNewRule(p => ({...p, metric_name: e.target.value}))}
              className="p-2 bg-surface-container rounded-lg text-sm text-on-surface outline-none">
              <option value="">{t('sensors.metric_placeholder')}</option>
              <option value="cpu_usage">{t('sensors.metric_cpu_usage')}</option>
              <option value="temperature">{t('sensors.metric_temperature')}</option>
              <option value="memory_usage">{t('sensors.metric_memory_usage')}</option>
              <option value="disk_usage">{t('sensors.metric_disk_usage')}</option>
              <option value="power_kw">{t('sensors.metric_power_kw')}</option>
            </select>
            <select value={newRule.severity}
              onChange={e => setNewRule(p => ({...p, severity: e.target.value}))}
              className="p-2 bg-surface-container rounded-lg text-sm text-on-surface outline-none">
              <option value="warning">{t('sensors.severity_warning')}</option>
              <option value="critical">{t('sensors.severity_critical')}</option>
            </select>
            <input type="number" placeholder={t('sensors.threshold_placeholder')} value={newRule.threshold}
              onChange={e => setNewRule(p => ({...p, threshold: Number(e.target.value)}))}
              className="p-2 bg-surface-container rounded-lg text-sm text-on-surface outline-none" />
          </div>
          <div className="flex gap-2 justify-end">
            <button onClick={() => setShowAddRule(false)}
              className="px-3 py-1.5 rounded-lg bg-surface-container-high text-xs text-on-surface-variant">{t('common.cancel')}</button>
            <button onClick={() => {
              if (newRule.name && newRule.metric_name) {
                createAlertRuleMutate({
                  name: newRule.name,
                  metric_name: newRule.metric_name,
                  condition: { op: '>', threshold: newRule.threshold },
                  severity: newRule.severity,
                  enabled: true,
                }, {
                  onSuccess: () => {
                    setShowAddRule(false);
                    setNewRule({ name: '', metric_name: '', severity: 'warning', threshold: 80 });
                  }
                });
              }
            }} disabled={!newRule.name || !newRule.metric_name}
              className="px-3 py-1.5 rounded-lg bg-primary text-on-primary-container text-xs font-semibold disabled:opacity-40">
              {t('sensors.create_rule')}
            </button>
          </div>
        </div>
      )}
    </Section>
  );
}
