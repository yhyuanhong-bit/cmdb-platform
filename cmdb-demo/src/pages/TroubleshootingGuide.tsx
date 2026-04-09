import { memo, useState, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

/* ──────────────────────────────────────────────
   Types & mock data
   ────────────────────────────────────────────── */

interface Category {
  icon: string;
  labelKey: string;
  filterKey: string;
  count: number;
  color: string;
}

interface ResolutionStep {
  step: number;
  instructionKey: string;
}

interface CommonIssue {
  id: string;
  titleKey: string;
  categoryKey: string;
  filterCategory: string;
  severity: "critical" | "high" | "medium" | "low";
  lastSeenKey: string;
  steps: ResolutionStep[];
}

const CATEGORIES: Category[] = [
  { icon: "lan", labelKey: "troubleshooting.category_network", filterKey: "Network", count: 24, color: "#9ecaff" },
  { icon: "memory", labelKey: "troubleshooting.category_hardware", filterKey: "Hardware", count: 18, color: "#ffb5a0" },
  { icon: "bug_report", labelKey: "troubleshooting.category_software", filterKey: "Software", count: 31, color: "#ff6b6b" },
  { icon: "speed", labelKey: "troubleshooting.category_performance", filterKey: "Performance", count: 15, color: "#ffa94d" },
  { icon: "shield", labelKey: "troubleshooting.category_security", filterKey: "Security", count: 9, color: "#69db7c" },
  { icon: "thermostat", labelKey: "troubleshooting.category_power_cooling", filterKey: "Power", count: 12, color: "#c4b5fd" },
];

const COMMON_ISSUES: CommonIssue[] = [
  {
    id: "NET-001",
    titleKey: "troubleshooting.issue_net001_title",
    categoryKey: "troubleshooting.cat_network",
    filterCategory: "Network",
    severity: "critical",
    lastSeenKey: "troubleshooting.issue_net001_last_seen",
    steps: [
      { step: 1, instructionKey: "troubleshooting.issue_net001_step1" },
      { step: 2, instructionKey: "troubleshooting.issue_net001_step2" },
      { step: 3, instructionKey: "troubleshooting.issue_net001_step3" },
      { step: 4, instructionKey: "troubleshooting.issue_net001_step4" },
      { step: 5, instructionKey: "troubleshooting.issue_net001_step5" },
      { step: 6, instructionKey: "troubleshooting.issue_net001_step6" },
    ],
  },
  {
    id: "PERF-002",
    titleKey: "troubleshooting.issue_perf002_title",
    categoryKey: "troubleshooting.cat_performance",
    filterCategory: "Performance",
    severity: "high",
    lastSeenKey: "troubleshooting.issue_perf002_last_seen",
    steps: [
      { step: 1, instructionKey: "troubleshooting.issue_perf002_step1" },
      { step: 2, instructionKey: "troubleshooting.issue_perf002_step2" },
      { step: 3, instructionKey: "troubleshooting.issue_perf002_step3" },
      { step: 4, instructionKey: "troubleshooting.issue_perf002_step4" },
      { step: 5, instructionKey: "troubleshooting.issue_perf002_step5" },
    ],
  },
  {
    id: "HW-003",
    titleKey: "troubleshooting.issue_hw003_title",
    categoryKey: "troubleshooting.cat_hardware",
    filterCategory: "Hardware",
    severity: "critical",
    lastSeenKey: "troubleshooting.issue_hw003_last_seen",
    steps: [
      { step: 1, instructionKey: "troubleshooting.issue_hw003_step1" },
      { step: 2, instructionKey: "troubleshooting.issue_hw003_step2" },
      { step: 3, instructionKey: "troubleshooting.issue_hw003_step3" },
      { step: 4, instructionKey: "troubleshooting.issue_hw003_step4" },
      { step: 5, instructionKey: "troubleshooting.issue_hw003_step5" },
    ],
  },
  {
    id: "SEC-004",
    titleKey: "troubleshooting.issue_sec004_title",
    categoryKey: "troubleshooting.cat_security",
    filterCategory: "Security",
    severity: "medium",
    lastSeenKey: "troubleshooting.issue_sec004_last_seen",
    steps: [
      { step: 1, instructionKey: "troubleshooting.issue_sec004_step1" },
      { step: 2, instructionKey: "troubleshooting.issue_sec004_step2" },
      { step: 3, instructionKey: "troubleshooting.issue_sec004_step3" },
      { step: 4, instructionKey: "troubleshooting.issue_sec004_step4" },
      { step: 5, instructionKey: "troubleshooting.issue_sec004_step5" },
    ],
  },
  {
    id: "PWR-005",
    titleKey: "troubleshooting.issue_pwr005_title",
    categoryKey: "troubleshooting.cat_power",
    filterCategory: "Power",
    severity: "high",
    lastSeenKey: "troubleshooting.issue_pwr005_last_seen",
    steps: [
      { step: 1, instructionKey: "troubleshooting.issue_pwr005_step1" },
      { step: 2, instructionKey: "troubleshooting.issue_pwr005_step2" },
      { step: 3, instructionKey: "troubleshooting.issue_pwr005_step3" },
      { step: 4, instructionKey: "troubleshooting.issue_pwr005_step4" },
      { step: 5, instructionKey: "troubleshooting.issue_pwr005_step5" },
    ],
  },
];

const SEVERITY_STYLES: Record<string, { bg: string; text: string; labelKey: string }> = {
  critical: { bg: "bg-red-500/15", text: "text-red-400", labelKey: "troubleshooting.severity_critical" },
  high: { bg: "bg-orange-500/15", text: "text-orange-400", labelKey: "troubleshooting.severity_high" },
  medium: { bg: "bg-yellow-500/15", text: "text-yellow-400", labelKey: "troubleshooting.severity_medium" },
  low: { bg: "bg-green-500/15", text: "text-green-400", labelKey: "troubleshooting.severity_low" },
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
              {t(sev.labelKey)}
            </span>
          </div>
          <p className="text-sm font-medium text-on-surface mt-0.5 truncate">
            {t(issue.titleKey)}
          </p>
        </div>
        <span className="text-xs text-on-surface-variant bg-surface-container-high px-2.5 py-1 rounded-md shrink-0">
          {t(issue.categoryKey)}
        </span>
        <span className="text-xs text-on-surface-variant shrink-0">
          {t(issue.lastSeenKey)}
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
                  {t(s.instructionKey)}
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
    const translatedTitle = t(issue.titleKey).toLowerCase();
    const translatedCategory = t(issue.categoryKey).toLowerCase();
    const matchesSearch =
      !searchQuery ||
      translatedTitle.includes(searchQuery.toLowerCase()) ||
      issue.id.toLowerCase().includes(searchQuery.toLowerCase()) ||
      translatedCategory.includes(searchQuery.toLowerCase());
    const matchesCategory =
      !activeCategory || issue.filterCategory === activeCategory;
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
          {t('troubleshooting.breadcrumb_help')}
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
            const isActive = activeCategory === cat.filterKey;
            return (
              <button
                key={cat.filterKey}
                onClick={() =>
                  setActiveCategory(
                    isActive ? null : cat.filterKey
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
                    {t(cat.labelKey)}
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
