import { memo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useAuditEvents } from "../hooks/useAudit";

/* ──────────────────────────────────────────────
   Constants
   ────────────────────────────────────────────── */

const TABS = ["Real-time", "Historical", "Archived"] as const;

const ACTION_COLORS: Record<string, string> = {
  create: "bg-[#69db7c]/20 text-[#69db7c]",
  update: "bg-primary/20 text-primary",
  delete: "bg-error/20 text-error",
  default: "bg-[#ffa94d]/20 text-[#ffa94d]",
};

const EVENT_TYPES = ["All Events", "Maintenance", "Resource Alert", "Tag Update", "Deploy"];
const USERS = ["All Users"];

/* ──────────────────────────────────────────────
   Small reusable pieces
   ────────────────────────────────────────────── */

function Icon({ name, className = "" }: { name: string; className?: string }) {
  return (
    <span className={`material-symbols-outlined ${className}`}>{name}</span>
  );
}

function StatCard({
  label,
  value,
  sub,
  icon,
}: {
  label: string;
  value: string;
  sub?: string;
  icon: string;
}) {
  return (
    <div className="rounded-lg bg-surface-container p-4">
      <div className="mb-1 flex items-center gap-2 text-on-surface-variant">
        <Icon name={icon} className="text-base" />
        <span className="text-[11px] font-medium uppercase tracking-wider">
          {label}
        </span>
      </div>
      <p className="font-headline text-2xl font-bold text-on-surface">{value}</p>
      {sub && (
        <span className="mt-1 inline-block text-xs font-semibold text-[#69db7c]">
          {sub}
        </span>
      )}
    </div>
  );
}

function InfoBlock({ icon, title, lines }: { icon: string; title: string; lines: string[] }) {
  return (
    <div className="rounded-lg bg-surface-container-low p-4">
      <div className="mb-2 flex items-center gap-2">
        <Icon name={icon} className="text-base text-primary" />
        <span className="text-[11px] font-semibold uppercase tracking-wider text-on-surface-variant">
          {title}
        </span>
      </div>
      {lines.map((line, i) => (
        <p key={i} className="text-xs text-on-surface-variant leading-relaxed">
          {line}
        </p>
      ))}
    </div>
  );
}

/* ──────────────────────────────────────────────
   Main Component
   ────────────────────────────────────────────── */

function AuditHistory() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<string>("Historical");
  const [expandedRow, setExpandedRow] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [eventTypeFilter, setEventTypeFilter] = useState('');
  const [userFilter, setUserFilter] = useState('');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');

  const { data: eventsResponse, isLoading, error } = useAuditEvents();
  const auditEvents = eventsResponse?.data ?? [];

  // Map API events to table-friendly shape
  const AUDIT_ENTRIES = auditEvents.map((e) => ({
    id: e.id,
    timestamp: e.created_at ? new Date(e.created_at).toLocaleString() : '—',
    operator: e.operator_id ?? 'System',
    role: e.module?.toUpperCase() ?? '',
    actionType: e.action?.toUpperCase() ?? '',
    actionColor: ACTION_COLORS[e.action] ?? ACTION_COLORS.default,
    description: `${e.action} on ${e.target_type} (${e.target_id})`,
    source: e.module ?? '',
    sourceIcon: 'receipt_long',
    expandable: e.diff && Object.keys(e.diff).length > 0,
    expandedData: e.diff as Record<string, string> | undefined,
    created_at: e.created_at,
  }));

  // Apply client-side filters
  let filtered = AUDIT_ENTRIES;
  if (search) filtered = filtered.filter(e => JSON.stringify(e).toLowerCase().includes(search.toLowerCase()));
  if (eventTypeFilter) filtered = filtered.filter(e => e.actionType?.includes(eventTypeFilter.toUpperCase()) || e.source?.toLowerCase().includes(eventTypeFilter.toLowerCase()));
  if (userFilter) filtered = filtered.filter(e => e.operator?.includes(userFilter));
  if (dateFrom) filtered = filtered.filter(e => e.created_at && e.created_at >= dateFrom);
  if (dateTo) filtered = filtered.filter(e => e.created_at && e.created_at <= dateTo + 'T23:59:59');

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
        <p className="text-error text-sm">Failed to load audit events</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen space-y-6 bg-surface px-6 py-5">
      {/* ── Breadcrumb ── */}
      <nav
        aria-label="Breadcrumb"
        className="flex items-center gap-1.5 text-xs uppercase tracking-widest text-on-surface-variant"
      >
        {["ASSETS", "AUDIT HISTORY"].map((crumb, i, arr) => (
          <span key={crumb} className="flex items-center gap-1.5">
            <span
              className="cursor-pointer transition-colors hover:text-primary"
              onClick={() => {
                if (crumb === "ASSETS") navigate('/assets');
              }}
            >
              {crumb}
            </span>
            {i < arr.length - 1 && (
              <Icon name="chevron_right" className="text-[14px] opacity-40" />
            )}
          </span>
        ))}
      </nav>

      {/* ── Tabs ── */}
      <div className="flex gap-1 rounded-lg bg-surface-container-low p-1">
        {TABS.map((tab) => (
          <button
            key={tab}
            type="button"
            onClick={() => setActiveTab(tab)}
            className={`rounded-md px-5 py-2 text-xs font-semibold uppercase tracking-wider transition-colors ${
              activeTab === tab
                ? "bg-surface-container-highest text-on-surface"
                : "text-on-surface-variant hover:text-on-surface"
            }`}
          >
            {tab}
          </button>
        ))}
      </div>

      {/* ── Asset Header ── */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-4">
          <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-surface-container-high">
            <Icon name="dns" className="text-2xl text-primary" />
          </div>
          <div>
            <h1 className="font-headline text-2xl font-bold text-on-surface">
              SRV-PROD-001
            </h1>
            <div className="mt-1 flex items-center gap-3">
              <span className="inline-flex items-center gap-1.5 rounded bg-[#69db7c]/15 px-2.5 py-0.5 text-[11px] font-bold uppercase tracking-wide text-[#69db7c]">
                <span className="h-1.5 w-1.5 rounded-full bg-[#69db7c]" />
                Operational
              </span>
              <span className="text-xs text-on-surface-variant">
                {t('audit.last_heartbeat')}: <span className="font-semibold text-on-surface">2m {t('audit.ago')}</span>
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* ── Stats Row ── */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard
          label={t('audit.total_events')}
          value={String(auditEvents.length || 0)}
          sub={`${t('audit.vs_last_period')}`}
          icon="receipt_long"
        />
        <StatCard
          label={t('audit.risk_level')}
          value="LOW"
          icon="shield"
        />
        <StatCard
          label={t('audit.config_changes')}
          value={String(filtered.length)}
          sub={t('audit.last_30_days')}
          icon="settings"
        />
        <StatCard
          label={t('audit.active_sessions')}
          value="3"
          icon="group"
        />
      </div>

      {/* ── Filter Row ── */}
      <div className="flex flex-wrap items-center gap-3 rounded-lg bg-surface-container p-4">
        {/* Search */}
        <div className="flex flex-1 items-center gap-2 rounded-md bg-surface-container-low px-3 py-2">
          <Icon name="search" className="text-lg text-on-surface-variant" />
          <input
            type="text"
            placeholder={t('audit.search_placeholder')}
            value={search}
            onChange={e => setSearch(e.target.value)}
            className="w-full min-w-[120px] bg-transparent text-sm text-on-surface placeholder:text-on-surface-variant/60 outline-none"
          />
        </div>

        {/* Event type dropdown */}
        <select
          value={eventTypeFilter}
          onChange={e => setEventTypeFilter(e.target.value)}
          className="appearance-none rounded-md bg-surface-container-low px-3 py-2 text-xs font-medium text-on-surface-variant outline-none"
        >
          <option value="">All Events</option>
          {EVENT_TYPES.filter(t => t !== 'All Events').map((t) => (
            <option key={t} value={t}>{t}</option>
          ))}
        </select>

        {/* User dropdown */}
        <select
          value={userFilter}
          onChange={e => setUserFilter(e.target.value)}
          className="appearance-none rounded-md bg-surface-container-low px-3 py-2 text-xs font-medium text-on-surface-variant outline-none"
        >
          <option value="">All Users</option>
          {USERS.filter(u => u !== 'All Users').map((u) => (
            <option key={u} value={u}>{u}</option>
          ))}
        </select>

        {/* Date range */}
        <div className="flex items-center gap-2 rounded-md bg-surface-container-low px-3 py-2">
          <Icon name="calendar_today" className="text-base text-on-surface-variant" />
          <input type="date" value={dateFrom} onChange={e => setDateFrom(e.target.value)} className="bg-transparent text-xs text-on-surface-variant outline-none" />
          <span className="text-on-surface-variant">—</span>
          <input type="date" value={dateTo} onChange={e => setDateTo(e.target.value)} className="bg-transparent text-xs text-on-surface-variant outline-none" />
        </div>

        {/* Advanced */}
        <button
          type="button"
          onClick={() => alert('Coming Soon')}
          className="flex items-center gap-1.5 rounded-md bg-surface-container-high px-3 py-2 text-xs font-semibold uppercase tracking-wider text-on-surface-variant transition-colors hover:text-on-surface"
        >
          <Icon name="tune" className="text-base" />
          {t('common.advanced_filters')}
        </button>
      </div>

      {/* ── Audit Log Table ── */}
      <div className="overflow-x-auto rounded-lg bg-surface-container">
        {/* Table Header */}
        <div className="grid grid-cols-[160px_180px_140px_1fr_100px_80px] gap-px bg-surface-container-high px-5 py-3">
          {[t('audit.table_timestamp'), t('audit.table_operator'), t('audit.table_action_type'), t('audit.table_description'), t('audit.table_source'), t('audit.table_details')].map(
            (col) => (
              <span
                key={col}
                className="text-[11px] font-semibold uppercase tracking-wider text-on-surface-variant"
              >
                {col}
              </span>
            ),
          )}
        </div>

        {/* Table Rows */}
        {filtered.map((entry) => (
          <div key={entry.id}>
            <div
              onClick={() => navigate('/audit/detail?id=' + entry.id)}
              className="grid grid-cols-[160px_180px_140px_1fr_100px_80px] items-center gap-px px-5 py-3.5 transition-colors hover:bg-surface-container-low cursor-pointer">
              {/* Timestamp */}
              <span className="font-body text-xs tabular-nums text-on-surface-variant">
                {entry.timestamp}
              </span>

              {/* Operator */}
              <div>
                <p className="text-sm font-semibold text-on-surface">
                  {entry.operator}
                </p>
                <span className="text-[10px] uppercase tracking-wider text-on-surface-variant">
                  {entry.role}
                </span>
              </div>

              {/* Action Type */}
              <span
                className={`inline-flex w-fit items-center rounded px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide ${entry.actionColor}`}
              >
                {entry.actionType}
              </span>

              {/* Description */}
              <p className="text-xs leading-relaxed text-on-surface-variant">
                {entry.description}
              </p>

              {/* Source */}
              <div className="flex items-center gap-1.5">
                <Icon name={entry.sourceIcon} className="text-sm text-on-surface-variant" />
                <span className="text-[11px] font-medium text-on-surface-variant">
                  {entry.source}
                </span>
              </div>

              {/* Details */}
              <button
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  entry.expandable &&
                  setExpandedRow(expandedRow === entry.id ? null : entry.id)
                }}
                className="flex items-center justify-center"
              >
                <Icon
                  name={entry.expandable ? (expandedRow === entry.id ? "expand_less" : "expand_more") : "open_in_new"}
                  className="text-lg text-on-surface-variant transition-colors hover:text-primary"
                />
              </button>
            </div>

            {/* Expanded Data */}
            {entry.expandable && expandedRow === entry.id && entry.expandedData && (
              <div className="mx-5 mb-3 grid grid-cols-4 gap-3 rounded-lg bg-surface-container-low p-4">
                {Object.entries(entry.expandedData).map(([key, val]) => (
                  <div key={key}>
                    <span className="text-[10px] uppercase tracking-wider text-on-surface-variant">
                      {key.replace(/_/g, " ")}
                    </span>
                    <p className="mt-0.5 text-sm font-semibold text-on-surface">{typeof val === 'object' ? JSON.stringify(val) : String(val)}</p>
                  </div>
                ))}
              </div>
            )}
          </div>
        ))}
      </div>

      {/* ── Bottom Info Sections ── */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <InfoBlock
          icon="memory"
          title={t('audit.cpu_architecture')}
          lines={["Intel Xeon E5-2690 v4", "2 Sockets / 28 Cores / 56 Threads", "Base Clock: 2.60 GHz"]}
        />
        <InfoBlock
          icon="storage"
          title={t('audit.storage_metrics')}
          lines={["4x 1.8TB SAS 10K RPM", "RAID-10 Configuration", "Usable Capacity: 3.2 TB (62% used)"]}
        />
        <InfoBlock
          icon="lan"
          title={t('audit.network_stack')}
          lines={["2x 10GbE Bonded (LACP)", "VLAN 220 — Production Tier", "MTU: 9000 (Jumbo Frames)"]}
        />
        <InfoBlock
          icon="verified_user"
          title={t('audit.compliance')}
          lines={["SOC2 Type II — Compliant", "PCI-DSS v3.2.1 — Scope: Yes", "Last Audit: 2023-09-14"]}
        />
      </div>

      {/* ── Run Report Button ── */}
      <div className="flex justify-end">
        <button
          type="button"
          onClick={() => {
            const blob = new Blob([JSON.stringify(filtered, null, 2)], { type: 'application/json' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url; a.download = 'audit-report.json'; a.click();
          }}
          className="machined-gradient flex items-center gap-2 rounded-lg px-8 py-3 text-sm font-bold uppercase tracking-wider text-[#001b34] transition-all hover:brightness-110"
        >
          <Icon name="summarize" className="text-lg" />
          {t('common.run_report')}
        </button>
      </div>
    </div>
  );
}

export default memo(AuditHistory);
