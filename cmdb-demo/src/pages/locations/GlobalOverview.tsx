import React, { useState, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { useRootLocations } from '../../hooks/useTopology';
import { useDashboardStats } from '../../hooks/useDashboard';
import { useAlerts } from '../../hooks/useMonitoring';
import CreateLocationModal from '../../components/CreateLocationModal';
import type { Location } from '../../lib/api/topology';

// ---------------------------------------------------------------------------
// Mock Data (kept: alerts need backend aggregation endpoint)
// ---------------------------------------------------------------------------

const LAST_SYNC = '2026-03-28T14:32:08Z';

// TODO: needs backend endpoint for summary KPIs (PUE, uptime, energy trend)
const SUMMARY_KPI = {
  averagePUE: 1.24,
  totalPowerKW: 12_950,
  globalUptime: 99.97,
  energyTrend: [82, 79, 84, 81, 78, 80, 77],
};

interface CountryData {
  slug: string;
  nameCn: string;
  nameEn: string;
  flag: string;
  idcCount: number;
  regionCount: number;
  totalAssets: number;
  pue: number;
  rackOccupancy: number;
  criticalAlerts: number;
  powerTrend: number[];
  mapLeft: string;
  mapTop: string;
  markerSize: string;
  healthy: boolean;
}

// Map position metadata for known locations (keyed by slug)
const MAP_META: Record<string, { flag: string; mapLeft: string; mapTop: string; markerSize: string }> = {
  china:     { flag: '\u{1F1E8}\u{1F1F3}', mapLeft: '70%', mapTop: '35%', markerSize: '3rem' },
  japan:     { flag: '\u{1F1EF}\u{1F1F5}', mapLeft: '80%', mapTop: '30%', markerSize: '2.25rem' },
  singapore: { flag: '\u{1F1F8}\u{1F1EC}', mapLeft: '72%', mapTop: '55%', markerSize: '1.75rem' },
};

function locationToCountry(loc: Location): CountryData {
  const meta = MAP_META[loc.slug] ?? { flag: '\u{1F30D}', mapLeft: '50%', mapTop: '50%', markerSize: '2rem' };
  return {
    slug: loc.slug,
    nameCn: loc.name,
    nameEn: loc.name_en || loc.name,
    ...meta,
    // These stats are not yet in the Location schema; use metadata or fallback
    idcCount: (loc.metadata?.idc_count as number) ?? 0,
    regionCount: (loc.metadata?.region_count as number) ?? 0,
    totalAssets: (loc.metadata?.total_assets as number) ?? 0,
    pue: (loc.metadata?.pue as number) ?? 0,
    rackOccupancy: (loc.metadata?.rack_occupancy as number) ?? 0,
    criticalAlerts: (loc.metadata?.critical_alerts as number) ?? 0,
    powerTrend: (loc.metadata?.power_trend as number[]) ?? [],
    healthy: ((loc.metadata?.critical_alerts as number) ?? 0) === 0,
  };
}

interface AlertData {
  id: string;
  severity: 'CRITICAL' | 'WARNING';
  idc: string;
  assetId: string;
  message: string;
  timeAgo: string;
}

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}

function apiAlertToAlertData(a: any): AlertData {
  return {
    id: a.id,
    severity: (a.severity ?? '').toUpperCase() as 'CRITICAL' | 'WARNING',
    idc: '',
    assetId: a.ci_id ?? a.asset_id ?? '-',
    message: a.message ?? '',
    timeAgo: a.fired_at ? timeAgo(a.fired_at) : '',
  };
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatNumber(n: number): string {
  return n.toLocaleString('en-US');
}

function Sparkline({ data, color = '#9ecaff', width = 80, height = 24 }: { data: number[]; color?: string; width?: number; height?: number }) {
  const min = Math.min(...data);
  const max = Math.max(...data);
  const range = max - min || 1;
  const points = data
    .map((v, i) => {
      const x = (i / (data.length - 1)) * width;
      const y = height - ((v - min) / range) * height;
      return `${x},${y}`;
    })
    .join(' ');

  return (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} className="inline-block align-middle">
      <polyline fill="none" stroke={color} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" points={points} />
    </svg>
  );
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function PulsingDot({ color = 'bg-green-400', size = 'h-2.5 w-2.5' }: { color?: string; size?: string }) {
  return (
    <span className="relative flex">
      <span className={`absolute inline-flex ${size} rounded-full ${color} opacity-75 animate-ping`} />
      <span className={`relative inline-flex ${size} rounded-full ${color}`} />
    </span>
  );
}

function KpiCard({ icon, label, value, accent }: { icon: string; label: string; value: string | number; accent?: string }) {
  return (
    <div className="bg-surface-container rounded-lg p-4 flex items-center gap-4">
      <span className={`material-symbols-outlined text-3xl ${accent ?? 'text-primary'}`}>{icon}</span>
      <div>
        <p className="text-on-surface-variant text-xs uppercase tracking-wider font-body">{label}</p>
        <p className="text-on-surface text-xl font-bold font-headline">{typeof value === 'number' ? formatNumber(value) : value}</p>
      </div>
    </div>
  );
}

function MapMarker({ country, onClick }: { country: CountryData; onClick: () => void }) {
  const [hovered, setHovered] = useState(false);
  const markerColor = country.criticalAlerts > 3 ? 'bg-orange-400' : 'bg-green-400';
  const ringColor = country.criticalAlerts > 3 ? 'ring-orange-400/30' : 'ring-green-400/30';

  return (
    <button
      type="button"
      onClick={onClick}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      className="absolute flex flex-col items-center group focus:outline-none"
      style={{ left: country.mapLeft, top: country.mapTop, transform: 'translate(-50%, -50%)' }}
      aria-label={`${country.nameEn} - ${country.idcCount} IDCs`}
    >
      {/* Pulsing marker */}
      <span
        className={`relative rounded-full ${markerColor} ring-4 ${ringColor} transition-transform duration-200 ${hovered ? 'scale-125' : 'scale-100'}`}
        style={{ width: country.markerSize, height: country.markerSize }}
      >
        <span className={`absolute inset-0 rounded-full ${markerColor} opacity-40 animate-ping`} />
      </span>

      {/* Label */}
      <span className="mt-1.5 text-xs font-bold font-headline text-on-surface whitespace-nowrap">
        {country.nameEn}
      </span>
      <span className="text-[10px] text-on-surface-variant whitespace-nowrap">
        {country.idcCount} IDC
      </span>

      {/* Hover tooltip */}
      {hovered && (
        <div className="absolute top-full mt-2 bg-surface-container-high rounded-lg p-3 min-w-[180px] z-10 text-left shadow-lg">
          <p className="text-sm font-bold text-on-surface font-headline">{country.flag} {country.nameCn} {country.nameEn}</p>
          <div className="mt-2 space-y-1 text-xs text-on-surface-variant font-body">
            <p>{country.idcCount} IDCs across {country.regionCount} Region{country.regionCount > 1 ? 's' : ''}</p>
            <p>{formatNumber(country.totalAssets)} Assets</p>
            <p>PUE {country.pue.toFixed(2)}</p>
            <p>Rack Occupancy {country.rackOccupancy}%</p>
            {country.criticalAlerts > 0 && (
              <p className="text-error font-semibold">{country.criticalAlerts} Critical Alert{country.criticalAlerts > 1 ? 's' : ''}</p>
            )}
          </div>
        </div>
      )}
    </button>
  );
}

function CountryCard({ country, onClick }: { country: CountryData; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="bg-surface-container rounded-lg p-5 text-left hover:bg-surface-container-high transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-primary/50 w-full"
      aria-label={`View ${country.nameEn} details`}
    >
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <p className="text-lg font-bold font-headline text-on-surface">{country.flag} {country.nameCn} {country.nameEn}</p>
          <p className="text-xs text-on-surface-variant font-body mt-0.5">
            {country.idcCount} IDC &middot; {country.regionCount} Region{country.regionCount > 1 ? 's' : ''}
          </p>
        </div>
        {country.criticalAlerts > 0 && (
          <span className="bg-error/20 text-error text-xs font-bold px-2 py-0.5 rounded-full">
            {country.criticalAlerts} Alert{country.criticalAlerts > 1 ? 's' : ''}
          </span>
        )}
      </div>

      {/* Stats */}
      <div className="mt-4 grid grid-cols-2 gap-3 text-sm font-body">
        <div>
          <p className="text-on-surface-variant text-xs">Total Assets</p>
          <p className="text-on-surface font-semibold">{formatNumber(country.totalAssets)}</p>
        </div>
        <div>
          <p className="text-on-surface-variant text-xs">PUE</p>
          <p className={`font-semibold ${country.pue < 1.3 ? 'text-green-400' : 'text-orange-400'}`}>{country.pue.toFixed(2)}</p>
        </div>
      </div>

      {/* Rack Occupancy */}
      <div className="mt-4">
        <div className="flex items-center justify-between text-xs font-body mb-1">
          <span className="text-on-surface-variant">Rack Occupancy</span>
          <span className="text-on-surface font-semibold">{country.rackOccupancy}%</span>
        </div>
        <div className="w-full h-2 bg-surface-container-low rounded-full overflow-hidden">
          <div
            className={`h-full rounded-full transition-all duration-500 ${
              country.rackOccupancy > 80 ? 'bg-orange-400' : 'bg-primary'
            }`}
            style={{ width: `${country.rackOccupancy}%` }}
          />
        </div>
      </div>

      {/* Power Trend */}
      <div className="mt-4 flex items-center justify-between">
        <span className="text-xs text-on-surface-variant font-body">Power Trend (7d)</span>
        <Sparkline data={country.powerTrend} />
      </div>
    </button>
  );
}

function AlertCard({ alert, onClick }: { alert: AlertData; onClick?: () => void }) {
  const isCritical = alert.severity === 'CRITICAL';
  return (
    <div
      onClick={onClick}
      className={`rounded-lg p-4 cursor-pointer hover:opacity-80 transition-opacity ${isCritical ? 'bg-error/10' : 'bg-orange-400/10'}`}
    >
      <div className="flex items-center gap-2 mb-2">
        <span
          className={`text-[10px] font-bold uppercase tracking-wider px-2 py-0.5 rounded ${
            isCritical ? 'bg-error/20 text-error' : 'bg-orange-400/20 text-orange-300'
          }`}
        >
          {alert.severity}
        </span>
        <span className="text-xs text-on-surface-variant font-body">{alert.idc}</span>
      </div>
      <p className="text-sm font-semibold text-on-surface font-body">{alert.assetId}</p>
      <p className="text-xs text-on-surface-variant font-body mt-1">{alert.message}</p>
      <p className="text-[10px] text-on-surface-variant/60 font-body mt-2">{alert.timeAgo}</p>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Page Component
// ---------------------------------------------------------------------------

const GlobalOverview: React.FC = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [showCreateLocation, setShowCreateLocation] = useState(false);

  // Fetch root locations (countries) from API
  const rootLocationsQ = useRootLocations();
  const dashStatsQ = useDashboardStats();
  const alertsQ = useAlerts({ status: 'firing' });
  const stats = dashStatsQ.data?.data;

  // Convert API alerts to display format
  const ALERTS: AlertData[] = useMemo(() => {
    const raw = alertsQ.data?.data ?? [];
    return raw.map(apiAlertToAlertData);
  }, [alertsQ.data]);

  // Convert API locations to CountryData for rendering
  const COUNTRIES: CountryData[] = useMemo(() => {
    const locs = rootLocationsQ.data?.data ?? [];
    return locs.map(locationToCountry);
  }, [rootLocationsQ.data]);

  const GLOBAL_KPI = useMemo(() => ({
    countries: COUNTRIES.length,
    regions: COUNTRIES.reduce((s, c) => s + c.regionCount, 0),
    idcs: COUNTRIES.reduce((s, c) => s + c.idcCount, 0),
    totalAssets: stats?.total_assets ?? COUNTRIES.reduce((s, c) => s + c.totalAssets, 0),
  }), [COUNTRIES, stats]);

  const syncDate = new Date(LAST_SYNC);
  const syncTimeStr = syncDate.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
  const syncDateStr = syncDate.toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' });

  if (rootLocationsQ.isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-sky-400 border-t-transparent" />
      </div>
    );
  }

  if (rootLocationsQ.error) {
    return (
      <div className="p-6">
        <div className="rounded-lg bg-red-900/20 p-4 text-red-300 text-sm">
          Failed to load locations.{' '}
          <button onClick={() => rootLocationsQ.refetch()} className="underline">Retry</button>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-surface text-on-surface font-body overflow-hidden">
      {/* ----------------------------------------------------------------- */}
      {/* Header                                                            */}
      {/* ----------------------------------------------------------------- */}
      <header className="flex flex-col sm:flex-row items-start sm:items-center justify-between px-5 py-4 bg-surface-container-low rounded-lg">
        <div>
          <h1 className="font-headline text-2xl font-bold uppercase tracking-wider text-on-surface">
            IRONGRID GLOBAL COMMAND CENTER
          </h1>
          <p className="text-on-surface-variant text-sm mt-0.5 font-body">
            集團全域資料中心指揮總覽
          </p>
        </div>
        <div className="flex items-center gap-3 mt-3 sm:mt-0">
          <button
            onClick={() => setShowCreateLocation(true)}
            className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-on-primary-container text-white text-sm font-semibold hover:bg-on-primary-container/90 transition-colors"
          >
            <span className="material-symbols-outlined text-[16px]">add</span>
            Add Location
          </button>
          <PulsingDot />
          <span className="text-xs text-on-surface-variant font-body">
            Last Sync: {syncDateStr} {syncTimeStr}
          </span>
        </div>
      </header>

      <div className="px-6 py-5 space-y-6">
        {/* --------------------------------------------------------------- */}
        {/* Global KPI Bar                                                   */}
        {/* --------------------------------------------------------------- */}
        <section className="grid grid-cols-2 lg:grid-cols-4 gap-4" aria-label="Global KPIs">
          <KpiCard icon="public" label="Countries" value={GLOBAL_KPI.countries} />
          <KpiCard icon="pin_drop" label="Regions" value={GLOBAL_KPI.regions} />
          <KpiCard icon="domain" label="IDCs" value={GLOBAL_KPI.idcs} />
          <KpiCard icon="inventory_2" label="Total Assets" value={GLOBAL_KPI.totalAssets} />
        </section>

        {/* --------------------------------------------------------------- */}
        {/* Main Content: Map + Summary                                      */}
        {/* --------------------------------------------------------------- */}
        <section className="grid grid-cols-1 lg:grid-cols-[3fr_2fr] gap-4">
          {/* Left: World Map */}
          <div
            className="bg-surface-container-low rounded-lg p-6 relative overflow-hidden"
            style={{
              minHeight: '400px',
              backgroundImage:
                'radial-gradient(circle, rgba(158,202,255,0.06) 1px, transparent 1px)',
              backgroundSize: '24px 24px',
            }}
          >
            <h2 className="font-headline text-sm font-bold uppercase tracking-wider text-on-surface-variant mb-4">
              <span className="material-symbols-outlined text-base align-middle mr-1.5">map</span>
              GLOBAL INFRASTRUCTURE MAP
            </h2>

            {/* Simplified continent outlines using CSS shapes */}
            <div className="absolute inset-0" style={{ top: '60px' }}>
              {/* Subtle region labels */}
              <span className="absolute text-[10px] text-on-surface-variant/30 uppercase tracking-widest" style={{ left: '12%', top: '28%' }}>
                Europe
              </span>
              <span className="absolute text-[10px] text-on-surface-variant/30 uppercase tracking-widest" style={{ left: '55%', top: '18%' }}>
                East Asia
              </span>
              <span className="absolute text-[10px] text-on-surface-variant/30 uppercase tracking-widest" style={{ left: '58%', top: '60%' }}>
                SE Asia
              </span>
              <span className="absolute text-[10px] text-on-surface-variant/30 uppercase tracking-widest" style={{ left: '25%', top: '40%' }}>
                Middle East
              </span>
              <span className="absolute text-[10px] text-on-surface-variant/30 uppercase tracking-widest" style={{ left: '8%', top: '58%' }}>
                Africa
              </span>

              {/* Country markers */}
              {COUNTRIES.map((country) => (
                <MapMarker
                  key={country.slug}
                  country={country}
                  onClick={() => navigate(`/locations/${country.slug}`)}
                />
              ))}

              {/* Connection lines between markers (decorative) */}
              <svg className="absolute inset-0 w-full h-full pointer-events-none" aria-hidden="true">
                <line x1="70%" y1="35%" x2="80%" y2="30%" stroke="rgba(158,202,255,0.15)" strokeWidth="1" strokeDasharray="4 4" />
                <line x1="70%" y1="35%" x2="72%" y2="55%" stroke="rgba(158,202,255,0.15)" strokeWidth="1" strokeDasharray="4 4" />
                <line x1="80%" y1="30%" x2="72%" y2="55%" stroke="rgba(158,202,255,0.15)" strokeWidth="1" strokeDasharray="4 4" />
              </svg>
            </div>
          </div>

          {/* Right: Global KPI Summary */}
          <div className="bg-surface-container rounded-lg p-5 space-y-4">
            <h2 className="font-headline text-sm font-bold uppercase tracking-wider text-on-surface-variant">
              <span className="material-symbols-outlined text-base align-middle mr-1.5">analytics</span>
              Global KPI Summary
            </h2>

            <div className="space-y-3">
              {/* Total Racks */}
              <div className="flex items-center justify-between py-2">
                <span className="text-sm text-on-surface-variant font-body flex items-center gap-2">
                  <span className="material-symbols-outlined text-lg text-primary">view_column</span>
                  Total Racks
                </span>
                <span className="text-lg font-bold text-on-surface font-headline">{formatNumber(stats?.total_racks ?? 0)}</span>
              </div>

              {/* Average PUE */}
              <div className="flex items-center justify-between py-2">
                <span className="text-sm text-on-surface-variant font-body flex items-center gap-2">
                  <span className="material-symbols-outlined text-lg text-primary">speed</span>
                  Average PUE
                </span>
                <span className={`text-lg font-bold font-headline ${SUMMARY_KPI.averagePUE < 1.3 ? 'text-green-400' : 'text-orange-400'}`}>
                  {SUMMARY_KPI.averagePUE.toFixed(2)}
                </span>
              </div>

              {/* Total Power */}
              <div className="flex items-center justify-between py-2">
                <span className="text-sm text-on-surface-variant font-body flex items-center gap-2">
                  <span className="material-symbols-outlined text-lg text-primary">bolt</span>
                  Total Power
                </span>
                <span className="text-lg font-bold text-on-surface font-headline">{formatNumber(SUMMARY_KPI.totalPowerKW)} kW</span>
              </div>

              {/* Critical Alerts */}
              <div className="flex items-center justify-between py-2">
                <span className="text-sm text-on-surface-variant font-body flex items-center gap-2">
                  <span className="material-symbols-outlined text-lg text-error">warning</span>
                  Critical Alerts
                </span>
                <span className={`text-lg font-bold font-headline ${(stats?.critical_alerts ?? 0) > 0 ? 'text-error' : 'text-on-surface'}`}>
                  {stats?.critical_alerts ?? 0}
                </span>
              </div>

              {/* Global Uptime */}
              <div className="flex items-center justify-between py-2">
                <span className="text-sm text-on-surface-variant font-body flex items-center gap-2">
                  <span className="material-symbols-outlined text-lg text-green-400">check_circle</span>
                  Global Uptime
                </span>
                <span className="text-lg font-bold text-green-400 font-headline">{SUMMARY_KPI.globalUptime}%</span>
              </div>

              {/* Energy Trend */}
              <div className="flex items-center justify-between py-2">
                <span className="text-sm text-on-surface-variant font-body flex items-center gap-2">
                  <span className="material-symbols-outlined text-lg text-primary">trending_down</span>
                  Energy Trend (7d)
                </span>
                <Sparkline data={SUMMARY_KPI.energyTrend} color="#4ade80" width={100} height={28} />
              </div>
            </div>
          </div>
        </section>

        {/* --------------------------------------------------------------- */}
        {/* Country Cards                                                    */}
        {/* --------------------------------------------------------------- */}
        <section aria-label="Country Overview">
          <h2 className="font-headline text-sm font-bold uppercase tracking-wider text-on-surface-variant mb-4">
            <span className="material-symbols-outlined text-base align-middle mr-1.5">flag</span>
            Country Overview
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {COUNTRIES.map((country) => (
              <CountryCard
                key={country.slug}
                country={country}
                onClick={() => navigate(`/locations/${country.slug}`)}
              />
            ))}
          </div>
        </section>

        {/* --------------------------------------------------------------- */}
        {/* Global Alert Stream                                              */}
        {/* --------------------------------------------------------------- */}
        <section aria-label="Global Alert Stream">
          <div className="flex items-center gap-3 mb-4">
            <h2 className="font-headline text-sm font-bold uppercase tracking-wider text-on-surface-variant flex items-center gap-1.5">
              <span className="material-symbols-outlined text-base align-middle text-error">notifications_active</span>
              GLOBAL REAL-TIME ALERT STREAM
            </h2>
            <span className="bg-error/20 text-error text-xs font-bold px-2 py-0.5 rounded-full">
              {ALERTS.length}
            </span>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
            {ALERTS.map((alert) => (
              <AlertCard key={alert.id} alert={alert} onClick={() => navigate('/monitoring')} />
            ))}
          </div>
        </section>
      </div>

      <CreateLocationModal open={showCreateLocation} onClose={() => setShowCreateLocation(false)} />
    </div>
  );
};

export default GlobalOverview;
