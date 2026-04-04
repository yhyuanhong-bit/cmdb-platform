import { useState, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router-dom";
import { useRack, useRackAssets } from "../hooks/useTopology";

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

const equipment: Equipment[] = [
  { startU: 1, endU: 2, label: "APC PDU 9000", assetTag: "PWR-042-01", type: "power" },
  { startU: 3, endU: 4, label: "Cisco Nexus 9336C", assetTag: "NET-042-01", type: "network" },
  { startU: 5, endU: 5, label: "Patch Panel 48-Port", assetTag: "NET-042-PP1", type: "network" },
  { startU: 7, endU: 8, label: "DELL PowerEdge R650", assetTag: "SRV-042-01", type: "compute" },
  { startU: 9, endU: 10, label: "DELL PowerEdge R650", assetTag: "SRV-042-02", type: "compute" },
  { startU: 11, endU: 12, label: "DELL PowerEdge R650", assetTag: "SRV-042-03", type: "compute" },
  { startU: 14, endU: 17, label: "NetApp AFF A250", assetTag: "STR-042-01", type: "storage" },
  { startU: 18, endU: 19, label: "HP DL380 Gen10+", assetTag: "SRV-042-04", type: "compute" },
  { startU: 20, endU: 22, label: "HP DL380 Gen10+", assetTag: "SRV-042-05", type: "compute" },
  { startU: 24, endU: 25, label: "DELL PowerEdge R750", assetTag: "SRV-042-06", type: "compute" },
  { startU: 26, endU: 27, label: "DELL PowerEdge R750", assetTag: "SRV-042-07", type: "compute" },
  { startU: 28, endU: 29, label: "DELL PowerEdge R750", assetTag: "SRV-042-08", type: "compute" },
  { startU: 31, endU: 32, label: "Cisco Nexus 9300", assetTag: "NET-042-02", type: "network" },
  { startU: 33, endU: 34, label: "DELL PowerEdge R750", assetTag: "SRV-042-09", type: "compute" },
  { startU: 35, endU: 38, label: "DELL PowerEdge R750", assetTag: "APP-SRV-042-PROD", type: "compute" },
  { startU: 39, endU: 40, label: "Juniper EX4300", assetTag: "NET-042-03", type: "network" },
  { startU: 41, endU: 42, label: "APC SmartUPS 3000", assetTag: "PWR-042-02", type: "power" },
];

const alerts = [
  { severity: "warning", text: "CPU temperature above 75\u00b0C threshold", time: "2 min ago" },
  { severity: "info", text: "Firmware update available (v4.2.1)", time: "1 hour ago" },
  { severity: "warning", text: "Memory utilization at 89%", time: "3 hours ago" },
];

const networkConnections = [
  { port: "Eth1/1", device: "Core-SW-01", speed: "100GbE", status: "UP", vlan: "100,200,300" },
  { port: "Eth1/2", device: "Core-SW-02", speed: "100GbE", status: "UP", vlan: "100,200,300" },
  { port: "Eth2/1", device: "Dist-SW-M1-01", speed: "25GbE", status: "UP", vlan: "400-410" },
  { port: "Eth2/2", device: "Dist-SW-M1-02", speed: "25GbE", status: "DOWN", vlan: "400-410" },
  { port: "Eth3/1", device: "Storage-SW-01", speed: "25GbE", status: "UP", vlan: "500" },
  { port: "MGMT", device: "OOB-SW-01", speed: "1GbE", status: "UP", vlan: "999" },
];

const maintenanceHistory = [
  { date: "2026-03-25", type: "Preventive", description: "Scheduled quarterly PM - cleaned filters, checked cable management", engineer: "Chen, Wei-Lin", status: "COMPLETED" },
  { date: "2026-03-18", type: "Corrective", description: "Replaced faulty PDU breaker on Phase B circuit", engineer: "Wang, Jun", status: "COMPLETED" },
  { date: "2026-03-10", type: "Change", description: "Installed DELL PowerEdge R750 in U35-U38, asset SRV-A01-07", engineer: "Liu, Mei-Hua", status: "COMPLETED" },
  { date: "2026-02-28", type: "Firmware", description: "Updated Cisco Catalyst 9300 firmware to v17.9.4a", engineer: "Chen, Wei-Lin", status: "COMPLETED" },
];

const environmentMetrics = {
  temperature: { current: 23.1, min: 19.4, max: 26.8, unit: "\u00b0C", threshold: 30 },
  humidity: { current: 45, min: 38, max: 52, unit: "%", threshold: 60 },
  powerDraw: { current: 32.4, min: 28.1, max: 35.2, unit: "kW", threshold: 40 },
  airflow: { current: 1250, min: 1100, max: 1400, unit: "CFM", threshold: 1500 },
};

const recentActivity = [
  { action: "Firmware updated on NEXUS-C93180YC", time: "2 hours ago", icon: "system_update" },
  { action: "COMPUTE-NODE-03 thermal alert triggered", time: "4 hours ago", icon: "warning" },
  { action: "Inventory scan completed", time: "1 day ago", icon: "inventory_2" },
  { action: "PDU-A power cycling event", time: "2 days ago", icon: "power" },
  { action: "New asset BACKUP-APPLIANCE registered", time: "5 days ago", icon: "add_circle" },
];

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

function VisualizationTab({
  selectedAsset,
  setSelectedAsset,
  equipmentList,
}: {
  selectedAsset: Equipment | null;
  setSelectedAsset: (eq: Equipment | null) => void;
  equipmentList?: Equipment[];
}) {
  const eqList = equipmentList ?? equipment;
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [view, setView] = useState<"FRONT" | "REAR">("FRONT");

  return (
    <div>
      {/* View toggle */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-2">
          {(["compute", "network", "storage", "power"] as const).map((type) => (
            <div key={type} className="flex items-center gap-1.5">
              <div className={`w-3 h-3 rounded-sm ${getTypeAccent(type)}`} />
              <span className="text-xs text-on-surface-variant capitalize">{type}</span>
            </div>
          ))}
        </div>
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
                RACK-042 &mdash; {view} VIEW &mdash; 42U
              </h2>
            </div>
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
                  <p className="font-headline font-bold text-lg text-on-surface cursor-pointer text-primary hover:underline" onClick={() => navigate('/assets/detail')}>{selectedAsset.assetTag}</p>
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
                  onClick={() => navigate('/assets/detail')}
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
            <div className="flex flex-col gap-1">
              {alerts.map((alert, i) => (
                <div key={i} className="bg-surface-container-low rounded p-3 flex items-start gap-3">
                  <span
                    className={`material-symbols-outlined text-[16px] mt-0.5 ${
                      alert.severity === "warning" ? "text-tertiary" : "text-on-surface-variant"
                    }`}
                  >
                    {alert.severity === "warning" ? "warning" : "info"}
                  </span>
                  <div className="flex-1">
                    <p className="text-xs text-on-surface">{alert.text}</p>
                    <p className="text-[10px] text-on-surface-variant mt-0.5">{alert.time}</p>
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

function ConsoleTab() {
  const { t } = useTranslation();
  const [selectedSlot, setSelectedSlot] = useState<USlot | null>(
    uSlots.find((s) => s.label === "NEXUS-C93180YC") ?? null,
  );

  const totalU = 42;
  const uHeight = 22;

  const occupiedUs = new Set<number>();
  uSlots.forEach((slot) => {
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
                {uSlots.map((slot) => {
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

function NetworkTab() {
  const { t } = useTranslation();

  return (
    <div>
      <div className="flex items-center gap-2 mb-4">
        <span className="material-symbols-outlined text-primary">lan</span>
        <h2 className="font-headline font-bold text-sm tracking-widest uppercase text-on-surface">
          {t("rack_detail.network_connectivity")}
        </h2>
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
            </tr>
          </thead>
          <tbody>
            {networkConnections.map((conn) => (
              <tr key={conn.port} className="bg-surface-container hover:bg-surface-container-high transition-colors">
                <td className="px-5 py-3 font-label font-semibold text-on-surface">{conn.port}</td>
                <td className="px-5 py-3 text-on-surface-variant">
                  <div className="flex items-center gap-1.5">
                    <span className="material-symbols-outlined text-[16px]">router</span>
                    {conn.device}
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
                <td className="px-5 py-3 text-on-surface-variant font-mono text-xs">{conn.vlan}</td>
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

function MaintenanceTab() {
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

  // Build equipment list from real rack assets if available, fallback to hardcoded
  const liveEquipment: Equipment[] = useMemo(() => {
    if (!rackAssets || rackAssets.length === 0) return equipment;
    return rackAssets.map((a: any) => ({
      startU: a.attributes?.start_u ?? a.start_u ?? 1,
      endU: a.attributes?.end_u ?? a.end_u ?? 1,
      label: a.name ?? `${a.vendor} ${a.model}`,
      assetTag: a.asset_tag ?? a.id,
      type: (a.type?.toLowerCase() ?? 'compute') as Equipment['type'],
    }));
  }, [rackAssets]);

  const [activeTab, setActiveTab] = useState<string>("visualization");
  const [selectedAsset, setSelectedAsset] = useState<Equipment | null>(
    liveEquipment.find((e) => e.assetTag === "APP-SRV-042-PROD") ?? liveEquipment[0] ?? null,
  );

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
        </div>

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
          <VisualizationTab selectedAsset={selectedAsset} setSelectedAsset={setSelectedAsset} equipmentList={liveEquipment} />
        )}
        {activeTab === "console" && <ConsoleTab />}
        {activeTab === "network" && <NetworkTab />}
        {activeTab === "maintenance" && <MaintenanceTab />}
      </div>
    </div>
  );
}
