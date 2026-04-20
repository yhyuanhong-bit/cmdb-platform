import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { Rack } from "../../lib/api/topology";
import { type USlot, getSlotColor, getSlotAccent } from "./shared";

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

export function ConsoleTab({ recentActivity, slots, rack }: { recentActivity: Array<{ action: string; time: string; icon: string }>; slots: USlot[]; rack?: Rack }) {
  const { t } = useTranslation();
  const [selectedSlot, setSelectedSlot] = useState<USlot | null>(
    slots.find((s) => s.label === "NEXUS-C93180YC") ?? slots[0] ?? null,
  );

  const totalU = rack?.total_u ?? 42;
  const uHeight = 22;

  const occupiedUs = new Set<number>();
  slots.forEach((slot) => {
    for (let u = slot.startU; u <= slot.endU; u++) occupiedUs.add(u);
  });
  const occupiedCount = occupiedUs.size;
  const vacantCount = totalU - occupiedCount;

  // Fix #19: derive configuration from real rack data
  const rackHeight = rack?.total_u ? `${rack.total_u}U` : "\u2014";
  const maxPowerDraw = rack?.power_capacity_kw ? `${rack.power_capacity_kw}kW` : "\u2014";
  // Weight capacity is not tracked in the schema — show dash
  const weightCapacity = "\u2014";

  // Fix #18: derive gauge values from actual rack data
  const powerCurrentKw = rack?.power_current_kw ?? 0;
  const powerCapacityKw = rack?.power_capacity_kw ?? 0;
  const powerPct = powerCapacityKw > 0 ? Math.round((powerCurrentKw / powerCapacityKw) * 100) : 0;
  const powerDisplay = powerCurrentKw > 0 ? String(powerCurrentKw) : "\u2014";

  return (
    <div>
      {/* Configuration + Gauges row */}
      <div className="grid grid-cols-12 gap-6 mb-6">
        {/* Configuration */}
        <div className="col-span-4">
          <h3 className="text-[11px] font-semibold text-on-surface-variant uppercase tracking-wider mb-3">
            {t('rack_console.section_configuration')}
          </h3>
          <div className="space-y-2 text-xs">
            {[
              { label: t('rack_console.rack_height'), value: rackHeight },
              { label: t('rack_console.max_power_draw'), value: maxPowerDraw },
              { label: t('rack_console.weight_capacity'), value: weightCapacity },
            ].map((item) => (
              <div key={item.label} className="flex justify-between bg-surface-container rounded-lg px-3 py-2">
                <span className="text-on-surface-variant">{item.label}</span>
                <span className="text-on-surface font-medium">{item.value}</span>
              </div>
            ))}
          </div>
        </div>

        {/* Gauges — Fix #18: use actual rack data or dash */}
        <div className="col-span-4 flex items-center justify-center gap-4">
          <GaugeWidget label={t('rack_detail.gauge_active_power')} value={powerDisplay} unit="kW" percentage={powerPct} />
          <GaugeWidget label={t('rack_detail.gauge_intake_temp')} value={"\u2014"} unit={"\u00b0C"} percentage={0} status="NOMINAL" />
          <GaugeWidget label={t('rack_detail.gauge_humidity')} value={"\u2014"} unit="%" percentage={0} />
        </div>

        {/* Recent Activity */}
        <div className="col-span-4">
          <h3 className="text-[11px] font-semibold text-on-surface-variant uppercase tracking-wider mb-3">
            {t('rack_console.section_recent_activity')}
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
            <h2 className="text-sm font-headline font-semibold text-on-surface">{t('rack_console.section_u_position_map')}</h2>
            <div className="flex items-center gap-4 ml-4">
              <div className="flex items-center gap-1.5">
                <div className="w-2.5 h-2.5 rounded bg-on-primary-container/40" />
                <span className="text-[10px] text-on-surface-variant">{t('rack_console.legend_occupied')} ({occupiedCount})</span>
              </div>
              <div className="flex items-center gap-1.5">
                <div className="w-2.5 h-2.5 rounded bg-surface-container-highest/30" />
                <span className="text-[10px] text-on-surface-variant">{t('rack_console.legend_vacant')} ({vacantCount})</span>
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
                  <h3 className="text-sm font-headline font-semibold text-on-surface">{t('rack_console.section_slot_analytics')}</h3>
                </div>
                <p className="text-xs text-on-surface-variant">{selectedSlot.label}</p>
              </div>
              <div className="bg-surface-container-low rounded-xl p-3 space-y-2">
                {[
                  { label: t('rack_console.position'), value: `U${selectedSlot.startU}-${selectedSlot.endU}` },
                  { label: t('rack_console.size'), value: `${selectedSlot.endU - selectedSlot.startU + 1}U` },
                  { label: t('rack_console.label_type'), value: selectedSlot.type },
                ].map((row) => (
                  <div key={row.label} className="flex justify-between text-xs">
                    <span className="text-on-surface-variant">{row.label}</span>
                    <span className="text-on-surface font-medium capitalize">{row.value}</span>
                  </div>
                ))}
                <div className="flex justify-between text-xs">
                  <span className="text-on-surface-variant">{t('rack_console.label_status')}</span>
                  <span className={`font-medium ${selectedSlot.warning ? "text-tertiary" : "text-emerald-400"}`}>
                    {selectedSlot.warning ? t('rack_console.thermal_warning') : t('rack_console.operational')}
                  </span>
                </div>
              </div>

              {/* Power */}
              <div className="bg-surface-container-low rounded-xl p-3">
                <h4 className="text-[11px] font-semibold text-on-surface-variant mb-2">{t('rack_console.section_power_draw')}</h4>
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
                <h4 className="text-[11px] font-semibold text-on-surface-variant mb-2">{t('rack_console.section_thermal')}</h4>
                <div className="grid grid-cols-2 gap-2 text-xs">
                  <div>
                    <span className="text-on-surface-variant">{t('rack_console.label_intake')}</span>
                    <p className="text-on-surface font-medium">{selectedSlot.warning ? "31.2\u00b0C" : "23.8\u00b0C"}</p>
                  </div>
                  <div>
                    <span className="text-on-surface-variant">{t('rack_console.label_exhaust')}</span>
                    <p className="text-on-surface font-medium">{selectedSlot.warning ? "42.1\u00b0C" : "34.5\u00b0C"}</p>
                  </div>
                </div>
              </div>

              {/* Quick Actions */}
              <div>
                <h4 className="text-[11px] font-semibold text-on-surface-variant mb-2">{t('rack_console.section_quick_actions')}</h4>
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
              <p className="text-xs">{t('rack_console.prompt_select_slot')}</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
