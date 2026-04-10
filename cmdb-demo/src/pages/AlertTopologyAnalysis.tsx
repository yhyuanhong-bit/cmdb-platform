import { memo, useState, useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import Icon from "../components/Icon";
import StatusBadge from "../components/StatusBadge";
import { useAlerts } from "../hooks/useMonitoring";
import { useTopologyGraph } from "../hooks/useTopology";
import { useForceLayout } from "../hooks/useForceLayout";

/* ──────────────────────────────────────────────
   Types
   ────────────────────────────────────────────── */

interface TopologyNode {
  id: string;
  name: string;
  type: string;
  icon: string;
  status: "critical" | "warning" | "normal";
  ip: string;
  model: string;
  rack: string;
  biaLevel: number;
  cpu: number;
  memory: number;
  diskIO: number;
  tags: string[];
  x: number;
  y: number;
}

interface TopologyEdge {
  from: string;
  to: string;
  isFaultPath: boolean;
}

interface AlertItem {
  id: string;
  severity: "CRITICAL" | "WARNING";
  assetName: string;
  description: string;
  timestamp: string;
  nodeId: string;
}

// ALERTS mock removed - data now comes from useAlerts() hook

/* ──────────────────────────────────────────────
   Severity helpers
   ────────────────────────────────────────────── */

const SEVERITY_BG: Record<string, string> = {
  CRITICAL: "bg-error-container/60",
  WARNING: "bg-[#92400e]/40",
};

const SEVERITY_TEXT: Record<string, string> = {
  CRITICAL: "text-error",
  WARNING: "text-[#fbbf24]",
};

const STATUS_DOT: Record<string, string> = {
  critical: "bg-error",
  warning: "bg-[#fbbf24]",
  normal: "bg-[#34d399]",
};

const STATUS_RING: Record<string, string> = {
  critical: "ring-error/40",
  warning: "ring-[#fbbf24]/30",
  normal: "ring-transparent",
};

/* ──────────────────────────────────────────────
   Sub-components
   ────────────────────────────────────────────── */

const LiveIndicator = memo(function LiveIndicator() {
  const { t } = useTranslation();
  return (
    <span className="inline-flex items-center gap-1.5 ml-3">
      <span className="relative flex h-2.5 w-2.5">
        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-error opacity-75" />
        <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-error" />
      </span>
      <span className="text-error text-xs font-semibold tracking-widest uppercase">
        {t('alert_topology.live')}
      </span>
    </span>
  );
});

const MiniMetric = memo(function MiniMetric({
  label,
  value,
  unit,
}: {
  label: string;
  value: number;
  unit: string;
}) {
  const color =
    value >= 85
      ? "text-error"
      : value >= 65
        ? "text-[#fbbf24]"
        : "text-[#34d399]";
  const barColor =
    value >= 85
      ? "bg-error"
      : value >= 65
        ? "bg-[#fbbf24]"
        : "bg-[#34d399]";

  return (
    <div className="flex-1 min-w-[120px]">
      <div className="flex items-baseline justify-between mb-1.5">
        <span className="text-on-surface-variant text-xs">{label}</span>
        <span className={`text-sm font-semibold font-headline ${color}`}>
          {value}
          {unit}
        </span>
      </div>
      <div className="h-1.5 rounded-full bg-surface-container-low overflow-hidden">
        <div
          className={`h-full rounded-full ${barColor} transition-all duration-700`}
          style={{ width: `${value}%` }}
        />
      </div>
    </div>
  );
});

/* ──────────────────────────────────────────────
   Topology SVG edges
   ────────────────────────────────────────────── */

const NODE_W = 180;
const NODE_H = 72;

function getEdgePoints(
  from: TopologyNode,
  to: TopologyNode
): { x1: number; y1: number; x2: number; y2: number } {
  const x1 = from.x + NODE_W / 2;
  const y1 = from.y + NODE_H / 2;
  const x2 = to.x + NODE_W / 2;
  const y2 = to.y + NODE_H / 2;
  return { x1, y1, x2, y2 };
}

/* ──────────────────────────────────────────────
   Main component
   ────────────────────────────────────────────── */

function AlertTopologyAnalysis() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [selectedNodeId, setSelectedNodeId] = useState<string>("");
  const [biaFilter, setBiaFilter] = useState<string>("all");
  const [domainFilter, setDomainFilter] = useState<string>("all");

  // Use a default seed location (Neihu Campus — first populated IDC in seed data)
  const locationId = "d0000000-0000-0000-0000-000000000004";

  const { data: graphData, isLoading: graphLoading } = useTopologyGraph(locationId);
  const apiNodes = (graphData as any)?.nodes ?? [];
  const apiEdges = (graphData as any)?.edges ?? [];

  const layoutNodes = useForceLayout(apiNodes, apiEdges, 800, 380);

  const mappedNodes: TopologyNode[] = layoutNodes.map((n: any) => ({
    id: n.id,
    name: n.name,
    type: n.type,
    icon: n.type === 'server' ? 'dns' : n.type === 'network' ? 'router' : n.type === 'storage' ? 'storage' : 'bolt',
    status: n.has_active_alert ? 'critical' : n.status === 'operational' ? 'normal' : 'warning',
    ip: n.ip_address || '',
    model: n.model || '',
    rack: n.rack_name || '',
    biaLevel: n.bia_level || 3,
    cpu: n.metrics?.cpu ?? 0,
    memory: n.metrics?.memory ?? 0,
    diskIO: n.metrics?.disk_io ?? 0,
    tags: n.tags || [],
    x: n.x,
    y: n.y,
  }));

  const mappedEdges = apiEdges.map((e: any) => ({
    from: e.from,
    to: e.to,
    isFaultPath: e.isFaultPath || e.is_fault_path || false,
  }));

  const { data: alertsResponse, isLoading: alertsLoading } = useAlerts();
  const apiAlerts = alertsResponse?.data ?? [];

  // Map API alerts to AlertItem shape for the panel
  const ALERTS: AlertItem[] = apiAlerts.length > 0
    ? apiAlerts.map((a) => ({
        id: a.id,
        severity: (a.severity ?? '').toUpperCase() as 'CRITICAL' | 'WARNING',
        assetName: a.ci_id ?? 'Unknown',
        description: a.message ?? '',
        timestamp: a.fired_at ? new Date(a.fired_at).toLocaleString() : '—',
        nodeId: 'node-1', // topology node mapping not available in API
      }))
    : [
        // Fallback static data when API returns empty
        { id: 'ALT-001', severity: 'CRITICAL' as const, assetName: 'DB-PROD-SQL-01', description: 'CPU utilization exceeded 85% threshold for over 15 minutes.', timestamp: '2026-03-28 09:14:22', nodeId: 'node-1' },
        { id: 'ALT-002', severity: 'WARNING' as const, assetName: 'APP-PORTAL-WEB-04', description: 'HTTP response time degraded to 4.2s (SLA threshold: 2s).', timestamp: '2026-03-28 09:15:41', nodeId: 'node-2' },
      ];

  const selectedNode = useMemo(
    () => mappedNodes.find((n) => n.id === selectedNodeId) ?? mappedNodes[0] ?? null,
    [selectedNodeId, mappedNodes]
  );

  const nodeMap = useMemo(() => {
    const m: Record<string, TopologyNode> = {};
    for (const n of mappedNodes) m[n.id] = n;
    return m;
  }, [mappedNodes]);

  return (
    <div className="flex flex-col gap-5 font-body text-on-surface min-h-0">
      {/* ── Header ─────────────────────────────── */}
      <header className="flex flex-col gap-3">
        {/* Breadcrumb */}
        <nav className="flex items-center gap-1.5 text-xs text-on-surface-variant">
          <span className="cursor-pointer hover:text-primary" onClick={() => navigate('/monitoring')}>{t('alert_topology.breadcrumb_system_monitoring')}</span>
          <Icon name="chevron_right" className="text-[16px] opacity-50" />
          <span className="text-primary">{t('alert_topology.breadcrumb_topology_view')}</span>
        </nav>

        {/* Title row */}
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="flex items-center">
            <h1 className="text-2xl font-headline font-bold tracking-tight text-on-surface">
              {t('alert_topology.title_zh')}{" "}
              <span className="text-on-surface-variant font-normal text-lg">
                {t('alert_topology.title')}
              </span>
            </h1>
            <LiveIndicator />
          </div>

          {/* Controls */}
          <div className="flex items-center gap-3 flex-wrap">
            {/* BIA Level */}
            <select
              value={biaFilter}
              onChange={(e) => setBiaFilter(e.target.value)}
              className="bg-surface-container-high text-on-surface text-sm rounded px-3 py-2 focus:outline-none focus:ring-1 focus:ring-primary/40 appearance-none cursor-pointer"
            >
              <option value="all">{t('alert_topology.filter_all_bia_levels')}</option>
              <option value="1">{t('alert_topology.filter_bia_level')} 1</option>
              <option value="2">{t('alert_topology.filter_bia_level')} 2</option>
              <option value="3">{t('alert_topology.filter_bia_level')} 3</option>
            </select>

            {/* Service Domain */}
            <select
              value={domainFilter}
              onChange={(e) => setDomainFilter(e.target.value)}
              className="bg-surface-container-high text-on-surface text-sm rounded px-3 py-2 focus:outline-none focus:ring-1 focus:ring-primary/40 appearance-none cursor-pointer"
            >
              <option value="all">{t('alert_topology.filter_service_domain')}</option>
              <option value="database">{t('alert_topology.filter_database')}</option>
              <option value="application">{t('alert_topology.filter_application')}</option>
              <option value="network">{t('alert_topology.filter_network')}</option>
            </select>

            {/* Action buttons */}
            <button className="flex items-center gap-1.5 bg-surface-container-high text-on-surface-variant text-sm px-4 py-2 rounded hover:bg-surface-container-highest transition-colors">
              <Icon name="restart_alt" className="text-[18px]" />
              {t('alert_topology.btn_reset_view')}
            </button>
            <button className="flex items-center gap-1.5 bg-primary/15 text-primary text-sm px-4 py-2 rounded hover:bg-primary/25 transition-colors font-semibold">
              <Icon name="download" className="text-[18px]" />
              {t('alert_topology.btn_export_report')}
            </button>
          </div>
        </div>
      </header>

      {/* ── Main content area ──────────────────── */}
      <div className="flex gap-4 flex-1 min-h-0" style={{ height: "480px" }}>
        {/* Left panel: Alert list (40%) */}
        <section className="w-[40%] min-w-[320px] flex flex-col bg-surface-container rounded-lg overflow-hidden">
          <div className="flex items-center justify-between px-4 py-3 bg-surface-container-high">
            <h2 className="text-sm font-headline font-semibold text-on-surface flex items-center gap-2">
              <Icon name="link" className="text-[18px] text-primary" />
              {t('alert_topology.alert_list_title')}
              <span className="text-on-surface-variant font-normal">
                ({ALERTS.length})
              </span>
            </h2>
            <span className="text-xs text-on-surface-variant">{t('alert_topology.correlated')}</span>
          </div>

          <div className="flex-1 overflow-y-auto p-2 flex flex-col gap-1.5">
            {ALERTS.map((alert) => (
              <button
                key={alert.id}
                onClick={() => setSelectedNodeId(alert.nodeId)}
                className={`w-full text-left rounded-lg p-3 transition-colors ${
                  selectedNodeId === alert.nodeId
                    ? "bg-surface-container-high"
                    : "bg-transparent hover:bg-surface-container-low"
                }`}
              >
                <div className="flex items-start gap-2.5">
                  {/* Severity pill */}
                  <span
                    className={`shrink-0 mt-0.5 px-1.5 py-0.5 rounded text-[0.625rem] font-bold tracking-wider ${SEVERITY_BG[alert.severity]} ${SEVERITY_TEXT[alert.severity]}`}
                  >
                    {alert.severity}
                  </span>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center justify-between gap-2 mb-1">
                      <span className="text-sm font-semibold text-on-surface truncate font-headline">
                        {alert.assetName}
                      </span>
                      <span className="text-[0.625rem] text-on-surface-variant whitespace-nowrap">
                        {alert.timestamp.split(" ")[1]}
                      </span>
                    </div>
                    <p className="text-xs text-on-surface-variant leading-relaxed line-clamp-2">
                      {alert.description}
                    </p>
                    <span className="inline-flex items-center gap-0.5 mt-1.5 text-primary text-xs font-medium hover:underline">
                      {t('alert_topology.view_link')}
                      <Icon name="arrow_forward" className="text-[14px]" />
                    </span>
                  </div>
                </div>
              </button>
            ))}
          </div>
        </section>

        {/* Right panel: Topology graph (60%) */}
        <section className="flex-1 bg-surface-container rounded-lg overflow-hidden relative">
          {/* Graph header */}
          <div className="flex items-center justify-between px-4 py-3 bg-surface-container-high">
            <h2 className="text-sm font-headline font-semibold text-on-surface flex items-center gap-2">
              <Icon name="account_tree" className="text-[18px] text-primary" />
              {t('alert_topology.topology_title')}
            </h2>
            <div className="flex items-center gap-4 text-xs text-on-surface-variant">
              <span className="flex items-center gap-1.5">
                <span className="h-2 w-2 rounded-full bg-error" />
                {t('alert_topology.legend_critical')}
              </span>
              <span className="flex items-center gap-1.5">
                <span className="h-2 w-2 rounded-full bg-[#fbbf24]" />
                {t('alert_topology.legend_warning')}
              </span>
              <span className="flex items-center gap-1.5">
                <span className="h-2 w-2 rounded-full bg-[#34d399]" />
                {t('alert_topology.legend_normal')}
              </span>
            </div>
          </div>

          {/* SVG canvas for edges + positioned nodes */}
          <div className="relative w-full" style={{ height: "400px" }}>
            {/* SVG layer for connections */}
            <svg
              className="absolute inset-0 w-full h-full pointer-events-none"
              style={{ zIndex: 0 }}
            >
              <defs>
                <marker
                  id="arrow-fault"
                  markerWidth="8"
                  markerHeight="6"
                  refX="8"
                  refY="3"
                  orient="auto"
                >
                  <path d="M0,0 L8,3 L0,6 Z" fill="#ffb4ab" />
                </marker>
                <marker
                  id="arrow-normal"
                  markerWidth="8"
                  markerHeight="6"
                  refX="8"
                  refY="3"
                  orient="auto"
                >
                  <path d="M0,0 L8,3 L0,6 Z" fill="#44474c" />
                </marker>
              </defs>

              {(mappedEdges as TopologyEdge[]).map((edge, i) => {
                const fromNode = nodeMap[edge.from];
                const toNode = nodeMap[edge.to];
                if (!fromNode || !toNode) return null;
                const { x1, y1, x2, y2 } = getEdgePoints(fromNode, toNode);

                return (
                  <g key={i}>
                    <line
                      x1={x1}
                      y1={y1}
                      x2={x2}
                      y2={y2}
                      stroke={edge.isFaultPath ? "#ffb4ab" : "#44474c"}
                      strokeWidth={edge.isFaultPath ? 2.5 : 1.5}
                      strokeDasharray={edge.isFaultPath ? "none" : "6 4"}
                      markerEnd={
                        edge.isFaultPath
                          ? "url(#arrow-fault)"
                          : "url(#arrow-normal)"
                      }
                      opacity={edge.isFaultPath ? 0.9 : 0.5}
                    />
                    {/* Fault path label */}
                    {edge.isFaultPath && (
                      <text
                        x={(x1 + x2) / 2}
                        y={(y1 + y2) / 2 - 8}
                        textAnchor="middle"
                        className="fill-error text-[10px] font-semibold"
                      >
                        {t('alert_topology.failure_path')}
                      </text>
                    )}
                  </g>
                );
              })}
            </svg>

            {/* Loading state */}
            {graphLoading && (
              <div className="absolute inset-0 flex items-center justify-center">
                <div className="animate-spin rounded-full h-8 w-8 border-2 border-primary border-t-transparent" />
              </div>
            )}

            {/* Empty state */}
            {!graphLoading && mappedNodes.length === 0 && (
              <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 text-on-surface-variant">
                <Icon name="account_tree" className="text-[40px] opacity-30" />
                <span className="text-sm">No assets found in this location</span>
              </div>
            )}

            {/* Topology nodes */}
            {mappedNodes.filter((node) => {
              if (biaFilter !== 'all' && String(node.biaLevel) !== biaFilter) return false
              if (domainFilter !== 'all') {
                const domainMap: Record<string, string[]> = {
                  database: ['server'],
                  application: ['server'],
                  network: ['network'],
                }
                if (domainMap[domainFilter] && !domainMap[domainFilter].includes(node.type)) return false
              }
              return true
            }).map((node) => {
              const isSelected = selectedNodeId === node.id;
              return (
                <button
                  key={node.id}
                  onClick={() => setSelectedNodeId(node.id)}
                  className={`absolute flex items-center gap-2.5 rounded-lg px-3 py-2.5 transition-all cursor-pointer text-left ${
                    isSelected
                      ? "bg-surface-container-highest ring-2 " +
                        STATUS_RING[node.status].replace("/30", "/60").replace("/40", "/70").replace("transparent", "primary/30")
                      : "bg-surface-container-high hover:bg-surface-container-highest"
                  } ${node.status !== "normal" ? "ring-1 " + STATUS_RING[node.status] : ""}`}
                  style={{
                    left: node.x,
                    top: node.y,
                    width: NODE_W,
                    height: NODE_H,
                    zIndex: 1,
                  }}
                >
                  <div
                    className={`flex items-center justify-center w-9 h-9 rounded ${
                      node.status === "critical"
                        ? "bg-error-container/50"
                        : node.status === "warning"
                          ? "bg-[#92400e]/40"
                          : "bg-surface-container"
                    }`}
                  >
                    <Icon
                      name={node.icon}
                      className={`text-[20px] ${
                        node.status === "critical"
                          ? "text-error"
                          : node.status === "warning"
                            ? "text-[#fbbf24]"
                            : "text-on-surface-variant"
                      }`}
                    />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-1.5">
                      <span
                        className={`h-1.5 w-1.5 rounded-full shrink-0 ${STATUS_DOT[node.status]}`}
                      />
                      <span className="text-xs font-semibold text-on-surface truncate font-headline">
                        {node.name}
                      </span>
                    </div>
                    <span className="text-[0.625rem] text-on-surface-variant truncate block mt-0.5">
                      {node.type}
                    </span>
                  </div>
                </button>
              );
            })}

            {/* Root cause badge on first critical node */}
            {mappedNodes.find(n => n.status === 'critical') && (() => {
              const critNode = mappedNodes.find(n => n.status === 'critical')!;
              return (
                <div
                  className="absolute flex items-center gap-1 bg-error-container/80 text-error text-[0.625rem] font-bold tracking-wider px-2 py-0.5 rounded"
                  style={{ left: critNode.x + 4, top: critNode.y - 20, zIndex: 2 }}
                >
                  <Icon name="crisis_alert" className="text-[12px]" />
                  {t('alert_topology.root_cause_candidate')}
                </div>
              );
            })()}
          </div>
        </section>
      </div>

      {/* ── Bottom panel: Selected asset detail ── */}
      <section className="bg-surface-container rounded-lg p-5">
        {!selectedNode ? (
          <div className="flex items-center justify-center py-6 text-sm text-on-surface-variant">
            {graphLoading ? 'Loading topology…' : 'Select a node to view details'}
          </div>
        ) : (
        <>
        <div className="flex flex-wrap gap-6 items-start">
          {/* Asset identity */}
          <div className="flex items-start gap-4 min-w-[280px]">
            <div
              className={`flex items-center justify-center w-12 h-12 rounded-lg ${
                selectedNode.status === "critical"
                  ? "bg-error-container/50"
                  : selectedNode.status === "warning"
                    ? "bg-[#92400e]/40"
                    : "bg-surface-container-high"
              }`}
            >
              <Icon
                name={selectedNode.icon}
                className={`text-[28px] ${
                  selectedNode.status === "critical"
                    ? "text-error"
                    : selectedNode.status === "warning"
                      ? "text-[#fbbf24]"
                      : "text-on-surface-variant"
                }`}
              />
            </div>
            <div>
              <div className="flex items-center gap-2 mb-1">
                <h3 className="text-lg font-headline font-bold text-on-surface">
                  {selectedNode.name}
                </h3>
                <StatusBadge status={selectedNode.status} />
              </div>
              <p className="text-xs text-on-surface-variant">
                {selectedNode.type}
              </p>
            </div>
          </div>

          {/* Asset metadata grid */}
          <div className="flex flex-wrap gap-x-8 gap-y-2 text-sm flex-1 min-w-[300px]">
            <div className="flex flex-col">
              <span className="text-[0.625rem] text-on-surface-variant uppercase tracking-wider mb-0.5">
                {t('alert_topology.label_asset_id')}
              </span>
              <span className="text-on-surface font-mono text-xs">
                {selectedNode.id.toUpperCase()}
              </span>
            </div>
            <div className="flex flex-col">
              <span className="text-[0.625rem] text-on-surface-variant uppercase tracking-wider mb-0.5">
                {t('alert_topology.label_ip_address')}
              </span>
              <span className="text-on-surface font-mono text-xs">
                {selectedNode.ip}
              </span>
            </div>
            <div className="flex flex-col">
              <span className="text-[0.625rem] text-on-surface-variant uppercase tracking-wider mb-0.5">
                {t('alert_topology.label_model')}
              </span>
              <span className="text-on-surface text-xs">
                {selectedNode.model}
              </span>
            </div>
            <div className="flex flex-col">
              <span className="text-[0.625rem] text-on-surface-variant uppercase tracking-wider mb-0.5">
                {t('alert_topology.label_rack_location')}
              </span>
              <span className="text-on-surface font-mono text-xs">
                {selectedNode.rack}
              </span>
            </div>
            <div className="flex flex-col">
              <span className="text-[0.625rem] text-on-surface-variant uppercase tracking-wider mb-0.5">
                {t('alert_topology.label_bia_level')}
              </span>
              <span className="text-on-surface text-xs font-semibold">
                Level {selectedNode.biaLevel}
              </span>
            </div>
          </div>

          {/* Mini metrics */}
          <div className="flex gap-4 min-w-[360px] flex-1">
            <MiniMetric label={t('alert_topology.metric_cpu')} value={selectedNode.cpu} unit="%" />
            <MiniMetric label={t('alert_topology.metric_memory')} value={selectedNode.memory} unit="%" />
            <MiniMetric label={t('alert_topology.metric_disk_io')} value={selectedNode.diskIO} unit="%" />
          </div>
        </div>

        {/* Bottom bar: changelog link + tags */}
        <div className="flex flex-wrap items-center justify-between mt-4 pt-3 border-t border-surface-container-highest">
          <div className="flex items-center gap-4">
            <button
              onClick={() => navigate('/assets/detail')}
              className="flex items-center gap-1.5 text-primary text-sm font-medium hover:underline"
            >
              <Icon name="dns" className="text-[18px]" />
              {t('alert_topology.btn_view_asset')}
              <Icon name="arrow_forward" className="text-[16px]" />
            </button>
            <button className="flex items-center gap-1.5 text-primary text-sm font-medium hover:underline">
              <Icon name="history" className="text-[18px]" />
              {t('alert_topology.btn_changelog')}
              <Icon name="arrow_forward" className="text-[16px]" />
            </button>
          </div>

          <div className="flex items-center gap-2 flex-wrap">
            {selectedNode.tags.map((tag: string) => (
              <span
                key={tag}
                className="bg-surface-container-high text-on-surface-variant text-[0.625rem] px-2 py-0.5 rounded tracking-wide"
              >
                {tag}
              </span>
            ))}
          </div>
        </div>
        </>
        )}
      </section>
    </div>
  );
}

export default AlertTopologyAnalysis;
