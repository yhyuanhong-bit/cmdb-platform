import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useRacks, useRootLocations, useLocationChildren } from "../hooks/useTopology";
import { useLocationContext } from "../contexts/LocationContext";
import type { Rack } from "../lib/api/topology";

const recentEvents = [
  {
    time: "14:32",
    icon: "warning",
    text: "RACK-B01 temperature exceeds threshold (38.2°C)",
    severity: "error",
  },
  {
    time: "13:18",
    icon: "swap_vert",
    text: "Asset moved from RACK-A01 U12 to RACK-A02 U30",
    severity: "info",
  },
  {
    time: "11:45",
    icon: "check_circle",
    text: "RACK-C01 maintenance completed successfully",
    severity: "success",
  },
  {
    time: "09:02",
    icon: "add_circle",
    text: "New asset provisioned in RACK-A01 U35-U38",
    severity: "info",
  },
  {
    time: "08:15",
    icon: "power",
    text: "PDU firmware updated on RACK-A02",
    severity: "info",
  },
];

const rackA01Layout: Array<{
  startU: number;
  endU: number;
  label: string;
  color: string;
}> = [
  { startU: 1, endU: 2, label: "PDU / Cable Mgmt", color: "bg-surface-container-highest" },
  { startU: 3, endU: 6, label: "DELL R750xs #1", color: "bg-on-primary-container/30" },
  { startU: 7, endU: 10, label: "DELL R750xs #2", color: "bg-on-primary-container/30" },
  { startU: 11, endU: 12, label: "Cisco C9300-48T", color: "bg-tertiary-container" },
  { startU: 15, endU: 18, label: "HP DL380 Gen10", color: "bg-on-primary-container/30" },
  { startU: 20, endU: 23, label: "DELL R750xs #3", color: "bg-on-primary-container/30" },
  { startU: 25, endU: 28, label: "Storage: NetApp AFF", color: "bg-secondary-container" },
  { startU: 30, endU: 33, label: "DELL R750xs #4", color: "bg-on-primary-container/30" },
  { startU: 35, endU: 38, label: "DELL PowerEdge R750", color: "bg-on-primary-container/30" },
  { startU: 40, endU: 42, label: "UPS / Power Dist", color: "bg-surface-container-highest" },
];

function getStatusStyle(status: string) {
  switch (status) {
    case "OPERATIONAL":
      return "bg-on-primary-container/20 text-primary";
    case "MAINTENANCE":
      return "bg-error-container/40 text-error";
    case "DECOMMISSIONED":
      return "bg-surface-container-highest text-on-surface-variant";
    default:
      return "bg-surface-container-highest text-on-surface-variant";
  }
}

export default function RackManagement() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [search, setSearch] = useState("");
  const { path } = useLocationContext();
  const contextLocationId = path.idc?.id ?? path.campus?.id ?? "";

  // If no location selected in context, find the first campus as default
  const rootQ = useRootLocations();
  const firstCountryId = rootQ.data?.data?.[0]?.id ?? "";
  const countryChildrenQ = useLocationChildren(contextLocationId ? "" : firstCountryId);
  const firstRegionId = countryChildrenQ.data?.data?.[0]?.id ?? "";
  const regionChildrenQ = useLocationChildren(contextLocationId ? "" : firstRegionId);
  const firstCityId = regionChildrenQ.data?.data?.[0]?.id ?? "";
  const cityChildrenQ = useLocationChildren(contextLocationId ? "" : firstCityId);
  const firstCampusId = cityChildrenQ.data?.data?.[0]?.id ?? "";

  const locationId = contextLocationId || firstCampusId;
  const { data: racksResponse, isLoading: racksLoading, error } = useRacks(locationId);
  const isLoading = racksLoading || (!contextLocationId && (rootQ.isLoading || countryChildrenQ.isLoading || regionChildrenQ.isLoading || cityChildrenQ.isLoading));
  const racks: Rack[] = racksResponse?.data ?? [];

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
        <p className="text-error text-sm">Failed to load racks</p>
      </div>
    );
  }

  const filteredRacks = racks.filter(
    (r) =>
      r.id.toLowerCase().includes(search.toLowerCase()) ||
      r.name.toLowerCase().includes(search.toLowerCase()) ||
      r.status.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      {/* Breadcrumb */}
      <div className="bg-surface-container-low px-8 py-3">
        <div className="flex items-center gap-2 text-sm text-on-surface-variant">
          <span className="material-symbols-outlined text-[16px]">home</span>
          <span>IDC Alpha</span>
          <span className="material-symbols-outlined text-[14px]">chevron_right</span>
          <span>Module 1</span>
          <span className="material-symbols-outlined text-[14px]">chevron_right</span>
          <span className="text-primary">{t('racks.breadcrumb_rack_management')}</span>
        </div>
      </div>

      <div className="px-8 py-6">
        {/* Header */}
        <div className="flex items-start justify-between mb-8">
          <div>
            <h1 className="font-headline text-3xl font-bold tracking-tight text-on-surface mb-1">
              {t('racks.title_zh')}
            </h1>
            <p className="text-on-surface-variant text-sm tracking-widest uppercase">
              {t('racks.subtitle')}
            </p>
          </div>
          <button
            onClick={() => navigate('/racks/add')}
            className="machined-gradient text-on-primary font-label font-semibold text-sm px-5 py-2.5 rounded flex items-center gap-2 hover:opacity-90 transition-opacity cursor-pointer"
          >
            <span className="material-symbols-outlined text-[18px]">add</span>
            {t('racks.add_rack')}
          </button>
        </div>

        {/* Search + Stats */}
        <div className="flex items-center gap-6 mb-6">
          <div className="flex-1 relative">
            <span className="material-symbols-outlined absolute left-3 top-1/2 -translate-y-1/2 text-on-surface-variant text-[20px]">
              search
            </span>
            <input
              type="text"
              placeholder={t('racks.search_placeholder')}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full bg-surface-container-high text-on-surface placeholder:text-on-surface-variant/50 pl-10 pr-4 py-2.5 rounded text-sm font-body focus:outline-none focus:ring-1 focus:ring-primary/40"
            />
          </div>
          <div className="flex gap-6">
            <div className="bg-surface-container px-5 py-3 rounded">
              <p className="text-[11px] text-on-surface-variant uppercase tracking-widest mb-0.5">
                {t('racks.total_racks')}
              </p>
              <p className="text-2xl font-headline font-bold text-primary">{racks.length}</p>
            </div>
            <div className="bg-surface-container px-5 py-3 rounded">
              <p className="text-[11px] text-on-surface-variant uppercase tracking-widest mb-0.5">
                {t('racks.avg_occupancy')}
              </p>
              <p className="text-2xl font-headline font-bold text-tertiary">
                {racks.length > 0
                  ? Math.round(racks.reduce((sum, r) => sum + (r.total_u > 0 ? (r.used_u / r.total_u) * 100 : 0), 0) / racks.length)
                  : 0}%
              </p>
            </div>
          </div>
        </div>

        {/* Table */}
        <div className="bg-surface-container rounded overflow-hidden mb-8">
          <table className="w-full text-sm" role="table">
            <thead>
              <tr className="bg-surface-container-high text-on-surface-variant text-[11px] uppercase tracking-widest">
                <th className="text-left px-5 py-3 font-medium">{t('racks.table_rack_id_name')}</th>
                <th className="text-left px-5 py-3 font-medium">{t('racks.table_location')}</th>
                <th className="text-left px-5 py-3 font-medium">{t('racks.table_u_occupancy')}</th>
                <th className="text-left px-5 py-3 font-medium">{t('racks.table_power_kw')}</th>
                <th className="text-left px-5 py-3 font-medium">{t('racks.table_load_pct')}</th>
                <th className="text-left px-5 py-3 font-medium">{t('racks.table_status')}</th>
                <th className="text-right px-5 py-3 font-medium">{t('racks.table_actions')}</th>
              </tr>
            </thead>
            <tbody>
              {filteredRacks.map((rack) => {
                const occupancy = rack.total_u > 0 ? Math.round((rack.used_u / rack.total_u) * 100) : 0;
                const load = rack.power_capacity_kw > 0 ? Math.round((rack.power_current_kw / rack.power_capacity_kw) * 100) : 0;
                return (
                  <tr
                    key={rack.id}
                    onClick={() => navigate(`/racks/${rack.id}`)}
                    className="bg-surface-container hover:bg-surface-container-high transition-colors cursor-pointer"
                  >
                    <td className="px-5 py-3.5">
                      <p className="font-headline font-semibold text-on-surface">{rack.id}</p>
                      <p className="text-[11px] text-on-surface-variant">{rack.name}</p>
                    </td>
                    <td className="px-5 py-3.5 text-on-surface-variant">
                      <div className="flex items-center gap-1.5">
                        <span className="material-symbols-outlined text-[16px]">location_on</span>
                        {rack.row_label}
                      </div>
                    </td>
                    <td className="px-5 py-3.5">
                      <div className="flex items-center gap-3">
                        <span className="text-on-surface">
                          {rack.used_u}/{rack.total_u}U
                        </span>
                        <div className="w-20 h-1.5 bg-surface-container-lowest rounded-full overflow-hidden">
                          <div
                            className={`h-full rounded-full ${
                              occupancy >= 100
                                ? "bg-error"
                                : occupancy >= 80
                                  ? "bg-tertiary"
                                  : "bg-primary"
                            }`}
                            style={{ width: `${Math.min(occupancy, 100)}%` }}
                          />
                        </div>
                      </div>
                    </td>
                    <td className="px-5 py-3.5">
                      <span className="text-on-surface">{rack.power_current_kw}</span>
                      <span className="text-on-surface-variant"> / {rack.power_capacity_kw} kW</span>
                    </td>
                    <td className="px-5 py-3.5">
                      <span
                        className={
                          load >= 100
                            ? "text-error font-semibold"
                            : load >= 80
                              ? "text-tertiary"
                              : "text-on-surface"
                        }
                      >
                        {load}%
                      </span>
                    </td>
                    <td className="px-5 py-3.5">
                      <span
                        className={`inline-block px-2.5 py-0.5 rounded text-[11px] font-semibold tracking-wider ${getStatusStyle(rack.status)}`}
                      >
                        {t(`racks.status_${rack.status.toLowerCase()}`)}
                      </span>
                    </td>
                    <td className="px-5 py-3.5 text-right">
                      <div className="flex items-center justify-end gap-1">
                        <button
                          className="p-1.5 rounded hover:bg-surface-container-highest transition-colors text-on-surface-variant hover:text-primary"
                          aria-label={`View ${rack.id}`}
                        >
                          <span className="material-symbols-outlined text-[18px]">visibility</span>
                        </button>
                        <button
                          className="p-1.5 rounded hover:bg-surface-container-highest transition-colors text-on-surface-variant hover:text-primary"
                          aria-label={`Edit ${rack.id}`}
                        >
                          <span className="material-symbols-outlined text-[18px]">edit</span>
                        </button>
                        <button
                          className="p-1.5 rounded hover:bg-surface-container-highest transition-colors text-on-surface-variant hover:text-primary"
                          aria-label={`More options for ${rack.id}`}
                        >
                          <span className="material-symbols-outlined text-[18px]">more_vert</span>
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>

        {/* Bottom: Layout Viz + Recent Events */}
        <div className="grid grid-cols-3 gap-6">
          {/* Rack Layout Visualization */}
          <div className="col-span-2 bg-surface-container rounded p-5">
            <div className="flex items-center gap-3 mb-4">
              <span className="material-symbols-outlined text-primary">grid_view</span>
              <h2 className="font-headline font-bold text-sm tracking-widest uppercase text-on-surface">
                {t('racks.layout_visualization')}: RACK-A01
              </h2>
            </div>
            <div className="bg-surface-container-low rounded p-4">
              <div className="flex flex-col gap-px">
                {Array.from({ length: 42 }, (_, i) => {
                  const u = 42 - i;
                  const equipment = rackA01Layout.find(
                    (eq) => u >= eq.startU && u <= eq.endU
                  );
                  const isStart = equipment && u === equipment.endU;
                  const span = equipment
                    ? equipment.endU - equipment.startU + 1
                    : 0;

                  if (equipment && !isStart) {
                    return null;
                  }

                  if (equipment && isStart) {
                    return (
                      <div
                        key={u}
                        className={`flex items-center ${equipment.color} rounded`}
                        style={{ height: `${span * 22}px` }}
                      >
                        <span className="text-[10px] text-on-surface-variant w-10 text-right pr-2 shrink-0">
                          U{equipment.startU}-{equipment.endU}
                        </span>
                        <div className="flex-1 px-3">
                          <span className="text-xs font-label font-medium text-on-surface">
                            {equipment.label}
                          </span>
                        </div>
                      </div>
                    );
                  }

                  return (
                    <div
                      key={u}
                      className="flex items-center bg-surface-container-lowest rounded"
                      style={{ height: "22px" }}
                    >
                      <span className="text-[10px] text-on-surface-variant/40 w-10 text-right pr-2 shrink-0">
                        U{u}
                      </span>
                      <div className="flex-1 px-3">
                        <span className="text-[10px] text-on-surface-variant/20">{t('common.empty')}</span>
                      </div>
                    </div>
                  );
                }).filter(Boolean)}
              </div>
            </div>
          </div>

          {/* Recent Events */}
          <div className="bg-surface-container rounded p-5">
            <div className="flex items-center gap-3 mb-4">
              <span className="material-symbols-outlined text-primary">notifications</span>
              <h2 className="font-headline font-bold text-sm tracking-widest uppercase text-on-surface">
                {t('racks.recent_events')}
              </h2>
            </div>
            <div className="flex flex-col gap-1">
              {recentEvents.map((event, i) => (
                <div
                  key={i}
                  className="bg-surface-container-low rounded p-3 flex items-start gap-3"
                >
                  <span
                    className={`material-symbols-outlined text-[18px] mt-0.5 ${
                      event.severity === "error"
                        ? "text-error"
                        : event.severity === "success"
                          ? "text-primary"
                          : "text-on-surface-variant"
                    }`}
                  >
                    {event.icon}
                  </span>
                  <div className="flex-1 min-w-0">
                    <p className="text-xs text-on-surface leading-relaxed">{event.text}</p>
                    <p className="text-[10px] text-on-surface-variant mt-1">Today {event.time}</p>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
