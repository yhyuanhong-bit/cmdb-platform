import { apiClient } from './client'
import type { ApiResponse, ApiListResponse } from './types'

export interface Credential {
  id: string
  name: string
  type: string
  tenant_id?: string
  created_at?: string
  updated_at?: string
}

export interface ScanTarget {
  id: string
  name: string
  collector_type: string
  cidrs: string[]
  credential_id: string
  mode: string
  tenant_id?: string
}

export interface DiscoveryTask {
  id: string
  task_type: string
  status: string
  started_at?: string
  completed_at?: string
  result?: Record<string, unknown>
}

export interface ImportProgress {
  total: number
  processed: number
  imported: number
  failed: number
  status: string
}

export interface CreateCredentialInput {
  tenant_id?: string
  name: string
  type: string
  params: Record<string, string>
}

export interface UpdateCredentialInput {
  name?: string
  type?: string
  params?: Record<string, string>
}

export interface CreateScanTargetInput {
  name: string
  collector_type: string
  cidrs: string[]
  credential_id: string
  mode: string
  tenant_id?: string
}

export interface UpdateScanTargetInput {
  name?: string
  collector_type?: string
  cidrs?: string[]
  credential_id?: string
  mode?: string
  tenant_id?: string
}

export const ingestionApi = {
  // Credentials
  listCredentials: (params?: Record<string, string>) =>
    apiClient.get<ApiListResponse<Credential>>('/ingestion/credentials', params),
  createCredential: (data: CreateCredentialInput) =>
    apiClient.post<ApiResponse<Credential>>('/ingestion/credentials', data),
  updateCredential: (id: string, data: UpdateCredentialInput) =>
    apiClient.put<ApiResponse<Credential>>(`/ingestion/credentials/${id}`, data),
  deleteCredential: (id: string) =>
    apiClient.del(`/ingestion/credentials/${id}`),

  // Scan Targets
  listScanTargets: (params?: Record<string, string>) =>
    apiClient.get<ApiListResponse<ScanTarget>>('/ingestion/scan-targets', params),
  createScanTarget: (data: CreateScanTargetInput) =>
    apiClient.post<ApiResponse<ScanTarget>>('/ingestion/scan-targets', data),
  updateScanTarget: (id: string, data: UpdateScanTargetInput) =>
    apiClient.put<ApiResponse<ScanTarget>>(`/ingestion/scan-targets/${id}`, data),
  deleteScanTarget: (id: string) =>
    apiClient.del(`/ingestion/scan-targets/${id}`),

  // Discovery
  triggerScan: (targetIdOrData: string | { scan_target_ids: string[] }) =>
    apiClient.post<ApiResponse<{ task_id: string }>>('/ingestion/discovery/scan',
      typeof targetIdOrData === 'string' ? { scan_target_ids: [targetIdOrData] } : targetIdOrData),
  listTasks: (params?: Record<string, string>) =>
    apiClient.get<ApiListResponse<DiscoveryTask>>('/ingestion/discovery/tasks', params),
  getTask: (id: string) =>
    apiClient.get<ApiResponse<DiscoveryTask>>(`/ingestion/discovery/tasks/${id}`),

  // Collector test
  testCollector: (name: string, data: { credential_id: string; endpoint: string }) =>
    apiClient.post<ApiResponse<{ success: boolean; message?: string }>>(`/ingestion/collectors/${name}/test`, data),

  // Import
  confirmImport: (jobId: string) =>
    apiClient.post<ApiResponse<{ job_id: string; status: string }>>(`/ingestion/import/${jobId}/confirm`, {}),
  getImportProgress: (jobId: string) =>
    apiClient.get<ApiResponse<ImportProgress>>(`/ingestion/import/${jobId}/progress`),
}
