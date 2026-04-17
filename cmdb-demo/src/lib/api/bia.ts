import { apiClient } from './client'
import type { ApiResponse, ApiListResponse } from './types'

export interface BIAAssessment {
  id: string
  name?: string
  system_name?: string
  system_code?: string
  asset_id?: string
  service_id?: string
  rto_hours?: number
  rpo_hours?: number
  rpo_minutes?: number | null
  criticality?: 'low' | 'medium' | 'high' | 'critical'
  tier?: string
  bia_score?: number
  description?: string
  owner?: string | null
  data_compliance?: boolean
  asset_compliance?: boolean
  audit_compliance?: boolean
  metadata?: Record<string, unknown>
  created_at?: string
  updated_at?: string
}

export interface BIAStats {
  total: number
  by_tier: Record<string, number>
  by_criticality?: Record<string, number>
  avg_compliance?: number
  data_compliant?: number
  asset_compliant?: number
  audit_compliant?: number
  total_dependencies?: number
}

export interface BIARule {
  id: string
  name?: string
  tier_name?: string
  tier_level?: number
  display_name?: string
  description?: string
  color?: string
  icon?: string
  min_score?: number
  max_score?: number
  rto_threshold?: number | null
  rpo_threshold?: number | null
  enabled?: boolean
  threshold?: number
  action?: string
  metadata?: Record<string, unknown>
}

export interface BIADependency {
  id: string
  assessment_id?: string
  asset_id: string
  depends_on_asset_id?: string
  dependency_type?: string
  criticality?: string
  metadata?: Record<string, unknown>
}

export interface BIAImpact {
  asset_id: string
  impacted_services?: string[]
  tier?: string
  score?: number
  downstream_dependencies?: Array<{ id: string; name: string; tier?: string }>
  upstream_dependencies?: Array<{ id: string; name: string; tier?: string }>
}

export interface CreateBIAAssessmentData {
  name?: string
  system_name?: string
  system_code?: string
  asset_id?: string
  service_id?: string
  rto_hours?: number
  rpo_hours?: number
  rpo_minutes?: number
  criticality?: string
  tier?: string
  bia_score?: number
  description?: string
  owner?: string
  metadata?: Record<string, unknown>
  [key: string]: unknown
}

export interface UpdateBIAAssessmentData {
  name?: string
  rto_hours?: number
  rpo_hours?: number
  criticality?: 'low' | 'medium' | 'high' | 'critical'
  description?: string
  owner?: string
  metadata?: Record<string, unknown>
}

export interface UpdateBIARuleData {
  enabled?: boolean
  threshold?: number
  action?: string
  metadata?: Record<string, unknown>
}

export interface CreateBIADependencyData {
  depends_on_asset_id?: string
  asset_id?: string
  dependency_type?: string
  criticality?: string
  note?: string
  [key: string]: unknown
}

export const biaApi = {
  listAssessments: (params?: Record<string, string>) =>
    apiClient.get<ApiListResponse<BIAAssessment>>('/bia/assessments', params),
  getAssessment: (id: string) =>
    apiClient.get<ApiResponse<BIAAssessment>>(`/bia/assessments/${id}`),
  createAssessment: (data: CreateBIAAssessmentData) =>
    apiClient.post<ApiResponse<BIAAssessment>>('/bia/assessments', data),
  updateAssessment: (id: string, data: UpdateBIAAssessmentData) =>
    apiClient.put<ApiResponse<BIAAssessment>>(`/bia/assessments/${id}`, data),
  deleteAssessment: (id: string) =>
    apiClient.del(`/bia/assessments/${id}`),
  listRules: () =>
    apiClient.get<ApiResponse<BIARule[]>>('/bia/rules'),
  updateRule: (id: string, data: UpdateBIARuleData) =>
    apiClient.put<ApiResponse<BIARule>>(`/bia/rules/${id}`, data),
  listDependencies: (assessmentId: string) =>
    apiClient.get<ApiResponse<BIADependency[]>>(`/bia/assessments/${assessmentId}/dependencies`),
  createDependency: (assessmentId: string, data: CreateBIADependencyData) =>
    apiClient.post<ApiResponse<BIADependency>>(`/bia/assessments/${assessmentId}/dependencies`, data),
  getStats: () =>
    apiClient.get<ApiResponse<BIAStats>>('/bia/stats'),
  getImpact: (assetId: string) =>
    apiClient.get<ApiResponse<BIAImpact>>(`/bia/impact/${assetId}`),
}
