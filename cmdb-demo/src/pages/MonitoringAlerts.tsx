import { memo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import Icon from "../components/Icon";
import StatusBadge from "../components/StatusBadge";
import { useAlerts, useAcknowledgeAlert, useResolveAlert } from "../hooks/useMonitoring";

// Keep TREND_DATA as static (needs dedicated metrics endpoint)
const TREND_DATA = [
  { hour: "00", value: 8 },
  { hour: "02", value: 5 },
  { hour: "04", value: 3 },
  { hour: "06", value: 12 },
  { hour: "08", value: 22 },
  { hour: "10", value: 18 },
  { hour: "12", value: 31 },
  { hour: "14", value: 42 },
  { hour: "16", value: 28 },
  { hour: "18", value: 15 },
  { hour: "20", value: 9 },
  { hour: "22", value: 6 },
];

const SEVERITY_COLORS: Record<string, string> = {
  CRITICAL: "bg-red-900/50 text-error",
  WARNING: "bg-yellow-900/40 text-[#fbbf24]",
  INFO: "bg-[#1e3a5f] text-primary",
};

/* ──────────────────────────────────────────────
   Components
   ────────────────────────────────────────────── */

function SeverityBadge({ severity }: { severity: string }) {
  return (
    <span
      className={`inline-block rounded px-2.5 py-1 text-[0.6875rem] font-bold uppercase tracking-wider ${SEVERITY_COLORS[severity] ?? "bg-surface-container-highest text-on-surface-variant"}`}
    >
      {severity}
    </span>
  );
}

function SummaryCard({
  label,
  count,
  color,
  icon,
}: {
  label: string;
  count: number;
  color: string;
  icon: string;
}) {
  return (
    <div className="flex items-center gap-3 rounded-lg bg-surface-container p-4">
      <Icon name={icon} className={`text-2xl ${color}`} />
      <div>
        <p className="text-xs font-medium uppercase tracking-wider text-on-surface-variant">
          {label}
        </p>
        <p className={`font-headline text-2xl font-bold ${color}`}>{count}</p>
      </div>
    </div>
  );
}

/* ──────────────────────────────────────────────
   Main Page
   ────────────────────────────────────────────── */

function MonitoringAlerts() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [search, setSearch] = useState("");
  const [severity, setSeverity] = useState("all");
  const [currentPage, setCurrentPage] = useState(1);

  const filterParams: Record<string, string> = {};
  if (severity !== "all") filterParams.severity = severity;
  const { data: alertsResponse, isLoading, error } = useAlerts(filterParams);
  const acknowledgeAlert = useAcknowledgeAlert();
  const resolveAlert = useResolveAlert();
  const alerts = alertsResponse?.data ?? [];

  const filtered = alerts.filter((a) => {
    const matchSearch =
      search === "" ||
      a.message.toLowerCase().includes(search.toLowerCase());
    return matchSearch;
  });

  const maxBar = Math.max(...TREND_DATA.map((d) => d.value));

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-primary border-t-transparent" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center py-20 gap-3">
        <span className="material-symbols-outlined text-error text-4xl">error</span>
        <p className="text-error text-sm">Failed to load alerts</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen space-y-6 bg-surface px-6 py-5 font-body text-on-surface">
      {/* ── Header ── */}
      <div>
        <h1 className="font-headline text-2xl font-bold text-on-surface">
          {t('monitoring.title')}
        </h1>
        <p className="mt-1 flex items-center gap-2 text-sm font-semibold tracking-wide text-[#fbbf24]">
          <Icon name="bolt" className="text-base text-[#fbbf24]" />
          {t('monitoring.system_health_degraded')}
        </p>
      </div>

      {/* ── Filters ── */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative flex-1 min-w-[200px]">
          <Icon
            name="search"
            className="absolute left-3 top-1/2 -translate-y-1/2 text-lg text-on-surface-variant"
          />
          <input
            type="text"
            placeholder={t('monitoring.search_placeholder')}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full rounded-lg bg-surface-container py-2.5 pl-10 pr-4 text-sm text-on-surface placeholder-on-surface-variant/50 outline-none focus:ring-1 focus:ring-primary/40"
          />
        </div>

        <select
          value={severity}
          onChange={(e) => setSeverity(e.target.value)}
          className="rounded-lg bg-surface-container px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40"
        >
          <option value="all">{t('monitoring.all_severities')}</option>
          <option value="critical">{t('common.critical')}</option>
          <option value="warning">{t('common.warning')}</option>
          <option value="info">{t('common.info')}</option>
        </select>

        <input
          type="date"
          className="rounded-lg bg-surface-container px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40"
          defaultValue="2023-10-25"
        />

        <select className="rounded-lg bg-surface-container px-4 py-2.5 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary/40">
          <option>{t('monitoring.all_locations')}</option>
          <option>DC-1 Shanghai</option>
          <option>DC-2 Beijing</option>
          <option>DC-3 Shenzhen</option>
        </select>

        <div className="flex items-center gap-2 ml-auto">
          <button
            type="button"
            onClick={() => navigate('/monitoring/topology')}
            className="flex items-center gap-2 rounded-lg bg-surface-container-high px-4 py-2.5 text-sm font-semibold text-on-surface transition-colors hover:bg-surface-container-highest"
          >
            <Icon name="account_tree" className="text-base" />
            拓撲分析
          </button>
          <button
            type="button"
            className="flex items-center gap-2 rounded-lg bg-primary px-4 py-2.5 text-sm font-semibold text-on-primary-container transition-colors hover:brightness-110"
          >
            <Icon name="download" className="text-base" />
            {t('monitoring.export_report')}
          </button>
          <button
            type="button"
            className="flex items-center gap-2 rounded-lg bg-transparent px-4 py-2.5 text-sm font-semibold text-error ring-1 ring-error/40 transition-colors hover:bg-error/10"
          >
            <Icon name="notifications_off" className="text-base" />
            {t('monitoring.silence_management')}
          </button>
        </div>
      </div>

      {/* ── Summary Cards ── */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <SummaryCard label={t('common.critical')} count={alerts.filter(a => a.severity?.toLowerCase() === 'critical').length} color="text-error" icon="error" />
        <SummaryCard
          label={t('common.warning')}
          count={alerts.filter(a => a.severity?.toLowerCase() === 'warning').length}
          color="text-[#fbbf24]"
          icon="warning"
        />
        <SummaryCard label={t('common.info')} count={alerts.filter(a => a.severity?.toLowerCase() === 'info').length} color="text-primary" icon="info" />
      </div>

      {/* ── Alert Table ── */}
      <div className="overflow-x-auto rounded-lg bg-surface-container">
        <table className="w-full text-left text-sm">
          <thead>
            <tr className="bg-surface-container-high text-xs font-semibold uppercase tracking-wider text-on-surface-variant">
              <th className="px-5 py-3">
                <div className="flex items-center gap-1.5">
                  <Icon name="schedule" className="text-sm" />
                  {t('monitoring.table_timestamp')}
                </div>
              </th>
              <th className="px-5 py-3">{t('monitoring.table_severity')}</th>
              <th className="px-5 py-3">{t('monitoring.table_description')}</th>
              <th className="px-5 py-3">{t('monitoring.table_status')}</th>
              <th className="px-5 py-3 text-right">{t('monitoring.table_action')}</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((alert) => (
              <tr
                key={alert.id}
                onClick={() => navigate('/monitoring/topology')}
                className="transition-colors hover:bg-surface-container-low cursor-pointer"
              >
                <td className="whitespace-nowrap px-5 py-3.5 font-mono text-xs text-on-surface-variant">
                  {alert.fired_at ? new Date(alert.fired_at).toLocaleString() : '—'}
                </td>
                <td className="px-5 py-3.5">
                  <SeverityBadge severity={(alert.severity ?? '').toUpperCase()} />
                </td>
                <td className="px-5 py-3.5">
                  <p className="text-sm text-on-surface">{alert.message}</p>
                  {alert.ci_id && (
                    <p className="mt-0.5 text-xs text-on-surface-variant">
                      Asset: <span
                        className="cursor-pointer text-primary hover:underline"
                        onClick={(e) => { e.stopPropagation(); navigate('/assets/detail'); }}
                      >{alert.ci_id}</span>
                    </p>
                  )}
                </td>
                <td className="px-5 py-3.5">
                  <StatusBadge status={alert.status} />
                </td>
                <td className="px-5 py-3.5 text-right">
                  <div className="flex items-center justify-end gap-1">
                    <button
                      type="button"
                      onClick={(e) => { e.stopPropagation(); acknowledgeAlert.mutate(alert.id); }}
                      className="rounded p-1.5 text-on-surface-variant transition-colors hover:bg-surface-container-high hover:text-primary"
                      title={t('monitoring.acknowledge')}
                    >
                      <Icon name="check_circle" className="text-lg" />
                    </button>
                    <button
                      type="button"
                      onClick={(e) => { e.stopPropagation(); resolveAlert.mutate(alert.id); }}
                      className="rounded p-1.5 text-on-surface-variant transition-colors hover:bg-surface-container-high hover:text-primary"
                      title={t('monitoring.details')}
                    >
                      <Icon name="open_in_new" className="text-lg" />
                    </button>
                    <button
                      type="button"
                      className="rounded p-1.5 text-on-surface-variant transition-colors hover:bg-surface-container-high hover:text-error"
                      title={t('monitoring.silence')}
                    >
                      <Icon name="notifications_off" className="text-lg" />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* ── Pagination ── */}
      <div className="flex items-center justify-between text-xs text-on-surface-variant">
        <p>
          {t('monitoring.pagination_showing', { count: filtered.length, total: alerts.length })}
        </p>
        <div className="flex items-center gap-1">
          <button
            type="button"
            disabled={currentPage === 1}
            onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
            className="rounded p-1.5 transition-colors hover:bg-surface-container-high disabled:opacity-30"
          >
            <Icon name="chevron_left" className="text-lg" />
          </button>
          {[1, 2, 3, 4, 5].map((p) => (
            <button
              key={p}
              type="button"
              onClick={() => setCurrentPage(p)}
              className={`h-8 w-8 rounded text-sm font-semibold transition-colors ${
                currentPage === p
                  ? "bg-primary text-on-primary-container"
                  : "hover:bg-surface-container-high"
              }`}
            >
              {p}
            </button>
          ))}
          <button
            type="button"
            onClick={() => setCurrentPage((p) => Math.min(5, p + 1))}
            className="rounded p-1.5 transition-colors hover:bg-surface-container-high"
          >
            <Icon name="chevron_right" className="text-lg" />
          </button>
        </div>
      </div>

      {/* ── AIOps Section ── */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        {/* AI Recommendation */}
        <div className="rounded-lg bg-surface-container p-5">
          <div className="mb-4 flex items-center gap-2">
            <Icon name="psychology" className="text-xl text-primary" />
            <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
              {t('monitoring.aiops_analysis')}
            </h3>
          </div>
          <div className="space-y-3 rounded-lg bg-surface-container-low p-4">
            <div className="flex items-start gap-3">
              <Icon name="auto_awesome" className="mt-0.5 text-lg text-primary" />
              <div>
                <p className="text-sm font-semibold text-on-surface">
                  {t('monitoring.anomaly_correlation_detected')}
                </p>
                <p className="mt-1 text-xs leading-relaxed text-on-surface-variant">
                  Temperature spike in Rack A02 correlates with increased CPU load
                  on SRV-PROD-001 (r=0.94). Recommend activating supplemental
                  cooling unit HVAC-AUX-03 and redistributing workload to standby
                  nodes SRV-PROD-008/009.
                </p>
              </div>
            </div>
            <div className="flex items-start gap-3">
              <Icon name="auto_awesome" className="mt-0.5 text-lg text-[#fbbf24]" />
              <div>
                <p className="text-sm font-semibold text-on-surface">
                  {t('monitoring.predictive_failure_warning')}
                </p>
                <p className="mt-1 text-xs leading-relaxed text-on-surface-variant">
                  Storage Cluster B Disk Array 4 shows SMART degradation pattern
                  consistent with imminent multi-disk failure (confidence: 87%).
                  Estimated time to failure: 6-18 hours. Initiate proactive data
                  migration to Array 5 immediately.
                </p>
              </div>
            </div>
            <div className="flex items-start gap-3">
              <Icon name="auto_awesome" className="mt-0.5 text-lg text-error" />
              <div>
                <p className="text-sm font-semibold text-on-surface">
                  {t('monitoring.security_alert_analysis')}
                </p>
                <p className="mt-1 text-xs leading-relaxed text-on-surface-variant">
                  Unusual login pattern on Global Admin account matches known
                  credential stuffing signature. Source IP 203.0.113.42 flagged in
                  3 threat intelligence feeds. Recommend immediate password rotation
                  and MFA re-enrollment.
                </p>
              </div>
            </div>
          </div>
        </div>

        {/* 24-hour Trend Chart */}
        <div className="rounded-lg bg-surface-container p-5">
          <div className="mb-4 flex items-center gap-2">
            <Icon name="trending_up" className="text-xl text-primary" />
            <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
              {t('monitoring.alert_trend_24h')}
            </h3>
          </div>
          <div className="flex items-end gap-2" style={{ height: 180 }}>
            {TREND_DATA.map((d) => (
              <div key={d.hour} className="flex flex-1 flex-col items-center gap-1">
                <span className="text-[10px] font-semibold text-on-surface-variant">
                  {d.value}
                </span>
                <div
                  className="w-full rounded-t bg-primary/80 transition-all duration-300 hover:bg-primary"
                  style={{
                    height: `${(d.value / maxBar) * 140}px`,
                  }}
                />
                <span className="text-[10px] text-on-surface-variant">
                  {d.hour}
                </span>
              </div>
            ))}
          </div>
          <p className="mt-3 text-center text-[10px] uppercase tracking-widest text-on-surface-variant">
            {t('monitoring.hour_utc8')}
          </p>
        </div>
      </div>
    </div>
  );
}

export default memo(MonitoringAlerts);
