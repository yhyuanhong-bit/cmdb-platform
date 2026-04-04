import { useQuery } from '@tanstack/react-query'
import { apiClient } from '../lib/api/client'
import type { ApiResponse } from '../lib/api/types'

interface SystemHealth {
  database?: { status?: string; latency_ms?: number }
  redis?: { status?: string; latency_ms?: number }
  nats?: { status?: string; connected?: boolean }
}

export function useSystemHealth() {
  return useQuery({
    queryKey: ['systemHealth'],
    queryFn: () => apiClient.get<ApiResponse<SystemHealth>>('/system/health'),
    refetchInterval: 30000,
  })
}
