import { toast } from 'sonner'
import { useTranslation } from "react-i18next";
import Icon from "../../components/Icon";
import { useUpdateSensor } from "../../hooks/useSensors";
import {
  Section,
  POLLING_OPTIONS,
  THRESHOLDS,
  type Sensor,
  type ThresholdConfig,
  type AlertRule,
} from "./shared";

interface GlobalPollingPanelProps {
  globalPolling: number;
  setGlobalPolling: (value: number) => void;
  sensors: Sensor[];
  setSensors: React.Dispatch<React.SetStateAction<Sensor[]>>;
  updateSensorMutateAsync: ReturnType<typeof useUpdateSensor>['mutateAsync'];
  thresholds: ThresholdConfig[];
  setThresholds: React.Dispatch<React.SetStateAction<ThresholdConfig[]>>;
  rules: AlertRule[];
  setRules: React.Dispatch<React.SetStateAction<AlertRule[]>>;
}

export function GlobalPollingPanel({
  globalPolling,
  setGlobalPolling,
  sensors,
  setSensors,
  updateSensorMutateAsync,
  thresholds,
  setThresholds,
  rules,
  setRules,
}: GlobalPollingPanelProps) {
  const { t } = useTranslation();

  return (
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
                onClick={() => {
                  setGlobalPolling(opt);
                  // Batch update all enabled sensors
                  const enabled = sensors.filter(s => s.enabled);
                  if (enabled.length === 0) return;
                  Promise.all(
                    enabled.map(s => updateSensorMutateAsync({ id: s.id, data: { polling_interval: opt } }))
                  ).then(() => {
                    setSensors(ss => ss.map(s => s.enabled ? { ...s, pollingInterval: opt } : s));
                    toast.success(t('sensors.global_polling_applied', { count: enabled.length, defaultValue: `Polling interval applied to ${enabled.length} sensors` }));
                  }).catch(() => toast.error(t('sensors.polling_failed', 'Failed to update polling interval')));
                }}
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
              input.onchange = (e: Event) => {
                const file = (e.target as HTMLInputElement).files?.[0];
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
  );
}
