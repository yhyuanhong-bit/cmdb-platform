import { memo, useState, useMemo } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useRootLocations, useLocationChildren, useLocationStats } from '../../hooks/useTopology';
import type { Location } from '../../lib/api/topology';

/* ──────────────────────────────────────────────
   Types & helpers
   ────────────────────────────────────────────── */

interface CityData {
  slug: string;
  nameCn: string;
  nameEn: string;
  campuses: number;
  idcCount: number;
  racks: number;
  pue: number;
  occupancy: number;
  alerts: number;
  sparkline: number[];
  power: number;
  reliability: number;
}

function locationToCity(loc: Location): CityData {
  return {
    slug: loc.slug,
    nameCn: loc.name,
    nameEn: loc.name_en || loc.name,
    campuses: (loc.metadata?.campuses as number) ?? 0,
    idcCount: (loc.metadata?.idc_count as number) ?? 0,
    racks: (loc.metadata?.racks as number) ?? 0,
    pue: (loc.metadata?.pue as number) ?? 0,
    occupancy: (loc.metadata?.occupancy as number) ?? 0,
    alerts: (loc.metadata?.alerts as number) ?? 0,
    sparkline: (loc.metadata?.sparkline as number[]) ?? [],
    power: (loc.metadata?.power as number) ?? 0,
    reliability: (loc.metadata?.reliability as number) ?? 99.9,
  };
}

type SortKey = "name" | "idcCount" | "occupancy" | "alerts";

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
   Sparkline (CSS-based mini bar chart)
   ────────────────────────────────────────────── */

function Sparkline({ data }: { data: number[] }) {
  const max = Math.max(...data);
  return (
    <div className="flex h-8 items-end gap-px">
      {data.map((val, i) => (
        <div
          key={i}
          className="w-2 rounded-t bg-primary/60 transition-all"
          style={{ height: `${(val / max) * 100}%` }}
        />
      ))}
    </div>
  );
}

/* ──────────────────────────────────────────────
   City Card (card view)
   ────────────────────────────────────────────── */

const CityCard = memo(function CityCard({
  city,
  onClick,
}: {
  city: CityData;
  onClick: () => void;
}) {
  const pueColor =
    city.pue < 1.3
      ? "text-green-400"
      : city.pue < 1.5
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
            {city.nameCn}
          </h3>
          <span className="text-xs text-on-surface-variant">
            {city.nameEn}
          </span>
        </div>
        <div className="flex items-center gap-3">
          <Sparkline data={city.sparkline} />
          {city.alerts > 0 ? (
            <span className="flex items-center gap-1 rounded-full bg-red-500/15 px-2.5 py-0.5 text-xs font-medium text-red-400">
              <Icon name="warning" className="text-sm" />
              {city.alerts}
            </span>
          ) : (
            <span className="flex items-center gap-1 rounded-full bg-green-500/15 px-2.5 py-0.5 text-xs font-medium text-green-400">
              <Icon name="check_circle" className="text-sm" />
              OK
            </span>
          )}
        </div>
      </div>

      <div className="mb-4 grid grid-cols-4 gap-3">
        <div>
          <span className="text-[11px] uppercase tracking-wider text-on-surface-variant">
            Campus
          </span>
          <p className="font-headline text-sm font-semibold text-on-surface">
            {city.campuses}
          </p>
        </div>
        <div>
          <span className="text-[11px] uppercase tracking-wider text-on-surface-variant">
            IDC
          </span>
          <p className="font-headline text-sm font-semibold text-on-surface">
            {city.idcCount}
          </p>
        </div>
        <div>
          <span className="text-[11px] uppercase tracking-wider text-on-surface-variant">
            Racks
          </span>
          <p className="font-headline text-sm font-semibold text-on-surface">
            {city.racks.toLocaleString()}
          </p>
        </div>
        <div>
          <span className="text-[11px] uppercase tracking-wider text-on-surface-variant">
            PUE
          </span>
          <p className={`font-headline text-sm font-semibold ${pueColor}`}>
            {city.pue.toFixed(2)}
          </p>
        </div>
      </div>

      <div className="mb-2 flex items-center justify-between text-xs">
        <span className="text-on-surface-variant">Occupancy</span>
        <span className="font-medium text-on-surface">{city.occupancy}%</span>
      </div>
      <ProgressBar pct={city.occupancy} />
    </button>
  );
});

/* ──────────────────────────────────────────────
   City Row (list view)
   ────────────────────────────────────────────── */

const CityRow = memo(function CityRow({
  city,
  onClick,
}: {
  city: CityData;
  onClick: () => void;
}) {
  const pueColor =
    city.pue < 1.3
      ? "text-green-400"
      : city.pue < 1.5
        ? "text-amber-400"
        : "text-red-400";

  return (
    <button
      onClick={onClick}
      className="flex w-full items-center gap-4 rounded-lg bg-surface-container px-5 py-3 text-left transition-colors hover:bg-surface-container-high"
    >
      <div className="w-28">
        <span className="font-headline text-sm font-bold text-on-surface">
          {city.nameCn}
        </span>
        <span className="ml-2 text-xs text-on-surface-variant">
          {city.nameEn}
        </span>
      </div>
      <div className="w-16 text-center text-xs text-on-surface">
        {city.campuses} Campus
      </div>
      <div className="w-14 text-center text-xs text-on-surface">
        {city.idcCount} IDC
      </div>
      <div className="w-20 text-center text-xs text-on-surface">
        {city.racks.toLocaleString()} Racks
      </div>
      <div className={`w-14 text-center text-xs font-semibold ${pueColor}`}>
        {city.pue.toFixed(2)}
      </div>
      <div className="flex-1">
        <ProgressBar pct={city.occupancy} height="h-1.5" />
      </div>
      <span className="w-12 text-right text-xs font-medium text-on-surface">
        {city.occupancy}%
      </span>
      {city.alerts > 0 ? (
        <span className="flex w-14 items-center justify-end gap-1 text-xs font-medium text-red-400">
          <Icon name="warning" className="text-sm" />
          {city.alerts}
        </span>
      ) : (
        <span className="flex w-14 items-center justify-end gap-1 text-xs font-medium text-green-400">
          <Icon name="check_circle" className="text-sm" />
        </span>
      )}
      <Icon
        name="chevron_right"
        className="text-lg text-on-surface-variant"
      />
    </button>
  );
});

/* ──────────────────────────────────────────────
   Comparison bars (radar chart placeholder)
   ────────────────────────────────────────────── */

function ComparisonBars({ cities }: { cities: CityData[] }) {
  const { t } = useTranslation();
  const metrics: { key: keyof CityData; label: string; max: number }[] = [
    { key: "pue", label: "PUE (lower is better)", max: 2 },
    { key: "occupancy", label: t('locations.occupancy_pct'), max: 100 },
    { key: "power", label: t('locations.power_utilization'), max: 100 },
    { key: "reliability", label: t('locations.reliability_pct'), max: 100 },
  ];

  const colors = [
    "bg-primary",
    "bg-amber-400",
    "bg-green-400",
  ];

  return (
    <div className="rounded-lg bg-surface-container p-5">
      <div className="mb-4 flex items-center gap-2">
        <Icon name="analytics" className="text-lg text-primary" />
        <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
          {t('locations.city_comparison')}
        </h3>
      </div>

      {/* Legend */}
      <div className="mb-4 flex gap-4">
        {cities.map((city, i) => (
          <div key={city.slug} className="flex items-center gap-1.5">
            <div className={`h-2.5 w-2.5 rounded-full ${colors[i]}`} />
            <span className="text-xs text-on-surface-variant">
              {city.nameCn} ({city.nameEn})
            </span>
          </div>
        ))}
      </div>

      <div className="space-y-5">
        {metrics.map((metric) => (
          <div key={metric.key}>
            <span className="mb-2 block text-xs text-on-surface-variant">
              {metric.label}
            </span>
            <div className="space-y-1.5">
              {cities.map((city, i) => {
                const raw = city[metric.key] as number;
                const pct =
                  metric.key === "pue"
                    ? ((metric.max - raw) / metric.max) * 100
                    : (raw / metric.max) * 100;
                return (
                  <div key={city.slug} className="flex items-center gap-2">
                    <span className="w-10 text-right text-[11px] font-medium text-on-surface">
                      {metric.key === "reliability"
                        ? raw.toFixed(2)
                        : metric.key === "pue"
                          ? raw.toFixed(2)
                          : raw}
                    </span>
                    <div className="h-3 flex-1 rounded bg-surface-container-low">
                      <div
                        className={`h-3 rounded ${colors[i]} opacity-80 transition-all duration-500`}
                        style={{ width: `${Math.min(pct, 100)}%` }}
                      />
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

/* ──────────────────────────────────────────────
   Main component
   ────────────────────────────────────────────── */

function CityOverview() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { countrySlug, regionSlug } = useParams<{
    countrySlug: string;
    regionSlug: string;
  }>();

  // Chain: root locations -> find country -> children -> find region -> children (cities)
  const rootQ = useRootLocations();
  const countryLoc = useMemo(
    () => (rootQ.data?.data ?? []).find((l) => l.slug === (countrySlug ?? 'china')),
    [rootQ.data, countrySlug],
  );
  const countryChildrenQ = useLocationChildren(countryLoc?.id ?? '');
  const regionLoc = useMemo(
    () => (countryChildrenQ.data?.data ?? []).find((l) => l.slug === (regionSlug ?? 'east')),
    [countryChildrenQ.data, regionSlug],
  );
  const citiesQ = useLocationChildren(regionLoc?.id ?? '');
  const regionStatsQ = useLocationStats(regionLoc?.id ?? '');
  const rStats = regionStatsQ.data?.data;

  const cities: CityData[] = useMemo(
    () => (citiesQ.data?.data ?? []).map(locationToCity),
    [citiesQ.data],
  );

  const region = useMemo(() => ({
    nameCn: regionLoc?.name ?? '',
    nameEn: regionLoc?.name_en ?? regionLoc?.name ?? '',
    titleCn: regionLoc?.name ?? '',
    countrySlug: countrySlug ?? 'china',
    countryNameEn: countryLoc?.name_en ?? countryLoc?.name ?? '',
    idcs: cities.reduce((s, c) => s + c.idcCount, 0),
    campuses: cities.reduce((s, c) => s + c.campuses, 0),
    racks: rStats?.total_racks ?? cities.reduce((s, c) => s + c.racks, 0),
    pue: cities.length > 0 ? cities.reduce((s, c) => s + c.pue, 0) / cities.length : 0,
    alerts: rStats?.critical_alerts ?? cities.reduce((s, c) => s + c.alerts, 0),
    cities,
  }), [regionLoc, countrySlug, countryLoc, cities]);

  const [viewMode, setViewMode] = useState<"card" | "list">("card");
  const [sortBy, setSortBy] = useState<SortKey>("name");

  const isLoading = rootQ.isLoading || countryChildrenQ.isLoading || citiesQ.isLoading;
  const hasError = rootQ.error || countryChildrenQ.error || citiesQ.error;

  const sortedCities = useMemo(() => {
    const copy = [...cities];
    switch (sortBy) {
      case "name":
        return copy.sort((a, b) => a.nameEn.localeCompare(b.nameEn));
      case "idcCount":
        return copy.sort((a, b) => b.idcCount - a.idcCount);
      case "occupancy":
        return copy.sort((a, b) => b.occupancy - a.occupancy);
      case "alerts":
        return copy.sort((a, b) => b.alerts - a.alerts);
      default:
        return copy;
    }
  }, [cities, sortBy]);

  const country = countrySlug ?? "china";

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-sky-400 border-t-transparent" />
      </div>
    );
  }

  if (hasError) {
    return (
      <div className="p-6">
        <div className="rounded-lg bg-red-900/20 p-4 text-red-300 text-sm">
          Failed to load city data.{' '}
          <button onClick={() => { rootQ.refetch(); countryChildrenQ.refetch(); citiesQ.refetch(); }} className="underline">Retry</button>
        </div>
      </div>
    );
  }

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
        <button
          onClick={() => navigate(`/locations/${country}`)}
          className="font-medium uppercase tracking-wider text-primary hover:underline"
        >
          {region.countryNameEn.toUpperCase()}
        </button>
        <Icon name="chevron_right" className="text-sm text-on-surface-variant" />
        <span className="font-medium uppercase tracking-wider text-on-surface">
          {region.nameEn.toUpperCase()}
        </span>
      </nav>

      {/* Title */}
      <div className="mb-6">
        <h1 className="font-headline text-2xl font-bold text-on-surface">
          {region.titleCn}
        </h1>
        <p className="mt-1 text-sm text-on-surface-variant">
          {region.nameCn} {t('locations.city_title_suffix')}
        </p>
      </div>

      {/* KPI Bar */}
      <div className="mb-6 flex gap-3">
        <KpiCard icon="domain" label={t('locations.kpi_idcs')} value={region.idcs} />
        <KpiCard icon="apartment" label={t('locations.kpi_campuses')} value={region.campuses} />
        <KpiCard icon="dns" label={t('locations.kpi_racks')} value={region.racks} />
        <KpiCard icon="bolt" label={t('locations.kpi_pue')} value={region.pue.toFixed(2)} />
        <KpiCard icon="warning" label={t('locations.kpi_alerts')} value={region.alerts} alert />
      </div>

      {/* Toolbar: View toggle + Sort */}
      <div className="mb-4 flex items-center justify-between">
        <div className="flex items-center gap-1 rounded-lg bg-surface-container p-1">
          <button
            onClick={() => setViewMode("card")}
            className={`flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
              viewMode === "card"
                ? "bg-surface-container-high text-on-surface"
                : "text-on-surface-variant hover:text-on-surface"
            }`}
          >
            <Icon name="grid_view" className="text-base" />
            {t('locations.view_card')}
          </button>
          <button
            onClick={() => setViewMode("list")}
            className={`flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
              viewMode === "list"
                ? "bg-surface-container-high text-on-surface"
                : "text-on-surface-variant hover:text-on-surface"
            }`}
          >
            <Icon name="view_list" className="text-base" />
            {t('locations.view_list')}
          </button>
        </div>

        <div className="flex items-center gap-2">
          <Icon name="sort" className="text-base text-on-surface-variant" />
          <select
            value={sortBy}
            onChange={(e) => setSortBy(e.target.value as SortKey)}
            className="rounded-lg bg-surface-container px-3 py-1.5 text-xs text-on-surface outline-none"
          >
            <option value="name">{t('locations.sort_by_name')}</option>
            <option value="idcCount">{t('locations.sort_by_idc_count')}</option>
            <option value="occupancy">{t('locations.sort_by_occupancy')}</option>
            <option value="alerts">{t('locations.sort_by_alerts')}</option>
          </select>
        </div>
      </div>

      {/* City list */}
      {viewMode === "card" ? (
        <div className="mb-6 grid grid-cols-3 gap-4">
          {sortedCities.map((city) => (
            <CityCard
              key={city.slug}
              city={city}
              onClick={() =>
                navigate(
                  `/locations/${country}/${regionSlug ?? "east"}/${city.slug}`,
                )
              }
            />
          ))}
        </div>
      ) : (
        <div className="mb-6 space-y-2">
          {sortedCities.map((city) => (
            <CityRow
              key={city.slug}
              city={city}
              onClick={() =>
                navigate(
                  `/locations/${country}/${regionSlug ?? "east"}/${city.slug}`,
                )
              }
            />
          ))}
        </div>
      )}

      {/* Bottom: Comparison bars */}
      <ComparisonBars cities={region.cities} />
    </div>
  );
}

export default memo(CityOverview);
