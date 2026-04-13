import { apiClient } from './client'

export const qualityApi = {
  listRules: () => apiClient.get('/quality/rules'),
  createRule: (data: any) => apiClient.post('/quality/rules', data),
  getDashboard: () => apiClient.get('/quality/dashboard'),
  getAssetHistory: (assetId: string) => apiClient.get(`/quality/history/${assetId}`),
  triggerScan: () => apiClient.post('/quality/scan', {}),
}
