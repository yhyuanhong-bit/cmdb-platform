import { useMemo, memo } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from 'react-i18next';
import { useLocationContext } from '../contexts/LocationContext';
import LocationBreadcrumb from '../components/LocationBreadcrumb';
import { useDashboardStats, useRackStats, useLifecycleStats } from '../hooks/useDashboard';
import { useAlerts } from '../hooks/useMonitoring';
import type { AlertEvent } from '../lib/api/monitoring';
import { useBIAStats } from '../hooks/useBIA';

/* ──────────────────────────────────────────────
   Mock data
   ────────────────────────────────────────────── */

const BIA_SEGMENTS = [
  { labelKey: "common.critical", pct: 28, color: "#ff6b6b" },
  { labelKey: "common.important", pct: 24, color: "#ffa94d" },
  { labelKey: "common.normal", pct: 32, color: "#9ecaff" },
  { labelKey: "common.minor", pct: 16, color: "#69db7c" },
];

const HEATMAP_ROWS = 6;
const HEATMAP_COLS = 12;
const HEATMAP_SHADES = [
  "bg-[#0c2d3f]",
  "bg-[#0f3b52]",
  "bg-[#134a66]",
  "bg-[#17597a]",
  "bg-[#1c6f96]",
  "bg-[#2196c8]",
  "bg-[#3bb5e0]",
  "bg-[#6cd4f0]",
];
// Seeded "random" occupancy grid
const heatmapData: number[][] = Array.from({ length: HEATMAP_ROWS }, (_, r) =>
  Array.from({ length: HEATMAP_COLS }, (_, c) => {
    const v = ((r * 7 + c * 13 + r * c * 3) % HEATMAP_SHADES.length);
    return v;
  }),
);

const CRITICAL_EVENTS = [
  {
    id: "SRV-CN-0442",
    message: "Temp Exhaust > 42°C",
    severity: "CRITICAL",
    priority: "HIGH",
  },
  {
    id: "NET-SW-B8T",
    message: "Packet Loss 0.4%",
    severity: "WARNING",
    priority: "MEDIUM",
  },
  {
    id: "UPS-BAT-02",
    message: "Capacity < 15%",
    severity: "CRITICAL",
    priority: "MEDIUM-OFF",
  },
];

/* ──────────────────────────────────────────────
   Small reusable pieces
   ────────────────────────────────────────────── */

function Icon({ name, className = "" }: { name: string; className?: string }) {
  return (
    <span className={`material-symbols-outlined ${className}`}>{name}</span>
  );
}

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

/* ──────────────────────────────────────────────
   CSS-only donut chart
   ────────────────────────────────────────────── */

function DonutChart({
  segments,
}: {
  segments: { labelKey: string; pct: number; color: string }[];
}) {
  const { t } = useTranslation();
  // Build conic-gradient stops
  let cumulative = 0;
  const stops = segments.flatMap((s) => {
    const start = cumulative;
    cumulative += s.pct;
    return [`${s.color} ${start}%`, `${s.color} ${cumulative}%`];
  });

  return (
    <div
      className="relative mx-auto h-44 w-44 rounded-full"
      style={{
        background: `conic-gradient(${stops.join(", ")})`,
      }}
    >
      {/* Inner cut-out */}
      <div className="absolute inset-5 flex flex-col items-center justify-center rounded-full bg-surface-container">
        <span className="font-headline text-lg font-bold text-on-surface">
          100%
        </span>
        <span className="text-[11px] uppercase tracking-wider text-on-surface-variant">
          {t('dashboard.compliance')}
        </span>
      </div>
    </div>
  );
}

/* ──────────────────────────────────────────────
   Section wrapper
   ────────────────────────────────────────────── */

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
        <Icon name={icon} className="text-primary text-xl" />
        <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
          {title}
        </h3>
      </div>
      {children}
    </div>
  );
}

/* ──────────────────────────────────────────────
   Main Dashboard
   ────────────────────────────────────────────── */

function Dashboard() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { path } = useLocationContext();

  // Fetch real dashboard stats from API
  const statsQuery = useDashboardStats();
  const stats = statsQuery.data?.data;

  // Fetch rack stats (occupancy_pct, total_u, used_u)
  const { data: rackStatsData } = useRackStats();

  // Fetch asset lifecycle stats (by_status breakdown)
  const { data: lifecycleData } = useLifecycleStats();

  // Fetch real critical alerts for the events table
  const { data: alertsResponse } = useAlerts({ severity: 'critical' });
  const criticalAlerts = alertsResponse?.data ?? [];

  // Fetch BIA stats for compliance card and distribution chart
  const { data: biaResp } = useBIAStats();
  const biaStats = (biaResp as any)?.data;

  // Derive BIA distribution from aggregated /bia/stats endpoint (by_tier counts)
  // instead of fetching all assets and counting client-side.
  const biaDerived = useMemo(() => {
    const byTier = biaStats?.by_tier;
    if (!byTier) return BIA_SEGMENTS;
    const critical = byTier.critical ?? 0;
    const important = byTier.important ?? 0;
    const normal = byTier.normal ?? 0;
    const minor = byTier.minor ?? 0;
    const total = critical + important + normal + minor || 1;
    const critPct = Math.round((critical / total) * 100);
    const impPct = Math.round((important / total) * 100);
    const normPct = Math.round((normal / total) * 100);
    const minPct = 100 - critPct - impPct - normPct; // remainder to ensure sum = 100
    return [
      { labelKey: "common.critical", pct: critPct, color: "#ff6b6b" },
      { labelKey: "common.important", pct: impPct, color: "#ffa94d" },
      { labelKey: "common.normal", pct: normPct, color: "#9ecaff" },
      { labelKey: "common.minor", pct: minPct, color: "#69db7c" },
    ];
  }, [biaStats]);

  // Compute lifecycle financial breakdown from real API data
  const lifecycleBreakdown = useMemo(() => {
    const byStatus = lifecycleData?.data?.by_status ?? (lifecycleData as any)?.by_status;
    if (!byStatus) return null;
    const operational = byStatus.operational ?? 0;
    const maintenance = byStatus.maintenance ?? 0;
    const retired = byStatus.retired ?? 0;
    const disposed = byStatus.disposed ?? 0;
    const total = operational + maintenance + retired + disposed || 1;
    return {
      inUse: Math.round((operational / total) * 100),
      broken: Math.round((maintenance / total) * 100),
      disposed: Math.round(((retired + disposed) / total) * 100),
    };
  }, [lifecycleData]);

  // Display data derived from API stats (or fallback zeros while loading)
  const displayData = useMemo(() => ({
    assets: stats?.total_assets ?? 0,
    racks: stats?.total_racks ?? 0,
    criticalAlerts: stats?.critical_alerts ?? 0,
    activeOrders: stats?.active_orders ?? 0,
    occupancy: rackStatsData?.occupancy_pct ?? 0,
  }), [stats, rackStatsData]);

  return (
    <div className="min-h-screen space-y-6 bg-surface px-6 py-5">
      {/* ── Header ── */}
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div>
          <LocationBreadcrumb />
          <h1 className="font-headline text-2xl font-bold text-on-surface mt-2">
            {t('dashboard.title')}
          </h1>
        </div>
      </div>

      {/* Loading / Error states */}
      {statsQuery.isLoading && (
        <div className="flex items-center justify-center py-10">
          <div className="animate-spin rounded-full h-8 w-8 border-2 border-sky-400 border-t-transparent" />
        </div>
      )}
      {statsQuery.error && (
        <div className="rounded-lg bg-red-900/20 p-4 text-red-300 text-sm">
          Failed to load dashboard stats.{' '}
          <button onClick={() => statsQuery.refetch()} className="underline">Retry</button>
        </div>
      )}

      {/* ── Stat cards row ── */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {/* Total Assets */}
        <div onClick={() => navigate('/assets')} className="rounded-lg bg-surface-container p-5 cursor-pointer hover:bg-surface-container-high transition-colors">
          <div className="mb-1 flex items-center gap-2 text-on-surface-variant">
            <Icon name="dns" className="text-lg" />
            <span className="text-xs font-medium uppercase tracking-wider">
              {t('dashboard.total_assets')}
            </span>
          </div>
          <p className="font-headline text-3xl font-bold text-on-surface">
            {displayData.assets.toLocaleString()}
          </p>
          <span className="mt-1 inline-flex items-center gap-1 text-xs font-semibold text-green-400">
            <Icon name="trending_up" className="text-sm" />
            ▲ 12% {t('dashboard.vs_last_month')}
          </span>
        </div>

        {/* Rack Occupancy */}
        <div onClick={() => navigate('/racks')} className="rounded-lg bg-surface-container p-5 cursor-pointer hover:bg-surface-container-high transition-colors">
          <div className="mb-1 flex items-center gap-2 text-on-surface-variant">
            <Icon name="view_comfy_alt" className="text-lg" />
            <span className="text-xs font-medium uppercase tracking-wider">
              {t('dashboard.rack_occupancy')}
            </span>
          </div>
          <p className="font-headline text-3xl font-bold text-on-surface">
            {displayData.occupancy}%
          </p>
          <div className="mt-2">
            <ProgressBar pct={displayData.occupancy} />
          </div>
        </div>

        {/* Critical Alarms */}
        <div onClick={() => navigate('/monitoring')} className="rounded-lg bg-surface-container p-5 cursor-pointer hover:bg-surface-container-high transition-colors">
          <div className="mb-1 flex items-center gap-2 text-on-surface-variant">
            <Icon name="warning" className="text-lg" />
            <span className="text-xs font-medium uppercase tracking-wider">
              {t('dashboard.critical_alarms')}
            </span>
          </div>
          <p className="font-headline text-3xl font-bold text-error">{String(displayData.criticalAlerts).padStart(2, '0')}</p>
          <span className="mt-1 inline-block rounded bg-red-900/50 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-error">
            {t('dashboard.requires_intervention')}
          </span>
        </div>

        {/* Active Inventory Task */}
        <div onClick={() => navigate('/inventory')} className="rounded-lg bg-surface-container p-5 cursor-pointer hover:bg-surface-container-high transition-colors">
          <div className="mb-1 flex items-center gap-2 text-on-surface-variant">
            <Icon name="assignment" className="text-lg" />
            <span className="text-xs font-medium uppercase tracking-wider">
              {t('dashboard.active_inventory_task')}
            </span>
          </div>
          <p className="font-headline text-lg font-bold text-on-surface">
            Mar-24 Audit
          </p>
          <div className="mt-2 flex items-center gap-2">
            <ProgressBar pct={76} color="bg-[#9ecaff]" />
            <span className="shrink-0 text-xs font-semibold text-primary">
              76%
            </span>
          </div>
        </div>
      </div>

      {/* ── Second row: BIA + Heatmap ── */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-5">
        {/* BIA Distribution */}
        <Section
          title={t('dashboard.bia_distribution')}
          icon="donut_large"
          className="lg:col-span-2"
        >
          <DonutChart segments={biaDerived} />
          <div className="mt-4 grid grid-cols-2 gap-x-4 gap-y-2">
            {biaDerived.map((s) => (
              <div key={s.labelKey} className="flex items-center gap-2">
                <span
                  className="h-2.5 w-2.5 shrink-0 rounded-full"
                  style={{ backgroundColor: s.color }}
                />
                <span className="text-xs text-on-surface-variant">
                  {t(s.labelKey)}
                </span>
                <span className="ml-auto text-xs font-semibold text-on-surface">
                  {s.pct}%
                </span>
              </div>
            ))}
          </div>
        </Section>

        {/* Rack Utilization Heatmap */}
        <Section
          title={t('dashboard.rack_utilization_heatmap')}
          icon="grid_view"
          className="lg:col-span-3"
        >
          {/* Legend — top-right */}
          <div className="mb-3 flex items-center justify-end gap-4 text-[10px] uppercase tracking-wider text-on-surface-variant">
            <span className="flex items-center gap-1.5">
              EMPTY
              <span className="inline-block h-2.5 w-2.5 rounded-sm bg-[#0c2d3f]" />
            </span>
            <span className="flex items-center gap-1.5">
              LOW
              <span className="inline-block h-2.5 w-2.5 rounded-sm bg-[#17597a]" />
            </span>
            <span className="flex items-center gap-1.5">
              HIGH
              <span className="inline-block h-2.5 w-2.5 rounded-sm bg-[#6cd4f0]" />
            </span>
          </div>

          {/* Column labels */}
          <div className="mb-1 grid gap-1" style={{ gridTemplateColumns: `2rem repeat(${HEATMAP_COLS}, 1fr)` }}>
            <span />
            {Array.from({ length: HEATMAP_COLS }, (_, i) => (
              <span
                key={i}
                className="text-center text-[10px] text-on-surface-variant"
              >
                {String(i + 1).padStart(2, "0")}
              </span>
            ))}
          </div>

          {/* Grid */}
          <div className="space-y-1">
            {heatmapData.map((row, ri) => (
              <div
                key={ri}
                className="grid gap-1"
                style={{ gridTemplateColumns: `2rem repeat(${HEATMAP_COLS}, 1fr)` }}
              >
                <span className="flex items-center text-[10px] text-on-surface-variant">
                  {String.fromCharCode(65 + ri)}
                </span>
                {row.map((shade, ci) => (
                  <div
                    key={ci}
                    className={`aspect-square rounded-sm ${HEATMAP_SHADES[shade]} transition-colors hover:brightness-125`}
                    title={`${t('dashboard.row')} ${String.fromCharCode(65 + ri)} ${t('dashboard.rack')} ${ci + 1} — ${Math.round((shade / (HEATMAP_SHADES.length - 1)) * 100)}%`}
                  />
                ))}
              </div>
            ))}
          </div>
        </Section>
      </div>

      {/* ── BIA Compliance Card ── */}
      <div className="rounded-lg bg-surface-container p-5">
        <div className="mb-3 flex items-center gap-2">
          <span className="material-symbols-outlined text-primary text-xl">assessment</span>
          <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
            {t('dashboard.bia_compliance')}
          </h3>
        </div>
        <div className="grid grid-cols-3 gap-3 text-center">
          <div>
            <p className="font-headline text-2xl font-bold text-on-surface">{biaStats?.total ?? 0}</p>
            <p className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant">{t('dashboard.bia_systems')}</p>
          </div>
          <div>
            <p className="font-headline text-2xl font-bold text-[#34d399]">{biaStats?.avg_compliance?.toFixed(1) ?? 0}%</p>
            <p className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant">{t('dashboard.compliance')}</p>
          </div>
          <div>
            <p className="font-headline text-2xl font-bold text-error">{biaStats?.by_tier?.critical ?? 0}</p>
            <p className="text-[0.6875rem] uppercase tracking-wider text-on-surface-variant">{t('dashboard.bia_critical')}</p>
          </div>
        </div>
        <button onClick={() => navigate('/bia')} className="mt-3 w-full rounded-lg bg-surface-container-high py-2 text-xs font-semibold uppercase tracking-wider text-on-surface-variant hover:text-on-surface transition-colors">
          {t('dashboard.view_bia_modeler')} →
        </button>
      </div>

      {/* ── Third row: (Lifecycle + Task) left | Critical Events right ── */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-5">
        {/* Left column — stacked */}
        <div className="lg:col-span-2 flex flex-col gap-4">
          {/* Asset Lifecycle (Financial) */}
          <Section title={t('dashboard.asset_lifecycle_financial')} icon="account_balance">
            <div className="space-y-4">
              {[
                { label: t('dashboard.in_use_assets'), pct: lifecycleBreakdown?.inUse ?? 82, color: "bg-[#9ecaff]" },
                {
                  label: t('dashboard.broken_pending_repair'),
                  pct: lifecycleBreakdown?.broken ?? 13,
                  color: "bg-[#ffa94d]",
                },
                { label: t('dashboard.disposed_eol'), pct: lifecycleBreakdown?.disposed ?? 5, color: "bg-[#ff6b6b]" },
              ].map((item) => (
                <div key={item.label}>
                  <div className="mb-1.5 flex items-center justify-between">
                    <span className="text-xs text-on-surface-variant">
                      {item.label}
                    </span>
                    <span className="text-xs font-semibold text-on-surface">
                      {item.pct}%
                    </span>
                  </div>
                  <ProgressBar pct={item.pct} color={item.color} height="h-3" />
                </div>
              ))}
            </div>
          </Section>

          {/* Current Task Progress */}
          <Section title={t('dashboard.current_task_progress')} icon="task_alt">
            <div className="flex flex-col items-center">
              {/* Circular progress */}
              <div className="relative mb-4 h-36 w-36">
                <svg viewBox="0 0 120 120" className="h-full w-full -rotate-90">
                  <circle
                    cx="60"
                    cy="60"
                    r="52"
                    fill="none"
                    stroke="#121d23"
                    strokeWidth="10"
                  />
                  <circle
                    cx="60"
                    cy="60"
                    r="52"
                    fill="none"
                    stroke="#9ecaff"
                    strokeWidth="10"
                    strokeLinecap="round"
                    strokeDasharray={`${2 * Math.PI * 52}`}
                    strokeDashoffset={`${2 * Math.PI * 52 * (1 - 0.76)}`}
                    className="transition-all duration-700"
                  />
                </svg>
                <div className="absolute inset-0 flex flex-col items-center justify-center">
                  <span className="font-headline text-2xl font-bold text-on-surface">
                    76%
                  </span>
                  <span className="text-[10px] uppercase tracking-wider text-on-surface-variant">
                    {t('dashboard.complete')}
                  </span>
                </div>
              </div>

              <p className="mb-1 text-sm font-semibold text-on-surface">
                Mar-24 Audit
              </p>
              <p className="mb-4 text-xs text-on-surface-variant">
                9,760 / 12,842 {t('dashboard.assets_verified')}
              </p>

              <button
                type="button"
                onClick={() => navigate('/inventory')}
                className="flex items-center gap-2 rounded-md bg-primary px-5 py-2 text-xs font-bold uppercase tracking-wider text-on-primary-container transition-colors hover:brightness-110"
              >
                <Icon name="sync" className="text-base" />
                {t('dashboard.sync_now')}
              </button>
            </div>
          </Section>
        </div>

        {/* Right column — Critical Events (table layout) */}
        <div className="lg:col-span-3">
          <div className="rounded-lg bg-surface-container p-5 h-full">
            <div className="mb-4 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Icon name="emergency" className="text-primary text-xl" />
                <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
                  {t('dashboard.critical_events')}
                </h3>
              </div>
              <div className="flex items-center gap-2 text-[10px] uppercase tracking-wider text-on-surface-variant">
                <span className="relative flex h-2 w-2">
                  <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-red-500 opacity-75" />
                  <span className="relative inline-flex h-2 w-2 rounded-full bg-red-500" />
                </span>
                {t('dashboard.live_telemetry')}
              </div>
            </div>

            {/* Table header */}
            <div className="mb-2 grid grid-cols-[1fr_2fr_auto_auto] gap-3 px-3 text-[10px] font-semibold uppercase tracking-wider text-on-surface-variant">
              <span>{t('dashboard.table_asset_id')}</span>
              <span>{t('common.description')}</span>
              <span>{t('common.severity')}</span>
              <span>{t('common.priority')}</span>
            </div>

            {/* Table rows */}
            <div className="space-y-1">
              {criticalAlerts.length === 0 ? (
                <div className="rounded-md bg-surface-container-low px-3 py-4 text-center text-xs text-on-surface-variant">
                  {t('dashboard.no_critical_events')}
                </div>
              ) : criticalAlerts.slice(0, 8).map((evt: AlertEvent) => (
                <div
                  key={evt.id}
                  onClick={() => navigate('/monitoring')}
                  className="grid grid-cols-[1fr_2fr_auto_auto] items-center gap-3 rounded-md bg-surface-container-low px-3 py-2.5 cursor-pointer hover:bg-surface-container-high transition-colors"
                >
                  <span className="text-sm font-semibold text-on-surface">
                    {evt.ci_id ? `ASSET-${evt.ci_id.slice(0, 8)}` : 'N/A'}
                  </span>
                  <span className="text-xs text-on-surface-variant truncate">
                    {evt.message}
                  </span>
                  <span
                    className={`rounded px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wide ${
                      evt.severity === "critical"
                        ? "bg-red-900/50 text-error"
                        : "bg-yellow-900/40 text-[#ffa94d]"
                    }`}
                  >
                    {(evt.severity ?? '').toUpperCase()}
                  </span>
                  <span className="rounded bg-surface-container px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-on-surface-variant">
                    {evt.fired_at ? new Date(evt.fired_at).toLocaleTimeString() : '—'}
                  </span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* ── View All Monitoring + Predictive link ── */}
      <div className="flex justify-end gap-4">
        <button
          type="button"
          onClick={() => navigate('/predictive')}
          className="text-xs text-on-surface-variant hover:text-primary uppercase tracking-wider transition-colors"
        >
          {t('dashboard.predictive_analysis')} →
        </button>
        <button
          type="button"
          onClick={() => navigate('/monitoring')}
          className="text-xs text-on-surface-variant hover:text-primary uppercase tracking-wider transition-colors"
        >
          {t('dashboard.view_all_monitoring')} →
        </button>
      </div>
    </div>
  );
}

export default memo(Dashboard);
