import { useQuery } from '@tanstack/react-query'
import { apiClient } from '../lib/api/client'
import type { ApiResponse } from '../lib/api/types'
import { topologyApi } from '../lib/api/topology'
import { assetApi } from '../lib/api/assets'

interface DashboardStats {
  total_assets: number
  total_racks: number
  critical_alerts: number
  active_orders: number
  pending_work_orders: number
  energy_current_kw: number
  rack_utilization_pct: number
  avg_quality_score: number
}

export function useDashboardStats(params?: Record<string, string>) {
  return useQuery({
    queryKey: ['dashboardStats', params],
    queryFn: () => apiClient.get<ApiResponse<DashboardStats>>('/dashboard/stats', params),
  })
}

export interface AssetsTrendPoint {
  bucket: string
  count: number
  created: number
  deleted: number
}

interface AssetsTrendResponse {
  period: '7d' | '30d' | '90d'
  points: AssetsTrendPoint[]
}

export function useAssetsTrend(period: '7d' | '30d' | '90d' = '30d') {
  return useQuery({
    queryKey: ['assetsTrend', period],
    queryFn: () =>
      apiClient.get<ApiResponse<AssetsTrendResponse>>('/dashboard/assets-trend', { period }),
  })
}

export interface RackHeatmapCell {
  rack_id: string
  rack_name: string
  location_id: string
  row_label: string | null
  u_total: number
  u_used: number
  occupancy_pct: number
  power_capacity_kw: number | null
  status: 'healthy' | 'warning' | 'critical'
}

export function useRackHeatmap(locationId?: string) {
  return useQuery({
    queryKey: ['rackHeatmap', locationId],
    queryFn: () =>
      apiClient.get<ApiResponse<RackHeatmapCell[]>>(
        '/dashboard/rack-heatmap',
        locationId ? { location_id: locationId } : undefined,
      ),
  })
}

export function useRackStats() {
  return useQuery({
    queryKey: ['rackStats'],
    queryFn: () => topologyApi.getRackStats(),
  })
}

export function useLifecycleStats() {
  return useQuery({
    queryKey: ['lifecycleStats'],
    queryFn: () => assetApi.getLifecycleStats(),
  })
}
