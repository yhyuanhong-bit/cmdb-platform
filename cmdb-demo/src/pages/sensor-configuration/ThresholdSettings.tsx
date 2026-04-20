import { useTranslation } from "react-i18next";
import Icon from "../../components/Icon";
import { Section, type ThresholdConfig } from "./shared";

interface ThresholdSettingsProps {
  thresholds: ThresholdConfig[];
  updateThreshold: (index: number, field: "warning" | "critical", value: number) => void;
}

export function ThresholdSettings({ thresholds, updateThreshold }: ThresholdSettingsProps) {
  const { t } = useTranslation();

  return (
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
  );
}
