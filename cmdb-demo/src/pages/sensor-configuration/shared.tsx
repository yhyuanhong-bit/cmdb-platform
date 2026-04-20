import { useTranslation } from "react-i18next";
import Icon from "../../components/Icon";

/* ──────────────────────────────────────────────
   Types
   ────────────────────────────────────────────── */

export interface Sensor {
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

export interface ThresholdConfig {
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

export interface AlertRule {
  id: string;
  name: string;
  condition: string;
  action: string;
  enabled: boolean;
}

export interface ApiSensor {
  id: string;
  name: string;
  type: string;
  icon?: string;
  location?: string;
  enabled: boolean;
  pollingInterval?: number;
  lastSeen?: string;
  status?: 'Online' | 'Offline' | 'Degraded';
}

export interface SensorListResponse {
  sensors?: ApiSensor[];
}

export interface GroupedThresholdData {
  enabled: boolean;
  warning?: number;
  warningId?: string;
  critical?: number;
  criticalId?: string;
}

export interface NewRuleDraft {
  name: string;
  metric_name: string;
  severity: string;
  threshold: number;
}

export interface EditRuleDraft {
  name: string;
  condition: string;
  action: string;
}

/* ──────────────────────────────────────────────
   Constants
   ────────────────────────────────────────────── */

export const THRESHOLDS: ThresholdConfig[] = [
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

export const POLLING_OPTIONS = [5, 10, 15, 30, 60, 120, 300];

/* ──────────────────────────────────────────────
   Small shared components
   ────────────────────────────────────────────── */

export function Toggle({
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

export function Section({
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

export function StatusDot({ status }: { status: string }) {
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
