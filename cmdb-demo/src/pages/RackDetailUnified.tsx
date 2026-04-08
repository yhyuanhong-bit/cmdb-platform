import { useState, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { useRack, useRackAssets, useRackSlots, useUpdateRack, useDeleteRack, useRackNetworkConnections, useDeleteNetworkConnection } from "../hooks/useTopology";
import AssignAssetToRackModal from '../components/AssignAssetToRackModal';
import AddNetworkConnectionModal from '../components/AddNetworkConnectionModal';
import { useAlerts } from "../hooks/useMonitoring";
import { useActivityFeed } from "../hooks/useActivityFeed";
import { apiClient } from "../lib/api/client";
import { useMetrics } from "../hooks/useMetrics";

// ---------------------------------------------------------------------------
// Shared types & data (equipment slots — no API for sub-rack assets yet)
// ---------------------------------------------------------------------------

interface Equipment {
  startU: number;
  endU: number;
  label: string;
  assetTag: string;
  type: "compute" | "network" | "storage" | "power" | "empty";
}

// Equipment list is now derived from API data only (no hardcoded fallback)


// environmentMetrics is computed inside the main component from API data


// Console U-slot data
type SlotType = "pdu" | "compute" | "network" | "storage" | "ups" | "empty" | "warning";

interface USlot {
  startU: number;
  endU: number;
  label: string;
  type: SlotType;
  warning?: boolean;
}

const uSlots: USlot[] = [
  { startU: 39, endU: 42, label: "PDU-A-MANAGED", type: "pdu" },
  { startU: 35, endU: 38, label: "COMPUTE-NODE-01", type: "compute" },
  { startU: 31, endU: 34, label: "COMPUTE-NODE-02", type: "compute" },
  { startU: 27, endU: 30, label: "NEXUS-C93180YC", type: "network" },
  { startU: 25, endU: 26, label: "PATCH-PANEL-48P", type: "network" },
  { startU: 22, endU: 24, label: "COMPUTE-NODE-03", type: "warning", warning: true },
  { startU: 17, endU: 21, label: "VIRT-CLUSTER-01", type: "compute" },
  { startU: 13, endU: 16, label: "ALL-FLASH-STORAGE-01", type: "storage" },
  { startU: 9, endU: 12, label: "ALL-FLASH-STORAGE-02", type: "storage" },
  { startU: 5, endU: 8, label: "BACKUP-APPLIANCE", type: "storage" },
  { startU: 1, endU: 4, label: "UPS-BACKUP-SYSTEM", type: "ups" },
];

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

function getTypeColor(type: Equipment["type"]) {
  switch (type) {
    case "compute": return "bg-on-primary-container/25";
    case "network": return "bg-tertiary-container/60";
    case "storage": return "bg-secondary-container/60";
    case "power":   return "bg-surface-container-highest";
    default:        return "bg-surface-container-lowest";
  }
}

function getTypeAccent(type: Equipment["type"]) {
  switch (type) {
    case "compute": return "bg-on-primary-container";
    case "network": return "bg-on-tertiary-container";
    case "storage": return "bg-secondary";
    case "power":   return "bg-on-surface-variant";
    default:        return "bg-surface-container-highest";
  }
}

function getSlotColor(type: SlotType): string {
  switch (type) {
    case "pdu":     return "bg-surface-container-highest/80";
    case "compute": return "bg-on-primary-container/25";
    case "network": return "bg-primary/25";
    case "storage": return "bg-emerald-500/20";
    case "ups":     return "bg-secondary/20";
    case "warning": return "bg-tertiary/25";
    default:        return "bg-surface-container-low";
  }
}

function getSlotAccent(type: SlotType): string {
  switch (type) {
    case "pdu":     return "bg-on-surface-variant";
    case "compute": return "bg-on-primary-container";
    case "network": return "bg-primary";
    case "storage": return "bg-emerald-500";
    case "ups":     return "bg-secondary";
    case "warning": return "bg-tertiary";
    default:        return "bg-surface-container-highest";
  }
}

// ---------------------------------------------------------------------------
// Tab 1: Visualization
// ---------------------------------------------------------------------------

// BIA tier colors for rack slots
const SLOT_BIA_COLORS: Record<string, string> = {
  critical: 'bg-error-container text-on-error-container',
  important: 'bg-[#92400e] text-[#fbbf24]',
  normal: 'bg-[#1e3a5f] text-on-primary-container',
  minor: 'bg-surface-container-highest text-on-surface-variant',
}

interface LiveAlert {
  severity: string;
  text: string;
  time: string;
}

function VisualizationTab({
  selectedAsset,
  setSelectedAsset,
  equipmentList,
  rackSlots,
  totalU,
  liveAlerts,
}: {
  selectedAsset: Equipment | null;
  setSelectedAsset: (eq: Equipment | null) => void;
  equipmentList?: Equipment[];
  rackSlots?: any[];
  totalU?: number;
  liveAlerts?: LiveAlert[];
}) {
  const eqList = equipmentList ?? [];
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [view, setView] = useState<"FRONT" | "REAR">("FRONT");

  const hasSlots = rackSlots && rackSlots.length > 0;
  const gridU = totalU || 42;
  const uPositions = Array.from({ length: gridU }, (_, i) => gridU - i);
  const viewSlots = hasSlots
    ? rackSlots.filter((s: any) => (s.side || 'front') === view.toLowerCase())
    : [];

  return (
    <div>
      {/* View toggle */}
      <div className="flex items-center justify-between mb-6">
        {hasSlots ? (
          <div className="flex flex-wrap gap-3 text-[10px] text-on-surface-variant">
            <div className="flex items-center gap-1.5">
              <span className="w-3 h-3 rounded-sm bg-error-container" />
              <span>Critical</span>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="w-3 h-3 rounded-sm bg-[#92400e]" />
              <span>Important</span>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="w-3 h-3 rounded-sm bg-[#1e3a5f]" />
              <span>Normal</span>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="w-3 h-3 rounded-sm bg-surface-container-highest" />
              <span>Minor</span>
            </div>
          </div>
        ) : (
          <div className="flex items-center gap-2">
            {(["compute", "network", "storage", "power"] as const).map((type) => (
              <div key={type} className="flex items-center gap-1.5">
                <div className={`w-3 h-3 rounded-sm ${getTypeAccent(type)}`} />
                <span className="text-xs text-on-surface-variant capitalize">{type}</span>
              </div>
            ))}
          </div>
        )}
        <div className="flex items-center gap-2">
          {(["FRONT", "REAR"] as const).map((v) => (
            <button
              key={v}
              onClick={() => setView(v)}
              className={`px-4 py-2 rounded text-xs font-semibold tracking-wider transition-colors ${
                view === v
                  ? "bg-primary/20 text-primary"
                  : "bg-surface-container-high text-on-surface-variant hover:text-on-surface"
              }`}
            >
              {v}
            </button>
          ))}
        </div>
      </div>

      {/* Main grid */}
      <div className="grid grid-cols-12 gap-6">
        {/* Rack diagram */}
        <div className="col-span-7">
          <div className="bg-surface-container rounded p-4">
            <div className="flex items-center gap-2 mb-3">
              <span className="material-symbols-outlined text-primary text-[18px]">dns</span>
              <h2 className="text-xs font-headline font-bold tracking-widest uppercase text-on-surface-variant">
                RACK &mdash; {view} VIEW &mdash; {gridU}U
              </h2>
            </div>

            {hasSlots ? (
              /* Real slot-based rendering with BIA colors */
              <div className="border-2 border-outline-variant/30 rounded-lg bg-surface-container-low p-1">
                {uPositions.map(u => {
                  const slot = viewSlots.find((s: any) => u >= s.start_u && u <= s.end_u);
                  const isTopU = slot && u === slot.end_u;

                  return (
                    <div key={u} className="flex h-6 border-b border-outline-variant/10">
                      <div className="w-8 text-center text-[10px] text-on-surface-variant/50 leading-6 border-r border-outline-variant/10">
                        {u}
                      </div>
                      <div className="flex-1 relative">
                        {slot ? (
                          isTopU && (
                            <div
                              className={`absolute inset-x-0 z-10 m-px rounded flex items-center justify-center text-[10px] font-bold tracking-wide
                                ${SLOT_BIA_COLORS[slot.bia_level] || SLOT_BIA_COLORS.normal}`}
                              style={{ height: `${(slot.end_u - slot.start_u + 1) * 24 - 4}px` }}
                              title={`${slot.asset_name} (${slot.asset_tag}) — BIA: ${slot.bia_level}`}
                            >
                              {slot.asset_name || slot.asset_tag}
                            </div>
                          )
                        ) : (
                          <span className="text-[9px] text-on-surface-variant/20 ml-2 leading-6">&mdash;</span>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            ) : eqList.length > 0 ? (
              /* Asset-based rendering from rack assets API */
              <div className="bg-surface-container-low rounded p-3">
                <div className="flex flex-col gap-px">
                  {Array.from({ length: 42 }, (_, i) => {
                    const u = 42 - i;
                    const eq = eqList.find((e) => u >= e.startU && u <= e.endU);
                    const isStart = eq && u === eq.endU;
                    const span = eq ? eq.endU - eq.startU + 1 : 0;

                    if (eq && !isStart) return null;

                    if (eq && isStart) {
                      const isSelected = selectedAsset?.assetTag === eq.assetTag;
                      return (
                        <button
                          key={u}
                          onClick={() => setSelectedAsset(eq)}
                          className={`flex items-center ${getTypeColor(eq.type)} rounded text-left transition-all cursor-pointer ${
                            isSelected ? "ring-1 ring-primary" : ""
                          }`}
                          style={{ height: `${span * 24}px` }}
                        >
                          <div className={`w-1 h-full rounded-l ${getTypeAccent(eq.type)}`} />
                          <span className="text-[10px] text-on-surface-variant w-12 text-right pr-2 shrink-0">
                            U{eq.startU === eq.endU ? eq.startU : `${eq.startU}-${eq.endU}`}
                          </span>
                          <div className="flex-1 px-2 overflow-hidden">
                            <p className="text-xs font-label font-medium text-on-surface truncate">{eq.label}</p>
                            <p className="text-[10px] text-on-surface-variant truncate">{eq.assetTag}</p>
                          </div>
                        </button>
                      );
                    }

                    return (
                      <div
                        key={u}
                        className="flex items-center bg-surface-container-lowest rounded"
                        style={{ height: "24px" }}
                      >
                        <div className="w-1 h-full" />
                        <span className="text-[10px] text-on-surface-variant/30 w-12 text-right pr-2 shrink-0">
                          U{u}
                        </span>
                        <div className="flex-1 px-2">
                          <span className="text-[10px] text-on-surface-variant/15 tracking-widest">
                            {t("common.vacant")}
                          </span>
                        </div>
                      </div>
                    );
                  }).filter(Boolean)}
                </div>
              </div>
            ) : (
              /* No equipment data available */
              <div className="bg-surface-container-low rounded p-8 text-center text-on-surface-variant">
                <span className="material-symbols-outlined text-[36px] mb-2 block opacity-30">inventory_2</span>
                <p className="text-sm">No equipment</p>
                <p className="text-[10px] mt-1 opacity-60">No assets have been assigned to this rack yet</p>
              </div>
            )}
          </div>
        </div>

        {/* Right panel */}
        <div className="col-span-5 flex flex-col gap-6">
          {/* Selected asset detail */}
          <div className="bg-surface-container rounded p-5">
            <div className="flex items-center gap-2 mb-4">
              <span className="material-symbols-outlined text-primary text-[18px]">memory</span>
              <h2 className="text-xs font-headline font-bold tracking-widest uppercase text-on-surface-variant">
                {t("rack_visualization.selected_asset")}
              </h2>
            </div>
            {selectedAsset ? (
              <div>
                <div className="bg-surface-container-low rounded p-4 mb-4">
                  <p className="font-headline font-bold text-lg text-on-surface cursor-pointer text-primary hover:underline" onClick={() => navigate(`/assets/${selectedAsset.assetTag}`)}>{selectedAsset.assetTag}</p>
                  <p className="text-sm text-on-surface-variant">{selectedAsset.label}</p>
                  <p className="text-xs text-on-surface-variant mt-1">
                    U{selectedAsset.startU}
                    {selectedAsset.startU !== selectedAsset.endU && `-U${selectedAsset.endU}`}
                    {" "}&mdash; {selectedAsset.endU - selectedAsset.startU + 1}U
                  </p>
                </div>
                <div className="grid grid-cols-2 gap-3 mb-4">
                  {[
                    { label: "Type", value: "Intel Xeon Platinum 8380" },
                    { label: "Serial", value: "DR-DKY-22619" },
                    { label: "IP", value: "10.28.1.45" },
                    { label: "Power", value: "425W" },
                    { label: "Network", value: "2x 25GbE" },
                    { label: "Storage", value: "4x 1.92TB NVMe" },
                  ].map((item) => (
                    <div key={item.label} className="bg-surface-container-low rounded p-3">
                      <p className="text-[10px] text-on-surface-variant uppercase tracking-widest mb-0.5">{item.label}</p>
                      <p className="text-sm font-medium text-on-surface">{item.value}</p>
                    </div>
                  ))}
                </div>
                <button
                  onClick={() => navigate(`/assets/${selectedAsset?.assetTag ?? ''}`)}
                  className="w-full machined-gradient text-on-primary font-label font-semibold text-sm px-5 py-2.5 rounded flex items-center justify-center gap-2 hover:opacity-90 transition-opacity cursor-pointer"
                >
                  查看資產詳情 →
                </button>
              </div>
            ) : (
              <div className="text-center py-8 text-on-surface-variant">
                <span className="material-symbols-outlined text-[36px] mb-2 block opacity-30">touch_app</span>
                <p className="text-sm">Select equipment from the rack to view details</p>
              </div>
            )}
          </div>

          {/* Alerts */}
          <div className="bg-surface-container rounded p-5">
            <div className="flex items-center gap-2 mb-4">
              <span className="material-symbols-outlined text-tertiary text-[18px]">notification_important</span>
              <h2 className="text-xs font-headline font-bold tracking-widest uppercase text-on-surface-variant">
                Active Alerts
              </h2>
            </div>
            {liveAlerts && liveAlerts.length > 0 ? (
              <div className="flex flex-col gap-1">
                {liveAlerts.map((alert, i) => (
                  <div key={i} className="bg-surface-container-low rounded p-3 flex items-start gap-3">
                    <span
                      className={`material-symbols-outlined text-[16px] mt-0.5 ${
                        alert.severity === "warning" || alert.severity === "critical"
                          ? "text-tertiary"
                          : "text-on-surface-variant"
                      }`}
                    >
                      {alert.severity === "warning" || alert.severity === "critical" ? "warning" : "info"}
                    </span>
                    <div className="flex-1">
                      <p className="text-xs text-on-surface">{alert.text}</p>
                      <p className="text-[10px] text-on-surface-variant mt-0.5">{alert.time}</p>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-center py-6 text-on-surface-variant">
                <span className="material-symbols-outlined text-[28px] mb-1 block opacity-30">check_circle</span>
                <p className="text-xs">No active alerts</p>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Tab 2: Console
// ---------------------------------------------------------------------------

function GaugeWidget({
  label,
  value,
  unit,
  percentage,
  status,
}: {
  label: string;
  value: string;
  unit: string;
  percentage: number;
  status?: string;
}) {
  const circumference = 2 * Math.PI * 36;
  const dashOffset = circumference - (percentage / 100) * circumference;
  const gaugeColor =
    percentage > 80 ? "stroke-error" : percentage > 60 ? "stroke-tertiary" : "stroke-primary";

  return (
    <div className="flex flex-col items-center">
      <div className="relative w-20 h-20">
        <svg className="w-20 h-20 -rotate-90" viewBox="0 0 80 80">
          <circle cx="40" cy="40" r="36" fill="none" stroke="currentColor" strokeWidth="6" className="text-surface-container-highest" />
          <circle cx="40" cy="40" r="36" fill="none" strokeWidth="6" strokeLinecap="round" strokeDasharray={circumference} strokeDashoffset={dashOffset} className={gaugeColor} />
        </svg>
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <span className="text-sm font-headline font-bold text-on-surface">{value}</span>
          <span className="text-[9px] text-on-surface-variant">{unit}</span>
        </div>
      </div>
      <span className="text-[11px] text-on-surface-variant mt-1.5">{label}</span>
      {status && (
        <span className={`text-[9px] font-semibold mt-0.5 ${status === "NOMINAL" ? "text-emerald-400" : "text-tertiary"}`}>
          {status}
        </span>
      )}
    </div>
  );
}

function ConsoleTab({ recentActivity, slots }: { recentActivity: any[]; slots: USlot[] }) {
  const { t } = useTranslation();
  const [selectedSlot, setSelectedSlot] = useState<USlot | null>(
    slots.find((s) => s.label === "NEXUS-C93180YC") ?? slots[0] ?? null,
  );

  const totalU = 42;
  const uHeight = 22;

  const occupiedUs = new Set<number>();
  slots.forEach((slot) => {
    for (let u = slot.startU; u <= slot.endU; u++) occupiedUs.add(u);
  });
  const occupiedCount = occupiedUs.size;
  const vacantCount = totalU - occupiedCount;

  return (
    <div>
      {/* Configuration + Gauges row */}
      <div className="grid grid-cols-12 gap-6 mb-6">
        {/* Configuration */}
        <div className="col-span-4">
          <h3 className="text-[11px] font-semibold text-on-surface-variant uppercase tracking-wider mb-3">
            Configuration
          </h3>
          <div className="space-y-2 text-xs">
            {[
              { label: "Rack Height", value: "42U" },
              { label: "Max Power Draw", value: "15kW" },
              { label: "Weight Capacity", value: "1200kg" },
            ].map((item) => (
              <div key={item.label} className="flex justify-between bg-surface-container rounded-lg px-3 py-2">
                <span className="text-on-surface-variant">{item.label}</span>
                <span className="text-on-surface font-medium">{item.value}</span>
              </div>
            ))}
          </div>
        </div>

        {/* Gauges */}
        <div className="col-span-4 flex items-center justify-center gap-4">
          <GaugeWidget label="Active Power" value="11.2" unit="kW" percentage={75} />
          <GaugeWidget label="Intake Temp" value="24.5" unit={"\u00b0C"} percentage={52} status="NOMINAL" />
          <GaugeWidget label="Humidity" value="42" unit="%" percentage={42} />
        </div>

        {/* Recent Activity */}
        <div className="col-span-4">
          <h3 className="text-[11px] font-semibold text-on-surface-variant uppercase tracking-wider mb-3">
            Recent Activity
          </h3>
          <div className="space-y-1.5">
            {recentActivity.map((item, i) => (
              <div key={i} className="flex items-start gap-2 py-1">
                <span className="material-symbols-outlined text-sm text-on-surface-variant mt-0.5">{item.icon}</span>
                <div className="min-w-0">
                  <p className="text-[11px] text-on-surface leading-tight truncate">{item.action}</p>
                  <p className="text-[10px] text-on-surface-variant">{item.time}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* U-Position Map + Slot Analytics */}
      <div className="flex gap-6">
        {/* Map */}
        <div className="flex-1">
          <div className="flex items-center gap-3 mb-3">
            <h2 className="text-sm font-headline font-semibold text-on-surface">U-Position Map</h2>
            <div className="flex items-center gap-4 ml-4">
              <div className="flex items-center gap-1.5">
                <div className="w-2.5 h-2.5 rounded bg-on-primary-container/40" />
                <span className="text-[10px] text-on-surface-variant">Occupied ({occupiedCount})</span>
              </div>
              <div className="flex items-center gap-1.5">
                <div className="w-2.5 h-2.5 rounded bg-surface-container-highest/30" />
                <span className="text-[10px] text-on-surface-variant">Vacant ({vacantCount})</span>
              </div>
            </div>
          </div>
          <div className="flex justify-center overflow-y-auto py-4 px-2" style={{ maxHeight: "600px" }}>
            <div className="relative" style={{ width: "380px" }}>
              <div
                className="relative bg-surface-container-lowest rounded-xl overflow-hidden"
                style={{ height: `${totalU * uHeight}px` }}
              >
                {Array.from({ length: totalU }, (_, i) => {
                  const uNumber = totalU - i;
                  const isOccupied = occupiedUs.has(uNumber);
                  return (
                    <div
                      key={uNumber}
                      className="absolute left-0 right-0 flex items-center"
                      style={{ top: `${i * uHeight}px`, height: `${uHeight}px` }}
                    >
                      <div className="w-8 text-center text-[9px] font-mono text-on-surface-variant/30">{uNumber}</div>
                      {!isOccupied && <div className="flex-1 mx-1 h-[calc(100%-2px)] rounded bg-surface-container/30" />}
                    </div>
                  );
                })}
                {slots.map((slot) => {
                  const span = slot.endU - slot.startU + 1;
                  const topOffset = (totalU - slot.endU) * uHeight;
                  const height = span * uHeight;
                  const isSelected = selectedSlot?.label === slot.label;
                  return (
                    <button
                      key={slot.label}
                      onClick={() => setSelectedSlot(slot)}
                      className={`absolute left-8 right-2 rounded-lg flex items-center gap-2 px-3 transition-all cursor-pointer ${getSlotColor(slot.type)} ${
                        isSelected ? "ring-2 ring-primary ring-offset-1 ring-offset-surface-container-lowest" : "hover:brightness-125"
                      }`}
                      style={{ top: `${topOffset}px`, height: `${height - 2}px` }}
                    >
                      <div className={`absolute left-0 top-1 bottom-1 w-1 rounded-full ${getSlotAccent(slot.type)}`} />
                      <div className="ml-2 text-left min-w-0 flex items-center gap-2">
                        <span className="text-[11px] font-semibold text-on-surface truncate">{slot.label}</span>
                        {slot.warning && (
                          <span className="text-tertiary text-xs">
                            <span className="material-symbols-outlined text-sm">warning</span>
                          </span>
                        )}
                      </div>
                      <span className="ml-auto text-[9px] text-on-surface-variant/60 font-mono shrink-0">
                        U{slot.startU}-{slot.endU}
                      </span>
                    </button>
                  );
                })}
              </div>
            </div>
          </div>
        </div>

        {/* Slot Analytics */}
        <div className="w-72 bg-surface-container rounded-xl p-4">
          {selectedSlot ? (
            <div className="space-y-4">
              <div>
                <div className="flex items-center gap-2 mb-1">
                  <span className="material-symbols-outlined text-primary text-lg">analytics</span>
                  <h3 className="text-sm font-headline font-semibold text-on-surface">Slot Analytics</h3>
                </div>
                <p className="text-xs text-on-surface-variant">{selectedSlot.label}</p>
              </div>
              <div className="bg-surface-container-low rounded-xl p-3 space-y-2">
                {[
                  { label: "Position", value: `U${selectedSlot.startU}-${selectedSlot.endU}` },
                  { label: "Size", value: `${selectedSlot.endU - selectedSlot.startU + 1}U` },
                  { label: "Type", value: selectedSlot.type },
                ].map((row) => (
                  <div key={row.label} className="flex justify-between text-xs">
                    <span className="text-on-surface-variant">{row.label}</span>
                    <span className="text-on-surface font-medium capitalize">{row.value}</span>
                  </div>
                ))}
                <div className="flex justify-between text-xs">
                  <span className="text-on-surface-variant">Status</span>
                  <span className={`font-medium ${selectedSlot.warning ? "text-tertiary" : "text-emerald-400"}`}>
                    {selectedSlot.warning ? "Thermal Warning" : "Operational"}
                  </span>
                </div>
              </div>

              {/* Power */}
              <div className="bg-surface-container-low rounded-xl p-3">
                <h4 className="text-[11px] font-semibold text-on-surface-variant mb-2">Power Draw</h4>
                <div className="flex items-baseline gap-1">
                  <span className="text-lg font-headline font-bold text-on-surface">
                    {selectedSlot.type === "network" ? "185" : "340"}
                  </span>
                  <span className="text-xs text-on-surface-variant">W</span>
                </div>
                <div className="w-full h-1.5 bg-surface-container-highest rounded-full mt-2 overflow-hidden">
                  <div
                    className="h-full bg-primary rounded-full"
                    style={{ width: selectedSlot.type === "network" ? "35%" : "55%" }}
                  />
                </div>
              </div>

              {/* Thermal */}
              <div className="bg-surface-container-low rounded-xl p-3">
                <h4 className="text-[11px] font-semibold text-on-surface-variant mb-2">Thermal</h4>
                <div className="grid grid-cols-2 gap-2 text-xs">
                  <div>
                    <span className="text-on-surface-variant">Intake</span>
                    <p className="text-on-surface font-medium">{selectedSlot.warning ? "31.2\u00b0C" : "23.8\u00b0C"}</p>
                  </div>
                  <div>
                    <span className="text-on-surface-variant">Exhaust</span>
                    <p className="text-on-surface font-medium">{selectedSlot.warning ? "42.1\u00b0C" : "34.5\u00b0C"}</p>
                  </div>
                </div>
              </div>

              {/* Quick Actions */}
              <div>
                <h4 className="text-[11px] font-semibold text-on-surface-variant mb-2">Quick Actions</h4>
                <div className="flex gap-2">
                  <button className="flex-1 flex items-center justify-center gap-1.5 py-2 rounded-lg bg-surface-container-high text-on-surface text-xs font-medium hover:bg-surface-container-highest transition-colors">
                    <span className="material-symbols-outlined text-sm">qr_code_scanner</span>
                    SCAN
                  </button>
                  <button className="flex-1 flex items-center justify-center gap-1.5 py-2 rounded-lg bg-surface-container-high text-on-surface text-xs font-medium hover:bg-surface-container-highest transition-colors">
                    <span className="material-symbols-outlined text-sm">cable</span>
                    CABLE
                  </button>
                </div>
              </div>
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center h-full text-on-surface-variant">
              <span className="material-symbols-outlined text-4xl mb-2">touch_app</span>
              <p className="text-xs">Select a slot to view analytics</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Tab 3: Network
// ---------------------------------------------------------------------------

function NetworkTab({ networkConnections, rackId, onAddConnection }: { networkConnections: any[]; rackId: string; onAddConnection: () => void }) {
  const { t } = useTranslation();
  const deleteConn = useDeleteNetworkConnection();

  function handleDelete(connId: string) {
    if (confirm(t("rack_detail.confirm_delete_connection"))) {
      deleteConn.mutate({ rackId, connId });
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <span className="material-symbols-outlined text-primary">lan</span>
          <h2 className="font-headline font-bold text-sm tracking-widest uppercase text-on-surface">
            {t("rack_detail.network_connectivity")}
          </h2>
        </div>
        <button
          onClick={onAddConnection}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-primary/20 text-primary text-sm font-medium hover:bg-primary/30 transition-colors"
        >
          <span className="material-symbols-outlined text-[18px]">add</span>
          {t("rack_detail.btn_add_connection")}
        </button>
      </div>
      <div className="bg-surface-container rounded overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-surface-container-high text-on-surface-variant text-[11px] uppercase tracking-widest">
              <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_port")}</th>
              <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_connected_device")}</th>
              <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_speed")}</th>
              <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_status")}</th>
              <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_vlan")}</th>
              <th className="px-5 py-3" />
            </tr>
          </thead>
          <tbody>
            {networkConnections.map((conn) => (
              <tr key={conn.id ?? conn.source_port ?? conn.port} className="bg-surface-container hover:bg-surface-container-high transition-colors">
                <td className="px-5 py-3 font-label font-semibold text-on-surface">{conn.source_port ?? conn.port}</td>
                <td className="px-5 py-3 text-on-surface-variant">
                  <div className="flex items-center gap-1.5">
                    <span className="material-symbols-outlined text-[16px]">router</span>
                    {conn.external_device ?? conn.connected_asset_id ?? conn.device}
                  </div>
                </td>
                <td className="px-5 py-3 text-on-surface-variant">{conn.speed}</td>
                <td className="px-5 py-3">
                  <span
                    className={`inline-flex items-center gap-1 px-2.5 py-0.5 rounded text-[11px] font-semibold tracking-wider ${
                      conn.status === "UP"
                        ? "bg-on-primary-container/20 text-primary"
                        : "bg-error-container/40 text-error"
                    }`}
                  >
                    <span className={`w-1.5 h-1.5 rounded-full ${conn.status === "UP" ? "bg-primary" : "bg-error"}`} />
                    {conn.status}
                  </span>
                </td>
                <td className="px-5 py-3 text-on-surface-variant font-mono text-xs">
                  {Array.isArray(conn.vlans) ? conn.vlans.join(', ') : (conn.vlan ?? '')}
                </td>
                <td className="px-3 py-3">
                  <button
                    onClick={() => handleDelete(conn.id)}
                    disabled={deleteConn.isPending}
                    className="p-1 rounded text-on-surface-variant hover:text-error hover:bg-error-container/20 transition-colors disabled:opacity-50"
                    title={t("rack_detail.confirm_delete_connection")}
                  >
                    <span className="material-symbols-outlined text-[18px]">delete</span>
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Tab 4: Maintenance
// ---------------------------------------------------------------------------

function MetricGauge({
  label,
  current,
  min,
  max,
  unit,
  threshold,
  icon,
}: {
  label: string;
  current: number;
  min: number;
  max: number;
  unit: string;
  threshold: number;
  icon: string;
}) {
  const { t } = useTranslation();
  const pct = (current / threshold) * 100;
  const isWarning = pct > 80;
  return (
    <div className="bg-surface-container-low rounded p-4">
      <div className="flex items-center gap-2 mb-3">
        <span className={`material-symbols-outlined text-[18px] ${isWarning ? "text-tertiary" : "text-primary"}`}>{icon}</span>
        <span className="text-[10px] uppercase tracking-widest text-on-surface-variant">{label}</span>
      </div>
      <p className={`text-3xl font-headline font-bold mb-1 ${isWarning ? "text-tertiary" : "text-on-surface"}`}>
        {current}
        <span className="text-sm font-normal text-on-surface-variant ml-1">{unit}</span>
      </p>
      <div className="w-full h-1.5 bg-surface-container-lowest rounded-full overflow-hidden mb-2">
        <div
          className={`h-full rounded-full transition-all ${isWarning ? "bg-tertiary" : "bg-primary"}`}
          style={{ width: `${Math.min(pct, 100)}%` }}
        />
      </div>
      <div className="flex justify-between text-[10px] text-on-surface-variant">
        <span>{t("rack_detail.min")}: {min}{unit}</span>
        <span>{t("rack_detail.max")}: {max}{unit}</span>
        <span>{t("rack_detail.limit")}: {threshold}{unit}</span>
      </div>
    </div>
  );
}

function MaintenanceTab({ maintenanceHistory, environmentMetrics }: {
  maintenanceHistory: Array<{ date: string; type: string; description: string; engineer: string; status: string }>;
  environmentMetrics: {
    temperature: { current: number; min: number; max: number; threshold: number; unit: string };
    humidity: { current: number; min: number; max: number; threshold: number; unit: string };
    powerDraw: { current: number; min: number; max: number; threshold: number; unit: string };
    airflow: { current: number; min: number; max: number; threshold: number; unit: string };
  };
}) {
  const { t } = useTranslation();

  return (
    <div>
      {/* Environmental Monitoring */}
      <section className="mb-8">
        <div className="flex items-center gap-2 mb-4">
          <span className="material-symbols-outlined text-primary">monitoring</span>
          <h2 className="font-headline font-bold text-sm tracking-widest uppercase text-on-surface">
            {t("rack_detail.environmental_monitoring")}
          </h2>
        </div>
        <div className="grid grid-cols-4 gap-4">
          <MetricGauge label={t("rack_visualization.temperature")} icon="thermostat" {...environmentMetrics.temperature} />
          <MetricGauge label={t("rack_visualization.humidity")} icon="humidity_percentage" {...environmentMetrics.humidity} />
          <MetricGauge label={t("rack_detail.power_draw")} icon="bolt" {...environmentMetrics.powerDraw} />
          <MetricGauge label={t("rack_detail.airflow")} icon="air" {...environmentMetrics.airflow} />
        </div>
      </section>

      {/* Maintenance History */}
      <section>
        <div className="flex items-center gap-2 mb-4">
          <span className="material-symbols-outlined text-primary">build</span>
          <h2 className="font-headline font-bold text-sm tracking-widest uppercase text-on-surface">
            {t("rack_detail.recent_maintenance_history")}
          </h2>
        </div>
        <div className="bg-surface-container rounded overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-surface-container-high text-on-surface-variant text-[11px] uppercase tracking-widest">
                <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_date")}</th>
                <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_type")}</th>
                <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_description")}</th>
                <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_engineer")}</th>
                <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_status")}</th>
              </tr>
            </thead>
            <tbody>
              {maintenanceHistory.map((entry, i) => (
                <tr key={i} className="bg-surface-container hover:bg-surface-container-high transition-colors">
                  <td className="px-5 py-3 text-on-surface font-mono text-xs whitespace-nowrap">{entry.date}</td>
                  <td className="px-5 py-3">
                    <span
                      className={`inline-block px-2.5 py-0.5 rounded text-[11px] font-semibold tracking-wider ${
                        entry.type === "Corrective"
                          ? "bg-tertiary-container/60 text-tertiary"
                          : entry.type === "Firmware"
                            ? "bg-secondary-container/60 text-secondary"
                            : entry.type === "Change"
                              ? "bg-on-primary-container/20 text-primary"
                              : "bg-surface-container-highest text-on-surface-variant"
                      }`}
                    >
                      {entry.type.toUpperCase()}
                    </span>
                  </td>
                  <td className="px-5 py-3 text-on-surface-variant">{entry.description}</td>
                  <td className="px-5 py-3 text-on-surface-variant whitespace-nowrap">{entry.engineer}</td>
                  <td className="px-5 py-3">
                    <span className="inline-flex items-center gap-1 text-[11px] text-primary">
                      <span className="material-symbols-outlined text-[14px]">check_circle</span>
                      {entry.status}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Tab definitions
// ---------------------------------------------------------------------------

const tabs = [
  { id: "visualization", label: "\u8996\u89BA\u5316", icon: "dns" },
  { id: "console",       label: "\u63A7\u5236\u53F0", icon: "terminal" },
  { id: "network",       label: "\u7DB2\u8DEF\u9023\u63A5", icon: "lan" },
  { id: "maintenance",   label: "\u7DAD\u8B77\u6B77\u53F2", icon: "build" },
] as const;

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

export default function RackDetailUnified() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { id: rackId } = useParams<{ id: string }>();
  const { data: rackResponse, isLoading, error } = useRack(rackId ?? "");
  const rack = rackResponse?.data;
  const { data: rackAssetsResponse } = useRackAssets(rackId ?? "");
  const rackAssets = rackAssetsResponse?.data;
  const { data: slotsResp } = useRackSlots(rackId ?? "");
  const rackSlots = slotsResp?.data || [];

  // Build console slots from real rack slots API (falls back to hardcoded uSlots)
  const consoleSlots: USlot[] = useMemo(() => {
    if (!rackSlots || rackSlots.length === 0) return []
    return rackSlots.map((slot: any) => {
      const assetType = (slot.asset_type || slot.type || '').toLowerCase()
      let slotType: string = 'compute'
      if (assetType.includes('network') || assetType.includes('switch')) slotType = 'network'
      else if (assetType.includes('storage') || assetType.includes('nas') || assetType.includes('san')) slotType = 'storage'
      else if (assetType.includes('power') || assetType.includes('ups')) slotType = 'ups'
      else if (assetType.includes('pdu')) slotType = 'pdu'
      return {
        startU: slot.start_u ?? 1,
        endU: slot.end_u ?? 1,
        label: slot.asset_name || slot.asset_tag || `U${slot.start_u}`,
        type: slotType as any,
      }
    })
  }, [rackSlots])

  // Build equipment list from real rack assets API (empty array if no data yet)
  const liveEquipment: Equipment[] = useMemo(() => {
    if (!rackAssets || rackAssets.length === 0) return [];
    return rackAssets.map((a: any) => ({
      startU: a.attributes?.start_u ?? a.start_u ?? 1,
      endU: a.attributes?.end_u ?? a.end_u ?? 1,
      label: a.name ?? `${a.vendor} ${a.model}`,
      assetTag: a.asset_tag ?? a.id,
      type: (a.type?.toLowerCase() ?? 'compute') as Equipment['type'],
    }));
  }, [rackAssets]);

  // Network connections from API
  const { data: netData } = useRackNetworkConnections(rackId ?? "");
  const networkConnections = (netData as any)?.connections ?? [];

  // Maintenance history from API
  const { data: maintData } = useQuery({
    queryKey: ['rackMaintenance', rackId],
    queryFn: () => apiClient.get(`/racks/${rackId}/maintenance`),
    enabled: !!rackId,
  })
  const maintenanceHistory = ((maintData as any)?.maintenance ?? []).map((wo: any) => ({
    date: wo.scheduled_start ? new Date(wo.scheduled_start).toLocaleDateString() : new Date(wo.created_at).toLocaleDateString(),
    type: wo.type ?? 'inspection',
    description: wo.title,
    engineer: '-',
    status: wo.status,
  }))

  // Activity feed from API
  const { data: activityData } = useActivityFeed('rack', rackId ?? "");
  const recentActivity = ((activityData as any)?.events ?? []).map((e: any) => ({
    action: e.description || e.action,
    time: new Date(e.timestamp).toLocaleString(),
    icon: e.event_type === 'alert' ? 'warning' : e.event_type === 'maintenance' ? 'build' : 'history',
  }));

  // Fetch all alerts and filter to those belonging to assets in this rack
  const { data: alertsResponse } = useAlerts();
  const allAlerts = alertsResponse?.data ?? [];

  const rackAssetIds = useMemo(() => {
    if (!rackAssets) return new Set<string>();
    return new Set(rackAssets.map((a: any) => a.id as string));
  }, [rackAssets]);

  const filteredAlerts: LiveAlert[] = useMemo(() => {
    if (allAlerts.length === 0 || rackAssetIds.size === 0) return [];
    return allAlerts
      .filter((alert: any) => rackAssetIds.has(alert.ci_id))
      .map((alert: any) => ({
        severity: alert.severity ?? 'info',
        text: alert.message ?? '',
        time: alert.fired_at
          ? new Date(alert.fired_at).toLocaleString()
          : '',
      }));
  }, [allAlerts, rackAssetIds]);

  // Environmental metrics: temperature from API, power from rack data, humidity/airflow placeholder (Phase 4 Group 2)
  const firstAssetId = rackAssets?.[0]?.id || ''
  const { data: tempMetrics } = useMetrics({ asset_id: firstAssetId, metric_name: 'temperature', time_range: '1h' })
  const latestTemp = (tempMetrics as any)?.data?.[0]?.value ?? 23.0
  const environmentMetrics = {
    temperature: { current: Number(latestTemp.toFixed(1)), min: Number((latestTemp - 3).toFixed(1)), max: Number((latestTemp + 3).toFixed(1)), threshold: 30, unit: '°C' },
    humidity: { current: 45, min: 38, max: 52, threshold: 60, unit: '%' },
    powerDraw: { current: Number(rack?.power_current_kw ?? 0), min: 0, max: Number(rack?.power_capacity_kw ?? 40), threshold: Number(rack?.power_capacity_kw ?? 40), unit: 'kW' },
    airflow: { current: 1250, min: 1100, max: 1400, threshold: 1500, unit: 'CFM' },
  }

  const [editingRack, setEditingRack] = useState(false)
  const [rackEdit, setRackEdit] = useState({ name: '', status: '', total_u: 42 })
  const updateRack = useUpdateRack()
  const deleteRack = useDeleteRack()

  const [activeTab, setActiveTab] = useState<string>("visualization");
  const [showAssignModal, setShowAssignModal] = useState(false);
  const [showAddConnModal, setShowAddConnModal] = useState(false);
  const [selectedAsset, setSelectedAsset] = useState<Equipment | null>(
    liveEquipment.find((e) => e.assetTag === "APP-SRV-042-PROD") ?? liveEquipment[0] ?? null,
  );

  // Selected asset data looked up from API rack assets
  const selectedAssetData = rackAssets?.find((a: any) =>
    a.asset_tag === selectedAsset?.assetTag || a.id === selectedAsset?.assetTag
  )

  const occupiedUs = liveEquipment.reduce((acc, eq) => acc + (eq.endU - eq.startU + 1), 0);
  const occupancy = rack ? Math.round((rack.used_u / rack.total_u) * 100) : Math.round((occupiedUs / 42) * 100);

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
        <p className="text-error text-sm">Failed to load rack details</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      {/* Breadcrumb */}
      <div className="bg-surface-container-low px-8 py-3">
        <div className="flex items-center gap-2 text-sm text-on-surface-variant">
          <button onClick={() => navigate("/racks")} className="hover:text-primary transition-colors">
            <span className="material-symbols-outlined text-[16px]">arrow_back</span>
          </button>
          <button onClick={() => navigate("/racks")} className="hover:text-primary transition-colors">
            {t("racks.breadcrumb_rack_management")}
          </button>
          <span className="material-symbols-outlined text-[14px]">chevron_right</span>
          <span className="text-primary">{rack?.name ?? "RACK-042"}</span>
        </div>
      </div>

      <div className="px-8 py-6">
        {/* Title + status */}
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-3">
            <h1 className="font-headline text-3xl font-bold tracking-tight text-on-surface">
              {rack?.name ?? "RACK-042"} Management Console
            </h1>
            <span className="flex items-center gap-1.5 text-xs font-semibold px-2.5 py-1 rounded-full bg-emerald-500/15 text-emerald-400">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
              ONLINE
            </span>
          </div>
          <div className="flex items-center gap-2">
            <button onClick={() => setShowAssignModal(true)}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-sky-600 text-white text-sm font-semibold hover:bg-sky-500">
              <span className="material-symbols-outlined text-[18px]">add</span> {t('rack_detail.btn_assign_asset')}
            </button>
            <button onClick={() => {
              setEditingRack(true)
              setRackEdit({ name: rack?.name || '', status: rack?.status || '', total_u: rack?.total_u || 42 })
            }} className="px-3 py-1.5 rounded-lg bg-blue-500/20 text-blue-400 text-sm hover:bg-blue-500/30 transition-colors">Edit</button>
            <button onClick={() => {
              if (confirm('Delete this rack?')) deleteRack.mutate(rackId!, { onSuccess: () => navigate('/racks') })
            }} className="px-3 py-1.5 rounded-lg bg-red-500/20 text-red-400 text-sm hover:bg-red-500/30 transition-colors">
              {deleteRack.isPending ? 'Deleting...' : 'Delete'}
            </button>
          </div>
        </div>

        {/* Inline Rack Edit Panel */}
        {editingRack && (
          <div className="bg-surface-container rounded-lg p-5 mb-4 space-y-4">
            <h3 className="font-headline text-sm font-bold text-on-surface uppercase tracking-wider">Edit Rack</h3>
            <div className="grid grid-cols-3 gap-4">
              <div className="flex flex-col gap-1">
                <label className="text-[10px] uppercase tracking-widest text-on-surface-variant">Name</label>
                <input value={rackEdit.name} onChange={e => setRackEdit(p => ({ ...p, name: e.target.value }))}
                  className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1 text-white text-sm" />
              </div>
              <div className="flex flex-col gap-1">
                <label className="text-[10px] uppercase tracking-widest text-on-surface-variant">Status</label>
                <select value={rackEdit.status} onChange={e => setRackEdit(p => ({ ...p, status: e.target.value }))}
                  className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1.5 text-white text-sm">
                  <option value="active">{t('rack_detail.status_active')}</option>
                  <option value="maintenance">{t('rack_detail.status_maintenance')}</option>
                  <option value="decommissioned">{t('rack_detail.status_decommissioned')}</option>
                  <option value="staged">{t('rack_detail.status_staged')}</option>
                </select>
              </div>
              <div className="flex flex-col gap-1">
                <label className="text-[10px] uppercase tracking-widest text-on-surface-variant">Total U</label>
                <input type="number" value={rackEdit.total_u} onChange={e => setRackEdit(p => ({ ...p, total_u: Number(e.target.value) }))}
                  className="bg-[#0d1117] border border-gray-700 rounded px-2 py-1 text-white text-sm" />
              </div>
            </div>
            <div className="flex gap-2">
              <button onClick={() => {
                updateRack.mutate({ id: rackId!, data: rackEdit }, { onSuccess: () => setEditingRack(false) })
              }} disabled={updateRack.isPending}
                className="px-4 py-2 rounded bg-blue-600 text-white text-sm">
                {updateRack.isPending ? 'Saving...' : 'Save Changes'}
              </button>
              <button onClick={() => setEditingRack(false)}
                className="px-4 py-2 rounded bg-gray-700 text-white text-sm">Cancel</button>
            </div>
          </div>
        )}

        {/* Stats bar (always visible) */}
        <div className="flex gap-4 mb-6">
          {[
            { label: "Load", value: rack ? `${rack.power_current_kw}kW` : "32.4kW", icon: "bolt", color: "text-tertiary" },
            { label: "Occupancy", value: `${occupancy}%`, icon: "stacked_bar_chart", color: "text-primary" },
            { label: "Temp", value: "22.4\u00b0C", icon: "thermostat", color: "text-on-surface" },
          ].map((stat) => (
            <div key={stat.label} className="bg-surface-container rounded px-5 py-3 flex items-center gap-3">
              <span className={`material-symbols-outlined ${stat.color}`}>{stat.icon}</span>
              <div>
                <p className="text-[10px] text-on-surface-variant uppercase tracking-widest">{stat.label}</p>
                <p className={`text-xl font-headline font-bold ${stat.color}`}>{stat.value}</p>
              </div>
            </div>
          ))}
        </div>

        {/* Tab buttons */}
        <div className="flex items-center gap-1 bg-surface-container rounded-lg p-1 mb-6 w-fit">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`flex items-center gap-2 px-4 py-2.5 rounded-md text-sm font-medium transition-colors ${
                activeTab === tab.id
                  ? "bg-primary-container text-primary"
                  : "text-on-surface-variant hover:text-on-surface hover:bg-surface-container-high"
              }`}
            >
              <span className="material-symbols-outlined text-lg">{tab.icon}</span>
              {tab.label}
            </button>
          ))}
        </div>

        {/* Tab content */}
        {activeTab === "visualization" && (
          <VisualizationTab selectedAsset={selectedAsset} setSelectedAsset={setSelectedAsset} equipmentList={liveEquipment} rackSlots={rackSlots} totalU={rack?.total_u} liveAlerts={filteredAlerts} />
        )}
        {activeTab === "console" && <ConsoleTab recentActivity={recentActivity} slots={consoleSlots.length > 0 ? consoleSlots : uSlots} />}
        {activeTab === "network" && <NetworkTab networkConnections={networkConnections} rackId={rackId ?? ''} onAddConnection={() => setShowAddConnModal(true)} />}
        {activeTab === "maintenance" && <MaintenanceTab maintenanceHistory={maintenanceHistory} environmentMetrics={environmentMetrics} />}
      </div>

      <AssignAssetToRackModal
        open={showAssignModal}
        onClose={() => setShowAssignModal(false)}
        rackId={rackId ?? ''}
        totalU={rack?.total_u ?? 42}
      />
      <AddNetworkConnectionModal
        open={showAddConnModal}
        onClose={() => setShowAddConnModal(false)}
        rackId={rackId ?? ''}
      />
    </div>
  );
}
