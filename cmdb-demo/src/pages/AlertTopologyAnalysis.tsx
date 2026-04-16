import { memo, useState, useMemo, useCallback, useEffect, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  ReactFlow,
  Background,
  type Node,
  type Edge,
  type NodeProps,
  type NodeMouseHandler,
  Handle,
  Position,
  useNodesState,
  useEdgesState,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import type { ElkNode } from "elkjs/lib/elk-api";
import Icon from "../components/Icon";
import StatusBadge from "../components/StatusBadge";
import { useAlerts } from "../hooks/useMonitoring";
import type { AlertEvent } from "../lib/api/monitoring";
import { useTopologyGraph, useAllLocations } from "../hooks/useTopology";
import { useLocationContext } from "../contexts/LocationContext";
import { FALLBACK_ALERTS } from "../data/fallbacks/alerts";

/* ──────────────────────────────────────────────
   Types
   ────────────────────────────────────────────── */

interface ApiTopologyNode {
  id: string;
  name: string;
  type: string;
  status: string;
  has_active_alert: boolean;
  ip_address?: string;
  model?: string;
  rack_name?: string;
  bia_level?: number;
  metrics?: {
    cpu?: number;
    memory?: number;
    disk_io?: number;
  };
  tags?: string[];
}

interface ApiTopologyEdge {
  source?: string;
  from?: string;
  target?: string;
  to?: string;
  isFaultPath?: boolean;
  is_fault_path?: boolean;
}

interface TopologyGraphResponse {
  nodes: ApiTopologyNode[];
  edges: ApiTopologyEdge[];
}

interface TopologyNodeData {
  label: string;
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
  isRootCause?: boolean;
}

interface AlertItem {
  id: string;
  severity: "CRITICAL" | "WARNING";
  assetName: string;
  description: string;
  timestamp: string;
  nodeId: string;
}

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
        {t("alert_topology.live")}
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
    value >= 85 ? "text-error" : value >= 65 ? "text-[#fbbf24]" : "text-[#34d399]";
  const barColor =
    value >= 85 ? "bg-error" : value >= 65 ? "bg-[#fbbf24]" : "bg-[#34d399]";

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
   Custom ReactFlow Node
   ────────────────────────────────────────────── */

function AssetNode({ data, selected }: NodeProps) {
  const d = data as unknown as TopologyNodeData;
  const statusRing =
    d.status === "critical"
      ? "ring-error/60"
      : d.status === "warning"
        ? "ring-[#fbbf24]/40"
        : "";

  return (
    <>
      <Handle type="target" position={Position.Top} className="!bg-transparent !border-0 !w-0 !h-0" />
      <Handle type="source" position={Position.Bottom} className="!bg-transparent !border-0 !w-0 !h-0" />
      {d.isRootCause && (
        <div className="absolute -top-5 left-1 flex items-center gap-1 bg-error-container/80 text-error text-[0.6rem] font-bold tracking-wider px-2 py-0.5 rounded z-10">
          <span className="material-symbols-outlined text-[11px]">crisis_alert</span>
          根因候选
        </div>
      )}
      <div
        className={`flex items-center gap-2.5 rounded-lg px-3 py-2.5 cursor-pointer w-[180px] h-[68px] transition-all ${
          selected
            ? "bg-surface-container-highest ring-2 ring-primary/40"
            : "bg-surface-container-high hover:bg-surface-container-highest"
        } ${d.status !== "normal" ? `ring-1 ${statusRing}` : ""}`}
      >
        <div
          className={`flex items-center justify-center w-9 h-9 rounded shrink-0 ${
            d.status === "critical"
              ? "bg-error-container/50"
              : d.status === "warning"
                ? "bg-[#92400e]/40"
                : "bg-surface-container"
          }`}
        >
          <span
            className={`material-symbols-outlined text-[20px] ${
              d.status === "critical"
                ? "text-error"
                : d.status === "warning"
                  ? "text-[#fbbf24]"
                  : "text-on-surface-variant"
            }`}
          >
            {d.icon}
          </span>
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-1.5">
            <span className={`h-1.5 w-1.5 rounded-full shrink-0 ${STATUS_DOT[d.status]}`} />
            <span className="text-xs font-semibold text-on-surface truncate font-headline">
              {d.label}
            </span>
          </div>
          <span className="text-[0.625rem] text-on-surface-variant truncate block mt-0.5">
            {d.type}
          </span>
        </div>
      </div>
    </>
  );
}

const nodeTypes = { asset: AssetNode };

/* ──────────────────────────────────────────────
   Main component
   ────────────────────────────────────────────── */

function AlertTopologyAnalysis() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [selectedNodeId, setSelectedNodeId] = useState<string>("");
  const [biaFilter, setBiaFilter] = useState<string>("all");
  const [domainFilter, setDomainFilter] = useState<string>("all");

  // Derive location from context — use deepest available level, fallback to first campus/idc
  const { path } = useLocationContext();
  const contextLocationId = path.idc?.id ?? path.campus?.id ?? path.city?.id ?? '';

  const { data: allLocResp } = useAllLocations();
  const fallbackLocationId = useMemo(() => {
    const locs = allLocResp?.data ?? [];
    return locs.find(l => l.level === 'idc')?.id
      ?? locs.find(l => l.level === 'campus')?.id
      ?? locs[0]?.id
      ?? '';
  }, [allLocResp]);

  const locationId = contextLocationId || fallbackLocationId;

  const { data: graphData, isLoading: graphLoading } = useTopologyGraph(locationId);
  const apiNodes = (graphData as unknown as TopologyGraphResponse)?.nodes ?? [];
  const apiEdges = (graphData as unknown as TopologyGraphResponse)?.edges ?? [];

  // Find first critical node for root cause badge
  const firstCriticalId = useMemo(() => {
    const crit = apiNodes.find((n: ApiTopologyNode) => n.has_active_alert);
    return crit?.id ?? null;
  }, [apiNodes]);

  // Filter API nodes by current filters
  const filteredApiNodes = useMemo(
    () =>
      apiNodes.filter((n: ApiTopologyNode) => {
        if (biaFilter !== "all" && String(n.bia_level || 3) !== biaFilter) return false;
        if (domainFilter !== "all") {
          const domainMap: Record<string, string[]> = {
            database: ["server"],
            application: ["server"],
            network: ["network"],
          };
          if (domainMap[domainFilter] && !domainMap[domainFilter].includes(n.type)) return false;
        }
        return true;
      }),
    [apiNodes, biaFilter, domainFilter]
  );

  // Map API node => data payload (stable ref)
  const buildNodeData = useCallback(
    (n: ApiTopologyNode): TopologyNodeData => ({
      label: n.name,
      type: n.type,
      icon: n.type === "server" ? "dns" : n.type === "network" ? "router" : n.type === "storage" ? "storage" : "bolt",
      status: n.has_active_alert ? "critical" : n.status === "operational" ? "normal" : "warning",
      ip: n.ip_address || "",
      model: n.model || "",
      rack: n.rack_name || "",
      biaLevel: n.bia_level || 3,
      cpu: n.metrics?.cpu ?? 0,
      memory: n.metrics?.memory ?? 0,
      diskIO: n.metrics?.disk_io ?? 0,
      tags: n.tags || [],
      isRootCause: n.id === firstCriticalId,
    }),
    [firstCriticalId]
  );

  // Convert API edges => ReactFlow edges
  const flowEdges: Edge[] = useMemo(
    () =>
      apiEdges.map((e: ApiTopologyEdge) => {
        const isFault = e.isFaultPath || e.is_fault_path || false;
        return {
          id: `${e.source ?? e.from ?? ""}-${e.target ?? e.to ?? ""}`,
          source: e.source ?? e.from ?? "",
          target: e.target ?? e.to ?? "",
          animated: isFault,
          style: {
            stroke: isFault ? "#ffb4ab" : "#555",
            strokeWidth: isFault ? 2.5 : 1.5,
          },
          label: isFault ? t("alert_topology.failure_path") : undefined,
          labelStyle: isFault ? { fill: "#ffb4ab", fontSize: 10, fontWeight: 600 } : undefined,
          labelBgStyle: isFault ? { fill: "#1a1c1e", fillOpacity: 0.8 } : undefined,
        };
      }),
    [apiEdges, t]
  );

  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);

  // Cache: ELK layout positions computed once from ALL api nodes
  const layoutCacheRef = useRef<Record<string, { x: number; y: number }>>({});

  // Step 1: Run ELK layout ONCE when apiNodes/apiEdges change (not on filter change)
  useEffect(() => {
    if (apiNodes.length === 0) {
      layoutCacheRef.current = {};
      return;
    }

    const NODE_WIDTH = 190;
    const NODE_HEIGHT = 72;

    if (apiEdges.length === 0) {
      // Grid fallback for all nodes
      const cache: Record<string, { x: number; y: number }> = {};
      apiNodes.forEach((n: ApiTopologyNode, i: number) => {
        cache[n.id] = { x: (i % 4) * 230 + 30, y: Math.floor(i / 4) * 110 + 30 };
      });
      layoutCacheRef.current = cache;
      return;
    }

    import("elkjs/lib/elk.bundled.js").then((ELK) => {
      const elk = new ELK.default();
      const graph = {
        id: "root",
        layoutOptions: {
          "elk.algorithm": "layered",
          "elk.direction": "DOWN",
          "elk.spacing.nodeNode": "60",
          "elk.layered.spacing.nodeNodeBetweenLayers": "80",
          "elk.edgeRouting": "ORTHOGONAL",
        },
        children: apiNodes.map((n: ApiTopologyNode) => ({
          id: n.id, width: NODE_WIDTH, height: NODE_HEIGHT,
        })),
        edges: apiEdges.map((e: ApiTopologyEdge) => ({
          id: `e-${e.source ?? e.from ?? ""}-${e.target ?? e.to ?? ""}`,
          sources: [e.source ?? e.from ?? ""],
          targets: [e.target ?? e.to ?? ""],
        })),
      };
      elk.layout(graph).then((result: ElkNode) => {
        const cache: Record<string, { x: number; y: number }> = {};
        (result.children || []).forEach((elkNode: ElkNode) => {
          cache[elkNode.id] = { x: elkNode.x ?? 0, y: elkNode.y ?? 0 };
        });
        layoutCacheRef.current = cache;
        // Trigger re-render by applying filtered view
        applyFilteredView();
      }).catch(() => {
        const cache: Record<string, { x: number; y: number }> = {};
        apiNodes.forEach((n: ApiTopologyNode, i: number) => {
          cache[n.id] = { x: (i % 4) * 230 + 30, y: Math.floor(i / 4) * 110 + 30 };
        });
        layoutCacheRef.current = cache;
        applyFilteredView();
      });
    });
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [apiNodes.length, apiEdges.length]);

  // Step 2: Apply filter — just pick nodes/edges from the cached layout
  const applyFilteredView = useCallback(() => {
    const cache = layoutCacheRef.current;
    const filteredIds = new Set(filteredApiNodes.map(n => n.id));

    const visibleNodes: Node[] = filteredApiNodes.map((n: ApiTopologyNode, i: number) => ({
      id: n.id,
      type: "asset",
      position: cache[n.id] ?? { x: (i % 4) * 230 + 30, y: Math.floor(i / 4) * 110 + 30 },
      data: buildNodeData(n) as unknown as Record<string, unknown>,
    }));

    // Only show edges where both endpoints are visible
    const visibleEdges = flowEdges.filter(e => filteredIds.has(e.source) && filteredIds.has(e.target));

    setNodes(visibleNodes);
    setEdges(visibleEdges);
  }, [filteredApiNodes, flowEdges, buildNodeData, setNodes, setEdges]);

  // Re-apply when filters change (cheap — no ELK re-layout)
  useEffect(() => {
    if (Object.keys(layoutCacheRef.current).length > 0 || apiEdges.length === 0) {
      applyFilteredView();
    }
  }, [applyFilteredView]);

  // Handle node click
  const onNodeClick = useCallback<NodeMouseHandler>((_event, node) => {
    setSelectedNodeId(node.id);
  }, []);

  // Build selectedNode from current selection
  const selectedNode = useMemo(() => {
    const n = nodes.find((n) => n.id === selectedNodeId) ?? nodes[0];
    return n ? (n.data as unknown as TopologyNodeData) : null;
  }, [selectedNodeId, nodes]);

  const selectedNodeFull = useMemo(() => {
    if (!selectedNode) return null;
    const n = nodes.find((nd) => nd.id === selectedNodeId) ?? nodes[0];
    if (!n) return null;
    const d = n.data as unknown as TopologyNodeData;
    return { id: n.id, ...d };
  }, [selectedNode, selectedNodeId, nodes]);

  // Alerts
  const { data: alertsResponse } = useAlerts();
  const apiAlerts = alertsResponse?.data ?? [];
  const isUsingFallbackAlerts = apiAlerts.length === 0;
  const ALERTS: AlertItem[] =
    apiAlerts.length > 0
      ? apiAlerts.map((a: AlertEvent) => {
          const assetId = a.ci_id ?? "";
          const matchedNode = apiNodes.find((n: ApiTopologyNode) => n.id === assetId);
          return {
            id: a.id,
            severity: (a.severity ?? "").toUpperCase() as "CRITICAL" | "WARNING",
            assetName: matchedNode?.name ?? assetId.slice(0, 12) ?? "Unknown",
            description: a.message ?? "",
            timestamp: a.fired_at ? new Date(a.fired_at).toLocaleString() : "\u2014",
            nodeId: assetId,
          };
        })
      : FALLBACK_ALERTS;

  return (
    <div className="flex flex-col gap-5 font-body text-on-surface min-h-0">
      {/* ── Header ─────────────────────────────── */}
      <header className="flex flex-col gap-3">
        <nav className="flex items-center gap-1.5 text-xs text-on-surface-variant">
          <span className="cursor-pointer hover:text-primary" onClick={() => navigate("/monitoring")}>
            {t("alert_topology.breadcrumb_system_monitoring")}
          </span>
          <Icon name="chevron_right" className="text-[16px] opacity-50" />
          <span className="text-primary">{t("alert_topology.breadcrumb_topology_view")}</span>
        </nav>

        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="flex items-center">
            <h1 className="text-2xl font-headline font-bold tracking-tight text-on-surface">
              {t("alert_topology.title_zh")}{" "}
              <span className="text-on-surface-variant font-normal text-lg">{t("alert_topology.title")}</span>
            </h1>
            <LiveIndicator />
          </div>

          <div className="flex items-center gap-3 flex-wrap">
            <select
              value={biaFilter}
              onChange={(e) => setBiaFilter(e.target.value)}
              className="bg-surface-container-high text-on-surface text-sm rounded px-3 py-2 focus:outline-none focus:ring-1 focus:ring-primary/40 appearance-none cursor-pointer"
            >
              <option value="all">{t("alert_topology.filter_all_bia_levels")}</option>
              <option value="1">{t("alert_topology.filter_bia_level")} 1</option>
              <option value="2">{t("alert_topology.filter_bia_level")} 2</option>
              <option value="3">{t("alert_topology.filter_bia_level")} 3</option>
            </select>

            <select
              value={domainFilter}
              onChange={(e) => setDomainFilter(e.target.value)}
              className="bg-surface-container-high text-on-surface text-sm rounded px-3 py-2 focus:outline-none focus:ring-1 focus:ring-primary/40 appearance-none cursor-pointer"
            >
              <option value="all">{t("alert_topology.filter_service_domain")}</option>
              <option value="database">{t("alert_topology.filter_database")}</option>
              <option value="application">{t("alert_topology.filter_application")}</option>
              <option value="network">{t("alert_topology.filter_network")}</option>
            </select>

            <button className="flex items-center gap-1.5 bg-surface-container-high text-on-surface-variant text-sm px-4 py-2 rounded hover:bg-surface-container-highest transition-colors">
              <Icon name="restart_alt" className="text-[18px]" />
              {t("alert_topology.btn_reset_view")}
            </button>
            <button className="flex items-center gap-1.5 bg-primary/15 text-primary text-sm px-4 py-2 rounded hover:bg-primary/25 transition-colors font-semibold">
              <Icon name="download" className="text-[18px]" />
              {t("alert_topology.btn_export_report")}
            </button>
          </div>
        </div>
      </header>

      {/* ── Main content area ──────────────────── */}
      <div className="flex gap-4 flex-1 min-h-0" style={{ height: "520px" }}>
        {/* Left panel: Alert list (40%) */}
        <section className="w-[40%] min-w-[320px] flex flex-col bg-surface-container rounded-lg overflow-hidden">
          <div className="flex items-center justify-between px-4 py-3 bg-surface-container-high">
            <h2 className="text-sm font-headline font-semibold text-on-surface flex items-center gap-2">
              <Icon name="link" className="text-[18px] text-primary" />
              {t("alert_topology.alert_list_title")}
              <span className="text-on-surface-variant font-normal">({ALERTS.length})</span>
              {isUsingFallbackAlerts && (
                <span className="text-[0.6rem] bg-tertiary-container text-tertiary px-1.5 py-0.5 rounded font-bold tracking-wider">DEMO</span>
              )}
            </h2>
            <span className="text-xs text-on-surface-variant">{t("alert_topology.correlated")}</span>
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
                      {t("alert_topology.view_link")}
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
          <div className="flex items-center justify-between px-4 py-3 bg-surface-container-high">
            <h2 className="text-sm font-headline font-semibold text-on-surface flex items-center gap-2">
              <Icon name="account_tree" className="text-[18px] text-primary" />
              {t("alert_topology.topology_title")}
            </h2>
            <div className="flex items-center gap-4 text-xs text-on-surface-variant">
              <span className="flex items-center gap-1.5">
                <span className="h-2 w-2 rounded-full bg-error" />
                {t("alert_topology.legend_critical")}
              </span>
              <span className="flex items-center gap-1.5">
                <span className="h-2 w-2 rounded-full bg-[#fbbf24]" />
                {t("alert_topology.legend_warning")}
              </span>
              <span className="flex items-center gap-1.5">
                <span className="h-2 w-2 rounded-full bg-[#34d399]" />
                {t("alert_topology.legend_normal")}
              </span>
            </div>
          </div>

          <div style={{ height: "calc(100% - 48px)" }}>
            {graphLoading ? (
              <div className="flex items-center justify-center h-full">
                <div className="animate-spin rounded-full h-8 w-8 border-2 border-primary border-t-transparent" />
              </div>
            ) : nodes.length === 0 ? (
              <div className="flex flex-col items-center justify-center h-full gap-2 text-on-surface-variant">
                <Icon name="account_tree" className="text-[40px] opacity-30" />
                <span className="text-sm">No assets found in this location</span>
              </div>
            ) : (
              <ReactFlow
                nodes={nodes}
                edges={edges}
                onNodesChange={onNodesChange}
                onEdgesChange={onEdgesChange}
                onNodeClick={onNodeClick}
                nodeTypes={nodeTypes}
                fitView
                fitViewOptions={{ padding: 0.3 }}
                minZoom={0.3}
                maxZoom={1.5}
                proOptions={{ hideAttribution: true }}
                className="!bg-surface-container"
              >
                <Background color="#2a2d32" gap={24} size={1} />
              </ReactFlow>
            )}
          </div>
        </section>
      </div>

      {/* ── Bottom panel: Selected asset detail ── */}
      <section className="bg-surface-container rounded-lg p-5">
        {!selectedNodeFull ? (
          <div className="flex items-center justify-center py-6 text-sm text-on-surface-variant">
            {graphLoading ? "Loading topology\u2026" : "Select a node to view details"}
          </div>
        ) : (
          <>
            <div className="flex flex-wrap gap-6 items-start">
              <div className="flex items-start gap-4 min-w-[280px]">
                <div
                  className={`flex items-center justify-center w-12 h-12 rounded-lg ${
                    selectedNodeFull.status === "critical"
                      ? "bg-error-container/50"
                      : selectedNodeFull.status === "warning"
                        ? "bg-[#92400e]/40"
                        : "bg-surface-container-high"
                  }`}
                >
                  <Icon
                    name={selectedNodeFull.icon}
                    className={`text-[28px] ${
                      selectedNodeFull.status === "critical"
                        ? "text-error"
                        : selectedNodeFull.status === "warning"
                          ? "text-[#fbbf24]"
                          : "text-on-surface-variant"
                    }`}
                  />
                </div>
                <div>
                  <div className="flex items-center gap-2 mb-1">
                    <h3 className="text-lg font-headline font-bold text-on-surface">
                      {selectedNodeFull.label}
                    </h3>
                    <StatusBadge status={selectedNodeFull.status} />
                  </div>
                  <p className="text-xs text-on-surface-variant">{selectedNodeFull.type}</p>
                </div>
              </div>

              <div className="flex flex-wrap gap-x-8 gap-y-2 text-sm flex-1 min-w-[300px]">
                <div className="flex flex-col">
                  <span className="text-[0.625rem] text-on-surface-variant uppercase tracking-wider mb-0.5">
                    {t("alert_topology.label_asset_id")}
                  </span>
                  <span className="text-on-surface font-mono text-xs">{selectedNodeFull.id.toUpperCase()}</span>
                </div>
                <div className="flex flex-col">
                  <span className="text-[0.625rem] text-on-surface-variant uppercase tracking-wider mb-0.5">
                    {t("alert_topology.label_ip_address")}
                  </span>
                  <span className="text-on-surface font-mono text-xs">{selectedNodeFull.ip}</span>
                </div>
                <div className="flex flex-col">
                  <span className="text-[0.625rem] text-on-surface-variant uppercase tracking-wider mb-0.5">
                    {t("alert_topology.label_model")}
                  </span>
                  <span className="text-on-surface text-xs">{selectedNodeFull.model}</span>
                </div>
                <div className="flex flex-col">
                  <span className="text-[0.625rem] text-on-surface-variant uppercase tracking-wider mb-0.5">
                    {t("alert_topology.label_rack_location")}
                  </span>
                  <span className="text-on-surface font-mono text-xs">{selectedNodeFull.rack}</span>
                </div>
                <div className="flex flex-col">
                  <span className="text-[0.625rem] text-on-surface-variant uppercase tracking-wider mb-0.5">
                    {t("alert_topology.label_bia_level")}
                  </span>
                  <span className="text-on-surface text-xs font-semibold">Level {selectedNodeFull.biaLevel}</span>
                </div>
              </div>

              <div className="flex gap-4 min-w-[360px] flex-1">
                <MiniMetric label={t("alert_topology.metric_cpu")} value={selectedNodeFull.cpu} unit="%" />
                <MiniMetric label={t("alert_topology.metric_memory")} value={selectedNodeFull.memory} unit="%" />
                <MiniMetric label={t("alert_topology.metric_disk_io")} value={selectedNodeFull.diskIO} unit="%" />
              </div>
            </div>

            <div className="flex flex-wrap items-center justify-between mt-4 pt-3 border-t border-surface-container-highest">
              <div className="flex items-center gap-4">
                <button
                  onClick={() => navigate("/assets/detail")}
                  className="flex items-center gap-1.5 text-primary text-sm font-medium hover:underline"
                >
                  <Icon name="dns" className="text-[18px]" />
                  {t("alert_topology.btn_view_asset")}
                  <Icon name="arrow_forward" className="text-[16px]" />
                </button>
                <button className="flex items-center gap-1.5 text-primary text-sm font-medium hover:underline">
                  <Icon name="history" className="text-[18px]" />
                  {t("alert_topology.btn_changelog")}
                  <Icon name="arrow_forward" className="text-[16px]" />
                </button>
              </div>

              <div className="flex items-center gap-2 flex-wrap">
                {selectedNodeFull.tags.map((tag: string) => (
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
