import { apiClient } from './client'
import type { ApiResponse, ApiListResponse } from './types'

export interface DiscoveredAsset {
  id: string
  source: string
  external_id?: string
  hostname?: string
  ip_address?: string
  raw_data?: Record<string, unknown>
  status: string
  matched_asset_id?: string | null
  match_confidence?: number | null
  match_strategy?: string | null
  review_reason?: string | null
  diff_details?: Record<string, { old?: unknown; new?: unknown }> | null
  discovered_at: string
  reviewed_by?: string | null
  reviewed_at?: string | null
}

export interface DiscoveryStats {
  total: number
  pending: number
  conflict: number
  approved: number
  ignored: number
  matched: number
}

export interface DiscoveryIngestData {
  hostname?: string
  ip_address?: string
  mac_address?: string
  asset_type?: string
  manufacturer?: string
  model?: string
  serial?: string
  metadata?: Record<string, unknown>
}

export const discoveryApi = {
  list: (params?: Record<string, string>) => apiClient.get<ApiListResponse<DiscoveredAsset>>('/discovery/pending', params),
  ingest: (data: DiscoveryIngestData) => apiClient.post<ApiResponse<DiscoveredAsset>>('/discovery/ingest', data),
  approve: (id: string, reason?: string) =>
    apiClient.post<ApiResponse<DiscoveredAsset>>(`/discovery/${id}/approve`, { reason: reason ?? '' }),
  ignore: (id: string, reason: string) =>
    apiClient.post<ApiResponse<DiscoveredAsset>>(`/discovery/${id}/ignore`, { reason }),
  getStats: () => apiClient.get<ApiResponse<DiscoveryStats>>('/discovery/stats'),
}
