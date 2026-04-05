import { memo, useState, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

/* ──────────────────────────────────────────────
   Types & mock data
   ────────────────────────────────────────────── */

interface Category {
  icon: string;
  label: string;
  count: number;
  color: string;
}

interface ResolutionStep {
  step: number;
  instruction: string;
}

interface CommonIssue {
  id: string;
  title: string;
  category: string;
  severity: "critical" | "high" | "medium" | "low";
  lastSeen: string;
  steps: ResolutionStep[];
}

const CATEGORIES: Category[] = [
  { icon: "lan", label: "Network Issues", count: 24, color: "#9ecaff" },
  { icon: "memory", label: "Hardware Failures", count: 18, color: "#ffb5a0" },
  { icon: "bug_report", label: "Software Errors", count: 31, color: "#ff6b6b" },
  { icon: "speed", label: "Performance", count: 15, color: "#ffa94d" },
  { icon: "shield", label: "Security", count: 9, color: "#69db7c" },
  { icon: "thermostat", label: "Power & Cooling", count: 12, color: "#c4b5fd" },
];

const COMMON_ISSUES: CommonIssue[] = [
  {
    id: "NET-001",
    title: "Server not responding to ping",
    category: "Network",
    severity: "critical",
    lastSeen: "2 hours ago",
    steps: [
      { step: 1, instruction: "Verify the server is powered on and the network interface LED is active." },
      { step: 2, instruction: "Check the switch port status using 'show interface status' on the connected switch." },
      { step: 3, instruction: "Run 'ip addr show' or 'ifconfig' on the server to confirm the NIC has an IP address." },
      { step: 4, instruction: "Test connectivity from the same subnet to rule out routing issues." },
      { step: 5, instruction: "Check host-based firewall rules (iptables/firewalld) for ICMP blocking." },
      { step: 6, instruction: "If the issue persists, replace the network cable and test with a known-good NIC." },
    ],
  },
  {
    id: "PERF-002",
    title: "High CPU utilization (>90%)",
    category: "Performance",
    severity: "high",
    lastSeen: "45 minutes ago",
    steps: [
      { step: 1, instruction: "Run 'top' or 'htop' to identify the process consuming the most CPU." },
      { step: 2, instruction: "Check if the high-CPU process is a known application or potentially malicious." },
      { step: 3, instruction: "Review recent deployments or configuration changes that may have triggered the spike." },
      { step: 4, instruction: "If a runaway process is identified, attempt a graceful restart of the service." },
      { step: 5, instruction: "Monitor for 15 minutes after remediation to confirm CPU returns to normal levels." },
    ],
  },
  {
    id: "HW-003",
    title: "Disk array degraded",
    category: "Hardware",
    severity: "critical",
    lastSeen: "1 day ago",
    steps: [
      { step: 1, instruction: "Identify the failed drive using the RAID controller management tool (e.g., MegaCLI, storcli)." },
      { step: 2, instruction: "Check the drive bay LED indicators for physical identification." },
      { step: 3, instruction: "Verify a hot spare is available and rebuild has started automatically." },
      { step: 4, instruction: "If no hot spare exists, procure an identical replacement drive and initiate manual rebuild." },
      { step: 5, instruction: "Monitor rebuild progress and verify array returns to optimal state." },
    ],
  },
  {
    id: "SEC-004",
    title: "Authentication timeout",
    category: "Security",
    severity: "medium",
    lastSeen: "3 hours ago",
    steps: [
      { step: 1, instruction: "Verify LDAP/AD domain controller reachability with 'nslookup' and 'ping'." },
      { step: 2, instruction: "Check the authentication service logs for timeout errors or connection refusals." },
      { step: 3, instruction: "Validate DNS resolution for the authentication endpoint." },
      { step: 4, instruction: "Test with a known-good service account to isolate user-specific vs system-wide issues." },
      { step: 5, instruction: "If the domain controller is unreachable, failover to a secondary DC and investigate the primary." },
    ],
  },
  {
    id: "PWR-005",
    title: "UPS battery low warning",
    category: "Power",
    severity: "high",
    lastSeen: "6 hours ago",
    steps: [
      { step: 1, instruction: "Log in to the UPS management interface and confirm the battery charge level and estimated runtime." },
      { step: 2, instruction: "Check if the UPS is actively charging or if mains power has been interrupted." },
      { step: 3, instruction: "If mains power is down, initiate graceful shutdown of non-critical systems per the runbook." },
      { step: 4, instruction: "Inspect the battery pack age; replace if older than 3 years or showing degraded capacity." },
      { step: 5, instruction: "Schedule a battery replacement during the next maintenance window if capacity is below threshold." },
    ],
  },
];

const SEVERITY_STYLES: Record<string, { bg: string; text: string; label: string }> = {
  critical: { bg: "bg-red-500/15", text: "text-red-400", label: "Critical" },
  high: { bg: "bg-orange-500/15", text: "text-orange-400", label: "High" },
  medium: { bg: "bg-yellow-500/15", text: "text-yellow-400", label: "Medium" },
  low: { bg: "bg-green-500/15", text: "text-green-400", label: "Low" },
};

/* ──────────────────────────────────────────────
   Small reusable pieces
   ────────────────────────────────────────────── */

function Icon({ name, className = "" }: { name: string; className?: string }) {
  return (
    <span className={`material-symbols-outlined ${className}`}>{name}</span>
  );
}

/* ──────────────────────────────────────────────
   Issue Row (expandable)
   ────────────────────────────────────────────── */

const IssueRow = memo(function IssueRow({
  issue,
  isExpanded,
  onToggle,
}: {
  issue: CommonIssue;
  isExpanded: boolean;
  onToggle: () => void;
}) {
  const { t } = useTranslation();
  const sev = SEVERITY_STYLES[issue.severity];

  return (
    <div className="bg-surface-container rounded-xl border border-outline-variant/15 overflow-hidden transition-all">
      {/* Header row */}
      <button
        onClick={onToggle}
        className="w-full flex items-center gap-4 px-5 py-4 text-left hover:bg-surface-container-high/50 transition-colors"
        aria-expanded={isExpanded}
      >
        <Icon
          name={isExpanded ? "expand_less" : "expand_more"}
          className="text-[20px] text-on-surface-variant shrink-0"
        />
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-3">
            <span className="text-xs font-mono text-on-surface-variant">
              {issue.id}
            </span>
            <span className={`text-[0.65rem] font-semibold px-2 py-0.5 rounded-full ${sev.bg} ${sev.text}`}>
              {sev.label}
            </span>
          </div>
          <p className="text-sm font-medium text-on-surface mt-0.5 truncate">
            {issue.title}
          </p>
        </div>
        <span className="text-xs text-on-surface-variant bg-surface-container-high px-2.5 py-1 rounded-md shrink-0">
          {issue.category}
        </span>
        <span className="text-xs text-on-surface-variant shrink-0">
          {issue.lastSeen}
        </span>
      </button>

      {/* Expandable steps */}
      {isExpanded && (
        <div className="px-5 pb-5 pt-1 border-t border-outline-variant/10">
          <h4 className="text-xs font-semibold text-on-surface-variant uppercase tracking-wide mb-3">
            {t('troubleshooting.resolution_steps')}
          </h4>
          <ol className="space-y-2.5">
            {issue.steps.map((s) => (
              <li key={s.step} className="flex gap-3">
                <span className="shrink-0 w-6 h-6 rounded-full bg-on-primary-container/15 flex items-center justify-center text-[0.65rem] font-bold text-on-primary-container">
                  {s.step}
                </span>
                <p className="text-sm text-on-surface-variant leading-relaxed pt-0.5">
                  {s.instruction}
                </p>
              </li>
            ))}
          </ol>
        </div>
      )}
    </div>
  );
});

/* ──────────────────────────────────────────────
   Troubleshooting Guide Page
   ────────────────────────────────────────────── */

const TroubleshootingGuide = memo(function TroubleshootingGuide() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchQuery, setSearchQuery] = useState("");
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [activeCategory, setActiveCategory] = useState<string | null>(null);

  const handleToggle = useCallback((id: string) => {
    setExpandedId((prev) => (prev === id ? null : id));
  }, []);

  const filteredIssues = COMMON_ISSUES.filter((issue) => {
    const matchesSearch =
      !searchQuery ||
      issue.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
      issue.id.toLowerCase().includes(searchQuery.toLowerCase()) ||
      issue.category.toLowerCase().includes(searchQuery.toLowerCase());
    const matchesCategory =
      !activeCategory || issue.category === activeCategory;
    return matchesSearch && matchesCategory;
  });

  return (
    <div className="space-y-8">
      {/* Breadcrumb */}
      <nav
        aria-label="Breadcrumb"
        className="flex items-center gap-1.5 text-xs uppercase tracking-widest text-on-surface-variant"
      >
        <span
          className="cursor-pointer transition-colors hover:text-primary"
          onClick={() => navigate('/help/troubleshooting')}
        >
          幫助
        </span>
        <span className="text-[10px] opacity-40" aria-hidden="true">›</span>
        <span className="text-on-surface font-semibold">{t('troubleshooting.title')}</span>
      </nav>

      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-headline font-bold text-on-surface tracking-tight">
            {t('troubleshooting.title')}
          </h1>
          <p className="text-sm text-on-surface-variant mt-1">
            {t('troubleshooting.subtitle')}
          </p>
        </div>
        <button onClick={() => navigate('/maintenance/add')} className="inline-flex items-center gap-2 bg-gradient-to-r from-[#005f98] to-[#2aa7ff] text-white text-sm font-semibold px-5 py-2.5 rounded-xl shadow-lg shadow-[#005f98]/25 hover:shadow-xl transition-all active:scale-[0.98]">
          <Icon name="add" className="text-[18px]" />
          {t('troubleshooting.submit_new_issue')}
        </button>
      </div>

      {/* Search bar */}
      <div className="relative">
        <Icon
          name="search"
          className="absolute left-4 top-1/2 -translate-y-1/2 text-[20px] text-on-surface-variant"
        />
        <input
          type="text"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder={t('troubleshooting.search_placeholder')}
          className="w-full bg-surface-container border border-outline-variant/20 rounded-xl pl-12 pr-4 py-3.5 text-sm text-on-surface placeholder:text-on-surface-variant/60 focus:outline-none focus:ring-2 focus:ring-on-primary-container/40 focus:border-transparent transition-all"
          aria-label="Search troubleshooting issues"
        />
      </div>

      {/* Category cards */}
      <section>
        <h2 className="text-sm font-headline font-semibold text-on-surface-variant uppercase tracking-wide mb-4">
          {t('troubleshooting.categories')}
        </h2>
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
          {CATEGORIES.map((cat) => {
            const isActive = activeCategory === cat.label.split(" ")[0];
            return (
              <button
                key={cat.label}
                onClick={() =>
                  setActiveCategory(
                    isActive ? null : cat.label.split(" ")[0]
                  )
                }
                className={`flex flex-col items-center gap-2.5 p-4 rounded-xl border transition-all ${
                  isActive
                    ? "bg-surface-container-high border-on-primary-container/40 shadow-md"
                    : "bg-surface-container border-outline-variant/15 hover:bg-surface-container-high hover:border-outline-variant/30"
                }`}
                aria-pressed={isActive}
              >
                <div
                  className="w-10 h-10 rounded-lg flex items-center justify-center"
                  style={{ backgroundColor: `${cat.color}15`, color: cat.color }}
                >
                  <Icon name={cat.icon} className="text-[22px]" />
                </div>
                <div className="text-center">
                  <p className="text-xs font-semibold text-on-surface leading-tight">
                    {cat.label}
                  </p>
                  <p className="text-[0.65rem] text-on-surface-variant mt-0.5">
                    {cat.count} {t('troubleshooting.issues')}
                  </p>
                </div>
              </button>
            );
          })}
        </div>
      </section>

      {/* Common Issues */}
      <section>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-sm font-headline font-semibold text-on-surface-variant uppercase tracking-wide">
            {t('troubleshooting.common_issues')}
          </h2>
          <span className="text-xs text-on-surface-variant">
            {filteredIssues.length} {filteredIssues.length !== 1 ? t('common.results') : t('common.result')}
          </span>
        </div>
        <div className="space-y-3">
          {filteredIssues.length > 0 ? (
            filteredIssues.map((issue) => (
              <IssueRow
                key={issue.id}
                issue={issue}
                isExpanded={expandedId === issue.id}
                onToggle={() => handleToggle(issue.id)}
              />
            ))
          ) : (
            <div className="bg-surface-container rounded-xl border border-outline-variant/15 py-16 flex flex-col items-center gap-3">
              <Icon
                name="search_off"
                className="text-[40px] text-on-surface-variant/40"
              />
              <p className="text-sm text-on-surface-variant">
                {t('troubleshooting.no_issues_match')}
              </p>
            </div>
          )}
        </div>
      </section>
    </div>
  );
});

export default TroubleshootingGuide;
