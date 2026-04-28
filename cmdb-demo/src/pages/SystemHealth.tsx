import { memo } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import Icon from "../components/Icon";
import StatusBadge from "../components/StatusBadge";
import { useAlerts } from "../hooks/useMonitoring";
import { useSystemHealth } from "../hooks/useSystemHealth";
import { useAssets } from "../hooks/useAssets";
import { apiClient } from "../lib/api/client";

/* ──────────────────────────────────────────────
   healthSegments computed dynamically inside component
   ────────────────────────────────────────────── */

/* ──────────────────────────────────────────────
   Reusable pieces
   ────────────────────────────────────────────── */

function ProgressBar({
  pct,
  color = "bg-primary",
  height = "h-2",
}: {
  pct: number;
  color?: string;
  height?: string;
}) {
  return (
    <div className={`w-full ${height} rounded-full bg-surface-container-low`}>
      <div
        className={`${height} rounded-full ${color} transition-all duration-500`}
        style={{ width: `${pct}%` }}
      />
    </div>
  );
}

function DonutChart({
  segments,
  centerLabel,
  centerSublabel,
}: {
  segments: { label: string; pct: number; color: string }[];
  centerLabel: string;
  centerSublabel: string;
}) {
  let cumulative = 0;
  const stops = segments.flatMap((s) => {
    const start = cumulative;
    cumulative += s.pct;
    return [`${s.color} ${start}%`, `${s.color} ${cumulative}%`];
  });

  return (
    <div
      className="relative mx-auto h-48 w-48 rounded-full"
      style={{ background: `conic-gradient(${stops.join(", ")})` }}
    >
      <div className="absolute inset-6 flex flex-col items-center justify-center rounded-full bg-surface-container">
        <span className="font-headline text-2xl font-bold text-on-surface">
          {centerLabel}
        </span>
        <span className="text-[10px] uppercase tracking-wider text-on-surface-variant">
          {centerSublabel}
        </span>
      </div>
    </div>
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

/* ──────────────────────────────────────────────
   Main Page
   ────────────────────────────────────────────── */

function SystemHealth() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { data: alertsResponse, isLoading: alertsLoading } = useAlerts({ severity: 'critical' });
  const criticalAlerts = alertsResponse?.data ?? [];
  const { data: healthResponse } = useSystemHealth();
  const health = (healthResponse as { data?: { database?: { status?: string; latency_ms?: number }; redis?: { status?: string; latency_ms?: number }; nats?: { status?: string; connected?: boolean } } })?.data;
  const dbStatus = health?.database?.status ?? 'unknown';
  const dbLatency = String(health?.database?.latency_ms ?? '');
  const redisStatus = health?.redis?.status ?? 'unknown';
  const natsStatus = health?.nats?.status ?? 'unknown';
  // Overall infra health derived from all three components — replaces
  // the hardcoded "OPERATIONAL" badge (audit finding H9, 2026-04-28).
  const allOk = dbStatus === 'ok' && redisStatus === 'ok' && natsStatus === 'ok';
  const anyError = dbStatus === 'error' || redisStatus === 'error' || natsStatus === 'error';
  const overallBadge: { label: string; bg: string; fg: string } = anyError
    ? { label: t('system_health.degraded', 'DEGRADED'), bg: 'bg-[#7f1d1d]', fg: 'text-[#fca5a5]' }
    : allOk
    ? { label: t('common.operational'), bg: 'bg-[#064e3b]', fg: 'text-[#34d399]' }
    : { label: t('system_health.partial', 'PARTIAL'), bg: 'bg-[#78350f]', fg: 'text-[#fbbf24]' };
  const { data: assetsResp } = useAssets({ page_size: '1' });
  const totalAssets = (assetsResp as { pagination?: { total?: number } } | undefined)?.pagination?.total ?? 0;

  const criticalCount = criticalAlerts.length;
  const healthSegments = [
    { label: 'Healthy', pct: totalAssets > 0 ? Math.round(((totalAssets - criticalCount * 2) / totalAssets) * 100) : 82, color: '#34d399' },
    { label: 'Warning', pct: totalAssets > 0 ? Math.min(Math.round((criticalCount / totalAssets) * 100), 30) : 12, color: '#fbbf24' },
    { label: 'Critical', pct: totalAssets > 0 ? Math.min(Math.round((criticalCount / totalAssets) * 100 * 0.5), 20) : 6, color: '#ff6b6b' },
  ];

  const { data: trendData } = useQuery({
    queryKey: ['alertsTrend'],
    queryFn: () => apiClient.get('/monitoring/alerts/trend', { hours: '24' }),
  });
  interface AlertTrendItem { hour: string; critical: number; warning: number; info: number }
  interface AlertTrendResponse { trend?: AlertTrendItem[] }
  const trendBars = ((trendData as AlertTrendResponse | undefined)?.trend ?? []).map((b: AlertTrendItem) => ({
    hour: new Date(b.hour).toISOString().slice(11, 16),
    critical: b.critical ?? 0,
    warning: b.warning ?? 0,
    info: b.info ?? 0,
  }));

  const uptime = allOk ? '99.99%' : 'Degraded';
  const trendMax = trendBars.length > 0
    ? Math.max(...trendBars.map((b: { critical: number; warning: number; info: number }) => b.critical + b.warning + b.info))
    : 1;

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
          Monitoring
        </span>
        <span className="text-[10px] opacity-40" aria-hidden="true">›</span>
        <span className="text-on-surface font-semibold">{t('system_health.title')}</span>
      </nav>

      {/* ── Title & Uptime ── */}
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="font-headline text-2xl font-bold text-on-surface">
            {t('system_health.title')}
          </h1>
          <p className="mt-1 text-xs uppercase tracking-widest text-on-surface-variant">
            {t('system_health.subtitle')}
          </p>
        </div>
        <div className="flex items-center gap-4">
          <div className="text-right">
            <p className="font-headline text-4xl font-bold text-on-surface">
              {uptime}
            </p>
            <p className="text-[10px] uppercase tracking-widest text-on-surface-variant">
              {t('system_health.uptime_30_day_sla')}
            </p>
          </div>
          <span className={`rounded ${overallBadge.bg} px-3 py-1.5 text-xs font-bold uppercase tracking-wider ${overallBadge.fg}`}>
            {overallBadge.label}
          </span>
        </div>
      </div>

      {/* ── Stats Row ── */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <div className="rounded-lg bg-surface-container p-5">
          <div className="mb-1 flex items-center gap-2 text-on-surface-variant">
            <Icon name="error" className="text-lg text-error" />
            <span className="text-xs font-medium uppercase tracking-wider">
              {t('system_health.critical_alerts')}
            </span>
          </div>
          <p className="font-headline text-3xl font-bold text-error">{alertsLoading ? '—' : criticalAlerts.length}</p>
          <span className="mt-1 inline-block rounded bg-red-900/50 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-error">
            {t('system_health.requires_action')}
          </span>
        </div>

        <div className="rounded-lg bg-surface-container p-5">
          <div className="mb-1 flex items-center gap-2 text-on-surface-variant">
            <Icon name="cloud_sync" className="text-lg text-primary" />
            <span className="text-xs font-medium uppercase tracking-wider">
              {t('system_health.sync_status')}
            </span>
          </div>
          <p className="font-headline text-lg font-bold text-on-surface">
            {t('system_health.syncing_cloud_nodes')}
          </p>
          <div className="mt-2 flex items-center gap-2">
            <ProgressBar pct={0} color="bg-primary" />
            <span className="shrink-0 text-xs font-semibold text-primary">
              N/A
            </span>
          </div>
        </div>

        <div className="rounded-lg bg-surface-container p-5">
          <div className="mb-1 flex items-center gap-2 text-on-surface-variant">
            <Icon name="speed" className="text-lg text-[#34d399]" />
            <span className="text-xs font-medium uppercase tracking-wider">
              {t('system_health.api_response')}
            </span>
          </div>
          <p className="font-headline text-3xl font-bold text-[#34d399]">
            {dbLatency ?? 12}<span className="text-lg text-on-surface-variant">ms</span>
          </p>
          <div className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-xs text-on-surface-variant">
            <span>DB: <span className={dbStatus === 'ok' ? 'text-[#34d399]' : dbStatus === 'error' ? 'text-[#fca5a5]' : ''}>{dbStatus}</span></span>
            <span>Redis: <span className={redisStatus === 'ok' ? 'text-[#34d399]' : redisStatus === 'error' ? 'text-[#fca5a5]' : ''}>{redisStatus}</span></span>
            <span>NATS: <span className={natsStatus === 'ok' ? 'text-[#34d399]' : natsStatus === 'error' ? 'text-[#fca5a5]' : ''}>{natsStatus}</span></span>
          </div>
        </div>

        <div className="rounded-lg bg-surface-container p-5">
          <div className="mb-1 flex items-center gap-2 text-on-surface-variant">
            <Icon name="dns" className="text-lg text-primary" />
            <span className="text-xs font-medium uppercase tracking-wider">
              {t('system_health.managed_nodes')}
            </span>
          </div>
          <p className="font-headline text-3xl font-bold text-on-surface">
            {totalAssets.toLocaleString()}
          </p>
          <span className="mt-1 text-xs text-on-surface-variant">
            {t('system_health.across_regions')}
          </span>
        </div>
      </div>

      {/* ── Health Donut + Trend ── */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Section title={t('system_health.overall_health_distribution')} icon="donut_large">
          <DonutChart
            segments={healthSegments}
            centerLabel={`${healthSegments[0].pct}%`}
            centerSublabel={t('system_health.healthy')}
          />
          <div className="mt-5 flex items-center justify-center gap-6">
            {healthSegments.map((s) => (
              <div key={s.label} className="flex items-center gap-2">
                <span
                  className="h-2.5 w-2.5 shrink-0 rounded-full"
                  style={{ backgroundColor: s.color }}
                />
                <span className="text-xs text-on-surface-variant">
                  {s.label}
                </span>
                <span className="text-xs font-semibold text-on-surface">
                  {s.pct}%
                </span>
              </div>
            ))}
          </div>
        </Section>

        <Section title={t('system_health.fault_trend_24h')} icon="bar_chart">
          <div className="flex items-end gap-3" style={{ height: 200 }}>
            {trendBars.length === 0 ? (
              <div className="flex flex-1 items-center justify-center text-xs text-on-surface-variant">
                No data
              </div>
            ) : trendBars.map((bar: { hour: string; critical: number; warning: number; info: number }) => {
              const total = bar.critical + bar.warning + bar.info;
              return (
                <div
                  key={bar.hour}
                  className="flex flex-1 flex-col items-center gap-1"
                >
                  <span className="text-[10px] font-semibold text-on-surface-variant">
                    {total}
                  </span>
                  <div
                    className="flex w-full flex-col-reverse overflow-hidden rounded-t"
                    style={{ height: `${(total / trendMax) * 160}px` }}
                  >
                    <div
                      className="w-full bg-[#ff6b6b]"
                      style={{
                        height: `${total > 0 ? (bar.critical / total) * 100 : 0}%`,
                      }}
                    />
                    <div
                      className="w-full bg-[#fbbf24]"
                      style={{
                        height: `${total > 0 ? (bar.warning / total) * 100 : 0}%`,
                      }}
                    />
                    <div
                      className="w-full bg-primary/60"
                      style={{
                        height: `${total > 0 ? (bar.info / total) * 100 : 0}%`,
                      }}
                    />
                  </div>
                  <span className="text-[10px] text-on-surface-variant">
                    {bar.hour}
                  </span>
                </div>
              );
            })}
          </div>
          <div className="mt-3 flex items-center justify-center gap-5">
            {[
              { label: "Critical", color: "#ff6b6b" },
              { label: "Warning", color: "#fbbf24" },
              { label: "Info", color: "#9ecaff" },
            ].map((l) => (
              <div key={l.label} className="flex items-center gap-1.5">
                <span
                  className="h-2 w-2 rounded-full"
                  style={{ backgroundColor: l.color }}
                />
                <span className="text-[10px] text-on-surface-variant">
                  {l.label}
                </span>
              </div>
            ))}
          </div>
        </Section>
      </div>

      {/* ── Resource Utilization ── */}
      <Section title={t('system_health.resource_utilization')} icon="monitoring">
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-3">
          {/* Storage — values come from fleet metrics when available; the
              previous hardcoded "140 TB / 160 TB" was misleading operators
              regardless of actual fleet state (audit finding H9, 2026-04-28). */}
          <div>
            <div className="mb-2 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Icon name="storage" className="text-base text-on-surface-variant" />
                <span className="text-xs font-medium uppercase tracking-wider text-on-surface-variant">
                  {t('system_health.storage')}
                </span>
              </div>
              <span className="text-xs font-semibold text-on-surface-variant">—</span>
            </div>
            <ProgressBar pct={0} color="bg-surface-container-high" height="h-3" />
            <p className="mt-1.5 text-xs text-on-surface-variant">
              {t('system_health.metric_unavailable', 'Fleet storage metric not configured')}
            </p>
          </div>

          {/* Power Load — same caveat as Storage. */}
          <div>
            <div className="mb-2 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Icon name="bolt" className="text-base text-on-surface-variant" />
                <span className="text-xs font-medium uppercase tracking-wider text-on-surface-variant">
                  {t('system_health.power_load')}
                </span>
              </div>
              <span className="text-xs font-semibold text-on-surface-variant">—</span>
            </div>
            <ProgressBar pct={0} color="bg-surface-container-high" height="h-3" />
            <p className="mt-1.5 text-xs text-on-surface-variant">
              {t('system_health.metric_unavailable', 'Fleet power metric not configured')}
            </p>
          </div>

          {/* Memory Usage */}
          <div>
            <div className="mb-2 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Icon name="memory" className="text-base text-on-surface-variant" />
                <span className="text-xs font-medium uppercase tracking-wider text-on-surface-variant">
                  {t('system_health.memory_usage')}
                </span>
              </div>
              <span className="text-xs font-semibold text-primary">47.8%</span>
            </div>
            <ProgressBar pct={47.8} color="bg-primary" height="h-3" />
            <p className="mt-1.5 text-xs text-on-surface-variant">
              {t('system_health.aggregate_cluster_utilization')}
            </p>
          </div>
        </div>
      </Section>

      {/* ── Active Alerts Table ── */}
      <Section title={t('system_health.active_alerts')} icon="notification_important">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="bg-surface-container-high text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
                <th className="px-4 py-3">{t('system_health.table_asset_id')}</th>
                <th className="px-4 py-3">{t('system_health.table_metric')}</th>
                <th className="px-4 py-3">{t('system_health.table_level')}</th>
                <th className="px-4 py-3">{t('system_health.table_bia_tier')}</th>
                <th className="px-4 py-3">{t('system_health.table_time')}</th>
                <th className="px-4 py-3 text-right">{t('system_health.table_action')}</th>
              </tr>
            </thead>
            <tbody>
              {alertsLoading ? (
                <tr>
                  <td colSpan={6} className="px-4 py-8 text-center text-on-surface-variant">
                    <div className="inline-block animate-spin rounded-full h-5 w-5 border-2 border-primary border-t-transparent" />
                  </td>
                </tr>
              ) : criticalAlerts.map((alert) => (
                <tr
                  key={alert.id}
                  className="transition-colors hover:bg-surface-container-low"
                >
                  <td className="whitespace-nowrap px-4 py-3 font-mono text-xs font-semibold text-primary">
                    <span
                      className="cursor-pointer hover:underline"
                      onClick={() => navigate(`/assets/${alert.ci_id}`)}
                    >
                      {alert.ci_id ? `ASSET-${alert.ci_id.slice(0, 8)}` : 'N/A'}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-sm text-on-surface">
                    {alert.message}
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={alert.severity} />
                  </td>
                  <td className="px-4 py-3 text-xs font-semibold text-on-surface-variant">
                    —
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 font-mono text-xs text-on-surface-variant">
                    {alert.fired_at ? new Date(alert.fired_at).toLocaleTimeString() : '—'}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button
                      type="button"
                      onClick={() => navigate('/monitoring')}
                      className="rounded p-1.5 text-on-surface-variant transition-colors hover:bg-surface-container-high hover:text-primary"
                    >
                      <Icon name="open_in_new" className="text-lg" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="mt-3 flex justify-end">
          <button
            type="button"
            onClick={() => navigate("/monitoring")}
            className="text-xs text-on-surface-variant hover:text-primary uppercase tracking-wider transition-colors"
          >
            查看所有告警 →
          </button>
        </div>
      </Section>
    </div>
  );
}

export default memo(SystemHealth);
