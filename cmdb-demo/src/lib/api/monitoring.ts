import { apiClient } from './client'
import type { ApiResponse } from './types'
import type { components } from '../../generated/api-types'

export type AlertEvent = components['schemas']['AlertEvent']

export interface AlertRule {
  id: string
  name: string
  metric_name: string
  condition: Record<string, any>
  severity: string
  enabled: boolean
}

export interface Incident {
  id: string
  title: string
  status: string
  severity: string
  started_at: string
  resolved_at: string | null
}

export const monitoringApi = {
  ingestMetrics: (batch: any) =>
    apiClient.post<ApiResponse<any>>('/monitoring/metrics', batch),
  queryMetrics: (params: Record<string, string>) =>
    apiClient.get<ApiResponse<any[]>>('/monitoring/metrics', params),
  listAlertRules: () =>
    apiClient.get<ApiResponse<AlertRule[]>>('/monitoring/rules'),
  createAlertRule: (data: any) =>
    apiClient.post<ApiResponse<AlertRule>>('/monitoring/rules', data),
  listAlerts: (params?: Record<string, string>) =>
    apiClient.get<ApiResponse<AlertEvent[]>>('/monitoring/alerts', params),
  acknowledgeAlert: (id: string) =>
    apiClient.post<void>(`/monitoring/alerts/${id}/ack`),
  resolveAlert: (id: string) =>
    apiClient.post<void>(`/monitoring/alerts/${id}/resolve`),
  listIncidents: (params?: Record<string, string>) =>
    apiClient.get<ApiResponse<Incident[]>>('/monitoring/incidents', params),
  createIncident: (data: any) =>
    apiClient.post<ApiResponse<Incident>>('/monitoring/incidents', data),
  getIncident: (id: string) =>
    apiClient.get<ApiResponse<Incident>>(`/monitoring/incidents/${id}`),
  updateIncident: (id: string, data: any) =>
    apiClient.put<ApiResponse<Incident>>(`/monitoring/incidents/${id}`, data),
}
