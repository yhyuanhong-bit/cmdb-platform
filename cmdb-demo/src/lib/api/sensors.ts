import { apiClient } from './client'

export interface CreateSensorData {
  name: string
  type: string
  location?: string
  asset_id?: string
  enabled?: boolean
  polling_interval?: number
  metadata?: Record<string, unknown>
}

export interface UpdateSensorData {
  name?: string
  type?: string
  location?: string
  enabled?: boolean
  polling_interval?: number
  metadata?: Record<string, unknown>
}

export interface SensorHeartbeatData {
  value?: number
  unit?: string
  timestamp?: string
  metadata?: Record<string, unknown>
}

export const sensorApi = {
  list: (params?: Record<string, string>) => apiClient.get('/sensors', params),
  create: (data: CreateSensorData) => apiClient.post('/sensors', data),
  update: (id: string, data: UpdateSensorData) => apiClient.put(`/sensors/${id}`, data),
  delete: (id: string) => apiClient.del(`/sensors/${id}`),
  heartbeat: (id: string, data?: SensorHeartbeatData) => apiClient.post(`/sensors/${id}/heartbeat`, data ?? {}),
}
