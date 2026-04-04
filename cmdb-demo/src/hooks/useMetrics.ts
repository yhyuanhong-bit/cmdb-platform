import { useQuery } from '@tanstack/react-query'
import { apiClient } from '../lib/api/client'
import type { ApiResponse } from '../lib/api/types'

interface MetricPoint {
  time: string
  name: string
  value: number
  avg_val?: number
  max_val?: number
  min_val?: number
}

export function useMetrics(params: { asset_id?: string; metric_name?: string; time_range?: string; location_id?: string }) {
  const queryParams: Record<string, string> = {}
  if (params.asset_id) queryParams.asset_id = params.asset_id
  if (params.metric_name) queryParams.metric_name = params.metric_name
  if (params.time_range) queryParams.time_range = params.time_range
  if (params.location_id) queryParams.location_id = params.location_id

  return useQuery({
    queryKey: ['metrics', params],
    queryFn: () => apiClient.get<ApiResponse<MetricPoint[]>>('/monitoring/metrics', queryParams),
    enabled: !!(params.asset_id || params.location_id),
  })
}
