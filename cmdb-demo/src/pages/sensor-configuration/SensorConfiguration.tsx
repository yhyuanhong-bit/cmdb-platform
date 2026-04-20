import { toast } from 'sonner'
import { memo, useState, useEffect, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useAssets } from "../../hooks/useAssets";
import { useAlertRules, useUpdateAlertRule, useCreateAlertRule, useDeleteAlertRule } from "../../hooks/useMonitoring";
import { useSensors, useUpdateSensor } from "../../hooks/useSensors";
import {
  THRESHOLDS,
  type Sensor,
  type ThresholdConfig,
  type AlertRule,
  type ApiSensor,
  type SensorListResponse,
  type GroupedThresholdData,
  type EditRuleDraft,
} from "./shared";
import { SensorPageHeader } from "./SensorPageHeader";
import { SensorInventoryTable } from "./SensorInventoryTable";
import { ThresholdSettings } from "./ThresholdSettings";
import { GlobalPollingPanel } from "./GlobalPollingPanel";
import { AlertRulesSection } from "./AlertRulesSection";

function SensorConfiguration() {
  const { t } = useTranslation();

  // Preserve original hook calls exactly (some destructured values are unused in
  // the original and kept here to keep the hook lifecycle identical).
  const { data: assetsResp, isLoading, isError: assetsError } = useAssets();
  void isLoading;
  const allAssets = assetsResp?.data ?? [];
  void allAssets;

  const { data: sensorData, isError: sensorsError } = useSensors();
  const apiSensors: ApiSensor[] = (sensorData as SensorListResponse | undefined)?.sensors ?? [];
  const updateSensor = useUpdateSensor();

  const { data: rulesResp, isLoading: rulesLoading } = useAlertRules();
  void rulesLoading;
  const apiRules = rulesResp?.data || [];
  const updateAlertRule = useUpdateAlertRule();
  const createAlertRule = useCreateAlertRule();
  const deleteAlertRule = useDeleteAlertRule();

  const [sensors, setSensors] = useState<Sensor[]>([]);
  const [saving, setSaving] = useState(false);
  const [showAddRule, setShowAddRule] = useState(false);
  const [newRule, setNewRule] = useState({ name: '', metric_name: '', severity: 'warning', threshold: 80 });
  const [editingRuleId, setEditingRuleId] = useState<string | null>(null);
  const [editDraft, setEditDraft] = useState<EditRuleDraft | null>(null);

  const sensorIdKey = useMemo(() => apiSensors.map((s: ApiSensor) => s.id).join(','), [apiSensors]);
  useEffect(() => {
    if (apiSensors.length > 0) {
      setSensors(apiSensors.map((s: ApiSensor) => ({
        id: s.id,
        name: s.name,
        type: s.type,
        icon: s.icon || 'sensors',
        location: s.location || '',
        enabled: s.enabled,
        pollingInterval: s.pollingInterval || 30,
        lastSeen: s.lastSeen ? new Date(s.lastSeen).toLocaleString() : 'Never',
        status: s.status || 'Offline',
      })));
    }
  }, [sensorIdKey, apiSensors]);

  const [rules, setRules] = useState<AlertRule[]>([]);
  const [thresholds, setThresholds] = useState<ThresholdConfig[]>(THRESHOLDS);

  useEffect(() => {
    if (apiRules.length === 0) return;

    const META: Record<string, { icon: string; unit: string; min: number; max: number; step: number }> = {
      cpu_usage:    { icon: 'memory', unit: '%', min: 0, max: 100, step: 5 },
      temperature:  { icon: 'thermostat', unit: '\u00b0C', min: 10, max: 60, step: 1 },
      disk_usage:   { icon: 'storage', unit: '%', min: 0, max: 100, step: 5 },
      memory_usage: { icon: 'memory', unit: '%', min: 0, max: 100, step: 5 },
      power_kw:     { icon: 'bolt', unit: 'kW', min: 0, max: 10, step: 0.5 },
    };

    // Group by metric_name, merge warning + critical (newest rule wins on duplicates)
    const grouped: Record<string, GroupedThresholdData> = {};
    const sortedRules = [...apiRules].sort((a, b) =>
      new Date(b.created_at ?? 0).getTime() - new Date(a.created_at ?? 0).getTime()
    );
    sortedRules.forEach(rule => {
      const key = rule.metric_name;
      if (!grouped[key]) grouped[key] = { enabled: true };
      const threshold = (rule.condition as Record<string, unknown>)?.threshold as number ?? 0;
      if (rule.severity === 'warning' && !grouped[key].warningId) {
        grouped[key].warning = threshold;
        grouped[key].warningId = rule.id;
      } else if (rule.severity === 'critical' && !grouped[key].criticalId) {
        grouped[key].critical = threshold;
        grouped[key].criticalId = rule.id;
      }
      if (!rule.enabled) grouped[key].enabled = false;
    });

    setThresholds(Object.entries(grouped).map(([metric, data]) => ({
      metric,
      icon: META[metric]?.icon || 'sensors',
      unit: META[metric]?.unit || '',
      warning: data.warning ?? 0,
      critical: data.critical ?? 100,
      min: META[metric]?.min ?? 0,
      max: META[metric]?.max ?? 100,
      step: META[metric]?.step ?? 1,
      warningId: data.warningId,
      criticalId: data.criticalId,
    })));

    // Also update rules list from API
    setRules(apiRules.map(r => ({
      id: r.id,
      name: r.name,
      condition: `${r.metric_name} ${(r.condition as Record<string, unknown>)?.op as string || '>'} ${(r.condition as Record<string, unknown>)?.threshold as number || 0}`,
      action: r.severity === 'critical' ? 'Page on-call + Escalate' : 'Notify Team',
      enabled: r.enabled,
    })));
  }, [apiRules]);

  const [globalPolling, setGlobalPolling] = useState(30);

  const toggleSensor = (id: string) => {
    const sensor = sensors.find(s => s.id === id);
    if (!sensor) return;
    const prev = sensor.enabled;
    // Optimistic update
    setSensors((ss) => ss.map((s) => (s.id === id ? { ...s, enabled: !prev } : s)));
    updateSensor.mutate(
      { id, data: { enabled: !prev } },
      { onError: () => {
        // Rollback on failure
        setSensors((ss) => ss.map((s) => (s.id === id ? { ...s, enabled: prev } : s)));
        toast.error(t('sensors.toggle_failed', 'Failed to toggle sensor'));
      }},
    );
  };

  const toggleRule = (id: string) => {
    setRules((prev) =>
      prev.map((r) => (r.id === id ? { ...r, enabled: !r.enabled } : r)),
    );
  };

  const updateThreshold = (
    index: number,
    field: "warning" | "critical",
    value: number,
  ) => {
    setThresholds((prev) =>
      prev.map((t, i) => (i === index ? { ...t, [field]: value } : t)),
    );
  };

  return (
    <div className="min-h-screen space-y-6 bg-surface px-6 py-5 font-body text-on-surface">
      <SensorPageHeader
        sensors={sensors}
        thresholds={thresholds}
        saving={saving}
        setSaving={setSaving}
        assetsError={assetsError}
        sensorsError={sensorsError}
        updateAlertRuleMutateAsync={updateAlertRule.mutateAsync}
      />

      <SensorInventoryTable
        sensors={sensors}
        setSensors={setSensors}
        toggleSensor={toggleSensor}
        updateSensorMutate={updateSensor.mutate}
      />

      {/* ── Threshold Settings + Polling ── */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <ThresholdSettings thresholds={thresholds} updateThreshold={updateThreshold} />
        <GlobalPollingPanel
          globalPolling={globalPolling}
          setGlobalPolling={setGlobalPolling}
          sensors={sensors}
          setSensors={setSensors}
          updateSensorMutateAsync={updateSensor.mutateAsync}
          thresholds={thresholds}
          setThresholds={setThresholds}
          rules={rules}
          setRules={setRules}
        />
      </div>

      <AlertRulesSection
        rules={rules}
        setRules={setRules}
        toggleRule={toggleRule}
        editingRuleId={editingRuleId}
        setEditingRuleId={setEditingRuleId}
        editDraft={editDraft}
        setEditDraft={setEditDraft}
        showAddRule={showAddRule}
        setShowAddRule={setShowAddRule}
        newRule={newRule}
        setNewRule={setNewRule}
        updateAlertRuleMutate={updateAlertRule.mutate}
        createAlertRuleMutate={createAlertRule.mutate}
        deleteAlertRuleMutate={deleteAlertRule.mutate}
      />
    </div>
  );
}

export default memo(SensorConfiguration);
