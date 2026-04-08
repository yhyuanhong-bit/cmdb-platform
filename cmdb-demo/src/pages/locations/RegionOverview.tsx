import { memo, useMemo } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useRootLocations, useLocationChildren, useLocationStats } from '../../hooks/useTopology';
import type { Location } from '../../lib/api/topology';

/* ──────────────────────────────────────────────
   Helpers: map Location to region display data
   ────────────────────────────────────────────── */

interface RegionDisplay {
  slug: string;
  nameCn: string;
  nameEn: string;
  idcCount: number;
  racks: number;
  assets: number;
  pue: number;
  occupancy: number;
  alerts: number;
}

function locationToRegion(loc: Location): RegionDisplay {
  return {
    slug: loc.slug,
    nameCn: loc.name,
    nameEn: loc.name_en || loc.name,
    idcCount: (loc.metadata?.idc_count as number) ?? 0,
    racks: (loc.metadata?.racks as number) ?? 0,
    assets: (loc.metadata?.assets as number) ?? 0,
    pue: (loc.metadata?.pue as number) ?? 0,
    occupancy: (loc.metadata?.occupancy as number) ?? 0,
    alerts: (loc.metadata?.alerts as number) ?? 0,
  };
}

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
        style={{ width: `${Math.min(pct, 100)}%` }}
      />
    </div>
  );
}

function KpiCard({
  icon,
  label,
  value,
  alert,
}: {
  icon: string;
  label: string;
  value: string | number;
  alert?: boolean;
}) {
  return (
    <div className="flex-1 rounded-lg bg-surface-container p-4">
      <div className="mb-2 flex items-center gap-2">
        <Icon
          name={icon}
          className={`text-lg ${alert ? "text-red-400" : "text-primary"}`}
        />
        <span className="text-xs uppercase tracking-wider text-on-surface-variant">
          {label}
        </span>
      </div>
      <span
        className={`font-headline text-2xl font-bold ${alert ? "text-red-400" : "text-on-surface"}`}
      >
        {typeof value === "number" ? value.toLocaleString() : value}
      </span>
    </div>
  );
}

/* ──────────────────────────────────────────────
   Region Card
   ────────────────────────────────────────────── */

const RegionCard = memo(function RegionCard({
  region,
  onClick,
}: {
  region: RegionDisplay;
  onClick: () => void;
}) {
  const pueColor =
    region.pue < 1.3
      ? "text-green-400"
      : region.pue < 1.5
        ? "text-amber-400"
        : "text-red-400";

  return (
    <button
      onClick={onClick}
      className="w-full rounded-lg bg-surface-container p-5 text-left transition-colors hover:bg-surface-container-high"
    >
      <div className="mb-3 flex items-center justify-between">
        <div>
          <h3 className="font-headline text-lg font-bold text-on-surface">
            {region.nameCn}
          </h3>
          <span className="text-xs text-on-surface-variant">
            {region.nameEn}
          </span>
        </div>
        {region.alerts > 0 ? (
          <span className="flex items-center gap-1 rounded-full bg-red-500/15 px-2.5 py-0.5 text-xs font-medium text-red-400">
            <Icon name="warning" className="text-sm" />
            {region.alerts}
          </span>
        ) : (
          <span className="flex items-center gap-1 rounded-full bg-green-500/15 px-2.5 py-0.5 text-xs font-medium text-green-400">
            <Icon name="check_circle" className="text-sm" />
            OK
          </span>
        )}
      </div>

      <div className="mb-4 grid grid-cols-3 gap-3">
        <div>
          <span className="text-[11px] uppercase tracking-wider text-on-surface-variant">
            IDC
          </span>
          <p className="font-headline text-sm font-semibold text-on-surface">
            {region.idcCount}
          </p>
        </div>
        <div>
          <span className="text-[11px] uppercase tracking-wider text-on-surface-variant">
            Racks
          </span>
          <p className="font-headline text-sm font-semibold text-on-surface">
            {region.racks.toLocaleString()}
          </p>
        </div>
        <div>
          <span className="text-[11px] uppercase tracking-wider text-on-surface-variant">
            Assets
          </span>
          <p className="font-headline text-sm font-semibold text-on-surface">
            {region.assets.toLocaleString()}
          </p>
        </div>
      </div>

      <div className="mb-2 flex items-center justify-between text-xs">
        <span className="text-on-surface-variant">Occupancy</span>
        <span className="font-medium text-on-surface">{region.occupancy}%</span>
      </div>
      <ProgressBar pct={region.occupancy} />

      <div className="mt-3 flex items-center justify-between text-xs">
        <span className="text-on-surface-variant">PUE</span>
        <span className={`font-headline font-semibold ${pueColor}`}>
          {region.pue.toFixed(2)}
        </span>
      </div>
    </button>
  );
});

/* ──────────────────────────────────────────────
   Main component
   ────────────────────────────────────────────── */

function RegionOverview() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { countrySlug } = useParams<{ countrySlug: string }>();

  // Find the country from root locations by slug
  const rootQ = useRootLocations();
  const countryLoc = useMemo(
    () => (rootQ.data?.data ?? []).find((l) => l.slug === (countrySlug ?? 'china')),
    [rootQ.data, countrySlug],
  );

  // Fetch children (regions) of the country
  const childrenQ = useLocationChildren(countryLoc?.id ?? '');

  // Fetch aggregate stats for the country (recursive)
  const countryStatsQ = useLocationStats(countryLoc?.id ?? '');
  const cStats = countryStatsQ.data?.data;

  // Map to display format
  const regions: RegionDisplay[] = useMemo(
    () => (childrenQ.data?.data ?? []).map(locationToRegion),
    [childrenQ.data],
  );

  const country = useMemo(() => ({
    nameCn: countryLoc?.name ?? '',
    nameEn: countryLoc?.name_en ?? countryLoc?.name ?? '',
    titleCn: countryLoc?.name ?? '',
    subtitleEn: countryLoc?.name_en ?? '',
    totalIdcs: regions.reduce((s, r) => s + r.idcCount, 0),
    totalRacks: cStats?.total_racks ?? regions.reduce((s, r) => s + r.racks, 0),
    totalAssets: cStats?.total_assets ?? regions.reduce((s, r) => s + r.assets, 0),
    criticalAlerts: cStats?.critical_alerts ?? regions.reduce((s, r) => s + r.alerts, 0),
  }), [countryLoc, regions, cStats]);

  if (rootQ.isLoading || childrenQ.isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-sky-400 border-t-transparent" />
      </div>
    );
  }

  if (rootQ.error || childrenQ.error) {
    return (
      <div className="p-6">
        <div className="rounded-lg bg-red-900/20 p-4 text-red-300 text-sm">
          Failed to load region data.{' '}
          <button onClick={() => { rootQ.refetch(); childrenQ.refetch(); }} className="underline">Retry</button>
        </div>
      </div>
    );
  }

  // Sort regions by occupancy descending for the comparison chart
  const sortedByOccupancy = [...regions].sort(
    (a, b) => b.occupancy - a.occupancy,
  );
  const sortedByPue = [...regions].sort((a, b) => a.pue - b.pue);

  return (
    <div className="min-h-screen bg-surface p-6">
      {/* Breadcrumb */}
      <nav className="mb-4 flex items-center gap-1.5 text-xs">
        <button
          onClick={() => navigate("/locations")}
          className="font-medium uppercase tracking-wider text-primary hover:underline"
        >
          GLOBAL
        </button>
        <Icon name="chevron_right" className="text-sm text-on-surface-variant" />
        <span className="font-medium uppercase tracking-wider text-on-surface">
          {country.nameEn.toUpperCase()}
        </span>
      </nav>

      {/* Title */}
      <div className="mb-6">
        <h1 className="font-headline text-2xl font-bold text-on-surface">
          {country.titleCn}
        </h1>
        <p className="mt-1 text-sm text-on-surface-variant">
          {country.subtitleEn} {t('locations.region_title_suffix')}
        </p>
      </div>

      {/* KPI Bar */}
      <div className="mb-6 flex gap-3">
        <KpiCard
          icon="domain"
          label={t('locations.kpi_total_idcs')}
          value={country.totalIdcs}
        />
        <KpiCard
          icon="dns"
          label={t('locations.kpi_total_racks')}
          value={country.totalRacks}
        />
        <KpiCard
          icon="devices"
          label={t('locations.kpi_total_assets')}
          value={country.totalAssets}
        />
        <KpiCard
          icon="warning"
          label={t('locations.kpi_critical_alerts')}
          value={country.criticalAlerts}
          alert
        />
      </div>

      {/* Two columns */}
      <div className="flex gap-6">
        {/* Left: Region cards 2x2 */}
        <div className="w-3/5">
          <div className="mb-3 flex items-center gap-2">
            <Icon name="map" className="text-lg text-primary" />
            <h2 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
              Regions
            </h2>
          </div>
          <div className="grid grid-cols-2 gap-4">
            {regions.map((region) => (
              <RegionCard
                key={region.slug}
                region={region}
                onClick={() =>
                  navigate(
                    `/locations/${countrySlug ?? "china"}/${region.slug}`,
                  )
                }
              />
            ))}
          </div>
        </div>

        {/* Right: Comparison charts */}
        <div className="w-2/5 space-y-4">
          {/* Occupancy comparison */}
          <div className="rounded-lg bg-surface-container p-5">
            <div className="mb-4 flex items-center gap-2">
              <Icon name="bar_chart" className="text-lg text-primary" />
              <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
                {t('locations.occupancy_comparison')}
              </h3>
            </div>
            <div className="space-y-3">
              {sortedByOccupancy.map((region) => (
                <div key={region.slug}>
                  <div className="mb-1 flex items-center justify-between text-xs">
                    <span className="text-on-surface">
                      {region.nameCn}{" "}
                      <span className="text-on-surface-variant">
                        ({region.nameEn})
                      </span>
                    </span>
                    <span className="font-medium text-on-surface">
                      {region.occupancy}%
                    </span>
                  </div>
                  <div className="h-5 w-full rounded bg-surface-container-low">
                    <div
                      className="flex h-5 items-center justify-end rounded bg-primary/80 pr-2 text-[10px] font-medium text-on-primary-container transition-all duration-500"
                      style={{ width: `${region.occupancy}%` }}
                    >
                      {region.racks.toLocaleString()} racks
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* PUE Ranking */}
          <div className="rounded-lg bg-surface-container p-5">
            <div className="mb-4 flex items-center gap-2">
              <Icon name="bolt" className="text-lg text-primary" />
              <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
                {t('locations.pue_ranking')}
              </h3>
            </div>
            <div className="space-y-2">
              {sortedByPue.map((region, idx) => {
                const pueColor =
                  region.pue < 1.3
                    ? "text-green-400"
                    : region.pue < 1.5
                      ? "text-amber-400"
                      : "text-red-400";
                const medal =
                  idx === 0
                    ? "emoji_events"
                    : idx === 1
                      ? "looks_two"
                      : idx === 2
                        ? "looks_3"
                        : "tag";
                return (
                  <div
                    key={region.slug}
                    className="flex items-center gap-3 rounded-lg bg-surface-container-low p-3"
                  >
                    <span className="flex h-7 w-7 items-center justify-center rounded-full bg-surface-container-high text-xs font-bold text-on-surface-variant">
                      {idx + 1}
                    </span>
                    <Icon
                      name={medal}
                      className={`text-lg ${idx === 0 ? "text-amber-400" : "text-on-surface-variant"}`}
                    />
                    <div className="flex-1">
                      <span className="text-sm font-medium text-on-surface">
                        {region.nameCn}
                      </span>
                      <span className="ml-2 text-xs text-on-surface-variant">
                        {region.nameEn}
                      </span>
                    </div>
                    <span
                      className={`font-headline text-lg font-bold ${pueColor}`}
                    >
                      {region.pue.toFixed(2)}
                    </span>
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default memo(RegionOverview);
