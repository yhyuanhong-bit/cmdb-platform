import { apiClient } from './client'
import type { ApiResponse, ApiListResponse } from './types'
import type { components } from '../../generated/api-types'

export type WorkOrder = components['schemas']['WorkOrder']

export interface CreateWorkOrderData {
  title: string
  type: string
  priority: string
  asset_id?: string
  location_id?: string
  assignee_id?: string
  description?: string
  reason?: string
  scheduled_start?: string
  scheduled_end?: string
}

export interface UpdateWorkOrderData {
  title?: string
  type?: string
  priority?: string
  asset_id?: string
  location_id?: string
  assignee_id?: string
  description?: string
  scheduled_start?: string
  scheduled_end?: string
  status?: string
}

export interface WorkOrderComment {
  content?: string
  text?: string
  [key: string]: unknown
}

export interface WorkOrderLog {
  id: string
  action: string
  performed_by: string
  performed_at: string
  note?: string
}

export const maintenanceApi = {
  list: (params?: Record<string, string>) =>
    apiClient.get<ApiListResponse<WorkOrder>>('/maintenance/orders', params),
  getById: (id: string) =>
    apiClient.get<ApiResponse<WorkOrder>>(`/maintenance/orders/${id}`),
  create: (data: CreateWorkOrderData) =>
    apiClient.post<ApiResponse<WorkOrder>>('/maintenance/orders', data),
  update: (id: string, data: UpdateWorkOrderData) =>
    apiClient.put<ApiResponse<WorkOrder>>(`/maintenance/orders/${id}`, data),
  transition: (id: string, data: { status: string; comment: string }) =>
    apiClient.post<ApiResponse<WorkOrder>>(`/maintenance/orders/${id}/transition`, data),
  listLogs: (id: string) =>
    apiClient.get<ApiResponse<WorkOrderLog[]>>(`/maintenance/orders/${id}/logs`),
  listComments: (orderId: string) => apiClient.get(`/maintenance/orders/${orderId}/comments`),
  createComment: (orderId: string, data: WorkOrderComment) => apiClient.post(`/maintenance/orders/${orderId}/comments`, data),
  delete: (id: string) => apiClient.del(`/maintenance/orders/${id}`),
}
