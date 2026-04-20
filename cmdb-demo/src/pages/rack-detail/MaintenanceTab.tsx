import { useTranslation } from "react-i18next";

function MetricGauge({
  label,
  current,
  min,
  max,
  unit,
  threshold,
  icon,
}: {
  label: string;
  current: number;
  min: number;
  max: number;
  unit: string;
  threshold: number;
  icon: string;
}) {
  const { t } = useTranslation();
  const pct = (current / threshold) * 100;
  const isWarning = pct > 80;
  return (
    <div className="bg-surface-container-low rounded p-4">
      <div className="flex items-center gap-2 mb-3">
        <span className={`material-symbols-outlined text-[18px] ${isWarning ? "text-tertiary" : "text-primary"}`}>{icon}</span>
        <span className="text-[10px] uppercase tracking-widest text-on-surface-variant">{label}</span>
      </div>
      <p className={`text-3xl font-headline font-bold mb-1 ${isWarning ? "text-tertiary" : "text-on-surface"}`}>
        {current}
        <span className="text-sm font-normal text-on-surface-variant ml-1">{unit}</span>
      </p>
      <div className="w-full h-1.5 bg-surface-container-lowest rounded-full overflow-hidden mb-2">
        <div
          className={`h-full rounded-full transition-all ${isWarning ? "bg-tertiary" : "bg-primary"}`}
          style={{ width: `${Math.min(pct, 100)}%` }}
        />
      </div>
      <div className="flex justify-between text-[10px] text-on-surface-variant">
        <span>{t("rack_detail.min")}: {min}{unit}</span>
        <span>{t("rack_detail.max")}: {max}{unit}</span>
        <span>{t("rack_detail.limit")}: {threshold}{unit}</span>
      </div>
    </div>
  );
}

export function MaintenanceTab({ maintenanceHistory, environmentMetrics }: {
  maintenanceHistory: Array<{ date: string; type: string; description: string; engineer: string; status: string }>;
  environmentMetrics: {
    temperature: { current: number; min: number; max: number; threshold: number; unit: string };
    humidity: { current: number; min: number; max: number; threshold: number; unit: string };
    powerDraw: { current: number; min: number; max: number; threshold: number; unit: string };
    airflow: { current: number; min: number; max: number; threshold: number; unit: string };
  };
}) {
  const { t } = useTranslation();

  return (
    <div>
      {/* Environmental Monitoring */}
      <section className="mb-8">
        <div className="flex items-center gap-2 mb-4">
          <span className="material-symbols-outlined text-primary">monitoring</span>
          <h2 className="font-headline font-bold text-sm tracking-widest uppercase text-on-surface">
            {t("rack_detail.environmental_monitoring")}
          </h2>
        </div>
        <div className="grid grid-cols-4 gap-4">
          <MetricGauge label={t("rack_visualization.temperature")} icon="thermostat" {...environmentMetrics.temperature} />
          <MetricGauge label={t("rack_visualization.humidity")} icon="humidity_percentage" {...environmentMetrics.humidity} />
          <MetricGauge label={t("rack_detail.power_draw")} icon="bolt" {...environmentMetrics.powerDraw} />
          <MetricGauge label={t("rack_detail.airflow")} icon="air" {...environmentMetrics.airflow} />
        </div>
      </section>

      {/* Maintenance History */}
      <section>
        <div className="flex items-center gap-2 mb-4">
          <span className="material-symbols-outlined text-primary">build</span>
          <h2 className="font-headline font-bold text-sm tracking-widest uppercase text-on-surface">
            {t("rack_detail.recent_maintenance_history")}
          </h2>
        </div>
        <div className="bg-surface-container rounded overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-surface-container-high text-on-surface-variant text-[11px] uppercase tracking-widest">
                <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_date")}</th>
                <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_type")}</th>
                <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_description")}</th>
                <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_engineer")}</th>
                <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_status")}</th>
              </tr>
            </thead>
            <tbody>
              {maintenanceHistory.map((entry, i) => (
                <tr key={i} className="bg-surface-container hover:bg-surface-container-high transition-colors">
                  <td className="px-5 py-3 text-on-surface font-mono text-xs whitespace-nowrap">{entry.date}</td>
                  <td className="px-5 py-3">
                    <span
                      className={`inline-block px-2.5 py-0.5 rounded text-[11px] font-semibold tracking-wider ${
                        entry.type === "Corrective"
                          ? "bg-tertiary-container/60 text-tertiary"
                          : entry.type === "Firmware"
                            ? "bg-secondary-container/60 text-secondary"
                            : entry.type === "Change"
                              ? "bg-on-primary-container/20 text-primary"
                              : "bg-surface-container-highest text-on-surface-variant"
                      }`}
                    >
                      {entry.type.toUpperCase()}
                    </span>
                  </td>
                  <td className="px-5 py-3 text-on-surface-variant">{entry.description}</td>
                  <td className="px-5 py-3 text-on-surface-variant whitespace-nowrap">{entry.engineer}</td>
                  <td className="px-5 py-3">
                    <span className="inline-flex items-center gap-1 text-[11px] text-primary">
                      <span className="material-symbols-outlined text-[14px]">check_circle</span>
                      {entry.status}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
