import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import type { Asset } from "../../lib/api/topology";
import {
  type Equipment,
  type RackSlotDetail,
  type LiveAlert,
  getTypeColor,
  getTypeAccent,
  SLOT_BIA_COLORS,
} from "./shared";

export function VisualizationTab({
  selectedAsset,
  setSelectedAsset,
  equipmentList,
  rackSlots,
  totalU,
  liveAlerts,
  selectedAssetData,
}: {
  selectedAsset: Equipment | null;
  setSelectedAsset: (eq: Equipment | null) => void;
  equipmentList?: Equipment[];
  rackSlots?: RackSlotDetail[];
  totalU?: number;
  liveAlerts?: LiveAlert[];
  selectedAssetData?: Asset;
}) {
  const eqList = equipmentList ?? [];
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [view, setView] = useState<"FRONT" | "REAR">("FRONT");

  const hasSlots = rackSlots && rackSlots.length > 0;
  const gridU = totalU || 42;
  const uPositions = Array.from({ length: gridU }, (_, i) => gridU - i);
  const viewSlots = hasSlots
    ? rackSlots.filter((s: RackSlotDetail) => (s.side || 'front') === view.toLowerCase())
    : [];

  return (
    <div>
      {/* View toggle */}
      <div className="flex items-center justify-between mb-6">
        {hasSlots ? (
          <div className="flex flex-wrap gap-3 text-[10px] text-on-surface-variant">
            <div className="flex items-center gap-1.5">
              <span className="w-3 h-3 rounded-sm bg-error-container" />
              <span>{t('common.critical')}</span>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="w-3 h-3 rounded-sm bg-[#92400e]" />
              <span>{t('common.important')}</span>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="w-3 h-3 rounded-sm bg-[#1e3a5f]" />
              <span>{t('common.normal')}</span>
            </div>
            <div className="flex items-center gap-1.5">
              <span className="w-3 h-3 rounded-sm bg-surface-container-highest" />
              <span>{t('common.minor')}</span>
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
                  const slot = viewSlots.find((s: RackSlotDetail) => u >= s.start_u && u <= s.end_u);
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
                                ${SLOT_BIA_COLORS[slot.bia_level ?? 'normal'] || SLOT_BIA_COLORS.normal}`}
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
                <p className="text-sm">{t('rack_detail.no_equipment')}</p>
                <p className="text-[10px] mt-1 opacity-60">{t('rack_detail.no_equipment_hint')}</p>
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
                    { label: t('rack_detail.label_vendor'), value: selectedAssetData?.vendor ?? '-' },
                    { label: t('rack_detail.label_model'), value: selectedAssetData?.model ?? '-' },
                    { label: t('rack_detail.label_serial'), value: selectedAssetData?.serial_number ?? '-' },
                    { label: t('rack_detail.label_ip'), value: String(selectedAssetData?.attributes?.ip_address ?? '-') },
                    { label: t('rack_detail.label_power'), value: String(selectedAssetData?.attributes?.power_draw ?? '-') },
                    { label: t('rack_detail.label_network'), value: String(selectedAssetData?.attributes?.network_specs ?? '-') },
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
                  {t('rack_detail.view_asset_detail')} →
                </button>
              </div>
            ) : (
              <div className="text-center py-8 text-on-surface-variant">
                <span className="material-symbols-outlined text-[36px] mb-2 block opacity-30">touch_app</span>
                <p className="text-sm">{t('rack_detail.select_equipment_prompt')}</p>
              </div>
            )}
          </div>

          {/* Alerts */}
          <div className="bg-surface-container rounded p-5">
            <div className="flex items-center gap-2 mb-4">
              <span className="material-symbols-outlined text-tertiary text-[18px]">notification_important</span>
              <h2 className="text-xs font-headline font-bold tracking-widest uppercase text-on-surface-variant">
                {t('rack_detail.active_alerts')}
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
                <p className="text-xs">{t('rack_detail.no_active_alerts')}</p>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
