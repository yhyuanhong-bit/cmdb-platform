import { apiClient } from './client'
import type { ApiResponse } from './types'
import type { components } from '../../generated/api-types'

export type AlertEvent = components['schemas']['AlertEvent']
export type MetricPoint = components['schemas']['MetricPoint']

export interface AlertRule {
  id: string
  name: string
  metric_name: string
  condition: Record<string, unknown>
  severity: string
  enabled: boolean
  created_at?: string
}

export interface Incident {
  id: string
  title: string
  status: string
  severity: string
  started_at: string
  resolved_at: string | null
}

export interface FleetMetrics {
  total_assets: number
  active_alerts: number
  avg_cpu: number
  avg_memory: number
  avg_disk: number
}

export interface CreateAlertRuleInput {
  name: string
  metric_name: string
  condition: Record<string, unknown>
  severity: string
  enabled?: boolean
}

export interface CreateIncidentInput {
  title: string
  severity: string
  description?: string
}

export interface UpdateIncidentInput {
  status?: string
  severity?: string
  title?: string
  resolved_at?: string | null
}

export const monitoringApi = {
  queryMetrics: (params: Record<string, string>) =>
    apiClient.get<ApiResponse<MetricPoint[]>>('/monitoring/metrics', params),
  listAlertRules: () =>
    apiClient.get<ApiResponse<AlertRule[]>>('/monitoring/rules'),
  createAlertRule: (data: CreateAlertRuleInput) =>
    apiClient.post<ApiResponse<AlertRule>>('/monitoring/rules', data),
  listAlerts: (params?: Record<string, string>) =>
    apiClient.get<ApiResponse<AlertEvent[]>>('/monitoring/alerts', params),
  acknowledgeAlert: (id: string) =>
    apiClient.post<void>(`/monitoring/alerts/${id}/ack`),
  resolveAlert: (id: string) =>
    apiClient.post<void>(`/monitoring/alerts/${id}/resolve`),
  listIncidents: (params?: Record<string, string>) =>
    apiClient.get<ApiResponse<Incident[]>>('/monitoring/incidents', params),
  createIncident: (data: CreateIncidentInput) =>
    apiClient.post<ApiResponse<Incident>>('/monitoring/incidents', data),
  getIncident: (id: string) =>
    apiClient.get<ApiResponse<Incident>>(`/monitoring/incidents/${id}`),
  updateIncident: (id: string, data: UpdateIncidentInput) =>
    apiClient.put<ApiResponse<Incident>>(`/monitoring/incidents/${id}`, data),
  updateAlertRule: (id: string, data: Partial<CreateAlertRuleInput>) =>
    apiClient.put<ApiResponse<AlertRule>>(`/monitoring/rules/${id}`, data),
  deleteAlertRule: (id: string) =>
    apiClient.del(`/monitoring/rules/${id}`),
  getFleetMetrics: () =>
    apiClient.get<ApiResponse<FleetMetrics>>('/fleet-metrics'),
}
