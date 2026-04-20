import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import type { Rack } from "../../lib/api/topology";
import { useUpdateRack, useDeleteRack } from "../../hooks/useTopology";

export interface RackEditState {
  name: string;
  status: string;
  total_u: number;
}

/**
 * Renders the top breadcrumb strip only. Kept separate from the title/stats
 * body so the shell can preserve the original DOM structure (breadcrumb outside
 * the main `px-8 py-6` container, title/stats inside).
 */
export function RackBreadcrumb({ rack }: { rack?: Rack }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  return (
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
  );
}

/**
 * Renders the title bar, inline edit panel, and stats strip. Intended to be
 * rendered inside the shell's page body container.
 */
export function RackHeader({
  rack,
  rackId,
  occupancy,
  editingRack,
  setEditingRack,
  rackEdit,
  setRackEdit,
  onAssignAsset,
}: {
  rack?: Rack;
  rackId: string;
  occupancy: number;
  editingRack: boolean;
  setEditingRack: (value: boolean) => void;
  rackEdit: RackEditState;
  setRackEdit: (updater: (prev: RackEditState) => RackEditState) => void;
  onAssignAsset: () => void;
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const updateRack = useUpdateRack();
  const deleteRack = useDeleteRack();

  return (
    <>
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
          <button onClick={onAssignAsset}
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-sky-600 text-white text-sm font-semibold hover:bg-sky-500">
            <span className="material-symbols-outlined text-[18px]">add</span> {t('rack_detail.btn_assign_asset')}
          </button>
          <button onClick={() => {
            setEditingRack(true)
            setRackEdit(() => ({ name: rack?.name || '', status: rack?.status || '', total_u: rack?.total_u || 42 }))
          }} className="px-3 py-1.5 rounded-lg bg-blue-500/20 text-blue-400 text-sm hover:bg-blue-500/30 transition-colors">Edit</button>
          <button onClick={() => {
            if (confirm(t('rack_detail.confirm_delete_rack'))) deleteRack.mutate(rackId, { onSuccess: () => navigate('/racks') })
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
              updateRack.mutate({ id: rackId, data: rackEdit }, { onSuccess: () => setEditingRack(false) })
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
          { label: t('rack_detail.label_load'), value: rack ? `${rack.power_current_kw}kW` : "32.4kW", icon: "bolt", color: "text-tertiary" },
          { label: t('rack_detail.label_occupancy'), value: `${occupancy}%`, icon: "stacked_bar_chart", color: "text-primary" },
          { label: t('rack_detail.label_temp'), value: "22.4\u00b0C", icon: "thermostat", color: "text-on-surface" },
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
    </>
  );
}
