import { apiClient } from './client'

export const sensorApi = {
  list: (params?: Record<string, string>) => apiClient.get('/sensors', params),
  create: (data: any) => apiClient.post('/sensors', data),
  update: (id: string, data: any) => apiClient.put(`/sensors/${id}`, data),
  delete: (id: string) => apiClient.del(`/sensors/${id}`),
  heartbeat: (id: string, data?: any) => apiClient.post(`/sensors/${id}/heartbeat`, data || {}),
}
