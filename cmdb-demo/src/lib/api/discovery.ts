import { apiClient } from './client'

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
  list: (params?: Record<string, string>) => apiClient.get('/discovery/pending', params),
  ingest: (data: DiscoveryIngestData) => apiClient.post('/discovery/ingest', data),
  approve: (id: string) => apiClient.post(`/discovery/${id}/approve`, {}),
  ignore: (id: string) => apiClient.post(`/discovery/${id}/ignore`, {}),
  getStats: () => apiClient.get('/discovery/stats'),
}
