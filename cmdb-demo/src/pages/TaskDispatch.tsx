import { memo, useState, useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useWorkOrders } from "../hooks/useMaintenance";
import type { WorkOrder } from "../lib/api/maintenance";

/* ──────────────────────────────────────────────
   Types & mapping
   ────────────────────────────────────────────── */

interface Task {
  id: string;
  title: string;
  location: string;
  priority: string;
  priorityLevel: string;
  priorityColor: string;
  status: string;
  statusColor: string;
}

function priorityToColor(p: string): string {
  switch (p?.toUpperCase()) {
    case "CRITICAL": return "bg-error/20 text-error";
    case "HIGH": return "bg-[#ffa94d]/20 text-[#ffa94d]";
    case "MEDIUM": return "bg-[#ffa94d]/20 text-[#ffa94d]";
    default: return "bg-primary/20 text-primary";
  }
}

function priorityToLevel(p: string): string {
  switch (p?.toUpperCase()) {
    case "CRITICAL": return "L1";
    case "HIGH": return "L2";
    case "MEDIUM": return "L3";
    default: return "L5";
  }
}

function statusToColor(s: string): string {
  switch (s?.toUpperCase()) {
    case "PENDING": return "bg-[#ffa94d]/20 text-[#ffa94d]";
    case "SCHEDULED": return "bg-[#69db7c]/20 text-[#69db7c]";
    default: return "bg-surface-container-highest text-on-surface-variant";
  }
}

function toTask(wo: WorkOrder): Task {
  return {
    id: wo.code || wo.id.slice(0, 8),
    title: wo.title,
    location: wo.description?.split(' ')[0] ?? '',
    priority: wo.priority?.toUpperCase() ?? 'MEDIUM',
    priorityLevel: priorityToLevel(wo.priority),
    priorityColor: priorityToColor(wo.priority),
    status: wo.status ?? 'PENDING',
    statusColor: statusToColor(wo.status),
  };
}

interface Technician {
  id: string;
  name: string;
  role: string;
  load: number;
}

const TECHNICIANS: Technician[] = [
  { id: "a", name: "技術員 A", role: "Senior Hardware", load: 90 },
  { id: "b", name: "網路工程師 B", role: "Systems Engineer", load: 95 },
  { id: "c", name: "技術員 C", role: "Junior Technician", load: 5 },
];

const ZONE_DATA = [
  { label: "Zone A", pct: 82, color: "bg-error" },
  { label: "Zone B", pct: 34, color: "bg-[#ffa94d]" },
  { label: "Zone C", pct: 12, color: "bg-[#69db7c]" },
  { label: "Zone D", pct: 58, color: "bg-primary" },
];

/* ──────────────────────────────────────────────
   Small reusable pieces
   ────────────────────────────────────────────── */

function Icon({ name, className = "" }: { name: string; className?: string }) {
  return (
    <span className={`material-symbols-outlined ${className}`}>{name}</span>
  );
}

function ProgressBar({
  pct,
  color = "bg-primary",
  height = "h-2",
}: {
  pct: number;
  color?: string;
  height?: string;
}) {
  return (
    <div className={`w-full ${height} rounded-full bg-surface-container-low`}>
      <div
        className={`${height} rounded-full ${color} transition-all duration-500`}
        style={{ width: `${pct}%` }}
      />
    </div>
  );
}

/* ──────────────────────────────────────────────
   Main Component
   ────────────────────────────────────────────── */

function TaskDispatch() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { data: woResponse, isLoading, error } = useWorkOrders();
  const apiOrders: WorkOrder[] = woResponse?.data ?? [];
  const TASKS = useMemo(
    () => apiOrders.filter((wo) => !['completed', 'closed', 'rejected'].includes(wo.status?.toLowerCase())).map(toTask),
    [apiOrders],
  );
  const [selectedTask, setSelectedTask] = useState<string>("");

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
        <p className="text-error text-sm">Failed to load dispatch tasks</p>
      </div>
    );
  }

  const effectiveSelectedTask = selectedTask || TASKS[0]?.id || "";

  function getLoadColor(load: number) {
    if (load >= 90) return "bg-error";
    if (load >= 60) return "bg-[#ffa94d]";
    return "bg-[#69db7c]";
  }

  function getLoadTextColor(load: number) {
    if (load >= 90) return "text-error";
    if (load >= 60) return "text-[#ffa94d]";
    return "text-[#69db7c]";
  }

  return (
    <div className="min-h-screen space-y-6 bg-surface px-6 py-5">
      {/* ── Breadcrumb ── */}
      <nav
        aria-label="Breadcrumb"
        className="flex items-center gap-1.5 text-xs uppercase tracking-widest text-on-surface-variant"
      >
        {[t('task_dispatch.breadcrumb_ops_center'), t('task_dispatch.breadcrumb_task_dispatch'), t('task_dispatch.breadcrumb_pending_queue')].map((crumb, i, arr) => (
          <span key={crumb} className="flex items-center gap-1.5">
            <span
              className="cursor-pointer transition-colors hover:text-primary"
              onClick={() => { if (i === 0) navigate('/maintenance'); }}
            >
              {crumb}
            </span>
            {i < arr.length - 1 && (
              <Icon name="chevron_right" className="text-[14px] opacity-40" />
            )}
          </span>
        ))}
      </nav>

      {/* ── Header ── */}
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="font-headline text-2xl font-bold text-on-surface">
            {t('task_dispatch.title')}
          </h1>
          <p className="mt-1 text-sm text-on-surface-variant">
            {t('task_dispatch.description_prefix')}<span className="font-semibold text-on-surface">12</span>{t('task_dispatch.description_tasks_suffix')}<span className="font-semibold text-error">3</span>{t('task_dispatch.description_high_priority_suffix')}
          </p>
        </div>
        <button
          type="button"
          className="machined-gradient flex items-center gap-2 rounded-lg px-5 py-2.5 text-xs font-bold uppercase tracking-wider text-[#001b34] transition-all hover:brightness-110"
        >
          <Icon name="add" className="text-base" />
          {t('task_dispatch.add_task')}
        </button>
      </div>

      {/* ── Stats Cards ── */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div className="rounded-lg bg-surface-container p-5">
          <div className="mb-1 flex items-center gap-2 text-on-surface-variant">
            <Icon name="priority_high" className="text-lg text-error" />
            <span className="text-[11px] font-medium uppercase tracking-wider">
              {t('task_dispatch.high_priority_tasks')}
            </span>
          </div>
          <p className="font-headline text-3xl font-bold text-error">3</p>
          <span className="mt-1 text-xs text-on-surface-variant">{t('common.tasks')}</span>
        </div>
        <div className="rounded-lg bg-surface-container p-5">
          <div className="mb-1 flex items-center gap-2 text-on-surface-variant">
            <Icon name="group" className="text-lg text-primary" />
            <span className="text-[11px] font-medium uppercase tracking-wider">
              {t('task_dispatch.pending_engineers')}
            </span>
          </div>
          <p className="font-headline text-3xl font-bold text-on-surface">8</p>
          <span className="mt-1 text-xs text-on-surface-variant">{t('common.available')}</span>
        </div>
        <div className="rounded-lg bg-surface-container p-5">
          <div className="mb-1 flex items-center gap-2 text-on-surface-variant">
            <Icon name="schedule" className="text-lg text-[#ffa94d]" />
            <span className="text-[11px] font-medium uppercase tracking-wider">
              {t('task_dispatch.avg_wait_time')}
            </span>
          </div>
          <p className="font-headline text-3xl font-bold text-on-surface">14</p>
          <span className="mt-1 text-xs text-on-surface-variant">{t('common.minutes')}</span>
        </div>
      </div>

      {/* ── Main Content: Task List + Technician Assignment ── */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[1fr_380px]">
        {/* Left: Task List */}
        <div className="space-y-3">
          <h2 className="text-[11px] font-semibold uppercase tracking-wider text-on-surface-variant">
            {t('task_dispatch.task_list')}
          </h2>

          {TASKS.map((task) => (
            <button
              key={task.id}
              type="button"
              onClick={() => setSelectedTask(task.id)}
              className={`flex w-full items-center gap-4 rounded-lg p-4 text-left transition-colors ${
                effectiveSelectedTask === task.id
                  ? "bg-surface-container-highest"
                  : "bg-surface-container hover:bg-surface-container-high"
              }`}
            >
              {/* Task Icon */}
              <div
                className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-lg ${
                  effectiveSelectedTask === task.id ? "bg-primary/15" : "bg-surface-container-low"
                }`}
              >
                <Icon
                  name={
                    task.priority === "CRITICAL"
                      ? "error"
                      : task.priority === "MEDIUM"
                        ? "warning"
                        : "check_circle"
                  }
                  className={`text-xl ${
                    task.priority === "CRITICAL"
                      ? "text-error"
                      : task.priority === "MEDIUM"
                        ? "text-[#ffa94d]"
                        : "text-primary"
                  }`}
                />
              </div>

              {/* Task Info */}
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span
                    className="text-xs font-bold tabular-nums cursor-pointer text-primary hover:underline"
                    onClick={(e) => { e.stopPropagation(); navigate('/maintenance/task'); }}
                  >
                    #{task.id}
                  </span>
                  <span className="text-sm font-semibold text-on-surface">
                    {task.title}
                  </span>
                  <span className="text-xs text-on-surface-variant">
                    — <span
                      className="cursor-pointer text-primary hover:underline"
                      onClick={(e) => { e.stopPropagation(); navigate('/assets/detail'); }}
                    >{task.location}</span>
                  </span>
                </div>
                <div className="mt-1.5 flex flex-wrap items-center gap-2">
                  <span
                    className={`inline-flex items-center rounded px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide ${task.priorityColor}`}
                  >
                    {task.priority} ({task.priorityLevel})
                  </span>
                  <span
                    className={`inline-flex items-center rounded px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide ${task.statusColor}`}
                  >
                    {task.status}
                  </span>
                </div>
              </div>

              {/* Assign Button */}
              <div className="flex items-center gap-2">
                <span className="rounded-md bg-surface-container-high px-3 py-1.5 text-[11px] font-semibold uppercase tracking-wider text-primary transition-colors hover:bg-surface-container-highest">
                  {t('task_dispatch.assign')}
                </span>
                <Icon name="chevron_right" className="text-lg text-on-surface-variant" />
              </div>
            </button>
          ))}
        </div>

        {/* Right: Technician Assignment Panel */}
        <div className="space-y-4">
          <div className="rounded-lg bg-surface-container p-5">
            <div className="mb-4 flex items-center gap-2">
              <Icon name="person_search" className="text-lg text-primary" />
              <h2 className="text-[11px] font-semibold uppercase tracking-wider text-on-surface-variant">
                {t('task_dispatch.technician_assignment')}
              </h2>
            </div>

            <div className="space-y-3">
              {TECHNICIANS.map((tech) => (
                <div
                  key={tech.id}
                  className="rounded-lg bg-surface-container-low p-4"
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-surface-container-high">
                        <Icon name="person" className="text-lg text-on-surface-variant" />
                      </div>
                      <div>
                        <p className="text-sm font-semibold text-on-surface">
                          {tech.name}
                        </p>
                        <p className="text-[10px] uppercase tracking-wider text-on-surface-variant">
                          {tech.role}
                        </p>
                      </div>
                    </div>
                    <span className={`text-xs font-bold tabular-nums ${getLoadTextColor(tech.load)}`}>
                      {tech.load}%
                    </span>
                  </div>
                  <div className="mt-3 flex items-center gap-2">
                    <ProgressBar
                      pct={tech.load}
                      color={getLoadColor(tech.load)}
                      height="h-1.5"
                    />
                    <span className="shrink-0 text-[10px] uppercase tracking-wider text-on-surface-variant">
                      {t('task_dispatch.loaded')}
                    </span>
                  </div>
                </div>
              ))}
            </div>

            {/* Action Buttons */}
            <div className="mt-5 flex gap-3">
              <button
                type="button"
                className="flex flex-1 items-center justify-center gap-2 rounded-lg bg-surface-container-high py-2.5 text-xs font-semibold uppercase tracking-wider text-on-surface-variant transition-colors hover:text-on-surface"
              >
                <Icon name="auto_fix_high" className="text-base" />
                {t('task_dispatch.auto_assign')}
              </button>
              <button
                type="button"
                className="machined-gradient flex flex-1 items-center justify-center gap-2 rounded-lg py-2.5 text-xs font-bold uppercase tracking-wider text-[#001b34] transition-all hover:brightness-110"
              >
                <Icon name="check_circle" className="text-base" />
                {t('task_dispatch.confirm_assign')}
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* ── Bottom: Site Zone Capacity ── */}
      <div className="rounded-lg bg-surface-container p-5">
        <div className="mb-4 flex items-center gap-2">
          <Icon name="location_on" className="text-lg text-primary" />
          <h2 className="text-[11px] font-semibold uppercase tracking-wider text-on-surface-variant">
            {t('task_dispatch.site_zone_capacity')}
          </h2>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {ZONE_DATA.map((zone) => (
            <div key={zone.label} className="rounded-lg bg-surface-container-low p-4">
              <div className="mb-2 flex items-center justify-between">
                <span className="text-xs font-semibold text-on-surface">
                  {zone.label}
                </span>
                <span
                  className={`text-xs font-bold tabular-nums ${
                    zone.pct >= 80
                      ? "text-error"
                      : zone.pct >= 50
                        ? "text-[#ffa94d]"
                        : "text-[#69db7c]"
                  }`}
                >
                  {zone.pct}%
                </span>
              </div>
              <ProgressBar pct={zone.pct} color={zone.color} height="h-3" />
              <p className="mt-1.5 text-[10px] uppercase tracking-wider text-on-surface-variant">
                {zone.pct >= 80 ? t('task_dispatch.near_capacity') : zone.pct >= 50 ? t('task_dispatch.moderate') : t('common.available').toUpperCase()}
              </p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

export default memo(TaskDispatch);
