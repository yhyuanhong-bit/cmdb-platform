import React, { useState, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { MapContainer, TileLayer, Marker, Popup } from 'react-leaflet';
import L from 'leaflet';
import { useRootLocations } from '../../hooks/useTopology';
import { useDashboardStats } from '../../hooks/useDashboard';
import { useAlerts } from '../../hooks/useMonitoring';
import CreateLocationModal from '../../components/CreateLocationModal';
import type { Location } from '../../lib/api/topology';

// ---------------------------------------------------------------------------
// Fix Leaflet default icon (webpack/vite asset resolution issue)
// ---------------------------------------------------------------------------

delete (L.Icon.Default.prototype as any)._getIconUrl;
L.Icon.Default.mergeOptions({
  iconRetinaUrl: 'https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.9.4/images/marker-icon-2x.png',
  iconUrl: 'https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.9.4/images/marker-icon.png',
  shadowUrl: 'https://cdnjs.cloudflare.com/ajax/libs/leaflet/1.9.4/images/marker-shadow.png',
});

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

interface TerritoryData {
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
  latitude?: number;
  longitude?: number;
  healthy: boolean;
}

// Flag emoji map for known location slugs
const FLAG_MAP: Record<string, string> = {
  china:     '\u{1F1E8}\u{1F1F3}',
  japan:     '\u{1F1EF}\u{1F1F5}',
  singapore: '\u{1F1F8}\u{1F1EC}',
  tw:        '\u{1F1F9}\u{1F1FC}',
};

function locationToTerritory(loc: Location): TerritoryData {
  const flag = FLAG_MAP[loc.slug] ?? '\u{1F30D}';
  return {
    slug: loc.slug,
    nameCn: loc.name,
    nameEn: loc.name_en || loc.name,
    flag,
    latitude: loc.latitude ?? undefined,
    longitude: loc.longitude ?? undefined,
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

function TerritoryCard({ territory, onClick }: { territory: TerritoryData; onClick: () => void }) {
  const { t } = useTranslation();
  return (
    <button
      type="button"
      onClick={onClick}
      className="bg-surface-container rounded-lg p-5 text-left hover:bg-surface-container-high transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-primary/50 w-full"
      aria-label={`View ${territory.nameEn} details`}
    >
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <p className="text-lg font-bold font-headline text-on-surface">{territory.flag} {territory.nameCn} {territory.nameEn}</p>
          <p className="text-xs text-on-surface-variant font-body mt-0.5">
            {territory.idcCount} IDC &middot; {territory.regionCount} Region{territory.regionCount > 1 ? 's' : ''}
          </p>
        </div>
        {territory.criticalAlerts > 0 && (
          <span className="bg-error/20 text-error text-xs font-bold px-2 py-0.5 rounded-full">
            {territory.criticalAlerts} Alert{territory.criticalAlerts > 1 ? 's' : ''}
          </span>
        )}
      </div>

      {/* Stats */}
      <div className="mt-4 grid grid-cols-2 gap-3 text-sm font-body">
        <div>
          <p className="text-on-surface-variant text-xs">{t('locations.kpi_total_assets')}</p>
          <p className="text-on-surface font-semibold">{formatNumber(territory.totalAssets)}</p>
        </div>
        <div>
          <p className="text-on-surface-variant text-xs">PUE</p>
          <p className={`font-semibold ${territory.pue < 1.3 ? 'text-green-400' : 'text-orange-400'}`}>{territory.pue.toFixed(2)}</p>
        </div>
      </div>

      {/* Rack Occupancy */}
      <div className="mt-4">
        <div className="flex items-center justify-between text-xs font-body mb-1">
          <span className="text-on-surface-variant">{t('locations.rack_occupancy')}</span>
          <span className="text-on-surface font-semibold">{territory.rackOccupancy}%</span>
        </div>
        <div className="w-full h-2 bg-surface-container-low rounded-full overflow-hidden">
          <div
            className={`h-full rounded-full transition-all duration-500 ${
              territory.rackOccupancy > 80 ? 'bg-orange-400' : 'bg-primary'
            }`}
            style={{ width: `${territory.rackOccupancy}%` }}
          />
        </div>
      </div>

      {/* Power Trend */}
      <div className="mt-4 flex items-center justify-between">
        <span className="text-xs text-on-surface-variant font-body">Power Trend (7d)</span>
        <Sparkline data={territory.powerTrend} />
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

  // Convert API locations to TerritoryData for rendering
  const TERRITORIES: TerritoryData[] = useMemo(() => {
    const locs = rootLocationsQ.data?.data ?? [];
    return locs.map(locationToTerritory);
  }, [rootLocationsQ.data]);

  const GLOBAL_KPI = useMemo(() => ({
    territories: TERRITORIES.length,
    regions: TERRITORIES.reduce((s, c) => s + c.regionCount, 0),
    idcs: TERRITORIES.reduce((s, c) => s + c.idcCount, 0),
    totalAssets: stats?.total_assets ?? TERRITORIES.reduce((s, c) => s + c.totalAssets, 0),
  }), [TERRITORIES, stats]);

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
            {t('locations.global_title').toUpperCase()}
          </h1>
          <p className="text-on-surface-variant text-sm mt-0.5 font-body">
            {t('locations.global_title_zh')}
          </p>
        </div>
        <div className="flex items-center gap-3 mt-3 sm:mt-0">
          <button
            onClick={() => setShowCreateLocation(true)}
            className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-on-primary-container text-white text-sm font-semibold hover:bg-on-primary-container/90 transition-colors"
          >
            <span className="material-symbols-outlined text-[16px]">add</span>
            {t('locations.btn_add_location')}
          </button>
          <PulsingDot />
          <span className="text-xs text-on-surface-variant font-body">
            {t('locations.last_sync')}: {syncDateStr} {syncTimeStr}
          </span>
        </div>
      </header>

      <div className="px-6 py-5 space-y-6">
        {/* --------------------------------------------------------------- */}
        {/* Global KPI Bar                                                   */}
        {/* --------------------------------------------------------------- */}
        <section className="grid grid-cols-2 lg:grid-cols-4 gap-4" aria-label="Global KPIs">
          <KpiCard icon="public" label={t('locations.kpi_territories')} value={GLOBAL_KPI.territories} />
          <KpiCard icon="pin_drop" label={t('locations.kpi_regions')} value={GLOBAL_KPI.regions} />
          <KpiCard icon="domain" label={t('locations.kpi_idcs')} value={GLOBAL_KPI.idcs} />
          <KpiCard icon="inventory_2" label={t('locations.kpi_total_assets')} value={GLOBAL_KPI.totalAssets} />
        </section>

        {/* --------------------------------------------------------------- */}
        {/* Main Content: Map + Summary                                      */}
        {/* --------------------------------------------------------------- */}
        <section className="grid grid-cols-1 lg:grid-cols-[3fr_2fr] gap-4">
          {/* Left: Leaflet World Map */}
          <div
            className="bg-surface-container-low rounded-lg p-6"
            style={{ minHeight: '400px' }}
          >
            <h2 className="font-headline text-sm font-bold uppercase tracking-wider text-on-surface-variant mb-4">
              <span className="material-symbols-outlined text-base align-middle mr-1.5">map</span>
              GLOBAL INFRASTRUCTURE MAP
            </h2>

            <div style={{ height: '340px', borderRadius: '0.75rem', overflow: 'hidden' }}>
              <MapContainer
                center={[25, 110]}
                zoom={3}
                style={{ height: '100%', width: '100%' }}
                scrollWheelZoom={true}
              >
                <TileLayer
                  attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
                  url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
                />
                {TERRITORIES.filter(t => t.latitude != null && t.longitude != null).map(territory => (
                  <Marker
                    key={territory.slug}
                    position={[territory.latitude!, territory.longitude!]}
                    icon={L.divIcon({
                      className: 'custom-marker',
                      html: `<div style="font-size:1.5rem;cursor:pointer;filter:drop-shadow(0 1px 2px rgba(0,0,0,0.5))">${territory.flag}</div>`,
                      iconSize: [30, 30],
                      iconAnchor: [15, 15],
                    })}
                    eventHandlers={{
                      click: () => navigate(`/locations/${territory.slug}`),
                    }}
                  >
                    <Popup>
                      <div style={{ textAlign: 'center', minWidth: '120px' }}>
                        <strong>{territory.flag} {territory.nameCn}</strong><br />
                        {territory.nameEn}<br />
                        <span style={{ fontSize: '0.85em', color: '#666' }}>
                          {territory.totalAssets} assets
                        </span>
                      </div>
                    </Popup>
                  </Marker>
                ))}
              </MapContainer>
            </div>
          </div>

          {/* Right: Global KPI Summary */}
          <div className="bg-surface-container rounded-lg p-5 space-y-4">
            <h2 className="font-headline text-sm font-bold uppercase tracking-wider text-on-surface-variant">
              <span className="material-symbols-outlined text-base align-middle mr-1.5">analytics</span>
              {t('locations.global_kpi_summary')}
            </h2>

            <div className="space-y-3">
              {/* Total Racks */}
              <div className="flex items-center justify-between py-2">
                <span className="text-sm text-on-surface-variant font-body flex items-center gap-2">
                  <span className="material-symbols-outlined text-lg text-primary">view_column</span>
                  {t('locations.kpi_total_racks')}
                </span>
                <span className="text-lg font-bold text-on-surface font-headline">{formatNumber(stats?.total_racks ?? 0)}</span>
              </div>

              {/* Average PUE */}
              <div className="flex items-center justify-between py-2">
                <span className="text-sm text-on-surface-variant font-body flex items-center gap-2">
                  <span className="material-symbols-outlined text-lg text-primary">speed</span>
                  {t('locations.avg_pue')}
                </span>
                <span className={`text-lg font-bold font-headline ${SUMMARY_KPI.averagePUE < 1.3 ? 'text-green-400' : 'text-orange-400'}`}>
                  {SUMMARY_KPI.averagePUE.toFixed(2)}
                </span>
              </div>

              {/* Total Power */}
              <div className="flex items-center justify-between py-2">
                <span className="text-sm text-on-surface-variant font-body flex items-center gap-2">
                  <span className="material-symbols-outlined text-lg text-primary">bolt</span>
                  {t('locations.total_power')}
                </span>
                <span className="text-lg font-bold text-on-surface font-headline">{formatNumber(SUMMARY_KPI.totalPowerKW)} kW</span>
              </div>

              {/* Critical Alerts */}
              <div className="flex items-center justify-between py-2">
                <span className="text-sm text-on-surface-variant font-body flex items-center gap-2">
                  <span className="material-symbols-outlined text-lg text-error">warning</span>
                  {t('locations.kpi_critical_alerts')}
                </span>
                <span className={`text-lg font-bold font-headline ${(stats?.critical_alerts ?? 0) > 0 ? 'text-error' : 'text-on-surface'}`}>
                  {stats?.critical_alerts ?? 0}
                </span>
              </div>

              {/* Global Uptime */}
              <div className="flex items-center justify-between py-2">
                <span className="text-sm text-on-surface-variant font-body flex items-center gap-2">
                  <span className="material-symbols-outlined text-lg text-green-400">check_circle</span>
                  {t('locations.global_uptime')}
                </span>
                <span className="text-lg font-bold text-green-400 font-headline">{SUMMARY_KPI.globalUptime}%</span>
              </div>

              {/* Energy Trend */}
              <div className="flex items-center justify-between py-2">
                <span className="text-sm text-on-surface-variant font-body flex items-center gap-2">
                  <span className="material-symbols-outlined text-lg text-primary">trending_down</span>
                  {t('locations.energy_trend')}
                </span>
                <Sparkline data={SUMMARY_KPI.energyTrend} color="#4ade80" width={100} height={28} />
              </div>
            </div>
          </div>
        </section>

        {/* --------------------------------------------------------------- */}
        {/* Territory Cards                                                  */}
        {/* --------------------------------------------------------------- */}
        <section aria-label="Territory Overview">
          <h2 className="font-headline text-sm font-bold uppercase tracking-wider text-on-surface-variant mb-4">
            <span className="material-symbols-outlined text-base align-middle mr-1.5">flag</span>
            {t('locations.territory_overview')}
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {TERRITORIES.map((territory) => (
              <TerritoryCard
                key={territory.slug}
                territory={territory}
                onClick={() => navigate(`/locations/${territory.slug}`)}
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
              {t('locations.alert_stream').toUpperCase()}
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
