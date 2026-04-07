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
}

export function useDashboardStats(params?: Record<string, string>) {
  return useQuery({
    queryKey: ['dashboardStats', params],
    queryFn: () => apiClient.get<ApiResponse<DashboardStats>>('/dashboard/stats', params),
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
