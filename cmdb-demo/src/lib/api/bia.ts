import { apiClient } from './client'

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
    apiClient.get('/bia/assessments', params),
  getAssessment: (id: string) =>
    apiClient.get(`/bia/assessments/${id}`),
  createAssessment: (data: CreateBIAAssessmentData) =>
    apiClient.post('/bia/assessments', data),
  updateAssessment: (id: string, data: UpdateBIAAssessmentData) =>
    apiClient.put(`/bia/assessments/${id}`, data),
  deleteAssessment: (id: string) =>
    apiClient.del(`/bia/assessments/${id}`),
  listRules: () =>
    apiClient.get('/bia/rules'),
  updateRule: (id: string, data: UpdateBIARuleData) =>
    apiClient.put(`/bia/rules/${id}`, data),
  listDependencies: (assessmentId: string) =>
    apiClient.get(`/bia/assessments/${assessmentId}/dependencies`),
  createDependency: (assessmentId: string, data: CreateBIADependencyData) =>
    apiClient.post(`/bia/assessments/${assessmentId}/dependencies`, data),
  getStats: () =>
    apiClient.get('/bia/stats'),
  getImpact: (assetId: string) =>
    apiClient.get(`/bia/impact/${assetId}`),
}
