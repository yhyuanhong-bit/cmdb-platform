import { toast } from 'sonner'
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import Icon from "../../components/Icon";
import { useUpdateAlertRule } from "../../hooks/useMonitoring";
import type { Sensor, ThresholdConfig } from "./shared";

interface SensorPageHeaderProps {
  sensors: Sensor[];
  thresholds: ThresholdConfig[];
  saving: boolean;
  setSaving: (value: boolean) => void;
  assetsError: boolean;
  sensorsError: boolean;
  updateAlertRuleMutateAsync: ReturnType<typeof useUpdateAlertRule>['mutateAsync'];
}

export function SensorPageHeader({
  sensors,
  thresholds,
  saving,
  setSaving,
  assetsError,
  sensorsError,
  updateAlertRuleMutateAsync,
}: SensorPageHeaderProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const onlineCount = sensors.filter((s) => s.status === "Online").length;
  const degradedCount = sensors.filter((s) => s.status === "Degraded").length;
  const offlineCount = sensors.filter((s) => s.status === "Offline").length;

  return (
    <>
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
            disabled={saving}
            onClick={async () => {
              setSaving(true);
              try {
                for (const th of thresholds) {
                  if (th.warningId) {
                    await updateAlertRuleMutateAsync({
                      id: th.warningId,
                      data: { condition: { op: '>', threshold: th.warning } }
                    });
                  }
                  if (th.criticalId) {
                    await updateAlertRuleMutateAsync({
                      id: th.criticalId,
                      data: { condition: { op: '>', threshold: th.critical } }
                    });
                  }
                }
                toast.success(t('sensors.configuration_saved'));
              } catch (e) {
                toast.error(t('sensors.save_failed', { message: (e as Error).message }));
              } finally {
                setSaving(false);
              }
            }}
            className="flex items-center gap-2 rounded-lg bg-primary px-4 py-2.5 text-sm font-semibold text-on-primary-container transition-colors hover:brightness-110 disabled:opacity-50"
          >
            <Icon name={saving ? "hourglass_top" : "save"} className="text-base" />
            {saving ? t('common.saving', 'Saving...') : t('common.save_configuration')}
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

      {/* ── Error Banner ── */}
      {(assetsError || sensorsError) && (
        <div className="flex items-center gap-3 rounded-lg bg-error-container/30 px-4 py-3 text-sm text-error">
          <Icon name="error" className="text-lg" />
          {t('sensors.load_error', 'Failed to load sensor data. Please check your connection and try again.')}
        </div>
      )}

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
    </>
  );
}
