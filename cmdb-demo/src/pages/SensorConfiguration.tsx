import { memo, useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import Icon from "../components/Icon";
import { useAssets } from "../hooks/useAssets";

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

const INITIAL_RULES: AlertRule[] = [
  {
    id: "RULE-001",
    name: "Temperature Breach Auto-Escalate",
    condition: "Temperature > Critical for 5 min",
    action: "Page on-call + Activate HVAC-AUX",
    enabled: true,
  },
  {
    id: "RULE-002",
    name: "Humidity Drift Alert",
    condition: "Humidity > Warning for 15 min",
    action: "Notify Facilities Team",
    enabled: true,
  },
  {
    id: "RULE-003",
    name: "Power Overload Protection",
    condition: "Power Draw > 90% capacity",
    action: "Shed non-critical loads + Alert NOC",
    enabled: true,
  },
  {
    id: "RULE-004",
    name: "Sensor Offline Detection",
    condition: "No heartbeat for 3 polling cycles",
    action: "Create incident ticket + Notify",
    enabled: false,
  },
  {
    id: "RULE-005",
    name: "Security Door Tamper",
    condition: "Door open > 60 sec during off-hours",
    action: "Security alert + Camera snapshot",
    enabled: true,
  },
];

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
  const color =
    status === "Online"
      ? "bg-[#34d399]"
      : status === "Degraded"
        ? "bg-[#fbbf24]"
        : "bg-[#ff6b6b]";
  return (
    <span className="flex items-center gap-1.5">
      <span className={`h-2 w-2 rounded-full ${color}`} />
      <span className="text-xs text-on-surface-variant">{status}</span>
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

  const [sensors, setSensors] = useState<Sensor[]>([]);

  useEffect(() => {
    if (allAssets.length > 0) {
      setSensors(allAssets.slice(0, 10).map(a => ({
        id: a.asset_tag,
        name: `${a.name} Sensor`,
        type: a.type === 'server' ? 'Temperature' : a.type === 'power' ? 'Power' : a.type === 'network' ? 'Network' : 'Humidity',
        icon: a.type === 'server' ? 'thermostat' : a.type === 'power' ? 'bolt' : a.type === 'network' ? 'lan' : 'humidity_percentage',
        location: `IDC / ${a.rack_id ? 'Rack ' + a.asset_tag.split('-')[0] : 'Floor'}`,
        enabled: a.status === 'operational',
        pollingInterval: 30,
        lastSeen: a.status === 'operational' ? 'live' : 'offline',
        status: a.status === 'operational' ? 'Online' as const : a.status === 'maintenance' ? 'Degraded' as const : 'Offline' as const,
      })));
    }
  }, [allAssets]);
  const [rules, setRules] = useState(INITIAL_RULES);
  const [thresholds, setThresholds] = useState(THRESHOLDS);
  const [globalPolling, setGlobalPolling] = useState(30);

  const toggleSensor = (id: string) => {
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
          監控
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
            onClick={() => alert('Save: Coming Soon')}
            className="flex items-center gap-2 rounded-lg bg-primary px-4 py-2.5 text-sm font-semibold text-on-primary-container transition-colors hover:brightness-110"
          >
            <Icon name="save" className="text-base" />
            {t('common.save_configuration')}
          </button>
          <button
            type="button"
            onClick={() => alert('Coming Soon')}
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
                      onChange={() => alert('Coming Soon')}
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
                        onChange={(e) =>
                          updateThreshold(i, "warning", Number(e.target.value))
                        }
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
                        onChange={(e) =>
                          updateThreshold(i, "critical", Number(e.target.value))
                        }
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
                onClick={() => alert('Reset to defaults')}
                className="flex w-full items-center gap-2 rounded-lg bg-surface-container-low p-3 text-sm text-on-surface-variant transition-colors hover:text-on-surface"
              >
                <Icon name="restart_alt" className="text-lg text-primary" />
                {t('sensors.reset_all_to_defaults')}
              </button>
              <button
                type="button"
                onClick={() => alert('Coming Soon')}
                className="flex w-full items-center gap-2 rounded-lg bg-surface-container-low p-3 text-sm text-on-surface-variant transition-colors hover:text-on-surface"
              >
                <Icon name="download" className="text-lg text-primary" />
                {t('common.export_configuration')}
              </button>
              <button
                type="button"
                onClick={() => alert('Coming Soon')}
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
            <div
              key={rule.id}
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
                onClick={() => alert('Coming Soon')}
                className="shrink-0 rounded p-1.5 text-on-surface-variant transition-colors hover:bg-surface-container-high hover:text-primary"
                title="Edit rule"
              >
                <Icon name="edit" className="text-lg" />
              </button>
              <button
                type="button"
                onClick={() => alert('Coming Soon')}
                className="shrink-0 rounded p-1.5 text-on-surface-variant transition-colors hover:bg-surface-container-high hover:text-error"
                title="Delete rule"
              >
                <Icon name="delete" className="text-lg" />
              </button>
            </div>
          ))}
        </div>
        <button
          type="button"
          onClick={() => alert('Coming Soon')}
          className="mt-4 flex items-center gap-2 rounded-lg bg-surface-container-low px-4 py-2.5 text-sm font-semibold text-primary transition-colors hover:bg-surface-container-high"
        >
          <Icon name="add" className="text-lg" />
          {t('sensors.add_new_rule')}
        </button>
      </Section>
    </div>
  );
}

export default memo(SensorConfiguration);
