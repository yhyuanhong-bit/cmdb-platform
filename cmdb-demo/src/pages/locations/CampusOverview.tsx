import { memo, useState, useMemo } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useLocationContext } from '../../contexts/LocationContext';
import { useRootLocations, useLocationChildren, useLocationDescendants, useRacks, useLocationStats } from '../../hooks/useTopology';
import type { Location } from '../../lib/api/topology';

/* ──────────────────────────────────────────────
   Types
   ────────────────────────────────────────────── */

interface IdcData {
  id: string;
  name: string;
  modules: number;
  racks: number;
  assets: number;
  pue: number;
  occupancy: number;
  alerts: number;
  used: number;
  available: number;
  reserved: number;
}

interface CampusData {
  id: string;
  nameCn: string;
  nameEn: string;
  addressCn: string;
  idcs: IdcData[];
}

function locationToCampus(loc: Location, childIdcs: Location[]): CampusData {
  return {
    id: loc.slug,
    nameCn: loc.name,
    nameEn: loc.name_en || loc.name,
    addressCn: (loc.metadata?.address as string) ?? '',
    idcs: childIdcs
      .filter((c) => c.parent_id === loc.id)
      .map((idc) => ({
        id: idc.slug,
        name: idc.name,
        modules: (idc.metadata?.modules as number) ?? 0,
        racks: (idc.metadata?.racks as number) ?? 0,
        assets: (idc.metadata?.assets as number) ?? 0,
        pue: (idc.metadata?.pue as number) ?? 0,
        occupancy: (idc.metadata?.occupancy as number) ?? 0,
        alerts: (idc.metadata?.alerts as number) ?? 0,
        used: (idc.metadata?.used as number) ?? 0,
        available: (idc.metadata?.available as number) ?? 0,
        reserved: (idc.metadata?.reserved as number) ?? 0,
      })),
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
   IDC Card
   ────────────────────────────────────────────── */

const IdcCard = memo(function IdcCard({
  idc,
  onNavigate,
}: {
  idc: IdcData;
  onNavigate: () => void;
}) {
  const { t } = useTranslation();
  const pueColor =
    idc.pue < 1.3
      ? "text-green-400"
      : idc.pue < 1.5
        ? "text-amber-400"
        : "text-red-400";

  const pueBg =
    idc.pue < 1.3
      ? "bg-green-500/10"
      : idc.pue < 1.5
        ? "bg-amber-500/10"
        : "bg-red-500/10";

  return (
    <div className="rounded-lg bg-surface-container-low p-5 transition-colors hover:bg-surface-container">
      {/* Header */}
      <div className="mb-4 flex items-center justify-between">
        <h4 className="font-headline text-xl font-bold text-on-surface">
          {idc.name}
        </h4>
        <div className="flex items-center gap-2">
          <span className={`rounded-lg px-2.5 py-1 text-xs font-semibold ${pueColor} ${pueBg}`}>
            PUE {idc.pue.toFixed(2)}
          </span>
          {idc.alerts > 0 ? (
            <span className="flex items-center gap-1 rounded-full bg-red-500/15 px-2.5 py-1 text-xs font-medium text-red-400">
              <Icon name="error" className="text-sm" />
              {idc.alerts} alerts
            </span>
          ) : (
            <span className="flex items-center gap-1 rounded-full bg-green-500/15 px-2.5 py-1 text-xs font-medium text-green-400">
              <Icon name="check_circle" className="text-sm" />
              OK
            </span>
          )}
        </div>
      </div>

      {/* Stats grid */}
      <div className="mb-4 grid grid-cols-3 gap-4">
        <div className="rounded-lg bg-surface-container p-3">
          <span className="text-[11px] uppercase tracking-wider text-on-surface-variant">
            {t('locations.modules')}
          </span>
          <p className="font-headline text-lg font-semibold text-on-surface">
            {idc.modules}
          </p>
        </div>
        <div className="rounded-lg bg-surface-container p-3">
          <span className="text-[11px] uppercase tracking-wider text-on-surface-variant">
            Racks
          </span>
          <p className="font-headline text-lg font-semibold text-on-surface">
            {idc.racks.toLocaleString()}
          </p>
        </div>
        <div className="rounded-lg bg-surface-container p-3">
          <span className="text-[11px] uppercase tracking-wider text-on-surface-variant">
            Assets
          </span>
          <p className="font-headline text-lg font-semibold text-on-surface">
            {idc.assets.toLocaleString()}
          </p>
        </div>
      </div>

      {/* Occupancy bar */}
      <div className="mb-4">
        <div className="mb-1.5 flex items-center justify-between text-xs">
          <span className="text-on-surface-variant">Occupancy</span>
          <span className="font-medium text-on-surface">{idc.occupancy}%</span>
        </div>
        <ProgressBar
          pct={idc.occupancy}
          color={
            idc.occupancy > 80
              ? "bg-amber-400"
              : idc.occupancy > 60
                ? "bg-primary"
                : "bg-green-400"
          }
          height="h-2.5"
        />
      </div>

      {/* Navigate link */}
      <button
        onClick={onNavigate}
        className="flex w-full items-center justify-center gap-2 rounded-lg bg-surface-container-high py-2.5 text-sm font-medium text-primary transition-colors hover:bg-primary/10"
      >
        <Icon name="dashboard" className="text-base" />
        {t('locations.btn_enter_dashboard')}
        <Icon name="arrow_forward" className="text-base" />
      </button>
    </div>
  );
});

/* ──────────────────────────────────────────────
   Campus Accordion
   ────────────────────────────────────────────── */

function CampusSection({
  campus,
  defaultExpanded,
  onIdcNavigate,
}: {
  campus: CampusData;
  defaultExpanded: boolean;
  onIdcNavigate: (campus: CampusData, idc: IdcData) => void;
}) {
  const [expanded, setExpanded] = useState(defaultExpanded);

  const totalRacks = campus.idcs.reduce((sum, idc) => sum + idc.racks, 0);
  const totalAlerts = campus.idcs.reduce((sum, idc) => sum + idc.alerts, 0);

  return (
    <div className="rounded-lg bg-surface-container">
      {/* Accordion header */}
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex w-full items-center gap-4 p-5 text-left transition-colors hover:bg-surface-container-high rounded-lg"
      >
        <Icon
          name={expanded ? "expand_more" : "chevron_right"}
          className="text-xl text-on-surface-variant"
        />
        <div className="flex-1">
          <h3 className="font-headline text-lg font-bold text-on-surface">
            {campus.nameCn}
          </h3>
          <span className="text-xs text-on-surface-variant">
            {campus.nameEn}
          </span>
        </div>
        <div className="flex items-center gap-4 text-xs text-on-surface-variant">
          <span>{campus.idcs.length} IDC</span>
          <span>{totalRacks.toLocaleString()} Racks</span>
          {totalAlerts > 0 ? (
            <span className="flex items-center gap-1 text-red-400">
              <Icon name="warning" className="text-sm" />
              {totalAlerts}
            </span>
          ) : (
            <span className="flex items-center gap-1 text-green-400">
              <Icon name="check_circle" className="text-sm" />
            </span>
          )}
        </div>
      </button>

      {/* Expanded content */}
      {expanded && (
        <div className="px-5 pb-5">
          {/* Address */}
          <div className="mb-4 flex items-center gap-2 rounded-lg bg-surface-container-low px-4 py-2.5">
            <Icon name="location_on" className="text-base text-primary" />
            <span className="text-xs text-on-surface-variant">
              {campus.addressCn}
            </span>
          </div>

          {/* IDC cards */}
          <div className="grid grid-cols-3 gap-4">
            {campus.idcs.map((idc) => (
              <IdcCard
                key={idc.id}
                idc={idc}
                onNavigate={() => onIdcNavigate(campus, idc)}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

/* ──────────────────────────────────────────────
   Capacity Planning Chart
   ────────────────────────────────────────────── */

function CapacityChart({ campuses }: { campuses: CampusData[] }) {
  const { t } = useTranslation();
  const allIdcs = campuses.flatMap((c) => c.idcs);
  const maxRacks = Math.max(...allIdcs.map((idc) => idc.racks));

  return (
    <div className="rounded-lg bg-surface-container p-5">
      <div className="mb-4 flex items-center gap-2">
        <Icon name="stacked_bar_chart" className="text-lg text-primary" />
        <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
          {t('locations.capacity_planning')}
        </h3>
      </div>

      {/* Legend */}
      <div className="mb-4 flex gap-5">
        <div className="flex items-center gap-1.5">
          <div className="h-2.5 w-2.5 rounded bg-primary" />
          <span className="text-xs text-on-surface-variant">{t('locations.used')}</span>
        </div>
        <div className="flex items-center gap-1.5">
          <div className="h-2.5 w-2.5 rounded bg-green-400" />
          <span className="text-xs text-on-surface-variant">{t('locations.available')}</span>
        </div>
        <div className="flex items-center gap-1.5">
          <div className="h-2.5 w-2.5 rounded bg-amber-400/60" />
          <span className="text-xs text-on-surface-variant">{t('locations.reserved')}</span>
        </div>
      </div>

      {/* Stacked bars */}
      <div className="space-y-3">
        {allIdcs.map((idc) => {
          const total = idc.used + idc.available + idc.reserved;
          const usedPct = (idc.used / total) * 100;
          const availablePct = (idc.available / total) * 100;
          const reservedPct = (idc.reserved / total) * 100;

          return (
            <div key={idc.id}>
              <div className="mb-1 flex items-center justify-between text-xs">
                <span className="font-medium text-on-surface">{idc.name}</span>
                <span className="text-on-surface-variant">
                  {idc.used} used / {idc.available} avail / {idc.reserved}{" "}
                  reserved ({total} total)
                </span>
              </div>
              <div className="flex h-6 w-full overflow-hidden rounded">
                <div
                  className="flex h-full items-center justify-center bg-primary text-[10px] font-medium text-on-primary-container transition-all"
                  style={{ width: `${usedPct}%` }}
                >
                  {usedPct > 15 && `${idc.used}`}
                </div>
                <div
                  className="flex h-full items-center justify-center bg-green-400 text-[10px] font-medium text-[#0a151a] transition-all"
                  style={{ width: `${availablePct}%` }}
                >
                  {availablePct > 10 && `${idc.available}`}
                </div>
                <div
                  className="flex h-full items-center justify-center bg-amber-400/60 text-[10px] font-medium text-[#0a151a] transition-all"
                  style={{ width: `${reservedPct}%` }}
                >
                  {reservedPct > 8 && `${idc.reserved}`}
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

/* ──────────────────────────────────────────────
   Main component
   ────────────────────────────────────────────── */

function CampusOverview() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { countrySlug, regionSlug, citySlug } = useParams<{
    countrySlug: string;
    regionSlug: string;
    citySlug: string;
  }>();

  const { setPath } = useLocationContext();

  // Chain location lookups: root -> country -> region -> city -> campuses (children of city)
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
  const regionChildrenQ = useLocationChildren(regionLoc?.id ?? '');
  const cityLoc = useMemo(
    () => (regionChildrenQ.data?.data ?? []).find((l) => l.slug === (citySlug ?? 'shanghai')),
    [regionChildrenQ.data, citySlug],
  );
  const campusChildrenQ = useLocationChildren(cityLoc?.id ?? '');

  // Fetch all descendants of city to get IDC-level locations
  const cityDescendantsQ = useLocationDescendants(cityLoc?.id ?? '');
  const allDescendants: Location[] = cityDescendantsQ.data?.data ?? [];

  // Campuses are the children of city; IDCs are children of campuses
  const campuses: CampusData[] = useMemo(() => {
    const campusLocs = campusChildrenQ.data?.data ?? [];
    // Pass all descendants so locationToCampus can filter by parent_id
    return campusLocs.map((loc) => locationToCampus(loc, allDescendants));
  }, [campusChildrenQ.data, allDescendants]);

  const racksQ = useRacks(cityLoc?.id ?? '');
  const cityStatsQ = useLocationStats(cityLoc?.id ?? '');
  const cStats = cityStatsQ.data?.data;

  const city = useMemo(() => ({
    nameCn: cityLoc?.name ?? '',
    nameEn: cityLoc?.name_en ?? cityLoc?.name ?? '',
    titleCn: cityLoc?.name ?? '',
    regionNameEn: regionLoc?.name_en ?? regionLoc?.name ?? '',
    regionSlug: regionSlug ?? 'east',
    countryNameEn: countryLoc?.name_en ?? countryLoc?.name ?? '',
    countrySlug: countrySlug ?? 'china',
    totalCampuses: campuses.length,
    totalIdcs: campuses.reduce((s, c) => s + c.idcs.length, 0),
    totalRacks: cStats?.total_racks ?? racksQ.data?.data?.length ?? campuses.reduce((s, c) => s + c.idcs.reduce((rs, idc) => rs + idc.racks, 0), 0),
    totalAlerts: cStats?.critical_alerts ?? campuses.reduce((s, c) => s + c.idcs.reduce((as2, idc) => as2 + idc.alerts, 0), 0),
    campuses,
  }), [cityLoc, regionLoc, countryLoc, campuses, racksQ.data, countrySlug, regionSlug]);

  const country = countrySlug ?? city.countrySlug;
  const region = regionSlug ?? city.regionSlug;

  const isLoading = rootQ.isLoading || countryChildrenQ.isLoading || regionChildrenQ.isLoading || campusChildrenQ.isLoading || cityDescendantsQ.isLoading;
  const hasError = rootQ.error || countryChildrenQ.error || regionChildrenQ.error || campusChildrenQ.error || cityDescendantsQ.error;

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
          Failed to load campus data.{' '}
          <button onClick={() => { rootQ.refetch(); }} className="underline">Retry</button>
        </div>
      </div>
    );
  }

  const handleIdcNavigate = (campus: CampusData, idc: IdcData) => {
    setPath({
      country: {
        id: country,
        slug: country,
        name: city.countryNameEn,
        nameEn: city.countryNameEn,
      },
      region: {
        id: region,
        slug: region,
        name: city.regionNameEn,
        nameEn: city.regionNameEn,
      },
      city: {
        id: citySlug ?? "shanghai",
        slug: citySlug ?? "shanghai",
        name: city.nameCn,
        nameEn: city.nameEn,
      },
      campus: {
        id: campus.id,
        slug: campus.id,
        name: campus.nameCn,
        nameEn: campus.nameEn,
      },
      idc: {
        id: idc.id,
        slug: idc.id,
        name: idc.name,
        nameEn: idc.name,
      },
    });
    navigate("/dashboard");
  };

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
          {city.countryNameEn.toUpperCase()}
        </button>
        <Icon name="chevron_right" className="text-sm text-on-surface-variant" />
        <button
          onClick={() => navigate(`/locations/${country}/${region}`)}
          className="font-medium uppercase tracking-wider text-primary hover:underline"
        >
          {city.regionNameEn.toUpperCase()}
        </button>
        <Icon name="chevron_right" className="text-sm text-on-surface-variant" />
        <span className="font-medium uppercase tracking-wider text-on-surface">
          {city.nameEn.toUpperCase()}
        </span>
      </nav>

      {/* Title */}
      <div className="mb-6">
        <h1 className="font-headline text-2xl font-bold text-on-surface">
          {city.titleCn}
        </h1>
        <p className="mt-1 text-sm text-on-surface-variant">
          {city.nameEn} {t('locations.campus_subtitle')}
        </p>
      </div>

      {/* KPI Bar */}
      <div className="mb-6 flex gap-3">
        <KpiCard
          icon="apartment"
          label={t('locations.kpi_campuses')}
          value={city.totalCampuses}
        />
        <KpiCard icon="domain" label={t('locations.kpi_idcs')} value={city.totalIdcs} />
        <KpiCard icon="dns" label={t('locations.kpi_racks')} value={city.totalRacks} />
        <KpiCard
          icon="warning"
          label={t('locations.kpi_alerts')}
          value={city.totalAlerts}
          alert
        />
      </div>

      {/* Campus sections */}
      <div className="mb-6 space-y-4">
        {city.campuses.map((campus, idx) => (
          <CampusSection
            key={campus.id}
            campus={campus}
            defaultExpanded={idx === 0}
            onIdcNavigate={handleIdcNavigate}
          />
        ))}
      </div>

      {/* Capacity Planning */}
      <CapacityChart campuses={city.campuses} />
    </div>
  );
}

export default memo(CampusOverview);
