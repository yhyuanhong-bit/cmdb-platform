import { useState, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import {
  useRack,
  useRackAssets,
  useRackSlots,
  useRackNetworkConnections,
} from "../../hooks/useTopology";
import type { Asset } from "../../lib/api/topology";
import type { AlertEvent } from "../../lib/api/monitoring";
import AssignAssetToRackModal from '../../components/AssignAssetToRackModal';
import AddNetworkConnectionModal from '../../components/AddNetworkConnectionModal';
import { useAlerts } from "../../hooks/useMonitoring";
import { useActivityFeed } from "../../hooks/useActivityFeed";
import { apiClient } from "../../lib/api/client";
import { useMetrics } from "../../hooks/useMetrics";
import {
  type Equipment,
  type RackSlotDetail,
  type LiveAlert,
  type SlotType,
  type USlot,
  type TabId,
  TABS,
  uSlots,
} from "./shared";
import { VisualizationTab } from "./VisualizationTab";
import { ConsoleTab } from "./ConsoleTab";
import { NetworkTab, type NetworkConnectionExt } from "./NetworkTab";
import { MaintenanceTab } from "./MaintenanceTab";
import { RackBreadcrumb, RackHeader, type RackEditState } from "./RackHeader";

export default function RackDetailUnified() {
  const { t } = useTranslation();
  const tabLabels: Record<string, string> = {
    visualization: t('rack_detail.tab_visualization'),
    console: t('rack_detail.tab_console'),
    network: t('rack_detail.tab_network'),
    maintenance: t('rack_detail.tab_maintenance'),
  };
  const { id: rackId } = useParams<{ id: string }>();
  const { data: rackResponse, isLoading, error } = useRack(rackId ?? "");
  const rack = rackResponse?.data;
  const { data: rackAssetsResponse } = useRackAssets(rackId ?? "");
  const rackAssets = rackAssetsResponse?.data;
  const { data: slotsResp } = useRackSlots(rackId ?? "");
  const rackSlots: RackSlotDetail[] = (slotsResp?.data || []) as RackSlotDetail[];

  // Build console slots from real rack slots API (falls back to hardcoded uSlots)
  const consoleSlots: USlot[] = useMemo(() => {
    if (!rackSlots || rackSlots.length === 0) return []
    return rackSlots.map((slot: RackSlotDetail) => {
      const assetType = (slot.asset_type || slot.type || '').toLowerCase()
      let slotType: SlotType = 'compute'
      if (assetType.includes('network') || assetType.includes('switch')) slotType = 'network'
      else if (assetType.includes('storage') || assetType.includes('nas') || assetType.includes('san')) slotType = 'storage'
      else if (assetType.includes('power') || assetType.includes('ups')) slotType = 'ups'
      else if (assetType.includes('pdu')) slotType = 'pdu'
      return {
        startU: slot.start_u ?? 1,
        endU: slot.end_u ?? 1,
        label: slot.asset_name || slot.asset_tag || `U${slot.start_u}`,
        type: slotType,
      }
    })
  }, [rackSlots])

  // Build equipment list from real rack assets API (empty array if no data yet)
  const liveEquipment: Equipment[] = useMemo(() => {
    if (!rackAssets || rackAssets.length === 0) return [];
    return rackAssets.map((a: Asset) => ({
      startU: (a.attributes?.start_u as number | undefined) ?? 1,
      endU: (a.attributes?.end_u as number | undefined) ?? 1,
      label: a.name ?? `${a.vendor} ${a.model}`,
      assetTag: a.asset_tag ?? a.id,
      type: (a.type?.toLowerCase() ?? 'compute') as Equipment['type'],
    }));
  }, [rackAssets]);

  // Network connections from API
  const { data: netData } = useRackNetworkConnections(rackId ?? "");
  const networkConnections = (netData as unknown as { connections?: NetworkConnectionExt[] })?.connections ?? [];

  // Maintenance history from API
  const { data: maintData } = useQuery({
    queryKey: ['rackMaintenance', rackId],
    queryFn: () => apiClient.get(`/racks/${rackId}/maintenance`),
    enabled: !!rackId,
  })
  type WorkOrder = { scheduled_start?: string; created_at: string; type?: string; title: string; status: string };
  const maintenanceHistory = ((maintData as unknown as { maintenance?: WorkOrder[] })?.maintenance ?? []).map((wo: WorkOrder) => ({
    date: wo.scheduled_start ? new Date(wo.scheduled_start).toLocaleDateString() : new Date(wo.created_at).toLocaleDateString(),
    type: wo.type ?? 'inspection',
    description: wo.title,
    engineer: '-',
    status: wo.status,
  }))

  // Activity feed from API
  const { data: activityData } = useActivityFeed('rack', rackId ?? "");
  type ActivityEvent = { description?: string; action?: string; timestamp: string; event_type?: string };
  type RecentActivityItem = { action: string; time: string; icon: string };
  const recentActivity = ((activityData as unknown as { events?: ActivityEvent[] })?.events ?? []).map((e: ActivityEvent): RecentActivityItem => ({
    action: e.description ?? e.action ?? '',
    time: new Date(e.timestamp).toLocaleString(),
    icon: e.event_type === 'alert' ? 'warning' : e.event_type === 'maintenance' ? 'build' : 'history',
  }));

  // Fetch all alerts and filter to those belonging to assets in this rack
  const { data: alertsResponse } = useAlerts();
  const allAlerts = alertsResponse?.data ?? [];

  const rackAssetIds = useMemo(() => {
    if (!rackAssets) return new Set<string>();
    return new Set(rackAssets.map((a: Asset) => a.id));
  }, [rackAssets]);

  const filteredAlerts: LiveAlert[] = useMemo(() => {
    if (allAlerts.length === 0 || rackAssetIds.size === 0) return [];
    return allAlerts
      .filter((alert: AlertEvent) => rackAssetIds.has(alert.ci_id))
      .map((alert: AlertEvent) => ({
        severity: alert.severity ?? 'info',
        text: alert.message ?? '',
        time: alert.fired_at
          ? new Date(alert.fired_at).toLocaleString()
          : '',
      }));
  }, [allAlerts, rackAssetIds]);

  // Environmental metrics: temperature + power are real; humidity/airflow have no
  // matching metric_name in the metrics hypertable today, so we surface them as
  // "sensor not configured" rather than fabricating numbers. Sensor JSON readings
  // exist on rack rows but are seed-only and not exposed via metrics API.
  const firstAssetId = rackAssets?.[0]?.id || ''
  const { data: tempMetrics } = useMetrics({ asset_id: firstAssetId, metric_name: 'temperature', time_range: '1h' })
  const tempPoints = tempMetrics?.data ?? []
  const latestTemp = tempPoints.length > 0 ? tempPoints[0].value : null
  const tempValues = tempPoints.map((p) => p.value)
  const tempMin = tempValues.length > 0 ? Math.min(...tempValues) : null
  const tempMax = tempValues.length > 0 ? Math.max(...tempValues) : null
  const environmentMetrics = {
    temperature: {
      current: latestTemp != null ? Number(latestTemp.toFixed(1)) : null,
      min: tempMin != null ? Number(tempMin.toFixed(1)) : null,
      max: tempMax != null ? Number(tempMax.toFixed(1)) : null,
      threshold: 30,
      unit: '°C',
    },
    humidity: { current: null, min: null, max: null, threshold: 60, unit: '%' },
    powerDraw: {
      current: rack?.power_current_kw != null ? Number(rack.power_current_kw) : null,
      min: 0,
      max: Number(rack?.power_capacity_kw ?? 40),
      threshold: Number(rack?.power_capacity_kw ?? 40),
      unit: 'kW',
    },
    airflow: { current: null, min: null, max: null, threshold: 1500, unit: 'CFM' },
  }

  const [editingRack, setEditingRack] = useState(false)
  const [rackEdit, setRackEdit] = useState<RackEditState>({ name: '', status: '', total_u: 42 })

  const [activeTab, setActiveTab] = useState<TabId>("visualization");
  const [showAssignModal, setShowAssignModal] = useState(false);
  const [showAddConnModal, setShowAddConnModal] = useState(false);
  const [selectedAsset, setSelectedAsset] = useState<Equipment | null>(
    liveEquipment.find((e) => e.assetTag === "APP-SRV-042-PROD") ?? liveEquipment[0] ?? null,
  );

  // Selected asset data looked up from API rack assets
  const selectedAssetData = rackAssets?.find((a: Asset) =>
    a.asset_tag === selectedAsset?.assetTag || a.id === selectedAsset?.assetTag
  )

  const occupiedUs = liveEquipment.reduce((acc, eq) => acc + (eq.endU - eq.startU + 1), 0);
  const occupancy = rack ? Math.round((rack.used_u / rack.total_u) * 100) : Math.round((occupiedUs / 42) * 100);

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
        <p className="text-error text-sm">Failed to load rack details</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-surface text-on-surface font-body">
      <RackBreadcrumb rack={rack} />

      <div className="px-8 py-6">
        <RackHeader
          rack={rack}
          rackId={rackId ?? ''}
          occupancy={occupancy}
          editingRack={editingRack}
          setEditingRack={setEditingRack}
          rackEdit={rackEdit}
          setRackEdit={setRackEdit}
          onAssignAsset={() => setShowAssignModal(true)}
        />

        {/* Tab buttons */}
        <div className="flex items-center gap-1 bg-surface-container rounded-lg p-1 mb-6 w-fit">
          {TABS.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`flex items-center gap-2 px-4 py-2.5 rounded-md text-sm font-medium transition-colors ${
                activeTab === tab.id
                  ? "bg-primary-container text-primary"
                  : "text-on-surface-variant hover:text-on-surface hover:bg-surface-container-high"
              }`}
            >
              <span className="material-symbols-outlined text-lg">{tab.icon}</span>
              {tabLabels[tab.id] ?? tab.id}
            </button>
          ))}
        </div>

        {/* Tab content */}
        {activeTab === "visualization" && (
          <VisualizationTab selectedAsset={selectedAsset} setSelectedAsset={setSelectedAsset} equipmentList={liveEquipment} rackSlots={rackSlots} totalU={rack?.total_u} liveAlerts={filteredAlerts} selectedAssetData={selectedAssetData} />
        )}
        {activeTab === "console" && <ConsoleTab recentActivity={recentActivity} slots={consoleSlots.length > 0 ? consoleSlots : uSlots} rack={rack} />}
        {activeTab === "network" && <NetworkTab networkConnections={networkConnections} rackId={rackId ?? ''} onAddConnection={() => setShowAddConnModal(true)} />}
        {activeTab === "maintenance" && <MaintenanceTab maintenanceHistory={maintenanceHistory} environmentMetrics={environmentMetrics} />}
      </div>

      <AssignAssetToRackModal
        open={showAssignModal}
        onClose={() => setShowAssignModal(false)}
        rackId={rackId ?? ''}
        totalU={rack?.total_u ?? 42}
      />
      <AddNetworkConnectionModal
        open={showAddConnModal}
        onClose={() => setShowAddConnModal(false)}
        rackId={rackId ?? ''}
      />
    </div>
  );
}
