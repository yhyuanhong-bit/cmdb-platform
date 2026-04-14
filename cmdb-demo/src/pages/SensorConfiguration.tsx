import { toast } from 'sonner'
import { memo, useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import Icon from "../components/Icon";
import { useAssets } from "../hooks/useAssets";
import { useAlertRules, useUpdateAlertRule, useCreateAlertRule, useDeleteAlertRule } from "../hooks/useMonitoring";
import { useSensors, useUpdateSensor } from "../hooks/useSensors";

/* ──────────────────────────────────────────────
   Mock data
   ────────────────────────────────────────────── */

interface Sensor {
  id: string;
  name: string;
  type: string;
  icon: string;
  location: string;
  enabled: boolean;
  pollingInterval: number;
  lastSeen: string;
  status: "Online" | "Offline" | "Degraded";
}

/* INITIAL_SENSORS removed - now fetched from assets API */

interface ThresholdConfig {
  metric: string;
  icon: string;
  unit: string;
  warning: number;
  critical: number;
  min: number;
  max: number;
  step: number;
  warningId?: string;
  criticalId?: string;
}

const THRESHOLDS: ThresholdConfig[] = [
  {
    metric: "Temperature",
    icon: "thermostat",
    unit: "°C",
    warning: 38,
    critical: 42,
    min: 20,
    max: 60,
    step: 1,
  },
  {
    metric: "Humidity",
    icon: "humidity_percentage",
    unit: "%",
    warning: 65,
    critical: 80,
    min: 20,
    max: 100,
    step: 5,
  },
  {
    metric: "Power Draw",
    icon: "bolt",
    unit: "kW",
    warning: 6.5,
    critical: 7.5,
    min: 0,
    max: 10,
    step: 0.5,
  },
];

interface AlertRule {
  id: string;
  name: string;
  condition: string;
  action: string;
  enabled: boolean;
}

// Alert rules are loaded from the API via useAlertRules — no static fallback.

const POLLING_OPTIONS = [5, 10, 15, 30, 60, 120, 300];

/* ──────────────────────────────────────────────
   Components
   ────────────────────────────────────────────── */

function Toggle({
  enabled,
  onToggle,
}: {
  enabled: boolean;
  onToggle: () => void;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={enabled}
      onClick={onToggle}
      className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer items-center rounded-full transition-colors duration-200 ${
        enabled ? "bg-primary" : "bg-surface-container-highest"
      }`}
    >
      <span
        className={`inline-block h-4 w-4 rounded-full bg-surface transition-transform duration-200 ${
          enabled ? "translate-x-6" : "translate-x-1"
        }`}
      />
    </button>
  );
}

function Section({
  title,
  icon,
  children,
  className = "",
}: {
  title: string;
  icon: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={`rounded-lg bg-surface-container p-5 ${className}`}>
      <div className="mb-4 flex items-center gap-2">
        <Icon name={icon} className="text-xl text-primary" />
        <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
          {title}
        </h3>
      </div>
      {children}
    </div>
  );
}

function StatusDot({ status }: { status: string }) {
  const { t } = useTranslation();
  const color =
    status === "Online"
      ? "bg-[#34d399]"
      : status === "Degraded"
        ? "bg-[#fbbf24]"
        : "bg-[#ff6b6b]";
  const statusLabel =
    status === "Online" ? t('common.online') :
    status === "Degraded" ? t('common.degraded') :
    t('common.offline');
  return (
    <span className="flex items-center gap-1.5">
      <span className={`h-2 w-2 rounded-full ${color}`} />
      <span className="text-xs text-on-surface-variant">{statusLabel}</span>
    </span>
  );
}

/* ──────────────────────────────────────────────
   Main Page
   ────────────────────────────────────────────── */

function SensorConfiguration() {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const { data: assetsResp, isLoading } = useAssets();
  const allAssets = assetsResp?.data ?? [];

  const { data: sensorData } = useSensors();
  const apiSensors = (sensorData as any)?.sensors ?? [];
  const updateSensor = useUpdateSensor();

  const { data: rulesResp, isLoading: rulesLoading } = useAlertRules();
  const apiRules = rulesResp?.data || [];
  const updateAlertRule = useUpdateAlertRule();
  const createAlertRule = useCreateAlertRule();
  const deleteAlertRule = useDeleteAlertRule();

  const [sensors, setSensors] = useState<Sensor[]>([]);
  const [showAddRule, setShowAddRule] = useState(false);
  const [newRule, setNewRule] = useState({ name: '', metric_name: '', severity: 'warning', threshold: 80 });
  const [editingRuleId, setEditingRuleId] = useState<string | null>(null);
  const [editDraft, setEditDraft] = useState<{ name: string; condition: string; action: string } | null>(null);

  useEffect(() => {
    if (apiSensors.length > 0) {
      setSensors(apiSensors.map((s: any) => ({
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [apiSensors.length, apiSensors.map((s: any) => s.id).join()]);
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

    // Group by metric_name, merge warning + critical
    const grouped: Record<string, any> = {};
    apiRules.forEach(rule => {
      const key = rule.metric_name;
      if (!grouped[key]) grouped[key] = { enabled: true };
      const threshold = (rule.condition as any)?.threshold ?? 0;
      if (rule.severity === 'warning') {
        grouped[key].warning = threshold;
        grouped[key].warningId = rule.id;
      } else if (rule.severity === 'critical') {
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
      condition: `${r.metric_name} ${(r.condition as any)?.op || '>'} ${(r.condition as any)?.threshold || 0}`,
      action: r.severity === 'critical' ? 'Page on-call + Escalate' : 'Notify Team',
      enabled: r.enabled,
    })));
  }, [apiRules]);
  const [globalPolling, setGlobalPolling] = useState(30);

  const toggleSensor = (id: string) => {
    const sensor = sensors.find(s => s.id === id);
    if (sensor) {
      updateSensor.mutate({ id, data: { enabled: !sensor.enabled } });
    }
    setSensors((prev) =>
      prev.map((s) => (s.id === id ? { ...s, enabled: !s.enabled } : s)),
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

  const onlineCount = sensors.filter((s) => s.status === "Online").length;
  const degradedCount = sensors.filter((s) => s.status === "Degraded").length;
  const offlineCount = sensors.filter((s) => s.status === "Offline").length;

  return (
    <div className="min-h-screen space-y-6 bg-surface px-6 py-5 font-body text-on-surface">
      {/* ── Breadcrumb ── */}
      <nav
        aria-label="Breadcrumb"
        className="flex items-center gap-1.5 text-xs uppercase tracking-widest text-on-surface-variant"
      >
        <span
          className="cursor-pointer transition-colors hover:text-primary"
          onClick={() => navigate("/monitoring")}
        >
          {t('common.monitoring')}
        </span>
        <span className="text-[10px] opacity-40" aria-hidden="true">›</span>
        <span className="text-on-surface font-semibold">{t('sensors.title')}</span>
      </nav>

      {/* ── Header ── */}
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="font-headline text-2xl font-bold text-on-surface">
            {t('sensors.title')}
          </h1>
          <p className="mt-1 text-xs uppercase tracking-widest text-on-surface-variant">
            {t('sensors.subtitle')}
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={async () => {
              try {
                for (const th of thresholds) {
                  if (th.warningId) {
                    await updateAlertRule.mutateAsync({
                      id: th.warningId,
                      data: { condition: { op: '>', threshold: th.warning } }
                    });
                  }
                  if (th.criticalId) {
                    await updateAlertRule.mutateAsync({
                      id: th.criticalId,
                      data: { condition: { op: '>', threshold: th.critical } }
                    });
                  }
                }
                toast.success(t('sensors.configuration_saved'));
              } catch (e) {
                toast.error(t('sensors.save_failed', { message: (e as Error).message }));
              }
            }}
            className="flex items-center gap-2 rounded-lg bg-primary px-4 py-2.5 text-sm font-semibold text-on-primary-container transition-colors hover:brightness-110"
          >
            <Icon name="save" className="text-base" />
            {t('common.save_configuration')}
          </button>
          <button
            type="button"
            onClick={() => toast.info(t('common.coming_soon'))}
            className="flex items-center gap-2 rounded-lg bg-surface-container-high px-4 py-2.5 text-sm font-semibold text-on-surface-variant transition-colors hover:text-on-surface"
          >
            <Icon name="sync" className="text-base" />
            {t('sensors.discover_sensors')}
          </button>
        </div>
      </div>

      {/* ── Status Summary ── */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-4">
        <div className="rounded-lg bg-surface-container p-4">
          <div className="flex items-center gap-2 text-on-surface-variant">
            <Icon name="sensors" className="text-lg" />
            <span className="text-xs font-medium uppercase tracking-wider">
              {t('sensors.total_sensors')}
            </span>
          </div>
          <p className="mt-1 font-headline text-3xl font-bold text-on-surface">
            {sensors.length}
          </p>
        </div>
        <div className="rounded-lg bg-surface-container p-4">
          <div className="flex items-center gap-2 text-[#34d399]">
            <Icon name="check_circle" className="text-lg" />
            <span className="text-xs font-medium uppercase tracking-wider">
              {t('common.online')}
            </span>
          </div>
          <p className="mt-1 font-headline text-3xl font-bold text-[#34d399]">
            {onlineCount}
          </p>
        </div>
        <div className="rounded-lg bg-surface-container p-4">
          <div className="flex items-center gap-2 text-[#fbbf24]">
            <Icon name="warning" className="text-lg" />
            <span className="text-xs font-medium uppercase tracking-wider">
              {t('common.degraded')}
            </span>
          </div>
          <p className="mt-1 font-headline text-3xl font-bold text-[#fbbf24]">
            {degradedCount}
          </p>
        </div>
        <div className="rounded-lg bg-surface-container p-4">
          <div className="flex items-center gap-2 text-error">
            <Icon name="cancel" className="text-lg" />
            <span className="text-xs font-medium uppercase tracking-wider">
              {t('common.offline')}
            </span>
          </div>
          <p className="mt-1 font-headline text-3xl font-bold text-error">
            {offlineCount}
          </p>
        </div>
      </div>

      {/* ── Sensor List ── */}
      <Section title={t('sensors.sensor_inventory')} icon="sensors">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="bg-surface-container-high text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                <th className="px-4 py-3">{t('sensors.table_enabled')}</th>
                <th className="px-4 py-3">{t('sensors.table_sensor')}</th>
                <th className="px-4 py-3">{t('sensors.table_type')}</th>
                <th className="px-4 py-3">{t('sensors.table_location')}</th>
                <th className="px-4 py-3">{t('sensors.table_polling')}</th>
                <th className="px-4 py-3">{t('sensors.table_status')}</th>
                <th className="px-4 py-3">{t('sensors.table_last_seen')}</th>
              </tr>
            </thead>
            <tbody>
              {sensors.length === 0 && (
                <tr>
                  <td colSpan={7} className="px-4 py-12">
                    <div className="flex flex-col items-center justify-center text-on-surface-variant">
                      <span className="material-symbols-outlined text-4xl mb-2">sensors_off</span>
                      <p className="text-sm">{t('sensor_config.no_sensors')}</p>
                    </div>
                  </td>
                </tr>
              )}
              {sensors.map((sensor) => (
                <tr
                  key={sensor.id}
                  className={`transition-colors hover:bg-surface-container-low ${!sensor.enabled ? "opacity-50" : ""}`}
                >
                  <td className="px-4 py-3">
                    <Toggle
                      enabled={sensor.enabled}
                      onToggle={() => toggleSensor(sensor.id)}
                    />
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2.5">
                      <Icon
                        name={sensor.icon}
                        className="text-lg text-primary"
                      />
                      <div>
                        <p className="text-sm font-semibold text-on-surface">
                          {sensor.name}
                        </p>
                        <p className="font-mono text-[10px] text-on-surface-variant">
                          {sensor.id}
                        </p>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3 text-xs text-on-surface-variant">
                    {sensor.type}
                  </td>
                  <td className="px-4 py-3 text-xs text-on-surface-variant">
                    {sensor.location}
                  </td>
                  <td className="px-4 py-3">
                    <select
                      value={sensor.pollingInterval}
                      onChange={() => toast.info(t('common.coming_soon'))}
                      className="rounded bg-surface-container-low px-2 py-1 text-xs text-on-surface outline-none"
                    >
                      {POLLING_OPTIONS.map((opt) => (
                        <option key={opt} value={opt}>
                          {opt}s
                        </option>
                      ))}
                    </select>
                  </td>
                  <td className="px-4 py-3">
                    <StatusDot status={sensor.status} />
                  </td>
                  <td className="px-4 py-3 text-xs text-on-surface-variant">
                    {sensor.lastSeen}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Section>

      {/* ── Threshold Settings + Polling ── */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        {/* Threshold Settings */}
        <Section
          title={t('sensors.threshold_configuration')}
          icon="tune"
          className="lg:col-span-2"
        >
          <div className="space-y-6">
            {thresholds.map((th, i) => (
              <div
                key={th.metric}
                className="rounded-lg bg-surface-container-low p-4"
              >
                <div className="mb-3 flex items-center gap-2">
                  <Icon name={th.icon} className="text-lg text-primary" />
                  <h4 className="text-sm font-semibold text-on-surface">
                    {th.metric}
                  </h4>
                </div>
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <div>
                    <label className="mb-1.5 block text-xs font-medium uppercase tracking-wider text-[#fbbf24]">
                      {t('sensors.warning_threshold')}
                    </label>
                    <div className="flex items-center gap-3">
                      <input
                        type="range"
                        min={th.min}
                        max={th.max}
                        step={th.step}
                        value={th.warning}
                        onChange={(e) => {
                          const newVal = Number(e.target.value);
                          if (newVal >= thresholds[i].critical) return;
                          updateThreshold(i, "warning", newVal);
                        }}
                        className="h-1.5 flex-1 cursor-pointer appearance-none rounded-full bg-surface-container accent-[#fbbf24]"
                      />
                      <span className="w-16 rounded bg-surface-container px-2 py-1 text-center text-sm font-semibold text-[#fbbf24]">
                        {th.warning}
                        {th.unit}
                      </span>
                    </div>
                  </div>
                  <div>
                    <label className="mb-1.5 block text-xs font-medium uppercase tracking-wider text-error">
                      {t('sensors.critical_threshold')}
                    </label>
                    <div className="flex items-center gap-3">
                      <input
                        type="range"
                        min={th.min}
                        max={th.max}
                        step={th.step}
                        value={th.critical}
                        onChange={(e) => {
                          const newVal = Number(e.target.value);
                          if (newVal <= thresholds[i].warning) return;
                          updateThreshold(i, "critical", newVal);
                        }}
                        className="h-1.5 flex-1 cursor-pointer appearance-none rounded-full bg-surface-container accent-error"
                      />
                      <span className="w-16 rounded bg-surface-container px-2 py-1 text-center text-sm font-semibold text-error">
                        {th.critical}
                        {th.unit}
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </Section>

        {/* Global Polling Interval */}
        <div className="space-y-4">
          <Section title={t('sensors.global_polling_interval')} icon="schedule">
            <div className="space-y-4">
              <p className="text-xs text-on-surface-variant">
                {t('sensors.global_polling_description')}
              </p>
              <div className="grid grid-cols-2 gap-2">
                {POLLING_OPTIONS.map((opt) => (
                  <button
                    key={opt}
                    type="button"
                    onClick={() => setGlobalPolling(opt)}
                    className={`rounded-lg px-3 py-2.5 text-sm font-semibold transition-colors ${
                      globalPolling === opt
                        ? "bg-primary text-on-primary-container"
                        : "bg-surface-container-low text-on-surface-variant hover:text-on-surface"
                    }`}
                  >
                    {opt < 60 ? `${opt}s` : `${opt / 60}m`}
                  </button>
                ))}
              </div>
              <div className="rounded-lg bg-surface-container-low p-3">
                <div className="flex items-center gap-2">
                  <Icon name="info" className="text-base text-primary" />
                  <p className="text-[11px] text-on-surface-variant">
                    {t('sensors.polling_recommendation')}
                  </p>
                </div>
              </div>
            </div>
          </Section>

          <Section title={t('sensors.quick_actions')} icon="flash_on">
            <div className="space-y-2">
              <button
                type="button"
                onClick={() => { setThresholds(THRESHOLDS); toast.success(t('sensors.thresholds_reset')) }}
                className="flex w-full items-center gap-2 rounded-lg bg-surface-container-low p-3 text-sm text-on-surface-variant transition-colors hover:text-on-surface"
              >
                <Icon name="restart_alt" className="text-lg text-primary" />
                {t('sensors.reset_all_to_defaults')}
              </button>
              <button
                type="button"
                onClick={() => {
                  const config = { thresholds, rules, globalPolling };
                  const blob = new Blob([JSON.stringify(config, null, 2)], { type: 'application/json' });
                  const url = URL.createObjectURL(blob);
                  const a = document.createElement('a');
                  a.href = url; a.download = 'sensor-config.json'; a.click();
                  URL.revokeObjectURL(url);
                }}
                className="flex w-full items-center gap-2 rounded-lg bg-surface-container-low p-3 text-sm text-on-surface-variant transition-colors hover:text-on-surface"
              >
                <Icon name="download" className="text-lg text-primary" />
                {t('common.export_configuration')}
              </button>
              <button
                type="button"
                onClick={() => {
                  const input = document.createElement('input');
                  input.type = 'file';
                  input.accept = '.json';
                  input.onchange = (e: any) => {
                    const file = e.target.files?.[0];
                    if (file) {
                      const reader = new FileReader();
                      reader.onload = (ev) => {
                        try {
                          const config = JSON.parse(ev.target?.result as string);
                          if (config.thresholds) setThresholds(config.thresholds);
                          if (config.rules) setRules(config.rules);
                          if (config.globalPolling) setGlobalPolling(config.globalPolling);
                          toast.success(t('sensors.config_imported'));
                        } catch { toast.error(t('sensors.config_invalid')) }
                      };
                      reader.readAsText(file);
                    }
                  };
                  input.click();
                }}
                className="flex w-full items-center gap-2 rounded-lg bg-surface-container-low p-3 text-sm text-on-surface-variant transition-colors hover:text-on-surface"
              >
                <Icon name="upload" className="text-lg text-primary" />
                {t('common.import_configuration')}
              </button>
            </div>
          </Section>
        </div>
      </div>

      {/* ── Alert Rules ── */}
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
                      deleteAlertRule.mutate(rule.id);
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
                        updateAlertRule.mutate({ id: rule.id, data: { name: editDraft.name } });
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
                  createAlertRule.mutate({
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
    </div>
  );
}

export default memo(SensorConfiguration);
