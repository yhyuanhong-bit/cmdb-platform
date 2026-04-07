import { apiClient } from './client'

export const ingestionApi = {
  // Credentials
  listCredentials: (params?: Record<string, string>) =>
    apiClient.get('/ingestion/credentials', params),
  createCredential: (data: any) =>
    apiClient.post('/ingestion/credentials', data),
  updateCredential: (id: string, data: any) =>
    apiClient.put(`/ingestion/credentials/${id}`, data),
  deleteCredential: (id: string) =>
    apiClient.del(`/ingestion/credentials/${id}`),

  // Scan Targets
  listScanTargets: (params?: Record<string, string>) =>
    apiClient.get('/ingestion/scan-targets', params),
  createScanTarget: (data: any) =>
    apiClient.post('/ingestion/scan-targets', data),
  updateScanTarget: (id: string, data: any) =>
    apiClient.put(`/ingestion/scan-targets/${id}`, data),
  deleteScanTarget: (id: string) =>
    apiClient.del(`/ingestion/scan-targets/${id}`),

  // Discovery
  triggerScan: (data: any) =>
    apiClient.post('/ingestion/discovery/scan', data),
  listTasks: (params?: Record<string, string>) =>
    apiClient.get('/ingestion/discovery/tasks', params),
  getTask: (id: string) =>
    apiClient.get(`/ingestion/discovery/tasks/${id}`),

  // Collector test
  testCollector: (name: string, data: { credential_id: string; endpoint: string }) =>
    apiClient.post(`/ingestion/collectors/${name}/test`, data),
}
