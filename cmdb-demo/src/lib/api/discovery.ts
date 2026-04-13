import { apiClient } from './client'

export const discoveryApi = {
  list: (params?: Record<string, string>) => apiClient.get('/discovery/pending', params),
  ingest: (data: any) => apiClient.post('/discovery/ingest', data),
  approve: (id: string) => apiClient.post(`/discovery/${id}/approve`, {}),
  ignore: (id: string) => apiClient.post(`/discovery/${id}/ignore`, {}),
  getStats: () => apiClient.get('/discovery/stats'),
}
