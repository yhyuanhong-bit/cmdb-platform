import { apiClient } from './client'
import type { ApiResponse, ApiListResponse } from './types'

/** Business Service entity — user-visible functions like "Order API",
 * "Payment Gateway". Wave 2 Service spec. */
export interface Service {
  id: string
  tenant_id: string
  code: string
  name: string
  description?: string | null
  tier: 'critical' | 'important' | 'normal' | 'low' | 'minor'
  owner_team?: string | null
  bia_assessment_id?: string | null
  status: 'active' | 'deprecated' | 'decommissioned'
  tags: string[]
  created_at: string
  updated_at: string
  created_by?: string | null
  sync_version: number
}

export interface ServiceAssetMember {
  service_id: string
  asset_id: string
  asset_tag?: string | null
  asset_name?: string | null
  asset_status?: string | null
  asset_type?: string | null
  role: 'primary' | 'replica' | 'cache' | 'proxy' | 'storage' | 'dependency' | 'component'
  is_critical: boolean
  created_at: string
}

export interface ServiceHealth {
  service_id: string
  status: 'healthy' | 'degraded' | 'unknown'
  critical_total: number
  critical_unhealthy: number
}

export interface AssetServiceMembership {
  id: string
  code: string
  name: string
  tier: Service['tier']
  status: Service['status']
  role: string
  is_critical: boolean
}

export interface ListServicesParams extends Record<string, string | number | undefined> {
  tier?: Service['tier']
  status?: Service['status']
  owner_team?: string
  page?: number
  page_size?: number
}

export interface CreateServiceRequest {
  code: string
  name: string
  description?: string
  tier?: Service['tier']
  owner_team?: string
  bia_assessment_id?: string
  tags?: string[]
}

export interface UpdateServiceRequest {
  name?: string
  description?: string
  tier?: Service['tier']
  owner_team?: string
  bia_assessment_id?: string
  status?: Service['status']
  tags?: string[]
}

export interface AddServiceAssetRequest {
  asset_id: string
  role?: ServiceAssetMember['role']
  is_critical?: boolean
}

// stringParams normalizes the optional filter record to the string-only
// shape the ApiClient expects. Number / undefined values get coerced or
// dropped so callers can pass { page: 1, tier: 'critical' } naturally.
function stringParams(p?: Record<string, string | number | undefined>): Record<string, string> | undefined {
  if (!p) return undefined
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(p)) {
    if (v === undefined) continue
    out[k] = typeof v === 'number' ? String(v) : v
  }
  return Object.keys(out).length > 0 ? out : undefined
}

export const servicesApi = {
  list: (params?: ListServicesParams) =>
    apiClient.get<ApiListResponse<Service>>('/services', stringParams(params)),

  get: (id: string) =>
    apiClient.get<ApiResponse<Service>>(`/services/${id}`),

  create: (body: CreateServiceRequest) =>
    apiClient.post<ApiResponse<Service>>('/services', body),

  update: (id: string, body: UpdateServiceRequest) =>
    apiClient.put<ApiResponse<Service>>(`/services/${id}`, body),

  remove: (id: string) =>
    apiClient.del(`/services/${id}`),

  listAssets: (id: string) =>
    apiClient.get<ApiResponse<ServiceAssetMember[]>>(`/services/${id}/assets`),

  addAsset: (id: string, body: AddServiceAssetRequest) =>
    apiClient.post<ApiResponse<ServiceAssetMember>>(`/services/${id}/assets`, body),

  removeAsset: (serviceId: string, assetId: string) =>
    apiClient.del(`/services/${serviceId}/assets/${assetId}`),

  health: (id: string) =>
    apiClient.get<ApiResponse<ServiceHealth>>(`/services/${id}/health`),

  listForAsset: (assetId: string) =>
    apiClient.get<ApiResponse<AssetServiceMembership[]>>(`/assets/${assetId}/services`),
}
