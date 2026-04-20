import { toast } from 'sonner'
import { useTranslation } from "react-i18next";
import Icon from "../../components/Icon";
import { useUpdateSensor } from "../../hooks/useSensors";
import { Section, StatusDot, Toggle, POLLING_OPTIONS, type Sensor } from "./shared";

interface SensorInventoryTableProps {
  sensors: Sensor[];
  setSensors: React.Dispatch<React.SetStateAction<Sensor[]>>;
  toggleSensor: (id: string) => void;
  updateSensorMutate: ReturnType<typeof useUpdateSensor>['mutate'];
}

export function SensorInventoryTable({
  sensors,
  setSensors,
  toggleSensor,
  updateSensorMutate,
}: SensorInventoryTableProps) {
  const { t } = useTranslation();

  return (
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
                    onChange={(e) => {
                      const interval = parseInt(e.target.value);
                      setSensors(ss => ss.map(s => s.id === sensor.id ? { ...s, pollingInterval: interval } : s));
                      updateSensorMutate(
                        { id: sensor.id, data: { polling_interval: interval } },
                        {
                          onSuccess: () => toast.success(t('sensors.polling_updated', 'Polling interval updated')),
                          onError: () => {
                            setSensors(ss => ss.map(s => s.id === sensor.id ? { ...s, pollingInterval: sensor.pollingInterval } : s));
                            toast.error(t('sensors.polling_failed', 'Failed to update polling interval'));
                          },
                        }
                      );
                    }}
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
  );
}
