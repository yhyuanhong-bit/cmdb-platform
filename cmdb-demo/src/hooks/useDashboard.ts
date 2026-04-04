import { useQuery } from '@tanstack/react-query'
import { apiClient } from '../lib/api/client'
import type { ApiResponse } from '../lib/api/types'

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
