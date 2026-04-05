import { memo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useInventoryTasks, useCompleteTask } from "../hooks/useInventory";
import CreateInventoryTaskModal from "../components/CreateInventoryTaskModal";

/* ──────────────────────────────────────────────
   Static UI data (scan progress, QR code interactive demos)
   ────────────────────────────────────────────── */

const STATIC_RACKS = [
  { id: "RACK-A1", status: "scanned" },
  { id: "RACK-A2", status: "scanned" },
  { id: "RACK-A3", status: "pending" },
  { id: "RACK-A4", status: "scanned" },
  { id: "RACK-A5", status: "pending" },
  { id: "RACK-B1", status: "scanned" },
  { id: "RACK-B2", status: "not_started" },
  { id: "RACK-B3", status: "scanned" },
  { id: "RACK-B4", status: "not_started" },
  { id: "RACK-B5", status: "pending" },
];

const DISCREPANCIES = [
  {
    id: "NODE-4483-B3",
    location: "Rack B3 / Slot 04",
    issue: "Status mismatch — Excel: Active, Scan: Powered Off",
    type: "status_mismatch",
    resolved: false,
  },
  {
    id: "NODE-4483-B3",
    location: "Rack B3 / Slot 08 / Pos 2",
    issue: "Asset physically removed — confirmed by operator",
    type: "cleared",
    resolved: true,
  },
  {
    id: "SWITCH-CORE-02",
    location: "Rack A3 / Slot 01",
    issue: "Scan mismatch — SN does not match import record",
    type: "scan_mismatch",
    resolved: false,
  },
  {
    id: "SRV-WEB-18",
    location: "Rack A1 / Slot 12",
    issue: "Asset not in import file — unregistered device detected",
    type: "unregistered",
    resolved: false,
  },
];

const IMPORT_ERRORS = [
  { row: 23, field: "Serial Number", error: "Duplicate entry" },
  { row: 67, field: "Location", error: "Invalid rack reference" },
  { row: 112, field: "Asset Tag", error: "Format mismatch" },
];

/* ──────────────────────────────────────────────
   Small reusable pieces
   ────────────────────────────────────────────── */

function Icon({ name, className = "" }: { name: string; className?: string }) {
  return (
    <span className={`material-symbols-outlined ${className}`}>{name}</span>
  );
}

function StatusBadge({
  label,
  variant,
}: {
  label: string;
  variant: "success" | "warning" | "error" | "info" | "neutral";
}) {
  const styles = {
    success: "bg-[#0a2e1a] text-[#69db7c]",
    warning: "bg-tertiary-container text-tertiary",
    error: "bg-error-container text-error",
    info: "bg-primary-container text-primary",
    neutral: "bg-surface-container-high text-on-surface-variant",
  };
  return (
    <span
      className={`text-[10px] font-label font-bold tracking-widest px-3 py-1 rounded-lg ${styles[variant]}`}
    >
      {label}
    </span>
  );
}

/* ──────────────────────────────────────────────
   Main page
   ────────────────────────────────────────────── */

const HighSpeedInventory = memo(function HighSpeedInventory() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [showErrors, setShowErrors] = useState(false);
  const [showCreateTask, setShowCreateTask] = useState(false);

  const completeTask = useCompleteTask()
  const { data: tasksResponse, isLoading } = useInventoryTasks();
  const tasks = tasksResponse?.data ?? [];
  // The current task (first active) - used for header display
  const currentTask = tasks.find((t) => t.status === 'in_progress') ?? tasks[0];

  // Use static rack data for scan visualization (interactive demo)
  const RACKS = STATIC_RACKS;
  const scannedCount = RACKS.filter((r) => r.status === "scanned").length;
  const pendingCount = RACKS.filter((r) => r.status === "pending").length;

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-primary border-t-transparent" />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-surface p-6 font-body text-on-surface">
      {/* Header */}
      <div className="flex items-start justify-between mb-6">
        <div>
          <div className="flex items-center gap-3 mb-1">
            <h1 className="font-headline text-2xl font-bold tracking-tight text-on-surface uppercase">
              {t('inventory.title')}
            </h1>
            <StatusBadge label={currentTask?.status?.toUpperCase() ?? 'ACTIVE'} variant="success" />
          </div>
          <div className="flex items-center gap-4 mt-2">
            <span className="text-xs text-on-surface-variant font-label">
              {t('inventory.task_id')}:{" "}
              <span className="text-on-surface tabular-nums">
                {currentTask?.code ?? '—'}
              </span>
            </span>
            <span className="text-xs text-on-surface-variant font-label">
              {t('inventory.operator')}:{" "}
              <span className="text-on-surface">{currentTask?.assigned_to ?? '—'}</span>
            </span>
            <span className="text-xs text-on-surface-variant font-label">
              {t('inventory.started')}:{" "}
              <span className="text-on-surface tabular-nums">
                {currentTask?.planned_date ?? '—'}
              </span>
            </span>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setShowCreateTask(true)}
            className="bg-on-primary-container hover:brightness-110 text-white px-4 py-2 rounded-xl text-sm font-label font-bold flex items-center gap-2 transition-all"
          >
            <Icon name="add" className="text-lg" />
            New Task
          </button>
          <button onClick={() => alert('Scan: Coming Soon')} className="bg-primary hover:opacity-90 text-on-primary px-4 py-2 rounded-xl text-sm font-label font-bold flex items-center gap-2 transition-opacity">
            <Icon name="qr_code_scanner" className="text-lg" />
            {t('inventory.scan_rack_qr')}
          </button>
          <button onClick={() => alert('Manual QR: Coming Soon')} className="bg-surface-container-high hover:bg-surface-container-highest text-on-surface-variant px-4 py-2 rounded-xl text-sm font-label font-bold flex items-center gap-2 transition-colors">
            <Icon name="edit" className="text-lg" />
            {t('inventory.manual_qr')}
          </button>
          <button onClick={() => alert('Report: Coming Soon')} className="bg-surface-container-high hover:bg-surface-container-highest text-on-surface-variant px-4 py-2 rounded-xl text-sm font-label font-bold flex items-center gap-2 transition-colors">
            <Icon name="summarize" className="text-lg" />
            {t('inventory.generate_report')}
          </button>
          {currentTask && currentTask.status === 'in_progress' && (
            <button onClick={() => {
              if (confirm('Mark this task as completed?')) completeTask.mutate(currentTask.id)
            }} disabled={completeTask.isPending}
              className="px-3 py-1.5 rounded-lg bg-green-500/20 text-green-400 text-sm hover:bg-green-500/30 transition-colors font-label font-bold flex items-center gap-2">
              <Icon name="check_circle" className="text-lg" />
              {completeTask.isPending ? 'Completing...' : 'Complete Task'}
            </button>
          )}
        </div>
      </div>

      {/* 3-column workflow */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        {/* Step 1: Excel Import */}
        <div className="bg-surface-container rounded-xl p-5 flex flex-col">
          <div className="flex items-center gap-2 mb-4">
            <div className="w-7 h-7 rounded-lg bg-surface-container-high flex items-center justify-center">
              <span className="text-xs font-headline font-bold text-primary">
                01
              </span>
            </div>
            <h3 className="font-headline text-sm font-bold text-on-surface uppercase tracking-wide">
              {t('inventory.excel_import')}
            </h3>
          </div>

          <div className="bg-surface-container-low rounded-xl p-4 mb-4">
            <div className="flex items-center justify-between mb-3">
              <span className="text-xs text-on-surface-variant font-label uppercase tracking-widest">
                {t('inventory.match_progress')}
              </span>
              <span className="text-xs text-primary font-label font-bold tabular-nums">
                142 / 150
              </span>
            </div>
            <div className="w-full h-3 rounded-full bg-surface-container">
              <div
                className="h-3 rounded-full bg-primary transition-all duration-500"
                style={{ width: `${(142 / 150) * 100}%` }}
              />
            </div>
            <div className="flex items-center justify-between mt-2">
              <span className="text-[10px] text-on-surface-variant font-label">
                {t('inventory.matched')}: 142
              </span>
              <span className="text-[10px] text-error font-label">
                {t('inventory.errors')}: {IMPORT_ERRORS.length}
              </span>
            </div>
          </div>

          <button
            onClick={() => setShowErrors(!showErrors)}
            className="text-xs text-error font-label hover:underline flex items-center gap-1 mb-3"
          >
            <Icon name="error" className="text-sm" />
            {t('inventory.resolve_import_errors', { count: IMPORT_ERRORS.length })}
            <Icon
              name={showErrors ? "expand_less" : "expand_more"}
              className="text-sm"
            />
          </button>

          {showErrors && (
            <div className="bg-surface-container-low rounded-xl p-3 flex flex-col gap-2">
              {IMPORT_ERRORS.map((e, i) => (
                <div
                  key={i}
                  className="flex items-center justify-between text-xs"
                >
                  <span className="text-on-surface-variant font-label">
                    {t('inventory.row')} {e.row} / {e.field}
                  </span>
                  <span className="text-error font-label">{e.error}</span>
                </div>
              ))}
            </div>
          )}

          <div className="mt-auto pt-4">
            <div className="flex items-center gap-2 text-xs text-on-surface-variant">
              <Icon name="upload_file" className="text-lg text-primary" />
              <span className="font-label">IDC01_Q3_assets.xlsx</span>
            </div>
          </div>
        </div>

        {/* Step 2: Scan Status */}
        <div className="bg-surface-container rounded-xl p-5 flex flex-col">
          <div className="flex items-center gap-2 mb-4">
            <div className="w-7 h-7 rounded-lg bg-surface-container-high flex items-center justify-center">
              <span className="text-xs font-headline font-bold text-primary">
                02
              </span>
            </div>
            <h3 className="font-headline text-sm font-bold text-on-surface uppercase tracking-wide">
              {t('inventory.scan_status')}
            </h3>
          </div>

          <div className="grid grid-cols-5 gap-2 mb-4">
            {RACKS.map((rack) => {
              const bg =
                rack.status === "scanned"
                  ? "bg-[#0a2e1a]"
                  : rack.status === "pending"
                  ? "bg-primary-container"
                  : "bg-surface-container-high";
              const iconColor =
                rack.status === "scanned"
                  ? "text-[#69db7c]"
                  : rack.status === "pending"
                  ? "text-primary"
                  : "text-on-surface-variant/40";
              const textColor =
                rack.status === "scanned"
                  ? "text-[#69db7c]"
                  : rack.status === "pending"
                  ? "text-primary"
                  : "text-on-surface-variant/40";

              return (
                <div
                  key={rack.id}
                  onClick={() => navigate('/inventory/detail')}
                  className={`${bg} rounded-xl p-3 flex flex-col items-center gap-1.5 cursor-pointer hover:opacity-80 transition-opacity`}
                >
                  <Icon name="dns" className={`text-2xl ${iconColor}`} />
                  <span
                    className={`text-[10px] font-label font-bold tracking-wider ${textColor}`}
                  >
                    {rack.id}
                  </span>
                  {rack.status === "scanned" && (
                    <Icon name="check_circle" className="text-sm text-[#69db7c]" />
                  )}
                  {rack.status === "pending" && (
                    <Icon name="pending" className="text-sm text-primary" />
                  )}
                  {rack.status === "not_started" && (
                    <Icon
                      name="radio_button_unchecked"
                      className="text-sm text-on-surface-variant/30"
                    />
                  )}
                </div>
              );
            })}
          </div>

          <div className="flex items-center gap-4 mt-auto">
            {[
              { label: t('inventory.scanned'), color: "bg-[#69db7c]", count: scannedCount },
              { label: t('common.pending'), color: "bg-primary", count: pendingCount },
              {
                label: t('inventory.not_started'),
                color: "bg-on-surface-variant/30",
                count: RACKS.length - scannedCount - pendingCount,
              },
            ].map((l) => (
              <div key={l.label} className="flex items-center gap-1.5">
                <div className={`w-2 h-2 rounded-full ${l.color}`} />
                <span className="text-[10px] text-on-surface-variant font-label">
                  {l.label} ({l.count})
                </span>
              </div>
            ))}
          </div>
        </div>

        {/* Step 3: Discrepancy Closing */}
        <div className="bg-surface-container rounded-xl p-5 flex flex-col">
          <div className="flex items-center gap-2 mb-4">
            <div className="w-7 h-7 rounded-lg bg-surface-container-high flex items-center justify-center">
              <span className="text-xs font-headline font-bold text-primary">
                03
              </span>
            </div>
            <h3 className="font-headline text-sm font-bold text-on-surface uppercase tracking-wide">
              {t('inventory.discrepancy_closing')}
            </h3>
            <span className="ml-auto text-[10px] font-label text-error">
              {DISCREPANCIES.filter((d) => !d.resolved).length} {t('inventory.open')}
            </span>
          </div>

          <div className="flex flex-col gap-2 flex-1">
            {DISCREPANCIES.map((d, i) => (
              <div
                key={`${d.id}-${i}`}
                onClick={() => navigate('/inventory/detail')}
                className={`rounded-xl p-3 cursor-pointer hover:opacity-80 transition-opacity ${
                  d.resolved
                    ? "bg-[#0a2e1a]/50"
                    : "bg-surface-container-low"
                }`}
              >
                <div className="flex items-start justify-between gap-2">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-xs font-headline font-bold text-on-surface">
                        {d.id}
                      </span>
                      {d.resolved && (
                        <Icon
                          name="check_circle"
                          className="text-sm text-[#69db7c]"
                        />
                      )}
                    </div>
                    <p className="text-[10px] text-on-surface-variant font-label mt-0.5">
                      {d.location}
                    </p>
                    <p className="text-[11px] text-on-surface-variant mt-1">
                      {d.issue}
                    </p>
                  </div>
                  <div className="shrink-0">
                    {d.type === "status_mismatch" && !d.resolved && (
                      <button onClick={(e) => { e.stopPropagation(); alert('Coming Soon'); }} className="bg-tertiary-container text-tertiary text-[10px] font-label font-bold tracking-wider px-3 py-1.5 rounded-lg hover:opacity-90 transition-opacity whitespace-nowrap">
                        {t('inventory.verify_volume')}
                      </button>
                    )}
                    {d.type === "cleared" && d.resolved && (
                      <span className="bg-[#0a2e1a] text-[#69db7c] text-[10px] font-label font-bold tracking-wider px-3 py-1.5 rounded-lg">
                        {t('inventory.asset_cleared')}
                      </span>
                    )}
                    {d.type === "scan_mismatch" && !d.resolved && (
                      <button onClick={(e) => { e.stopPropagation(); alert('Coming Soon'); }} className="bg-error-container text-error text-[10px] font-label font-bold tracking-wider px-3 py-1.5 rounded-lg hover:opacity-90 transition-opacity whitespace-nowrap">
                        {t('inventory.add_to_findings')}
                      </button>
                    )}
                    {d.type === "unregistered" && !d.resolved && (
                      <button onClick={(e) => { e.stopPropagation(); alert('Coming Soon'); }} className="bg-primary-container text-primary text-[10px] font-label font-bold tracking-wider px-3 py-1.5 rounded-lg hover:opacity-90 transition-opacity whitespace-nowrap">
                        {t('inventory.register_asset')}
                      </button>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Bottom stats bar */}
      <div className="bg-surface-container rounded-xl px-6 py-4 flex items-center justify-between">
        <div className="flex items-center gap-8">
          <div className="flex items-center gap-2">
            <Icon name="timer" className="text-on-surface-variant text-lg" />
            <div>
              <p className="text-[10px] text-on-surface-variant font-label uppercase tracking-widest">
                {t('inventory.time_elapsed')}
              </p>
              <p className="text-sm font-headline font-bold text-on-surface tabular-nums">
                12m
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Icon name="speed" className="text-[#69db7c] text-lg" />
            <div>
              <p className="text-[10px] text-on-surface-variant font-label uppercase tracking-widest">
                {t('common.status')}
              </p>
              <p className="text-sm font-headline font-bold text-[#69db7c]">
                {t('inventory.status_optimal')}
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Icon name="schedule" className="text-on-surface-variant text-lg" />
            <div>
              <p className="text-[10px] text-on-surface-variant font-label uppercase tracking-widest">
                {t('inventory.duration')}
              </p>
              <p className="text-sm font-headline font-bold text-on-surface tabular-nums">
                01:24:08
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Icon name="assignment_late" className="text-tertiary text-lg" />
            <div>
              <p className="text-[10px] text-on-surface-variant font-label uppercase tracking-widest">
                {t('inventory.extra_clearances')}
              </p>
              <p className="text-sm font-headline font-bold text-tertiary tabular-nums">
                3
              </p>
            </div>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2 bg-surface-container-low rounded-xl px-4 py-2">
            <Icon name="inventory" className="text-primary text-lg" />
            <span className="text-xs font-label text-on-surface-variant">
              {t('inventory.scanned')}:{" "}
              <span className="text-on-surface font-bold tabular-nums">
                {scannedCount}/{RACKS.length}
              </span>
            </span>
          </div>
          <div className="flex items-center gap-2 bg-surface-container-low rounded-xl px-4 py-2">
            <Icon name="error_outline" className="text-error text-lg" />
            <span className="text-xs font-label text-on-surface-variant">
              {t('inventory.discrepancies')}:{" "}
              <span className="text-on-surface font-bold tabular-nums">
                {DISCREPANCIES.filter((d) => !d.resolved).length}
              </span>
            </span>
          </div>
        </div>
      </div>

      <CreateInventoryTaskModal open={showCreateTask} onClose={() => setShowCreateTask(false)} />
    </div>
  );
});

export default HighSpeedInventory;
