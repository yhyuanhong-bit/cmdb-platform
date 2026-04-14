import { apiClient } from './client'
import type { ApiResponse, ApiListResponse } from './types'
import type { components } from '../../generated/api-types'

export type Asset = components['schemas']['Asset']

export const assetApi = {
  list: (params?: Record<string, string>) =>
    apiClient.get<ApiListResponse<Asset>>('/assets', params),
  getById: (id: string) =>
    apiClient.get<ApiResponse<Asset>>(`/assets/${id}`),
  create: (data: Partial<Asset>) =>
    apiClient.post<ApiResponse<Asset>>('/assets', data),
  update: (id: string, data: Partial<Asset>) =>
    apiClient.put<ApiResponse<Asset>>(`/assets/${id}`, data),
  delete: (id: string) =>
    apiClient.del(`/assets/${id}`),
  getLifecycleStats: () =>
    apiClient.get<ApiResponse<LifecycleStats>>('/assets/lifecycle-stats'),
  getLifecycle: (id: string) =>
    apiClient.get<ApiResponse<AssetLifecycleData>>(`/assets/${id}/lifecycle`),
  getUpgradeRecommendations: (assetId: string) =>
    apiClient.get(`/assets/${assetId}/upgrade-recommendations`),
  acceptUpgradeRecommendation: (assetId: string, category: string, data?: unknown) =>
    apiClient.post(`/assets/${assetId}/upgrade-recommendations/${category}/accept`, data ?? {}),
}

export interface LifecycleStats {
  by_status: Record<string, number | undefined>
  total_purchase_cost: number
  warranty_active_count: number
  warranty_expired_count: number
  approaching_eol_count: number
}

export interface LifecycleEvent {
  type: 'created' | 'updated' | 'status_change' | 'deleted' | 'warranty_start' | 'warranty_end' | 'eol' | 'other'
  action: string
  from_status?: string
  to_status?: string
  description?: string
  operator_id?: string
  date: string
  diff?: Record<string, unknown>
}

export interface AssetLifecycleSummary {
  asset_id: string
  asset_tag: string
  name: string
  status: string
  purchase_date: string | null
  purchase_cost: number | null
  warranty_start: string | null
  warranty_end: string | null
  warranty_vendor: string | null
  warranty_contract: string | null
  eol_date: string | null
  expected_lifespan_months: number | null
}

export interface AssetLifecycleData {
  summary: AssetLifecycleSummary
  timeline: LifecycleEvent[]
}
