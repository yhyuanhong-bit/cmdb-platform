import { toast } from 'sonner'
import { useState, useMemo, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { useQuery, useQueries } from "@tanstack/react-query";
import { useRacks, useRootLocations } from "../hooks/useTopology";
import { useLocationContext } from "../contexts/LocationContext";
import { useAlerts } from "../hooks/useMonitoring";
import { apiClient } from "../lib/api/client";
import { topologyApi } from "../lib/api/topology";
import type { Rack, Location } from "../lib/api/topology";
import type { AlertEvent } from "../lib/api/monitoring";

interface TreeNode {
  id: string;
  label: string;
  children?: TreeNode[];
  active?: boolean;
}

interface RackCell {
  row: number;
  col: number;
  id: string;
  label: string;
  status: "normal" | "critical" | "selected";
}


function buildRackGrid(racks: Rack[]): RackCell[] {
  if (racks.length === 0) {
    // Fallback: generate default grid
    const rows = 4;
    const cols = 8;
    const cells: RackCell[] = [];
    for (let r = 0; r < rows; r++) {
      for (let c = 0; c < cols; c++) {
        const rowLabel = String.fromCharCode(65 + r);
        const colLabel = String(c + 1).padStart(2, "0");
        const id = `fallback-${rowLabel}${colLabel}`;
        const label = `${rowLabel}${colLabel}`;
        cells.push({ row: r, col: c, id, label, status: "normal" });
      }
    }
    return cells;
  }
  // Group by row_label, assign positions
  const rowMap = new Map<string, Rack[]>();
  racks.forEach((r) => {
    const key = r.row_label || "A";
    if (!rowMap.has(key)) rowMap.set(key, []);
    rowMap.get(key)!.push(r);
  });
  const cells: RackCell[] = [];
  const sortedRows = Array.from(rowMap.keys()).sort();
  sortedRows.forEach((rowLabel, rowIdx) => {
    rowMap.get(rowLabel)!.forEach((rack, colIdx) => {
      const pct = rack.total_u > 0 ? (rack.used_u / rack.total_u) * 100 : 0;
      let status: RackCell["status"] = "normal";
      if (rack.status === "MAINTENANCE" || pct >= 95) status = "critical";
      cells.push({ row: rowIdx, col: colIdx, id: rack.id, label: rack.name || rack.id.slice(0, 6), status });
    });
  });
  return cells;
}

// alerts are fetched from API inside the component

function TreeItem({
  node,
  depth,
  expanded,
  onToggle,
}: {
  node: TreeNode;
  depth: number;
  expanded: Record<string, boolean>;
  onToggle: (id: string) => void;
}) {
  const { t } = useTranslation();
  const isExpanded = expanded[node.id] !== false;
  const hasChildren = node.children && node.children.length > 0;

  return (
    <div>
      <button
        onClick={() => onToggle(node.id)}
        className={`flex items-center gap-1.5 w-full text-left py-1.5 px-2 rounded-lg text-sm transition-colors ${
          node.active
            ? "bg-primary-container text-primary font-medium"
            : "text-on-surface-variant hover:bg-surface-container-high"
        }`}
        style={{ paddingLeft: `${depth * 16 + 8}px` }}
      >
        {hasChildren && (
          <span
            className="material-symbols-outlined text-base transition-transform"
            style={{
              transform: isExpanded ? "rotate(90deg)" : "rotate(0deg)",
            }}
          >
            chevron_right
          </span>
        )}
        {!hasChildren && <span className="w-4" />}
        <span className="material-symbols-outlined text-base">
          {hasChildren ? "folder" : node.active ? "check_circle" : "circle"}
        </span>
        <span className="truncate">{node.label}</span>
        {node.active && (
          <span className="ml-auto text-[10px] bg-on-primary-container/20 text-primary px-1.5 py-0.5 rounded">
            {t('datacenter_3d.label_active')}
          </span>
        )}
      </button>
      {hasChildren && isExpanded && (
        <div>
          {node.children!.map((child) => (
            <TreeItem
              key={child.id}
              node={child}
              depth={depth + 1}
              expanded={expanded}
              onToggle={onToggle}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export default function DataCenter3D() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { path } = useLocationContext();

  // Fetch ALL territories and their descendants to build the complete tree
  const { data: rootResp } = useRootLocations();
  const territories = rootResp?.data ?? [];

  // Fetch all descendants for every territory in parallel
  const allDescQueries = useQueries({
    queries: territories.map(t => ({
      queryKey: ['locations', t.id, 'descendants'] as const,
      queryFn: () => topologyApi.listDescendants(t.id),
      enabled: !!t.id,
    })),
  });

  // Merge all territories' descendants into one flat list (deduped by id)
  const allLocations = useMemo(() => {
    const seen = new Set<string>();
    const result: Location[] = [];
    for (const q of allDescQueries) {
      for (const loc of q.data?.data ?? []) {
        if (!seen.has(loc.id)) {
          seen.add(loc.id);
          result.push(loc);
        }
      }
    }
    return result;
  }, [allDescQueries]);

  const [selectedLocationId, setSelectedLocationId] = useState('');

  // Set initial selected location to deepest level with racks (room > module > idc > campus)
  useEffect(() => {
    if (!selectedLocationId && allLocations.length > 0) {
      const firstRoom = allLocations.find(l => l.level === 'room')
        || allLocations.find(l => l.level === 'module')
        || allLocations.find(l => l.level === 'idc')
        || allLocations.find(l => l.level === 'campus');
      if (firstRoom) setSelectedLocationId(firstRoom.id);
    }
  }, [allLocations, selectedLocationId]);

  // Build tree from flat locations
  function buildTree(parentId: string | null): TreeNode[] {
    return allLocations
      .filter(loc => {
        if (parentId === null) return false;
        return loc.parent_id === parentId;
      })
      .sort((a, b) => a.sort_order - b.sort_order)
      .map(loc => {
        const children = buildTree(loc.id);
        return {
          id: loc.id,
          label: loc.name_en || loc.name,
          children: children.length > 0 ? children : undefined,
          active: loc.id === selectedLocationId,
        };
      });
  }

  // Build a tree node for EACH territory (root level)
  const locationTree: TreeNode[] = territories
    .sort((a, b) => a.sort_order - b.sort_order)
    .map(t => ({
      id: t.id,
      label: t.name_en || t.name,
      children: buildTree(t.id),
      active: t.id === selectedLocationId,
    }));

  // Fallback to Neihu campus if no location context set
  const contextLocationId = path.idc?.id ?? path.campus?.id ?? 'd0000000-0000-0000-0000-000000000004';
  const effectiveLocationId = selectedLocationId || contextLocationId;
  const { data: racksResponse, isLoading } = useRacks(effectiveLocationId);
  const apiRacks: Rack[] = racksResponse?.data ?? [];
  const rackGrid = useMemo(() => buildRackGrid(apiRacks), [apiRacks]);

  // Alerts from API (Task 8)
  const { data: alertsResp } = useAlerts({ status: 'firing' })
  const alerts: Array<{ level: string; text: string; color: string }> = (alertsResp?.data ?? []).slice(0, 5).map((a: AlertEvent) => ({
    level: (a.severity ?? '').toUpperCase() === 'CRITICAL' ? 'CRITICAL' : 'WARNING',
    text: (a.message as string) || `Alert: ${String(a.id ?? '').slice(0, 8)}`,
    color: (a.severity ?? '') === 'critical' ? 'text-error' : 'text-tertiary',
  }))

  // Energy summary from API (Task 9)
  const { data: energySummary } = useQuery({
    queryKey: ['energySummary3d'],
    queryFn: () => apiClient.get('/energy/summary'),
  })
  const eSummary = energySummary as any

  const [activeTab, setActiveTab] = useState("global");
  const [heatMode, setHeatMode] = useState(false);
  const [hoveredRack, setHoveredRack] = useState<string | null>(null);
  const [treeExpanded, setTreeExpanded] = useState<Record<string, boolean>>({});

  // Auto-expand all nodes when data loads
  useEffect(() => {
    if (territories.length > 0 && allLocations.length > 0) {
      const expanded: Record<string, boolean> = {};
      territories.forEach(t => { expanded[t.id] = true; });
      allLocations.forEach(loc => { expanded[loc.id] = true; });
      setTreeExpanded(expanded);
    }
  }, [territories, allLocations]);

  const tabs = [
    { id: "global", label: t('datacenter_3d.tab_global_view'), icon: "public" },
    { id: "health", label: t('datacenter_3d.tab_rack_health'), icon: "monitor_heart" },
    { id: "network", label: t('datacenter_3d.tab_network_mesh'), icon: "hub" },
  ];

  const handleTreeNodeClick = (nodeId: string) => {
    // Toggle expand/collapse
    setTreeExpanded(prev => ({ ...prev, [nodeId]: prev[nodeId] === false }));
    // Select any leaf-level location that can contain racks (room > module > idc > campus)
    const loc = allLocations.find(l => l.id === nodeId);
    if (loc && ['room', 'module', 'idc', 'campus'].includes(loc.level)) {
      setSelectedLocationId(nodeId);
    }
  };

  const selectedLocation = allLocations.find(l => l.id === selectedLocationId) || territories[0];
  const subtitle = selectedLocation ? (selectedLocation.name_en || selectedLocation.name) : 'Data Center';

  const getRackColor = (status: RackCell["status"], isHovered: boolean) => {
    if (heatMode) {
      if (status === "critical") return "bg-error/80";
      return "bg-on-primary-container/40";
    }
    switch (status) {
      case "critical":
        return isHovered ? "bg-error/90" : "bg-error/70";
      case "selected":
        return isHovered ? "bg-primary/90" : "bg-primary/70";
      default:
        return isHovered
          ? "bg-emerald-500/70"
          : "bg-emerald-500/50";
    }
  };

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      {/* Header */}
      <div className="bg-surface-container-low px-6 py-4">
        <nav
          aria-label="Breadcrumb"
          className="flex items-center gap-1.5 text-xs uppercase tracking-widest text-on-surface-variant mb-2"
        >
          <span
            className="cursor-pointer transition-colors hover:text-primary"
            onClick={() => navigate("/racks")}
          >
            {t('datacenter_3d.rack_management')}
          </span>
          <span className="text-[10px] opacity-40" aria-hidden="true">›</span>
          <span className="text-on-surface font-semibold">{t('datacenter_3d.title')}</span>
        </nav>
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-headline font-bold text-on-surface">
              {t('datacenter_3d.title')}
            </h1>
            <div className="flex items-center gap-2 mt-1 text-on-surface-variant text-sm">
              <span className="material-symbols-outlined text-base">location_on</span>
              {subtitle}
            </div>
          </div>
          <div className="flex items-center gap-2">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                  activeTab === tab.id
                    ? "bg-primary-container text-primary"
                    : "bg-surface-container text-on-surface-variant hover:bg-surface-container-high"
                }`}
              >
                <span className="material-symbols-outlined text-lg">{tab.icon}</span>
                {tab.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Main content */}
      <div className="flex flex-1" style={{ height: "calc(100vh - 88px)" }}>
        {/* Left panel - Location hierarchy */}
        <div className="w-72 bg-surface-container-low p-4 overflow-y-auto flex flex-col">
          <div className="flex items-center gap-2 mb-3">
            <span className="material-symbols-outlined text-primary">account_tree</span>
            <h2 className="text-sm font-headline font-semibold text-on-surface">
              {t('datacenter_3d.section_location_hierarchy')}
            </h2>
          </div>
          <div className="flex-1 space-y-0.5">
            {locationTree.map((node) => (
              <TreeItem
                key={node.id}
                node={node}
                depth={0}
                expanded={treeExpanded}
                onToggle={handleTreeNodeClick}
              />
            ))}
          </div>
          <button onClick={() => toast.info('Coming Soon')} className="mt-4 flex items-center justify-center gap-2 w-full py-2.5 rounded-lg bg-surface-container-high text-on-surface-variant text-sm font-medium hover:bg-surface-container-highest transition-colors">
            <span className="material-symbols-outlined text-lg">deployed_code</span>
            {t('datacenter_3d.btn_deploy_agent')}
          </button>
        </div>

        {/* Center - 3D Floor */}
        <div className="flex-1 flex flex-col bg-surface overflow-hidden">
          <div className="flex-1 flex items-center justify-center p-8">
            <div
              className="relative"
              style={{
                perspective: "900px",
                perspectiveOrigin: "50% 40%",
              }}
            >
              <div
                style={{
                  transform: "rotateX(55deg) rotateZ(-30deg)",
                  transformStyle: "preserve-3d",
                }}
              >
                {/* Floor plane */}
                <div
                  className="relative bg-surface-container-lowest/40 rounded-lg"
                  style={{
                    width: "640px",
                    height: "360px",
                    boxShadow: "0 0 80px rgba(158, 202, 255, 0.05)",
                  }}
                >
                  {/* Grid lines */}
                  {Array.from({ length: 5 }).map((_, i) => (
                    <div
                      key={`h-${i}`}
                      className="absolute left-0 right-0 bg-surface-container-high/40"
                      style={{
                        top: `${(i / 4) * 100}%`,
                        height: "1px",
                      }}
                    />
                  ))}
                  {Array.from({ length: 9 }).map((_, i) => (
                    <div
                      key={`v-${i}`}
                      className="absolute top-0 bottom-0 bg-surface-container-high/40"
                      style={{
                        left: `${(i / 8) * 100}%`,
                        width: "1px",
                      }}
                    />
                  ))}

                  {/* Rack blocks */}
                  {rackGrid.map((rack) => {
                    const isHovered = hoveredRack === rack.id;
                    const x = rack.col * 76 + 14;
                    const y = rack.row * 84 + 14;
                    return (
                      <div
                        key={rack.id}
                        className={`absolute rounded cursor-pointer transition-all duration-200 flex items-center justify-center ${getRackColor(
                          rack.status,
                          isHovered
                        )}`}
                        style={{
                          left: `${x}px`,
                          top: `${y}px`,
                          width: "60px",
                          height: "56px",
                          transform: isHovered
                            ? "translateZ(20px)"
                            : "translateZ(8px)",
                          transformStyle: "preserve-3d",
                          boxShadow: isHovered
                            ? "0 8px 24px rgba(0,0,0,0.5)"
                            : "0 4px 12px rgba(0,0,0,0.3)",
                        }}
                        onMouseEnter={() => setHoveredRack(rack.id)}
                        onMouseLeave={() => setHoveredRack(null)}
                        onClick={() => {
                          navigate('/racks/' + rack.id);
                        }}
                      >
                        <span className="text-[11px] font-medium text-white/90 font-label select-none">
                          {rack.label}
                        </span>
                      </div>
                    );
                  })}
                </div>
              </div>
            </div>
          </div>

          {/* Bottom toolbar */}
          <div className="bg-surface-container-low px-6 py-3 flex items-center justify-between">
            <div className="flex items-center gap-6">
              {/* Heat mode toggle */}
              <div className="flex items-center gap-2">
                <span className="text-xs text-on-surface-variant">{t('datacenter_3d.label_heat_mode')}</span>
                <button
                  onClick={() => setHeatMode(!heatMode)}
                  className={`relative w-10 h-5 rounded-full transition-colors ${
                    heatMode ? "bg-on-primary-container" : "bg-surface-container-highest"
                  }`}
                >
                  <div
                    className={`absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform ${
                      heatMode ? "translate-x-5" : "translate-x-0.5"
                    }`}
                  />
                </button>
              </div>

              {/* Temperature range */}
              <div className="flex items-center gap-2 text-xs text-on-surface-variant">
                <span>18°C</span>
                <div className="w-32 h-1.5 rounded-full bg-gradient-to-r from-blue-500 via-yellow-500 to-red-500" />
                <span>28°C</span>
                <span className="text-error">32°C</span>
              </div>

              {/* Zoom */}
              <div className="flex items-center gap-1">
                <button onClick={() => toast.info('Coming Soon')} className="p-1 rounded bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest">
                  <span className="material-symbols-outlined text-lg">remove</span>
                </button>
                <span className="text-xs text-on-surface-variant px-1">100%</span>
                <button onClick={() => toast.info('Coming Soon')} className="p-1 rounded bg-surface-container-high text-on-surface-variant hover:bg-surface-container-highest">
                  <span className="material-symbols-outlined text-lg">add</span>
                </button>
              </div>

              {/* Floor display */}
              <div className="flex items-center gap-2 text-xs text-on-surface-variant">
                <span className="material-symbols-outlined text-base">layers</span>
                {t('datacenter_3d.label_floor_display')}: Floor 2
              </div>
            </div>

            <button
              onClick={() => navigate("/monitoring/energy")}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-primary-container text-primary text-sm font-medium hover:bg-on-primary-container/20 transition-colors"
            >
              {t('datacenter_3d.btn_view_full_analytics')}
              <span className="material-symbols-outlined text-lg">arrow_forward</span>
            </button>
          </div>
        </div>

        {/* Right panel - Overview */}
        <div className="w-80 bg-surface-container-low p-4 overflow-y-auto">
          <div className="flex items-center gap-2 mb-4">
            <span className="material-symbols-outlined text-primary">dashboard</span>
            <h2 className="text-sm font-headline font-semibold text-on-surface">
              {t('datacenter_3d.section_overview')}
            </h2>
          </div>

          {/* Last update */}
          <div className="text-xs text-on-surface-variant mb-4 flex items-center gap-1.5">
            <span className="material-symbols-outlined text-sm">schedule</span>
            {t('datacenter_3d.label_last_update')}: {new Date().toLocaleString()}
          </div>

          {/* Stats grid */}
          <div className="grid grid-cols-2 gap-2 mb-6">
            {[
              { label: t('datacenter_3d.stat_power_consumption'), value: `${eSummary?.total_kw?.toFixed(1) ?? '-'} kW`, icon: 'bolt' },
              { label: 'PUE', value: eSummary?.pue?.toFixed(2) ?? '-', icon: 'speed' },
              { label: t('datacenter_3d.stat_avg_temperature'), value: '-', icon: 'thermostat' },
              { label: t('datacenter_3d.stat_cooling_efficiency'), value: '-', icon: 'ac_unit' },
            ].map((stat) => (
              <div
                key={stat.label}
                className="bg-surface-container rounded-xl p-3"
              >
                <div className="flex items-center gap-1.5 text-on-surface-variant mb-1">
                  <span className="material-symbols-outlined text-sm">{stat.icon}</span>
                  <span className="text-[11px]">{stat.label}</span>
                </div>
                <div className="text-lg font-headline font-bold text-on-surface">
                  {stat.value}
                </div>
              </div>
            ))}
          </div>

          {/* Alerts */}
          <div className="mb-4">
            <div className="flex items-center gap-2 mb-3">
              <span className="material-symbols-outlined text-error text-lg">warning</span>
              <h3 className="text-sm font-headline font-semibold text-on-surface">
                {t('datacenter_3d.section_alerts')} ({alerts.length})
              </h3>
            </div>
            <div className="space-y-2">
              {alerts.map((alert, i) => (
                <div
                  key={i}
                  className={`rounded-xl p-3 ${
                    alert.level === "CRITICAL"
                      ? "bg-error-container/30"
                      : "bg-tertiary-container/30"
                  }`}
                >
                  <div className="flex items-center gap-1.5 mb-1">
                    <span
                      className={`text-[10px] font-bold px-1.5 py-0.5 rounded ${
                        alert.level === "CRITICAL"
                          ? "bg-error/20 text-error"
                          : "bg-tertiary/20 text-tertiary"
                      }`}
                    >
                      {alert.level}
                    </span>
                  </div>
                  <p className="text-xs text-on-surface-variant">{alert.text}</p>
                </div>
              ))}
            </div>
          </div>

          {/* Legend */}
          <div className="bg-surface-container rounded-xl p-3">
            <h3 className="text-xs font-semibold text-on-surface-variant mb-2">
              {t('datacenter_3d.section_rack_status_legend')}
            </h3>
            <div className="space-y-1.5">
              {[
                { color: "bg-emerald-500/50", label: t('datacenter_3d.legend_normal') },
                { color: "bg-error/70", label: t('datacenter_3d.legend_critical') },
                { color: "bg-primary/70", label: t('datacenter_3d.legend_selected') },
              ].map((item) => (
                <div key={item.label} className="flex items-center gap-2">
                  <div className={`w-3 h-3 rounded ${item.color}`} />
                  <span className="text-xs text-on-surface-variant">{item.label}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
