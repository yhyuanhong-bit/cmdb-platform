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

/** Incident lifecycle status — matches the DB CHECK constraint and the
 *  state machine enforced in monitoring.Service. The UI uses this to
 *  decide which lifecycle actions are reachable from the current state. */
export type IncidentStatus =
  | 'open'
  | 'acknowledged'
  | 'investigating'
  | 'resolved'
  | 'closed'

export type IncidentPriority = 'p1' | 'p2' | 'p3' | 'p4'

export interface Incident {
  id: string
  title: string
  status: IncidentStatus
  severity: string
  priority?: IncidentPriority | null
  description?: string | null
  impact?: string | null
  root_cause?: string | null
  assignee_user_id?: string | null
  affected_asset_id?: string | null
  affected_service_id?: string | null
  acknowledged_at?: string | null
  acknowledged_by?: string | null
  resolved_by?: string | null
  started_at: string
  resolved_at: string | null
  updated_at?: string | null
}

export interface IncidentComment {
  id: string
  incident_id: string
  author_id?: string | null
  author_username?: string | null
  kind: 'human' | 'system'
  body: string
  created_at: string
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

  // Wave 5.1: lifecycle transitions. Each is a dedicated POST so the
  // state-machine guard is visible at the URL level; the backend returns
  // 409 INCIDENT_INVALID_TRANSITION if the source state isn't allowed.
  acknowledgeIncident: (id: string, note?: string) =>
    apiClient.post<ApiResponse<Incident>>(`/monitoring/incidents/${id}/acknowledge`, { note: note ?? '' }),
  startInvestigatingIncident: (id: string) =>
    apiClient.post<ApiResponse<Incident>>(`/monitoring/incidents/${id}/start-investigating`),
  resolveIncident: (id: string, rootCause?: string, note?: string) =>
    apiClient.post<ApiResponse<Incident>>(`/monitoring/incidents/${id}/resolve`, { root_cause: rootCause ?? '', note: note ?? '' }),
  closeIncident: (id: string) =>
    apiClient.post<ApiResponse<Incident>>(`/monitoring/incidents/${id}/close`),
  reopenIncident: (id: string, reason?: string) =>
    apiClient.post<ApiResponse<Incident>>(`/monitoring/incidents/${id}/reopen`, { reason: reason ?? '' }),

  // Activity timeline (system + human comments).
  listIncidentComments: (id: string) =>
    apiClient.get<ApiResponse<IncidentComment[]>>(`/monitoring/incidents/${id}/comments`),
  createIncidentComment: (id: string, body: string) =>
    apiClient.post<ApiResponse<IncidentComment>>(`/monitoring/incidents/${id}/comments`, { body }),

  updateAlertRule: (id: string, data: Partial<CreateAlertRuleInput>) =>
    apiClient.put<ApiResponse<AlertRule>>(`/monitoring/rules/${id}`, data),
  deleteAlertRule: (id: string) =>
    apiClient.del(`/monitoring/rules/${id}`),
  getFleetMetrics: () =>
    apiClient.get<ApiResponse<FleetMetrics>>('/fleet-metrics'),
}
