import { memo, useState, useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useRacks } from "../hooks/useTopology";
import { useLocationContext } from "../contexts/LocationContext";
import type { Rack as ApiRack } from "../lib/api/topology";

/* ──────────────────────────────────────────────
   Types & styling
   ────────────────────────────────────────────── */

type RackStatus = "normal" | "warning" | "critical";

interface Rack {
  id: string;
  row: string;
  position: number;
  status: RackStatus;
  load: number;      // percentage
  temp: number;       // celsius
  assets: number;
  powerDraw: string;
}

function apiRackToLocal(r: ApiRack, idx: number): Rack {
  const loadPct = r.power_capacity_kw > 0 ? Math.round((r.power_current_kw / r.power_capacity_kw) * 100) : 0;
  let status: RackStatus = "normal";
  if (r.status === "MAINTENANCE" || loadPct >= 90) status = "critical";
  else if (loadPct >= 70) status = "warning";
  return {
    id: r.name || r.id.slice(0, 6),
    row: r.row_label || "A",
    position: idx + 1,
    status,
    load: loadPct,
    temp: 20 + ((r.id.charCodeAt(0) * 3) % 12),
    assets: r.used_u,
    powerDraw: `${r.power_current_kw.toFixed(1)} kW`,
  };
}

const STATUS_COLORS: Record<RackStatus, string> = {
  normal: "bg-[#166534]",
  warning: "bg-[#854d0e]",
  critical: "bg-[#991b1b]",
};

const STATUS_HOVER: Record<RackStatus, string> = {
  normal: "hover:bg-[#15803d]",
  warning: "hover:bg-[#a16207]",
  critical: "hover:bg-[#b91c1c]",
};

const STATUS_LABEL_COLORS: Record<RackStatus, string> = {
  normal: "text-[#34d399]",
  warning: "text-[#fbbf24]",
  critical: "text-[#ff6b6b]",
};

const STATUS_DOT_COLORS: Record<RackStatus, string> = {
  normal: "bg-[#34d399]",
  warning: "bg-[#fbbf24]",
  critical: "bg-[#ff6b6b]",
};

function buildRows(apiRacks: ApiRack[]): { label: string; racks: Rack[] }[] {
  const rowMap = new Map<string, Rack[]>();
  apiRacks.forEach((r, i) => {
    const local = apiRackToLocal(r, i);
    const key = local.row;
    if (!rowMap.has(key)) rowMap.set(key, []);
    rowMap.get(key)!.push(local);
  });
  return Array.from(rowMap.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([label, racks]) => ({ label: `Row ${label}`, racks }));
}

/* ──────────────────────────────────────────────
   Small reusable pieces
   ────────────────────────────────────────────── */

function Icon({ name, className = "" }: { name: string; className?: string }) {
  return (
    <span className={`material-symbols-outlined ${className}`}>{name}</span>
  );
}

/* ──────────────────────────────────────────────
   Main Page
   ────────────────────────────────────────────── */

function FacilityMap() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { path } = useLocationContext();
  // Fallback to Neihu campus if no location context set
  const locationId = path.idc?.id ?? path.campus?.id ?? "d0000000-0000-0000-0000-000000000004";
  const { data: racksResponse, isLoading, error } = useRacks(locationId);
  const apiRacks: ApiRack[] = racksResponse?.data ?? [];
  const ROWS = useMemo(() => buildRows(apiRacks), [apiRacks]);
  const ALL_RACKS = useMemo(() => ROWS.flatMap((r) => r.racks), [ROWS]);
  const TOTAL_RACKS = ALL_RACKS.length;
  const NORMAL_COUNT = ALL_RACKS.filter((r) => r.status === "normal").length;
  const WARNING_COUNT = ALL_RACKS.filter((r) => r.status === "warning").length;
  const CRITICAL_COUNT = ALL_RACKS.filter((r) => r.status === "critical").length;
  const [selectedRack, setSelectedRack] = useState<Rack | null>(null);

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
        <p className="text-error text-sm">Failed to load facility data</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-surface px-6 py-5 font-body">
      {/* Breadcrumb */}
      <nav
        aria-label="Breadcrumb"
        className="flex items-center gap-1.5 text-xs uppercase tracking-widest text-on-surface-variant"
      >
        {["FACILITY_NODE_09", "FLOOR_PLAN", "RACK_LOCATION"].map(
          (crumb, i, arr) => (
            <span key={crumb} className="flex items-center gap-1.5">
              <span className="cursor-pointer transition-colors hover:text-primary" onClick={() => navigate('/racks')}>
                {crumb}
              </span>
              {i < arr.length - 1 && (
                <Icon name="chevron_right" className="text-[14px] opacity-40" />
              )}
            </span>
          ),
        )}
      </nav>

      {/* Title */}
      <h1 className="mt-4 font-headline text-2xl font-bold text-on-surface">
        {t('facility_map.title')}
      </h1>

      {/* Room statistics bar */}
      <div className="mt-5 grid grid-cols-2 gap-4 sm:grid-cols-4">
        {[
          { icon: "square_foot", label: t('facility_map.total_area'), value: "2,400 sq ft" },
          { icon: "thermostat", label: t('facility_map.cooling_zones'), value: "4 zones" },
          { icon: "electrical_services", label: t('facility_map.power_distribution'), value: "3 PDU" },
          { icon: "dns", label: t('facility_map.total_racks'), value: `${TOTAL_RACKS} ${t('facility_map.units')}` },
        ].map((s) => (
          <div key={s.label} className="rounded-lg bg-surface-container p-4">
            <div className="flex items-center gap-2 text-on-surface-variant">
              <Icon name={s.icon} className="text-lg" />
              <span className="text-[10px] font-semibold uppercase tracking-widest">
                {s.label}
              </span>
            </div>
            <p className="mt-1 font-headline text-lg font-bold text-on-surface">
              {s.value}
            </p>
          </div>
        ))}
      </div>

      {/* Main content: floor plan + side panel */}
      <div className="mt-5 flex flex-col gap-4 lg:flex-row">
        {/* Floor plan */}
        <div className="flex-1 rounded-lg bg-surface-container p-5">
          <div className="mb-4 flex items-center gap-2">
            <Icon name="map" className="text-primary text-xl" />
            <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
              {t('facility_map.data_center_floor_plan')}
            </h3>
          </div>

          {/* Legend */}
          <div className="mb-5 flex flex-wrap items-center gap-5">
            {[
              { status: "normal" as RackStatus, label: "Normal", count: NORMAL_COUNT },
              { status: "warning" as RackStatus, label: "Warning", count: WARNING_COUNT },
              { status: "critical" as RackStatus, label: "Critical", count: CRITICAL_COUNT },
            ].map((item) => (
              <div key={item.label} className="flex items-center gap-2">
                <span
                  className={`h-3 w-3 rounded-sm ${STATUS_DOT_COLORS[item.status]}`}
                />
                <span className="text-xs text-on-surface-variant">
                  {item.label}{" "}
                  <span className="font-semibold text-on-surface">
                    ({item.count})
                  </span>
                </span>
              </div>
            ))}
          </div>

          {/* Rack grid */}
          <div className="space-y-5">
            {ROWS.map((row) => (
              <div key={row.label}>
                <span className="mb-2 block text-[10px] font-bold uppercase tracking-widest text-on-surface-variant">
                  {row.label}
                </span>
                <div className="flex flex-wrap gap-2">
                  {row.racks.map((rack) => {
                    const isSelected = selectedRack?.id === rack.id;
                    return (
                      <button
                        key={rack.id}
                        type="button"
                        onClick={() => setSelectedRack(rack)}
                        className={`flex h-16 w-14 flex-col items-center justify-center rounded-md transition-all ${STATUS_COLORS[rack.status]} ${STATUS_HOVER[rack.status]} ${
                          isSelected
                            ? "ring-2 ring-primary ring-offset-2 ring-offset-surface-container"
                            : ""
                        }`}
                        aria-label={`Rack ${rack.id}, status ${rack.status}`}
                      >
                        <span className="text-[10px] font-bold text-white/90">
                          {rack.id}
                        </span>
                        <span className="text-[9px] text-white/60">
                          {rack.load}%
                        </span>
                      </button>
                    );
                  })}
                </div>
              </div>
            ))}
          </div>

          {/* Floor markers */}
          <div className="mt-6 flex items-center gap-6">
            <div className="flex items-center gap-2 text-on-surface-variant">
              <Icon name="door_front" className="text-sm" />
              <span className="text-[10px] uppercase tracking-wider">{t('facility_map.entry')}</span>
            </div>
            <div className="flex items-center gap-2 text-on-surface-variant">
              <Icon name="emergency_home" className="text-sm" />
              <span className="text-[10px] uppercase tracking-wider">{t('facility_map.emergency_exit')}</span>
            </div>
            <div className="flex items-center gap-2 text-on-surface-variant">
              <Icon name="ac_unit" className="text-sm" />
              <span className="text-[10px] uppercase tracking-wider">{t('facility_map.cooling_unit')}</span>
            </div>
          </div>
        </div>

        {/* Side panel */}
        <div className="w-full shrink-0 rounded-lg bg-surface-container p-5 lg:w-80">
          <div className="mb-4 flex items-center gap-2">
            <Icon name="info" className="text-primary text-xl" />
            <h3 className="font-headline text-sm font-semibold uppercase tracking-wider text-on-surface-variant">
              {t('facility_map.rack_details')}
            </h3>
          </div>

          {selectedRack ? (
            <div className="space-y-4">
              {/* Rack header */}
              <div className="rounded-lg bg-surface-container-low p-4">
                <div className="flex items-center justify-between">
                  <span
                    className="font-headline text-xl font-bold cursor-pointer text-primary hover:underline"
                    onClick={() => {
                      const apiRack = apiRacks.find((r) => (r.name || r.id.slice(0, 6)) === selectedRack.id);
                      navigate('/racks/' + (apiRack?.id ?? selectedRack.id));
                    }}
                  >
                    {selectedRack.id}
                  </span>
                  <span
                    className={`flex items-center gap-1.5 rounded px-2.5 py-1 text-[10px] font-bold uppercase tracking-wider ${
                      selectedRack.status === "normal"
                        ? "bg-[#064e3b] text-[#34d399]"
                        : selectedRack.status === "warning"
                          ? "bg-[#92400e] text-[#fbbf24]"
                          : "bg-[#7f1d1d] text-[#ff6b6b]"
                    }`}
                  >
                    <span
                      className={`inline-block h-1.5 w-1.5 rounded-full ${STATUS_DOT_COLORS[selectedRack.status]}`}
                    />
                    {selectedRack.status}
                  </span>
                </div>
                <span className="text-xs text-on-surface-variant">
                  {t('facility_map.row')} {selectedRack.row} / {t('facility_map.position')} {selectedRack.position}
                </span>
              </div>

              {/* Stats */}
              {[
                { icon: "speed", label: t('facility_map.load'), value: `${selectedRack.load}%` },
                { icon: "thermostat", label: t('facility_map.temperature'), value: `${selectedRack.temp}°C` },
                { icon: "dns", label: t('common.assets'), value: `${selectedRack.assets} ${t('facility_map.units')}` },
                { icon: "bolt", label: t('facility_map.power_draw'), value: selectedRack.powerDraw },
              ].map((detail) => (
                <div
                  key={detail.label}
                  className="flex items-center gap-3 rounded-md bg-surface-container-low px-4 py-3"
                >
                  <Icon
                    name={detail.icon}
                    className="text-lg text-on-surface-variant"
                  />
                  <div className="flex-1">
                    <span className="block text-[10px] uppercase tracking-wider text-on-surface-variant">
                      {detail.label}
                    </span>
                    <span className="text-sm font-semibold text-on-surface">
                      {detail.value}
                    </span>
                  </div>
                </div>
              ))}

              {/* Load bar */}
              <div>
                <div className="mb-1 flex justify-between">
                  <span className="text-[10px] uppercase tracking-wider text-on-surface-variant">
                    {t('facility_map.capacity_utilization')}
                  </span>
                  <span className={`text-xs font-bold ${STATUS_LABEL_COLORS[selectedRack.status]}`}>
                    {selectedRack.load}%
                  </span>
                </div>
                <div className="h-2 w-full rounded-full bg-surface-container-low">
                  <div
                    className={`h-2 rounded-full transition-all duration-500 ${
                      selectedRack.status === "normal"
                        ? "bg-[#34d399]"
                        : selectedRack.status === "warning"
                          ? "bg-[#fbbf24]"
                          : "bg-[#ff6b6b]"
                    }`}
                    style={{ width: `${selectedRack.load}%` }}
                  />
                </div>
              </div>

              {/* Actions */}
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => {
                    const apiRack = apiRacks.find((r) => (r.name || r.id.slice(0, 6)) === selectedRack?.id);
                    navigate('/racks/' + (apiRack?.id ?? selectedRack?.id));
                  }}
                  className="flex flex-1 items-center justify-center gap-1.5 rounded-md bg-primary px-3 py-2 text-[10px] font-bold uppercase tracking-wider text-on-primary-container transition-colors hover:brightness-110"
                >
                  <Icon name="visibility" className="text-sm" />
                  {t('common.view_details')}
                </button>
                <button
                  type="button"
                  onClick={() => navigate('/maintenance/add')}
                  className="flex flex-1 items-center justify-center gap-1.5 rounded-md bg-surface-container-low px-3 py-2 text-[10px] font-bold uppercase tracking-wider text-on-surface-variant transition-colors hover:bg-surface-container-high"
                >
                  <Icon name="build" className="text-sm" />
                  {t('common.maintenance')}
                </button>
              </div>
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center py-16 text-center">
              <Icon
                name="touch_app"
                className="mb-3 text-4xl text-on-surface-variant/40"
              />
              <p className="text-sm text-on-surface-variant">
                {t('facility_map.select_rack_prompt')}
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default memo(FacilityMap);
