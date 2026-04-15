import { apiClient } from './client'

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
  type: string
  endpoint: string
  credential_id: string
}

export interface UpdateScanTargetInput {
  name?: string
  type?: string
  endpoint?: string
  credential_id?: string
}

export const ingestionApi = {
  // Credentials
  listCredentials: (params?: Record<string, string>) =>
    apiClient.get('/ingestion/credentials', params),
  createCredential: (data: CreateCredentialInput) =>
    apiClient.post('/ingestion/credentials', data),
  updateCredential: (id: string, data: UpdateCredentialInput) =>
    apiClient.put(`/ingestion/credentials/${id}`, data),
  deleteCredential: (id: string) =>
    apiClient.del(`/ingestion/credentials/${id}`),

  // Scan Targets
  listScanTargets: (params?: Record<string, string>) =>
    apiClient.get('/ingestion/scan-targets', params),
  createScanTarget: (data: CreateScanTargetInput) =>
    apiClient.post('/ingestion/scan-targets', data),
  updateScanTarget: (id: string, data: UpdateScanTargetInput) =>
    apiClient.put(`/ingestion/scan-targets/${id}`, data),
  deleteScanTarget: (id: string) =>
    apiClient.del(`/ingestion/scan-targets/${id}`),

  // Discovery
  triggerScan: (targetIdOrData: string | { scan_target_ids: string[] }) =>
    apiClient.post('/ingestion/discovery/scan',
      typeof targetIdOrData === 'string' ? { scan_target_ids: [targetIdOrData] } : targetIdOrData),
  listTasks: (params?: Record<string, string>) =>
    apiClient.get('/ingestion/discovery/tasks', params),
  getTask: (id: string) =>
    apiClient.get(`/ingestion/discovery/tasks/${id}`),

  // Collector test
  testCollector: (name: string, data: { credential_id: string; endpoint: string }) =>
    apiClient.post(`/ingestion/collectors/${name}/test`, data),

  // Import
  confirmImport: (jobId: string) =>
    apiClient.post(`/ingestion/import/${jobId}/confirm`, {}),
  getImportProgress: (jobId: string) =>
    apiClient.get(`/ingestion/import/${jobId}/progress`),
}
