import { toast } from 'sonner'
import { useState, useEffect, useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useRacks, useAllLocations, useUpdateRack, useDeleteRack, useRackSlots } from "../hooks/useTopology";
import { useLocationContext } from "../contexts/LocationContext";
import { useActivityFeed } from "../hooks/useActivityFeed";
import type { Rack, RackSlot } from "../lib/api/topology";

interface RackSlotDisplay {
  id: string;
  rack_id: string;
  asset_id: string;
  start_u: number;
  end_u?: number;
  height_u: number;
  side: string;
  asset_name?: string;
  asset_tag?: string;
  asset_type?: string;
}

interface ActivityEvent {
  timestamp: string;
  event_type: string;
  description?: string;
  action?: string;
  severity?: string;
}

interface RecentEvent {
  id?: string;
  time: string;
  icon: string;
  text: string;
  severity: string;
}


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
  const contextLocationId = path.idc?.id ?? path.campus?.id ?? path.city?.id ?? path.region?.id ?? path.territory?.id ?? "";

  // If no location selected in context, use the first territory (shows ALL racks via ltree)
  const { data: allLocResp } = useAllLocations();
  const fallbackTerritoryId = useMemo(() => {
    const locs = allLocResp?.data ?? [];
    return locs.find(l => !l.parent_id)?.id ?? '';
  }, [allLocResp]);

  const locationId = contextLocationId || fallbackTerritoryId;
  const { data: feedData } = useActivityFeed('location', locationId || '');
  const activityFeedData = feedData as { events?: ActivityEvent[] } | undefined;
  const recentEvents: RecentEvent[] = (activityFeedData?.events ?? []).map((e: ActivityEvent) => ({
    time: new Date(e.timestamp).toLocaleTimeString(),
    icon: e.event_type === 'alert' ? 'warning' : e.event_type === 'maintenance' ? 'build' : 'update',
    text: e.description ?? e.action ?? '',
    severity: e.severity || 'info',
  }));
  const { data: racksResponse, isLoading: racksLoading, error } = useRacks(locationId);
  const isLoading = racksLoading;
  const racks: Rack[] = racksResponse?.data ?? [];

  const updateRack = useUpdateRack();
  const deleteRack = useDeleteRack();
  const [editingRack, setEditingRack] = useState<Rack | null>(null);
  const [editName, setEditName] = useState("");
  const [editTotalU, setEditTotalU] = useState(0);
  const [editPower, setEditPower] = useState(0);
  const [editStatus, setEditStatus] = useState("");
  const [menuRackId, setMenuRackId] = useState<string | null>(null);

  // Fix #15: load rack slots for the first rack to populate layout visualization
  const firstRackId = racks[0]?.id ?? "";
  const { data: slotsResp } = useRackSlots(firstRackId);
  const firstRackSlots: RackSlotDisplay[] = (slotsResp?.data ?? []) as RackSlotDisplay[];

  useEffect(() => {
    if (!menuRackId) return;
    const handler = () => setMenuRackId(null);
    document.addEventListener('click', handler);
    return () => document.removeEventListener('click', handler);
  }, [menuRackId]);

  // Fix #14: sync edit form state when editingRack changes
  useEffect(() => {
    if (editingRack) {
      setEditName(editingRack.name);
      setEditTotalU(editingRack.total_u);
      setEditPower(editingRack.power_capacity_kw);
      setEditStatus(editingRack.status);
    }
  }, [editingRack]);

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
        <p className="text-error text-sm">{t('racks.load_failed')}</p>
      </div>
    );
  }

  const filteredRacks = racks.filter(
    (r) =>
      r.id.toLowerCase().includes(search.toLowerCase()) ||
      r.name.toLowerCase().includes(search.toLowerCase()) ||
      r.status.toLowerCase().includes(search.toLowerCase())
  );

  const breadcrumbParts = []
  if (path.territory) breadcrumbParts.push(path.territory.name)
  if (path.region) breadcrumbParts.push(path.region.name)
  if (path.city) breadcrumbParts.push(path.city.name)
  if (path.campus) breadcrumbParts.push(path.campus.name)
  if (path.idc) breadcrumbParts.push(path.idc.name)
  const breadcrumbText = breadcrumbParts.length > 0 ? breadcrumbParts.join(' / ') : t('racks.breadcrumb_rack_management')

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      {/* Breadcrumb */}
      <div className="bg-surface-container-low px-8 py-3">
        <div className="flex items-center gap-2 text-sm text-on-surface-variant">
          <span className="material-symbols-outlined text-[16px]">home</span>
          <span>{breadcrumbText}</span>
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
                          onClick={(e) => { e.stopPropagation(); navigate(`/racks/${rack.id}`); }}
                          className="p-1.5 rounded hover:bg-surface-container-highest transition-colors text-on-surface-variant hover:text-primary"
                          aria-label={`View ${rack.id}`}
                        >
                          <span className="material-symbols-outlined text-[18px]">visibility</span>
                        </button>
                        <button
                          onClick={(e) => { e.stopPropagation(); setEditingRack(rack); }}
                          className="p-1.5 rounded hover:bg-surface-container-highest transition-colors text-on-surface-variant hover:text-primary"
                          aria-label={`Edit ${rack.id}`}
                        >
                          <span className="material-symbols-outlined text-[18px]">edit</span>
                        </button>
                        <div className="relative">
                          <button
                            onClick={(e) => { e.stopPropagation(); setMenuRackId(menuRackId === rack.id ? null : rack.id); }}
                            className="p-1.5 rounded hover:bg-surface-container-highest transition-colors text-on-surface-variant hover:text-primary"
                            aria-label={`More options for ${rack.id}`}
                          >
                            <span className="material-symbols-outlined text-[18px]">more_vert</span>
                          </button>
                          {menuRackId === rack.id && (
                            <div className="absolute right-0 top-full mt-1 bg-surface-container-high rounded-lg shadow-lg py-1 z-20 min-w-[160px]">
                              <button
                                onClick={() => {
                                  updateRack.mutate({ id: rack.id, data: { status: rack.status === 'MAINTENANCE' ? 'ACTIVE' : 'MAINTENANCE' } });
                                  setMenuRackId(null);
                                  toast.success(rack.status === 'MAINTENANCE' ? t('racks.toast_set_active') : t('racks.toast_set_maintenance'));
                                }}
                                className="w-full text-left px-4 py-2 text-sm text-on-surface hover:bg-surface-container-highest"
                              >
                                <span className="material-symbols-outlined text-base align-middle mr-2">engineering</span>
                                {rack.status === 'MAINTENANCE' ? t('racks.set_active') : t('racks.set_maintenance')}
                              </button>
                              <button
                                onClick={() => {
                                  if (confirm(t('racks.confirm_delete'))) {
                                    deleteRack.mutate(rack.id);
                                    toast.success(t('racks.toast_deleted'));
                                  }
                                  setMenuRackId(null);
                                }}
                                className="w-full text-left px-4 py-2 text-sm text-error hover:bg-surface-container-highest"
                              >
                                <span className="material-symbols-outlined text-base align-middle mr-2">delete</span>
                                {t('racks.delete_rack')}
                              </button>
                            </div>
                          )}
                        </div>
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
          {/* Rack Layout Visualization — Fix #15: loads from API for first rack */}
          <div className="col-span-2 bg-surface-container rounded p-5">
            <div className="flex items-center gap-3 mb-4">
              <span className="material-symbols-outlined text-primary">grid_view</span>
              <h2 className="font-headline font-bold text-sm tracking-widest uppercase text-on-surface">
                {t('racks.layout_visualization')}: {racks[0]?.name ?? '—'}
              </h2>
            </div>
            <div className="bg-surface-container-low rounded p-4">
              {firstRackSlots.length === 0 ? (
                <div className="text-center text-on-surface-variant text-sm py-8">
                  {t('common.empty')} — {t('racks.no_slots_hint', 'No slot data. Assign assets to rack slots on the detail page.')}
                </div>
              ) : (
              <div className="flex flex-col gap-px">
                {(() => {
                  const totalU = racks[0]?.total_u ?? 42;
                  const slotLayout = firstRackSlots.map((s: RackSlotDisplay) => ({
                    startU: s.start_u,
                    endU: s.end_u ?? s.start_u + s.height_u - 1,
                    label: s.asset_name || s.asset_tag || `U${s.start_u}`,
                    color: (s.asset_type || '').toLowerCase().includes('network') ? 'bg-tertiary-container/60'
                      : (s.asset_type || '').toLowerCase().includes('storage') ? 'bg-secondary-container/60'
                      : 'bg-on-primary-container/25',
                  }));
                  return Array.from({ length: totalU }, (_, i) => {
                    const u = totalU - i;
                    const equipment = slotLayout.find(
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
                  }).filter(Boolean);
                })()}
              </div>
              )}
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
              {recentEvents.map((event: RecentEvent, i: number) => (
                <div
                  key={event.id ?? `${event.time}-${event.text}-${i}`}
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
                    <p className="text-[10px] text-on-surface-variant mt-1">{t('common.today')} {event.time}</p>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>

      {editingRack && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={() => setEditingRack(null)}>
          <div className="bg-[#1a1f2e] p-6 rounded-xl w-[28rem] space-y-4" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-bold text-white">{t('racks.edit_title')}</h3>

            <div>
              <label className="block text-sm text-gray-400 mb-1">{t('racks.field_name')}</label>
              <input value={editName} onChange={e => setEditName(e.target.value)}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" />
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-sm text-gray-400 mb-1">{t('racks.field_total_u')}</label>
                <input type="number" value={editTotalU} onChange={e => setEditTotalU(parseInt(e.target.value) || 0)}
                  className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" />
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">{t('racks.field_power_capacity')}</label>
                <input type="number" step="0.1" value={editPower} onChange={e => setEditPower(parseFloat(e.target.value) || 0)}
                  className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm" />
              </div>
            </div>

            <div>
              <label className="block text-sm text-gray-400 mb-1">{t('racks.field_status')}</label>
              <select value={editStatus} onChange={e => setEditStatus(e.target.value)}
                className="w-full p-2 bg-[#0d1117] rounded border border-gray-700 text-white text-sm">
                <option value="ACTIVE">{t('racks.status_active')}</option>
                <option value="MAINTENANCE">{t('racks.status_maintenance_label')}</option>
                <option value="DECOMMISSIONED">{t('racks.status_decommissioned_label')}</option>
              </select>
            </div>

            <div className="flex gap-2 justify-end pt-2">
              <button onClick={() => setEditingRack(null)} className="px-4 py-2 rounded bg-gray-700 text-white text-sm">{t('racks.btn_cancel')}</button>
              <button
                onClick={() => {
                  updateRack.mutate({ id: editingRack.id, data: { name: editName, total_u: editTotalU, power_capacity_kw: editPower, status: editStatus } }, {
                    onSuccess: () => { setEditingRack(null); toast.success(t('racks.toast_updated')); }
                  });
                }}
                className="px-4 py-2 rounded bg-blue-600 text-white text-sm"
              >
                {t('racks.btn_save')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
