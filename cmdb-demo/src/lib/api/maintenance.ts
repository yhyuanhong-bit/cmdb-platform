import { apiClient } from './client'
import type { ApiResponse, ApiListResponse } from './types'
import type { components } from '../../generated/api-types'

export type WorkOrder = components['schemas']['WorkOrder']

export const maintenanceApi = {
  list: (params?: Record<string, string>) =>
    apiClient.get<ApiListResponse<WorkOrder>>('/maintenance/orders', params),
  getById: (id: string) =>
    apiClient.get<ApiResponse<WorkOrder>>(`/maintenance/orders/${id}`),
  create: (data: any) =>
    apiClient.post<ApiResponse<WorkOrder>>('/maintenance/orders', data),
  update: (id: string, data: any) =>
    apiClient.put<ApiResponse<WorkOrder>>(`/maintenance/orders/${id}`, data),
  transition: (id: string, data: { status: string; comment: string }) =>
    apiClient.post<ApiResponse<WorkOrder>>(`/maintenance/orders/${id}/transition`, data),
  listLogs: (id: string) =>
    apiClient.get<ApiResponse<any[]>>(`/maintenance/orders/${id}/logs`),
  listComments: (orderId: string) => apiClient.get(`/maintenance/orders/${orderId}/comments`),
  createComment: (orderId: string, data: any) => apiClient.post(`/maintenance/orders/${orderId}/comments`, data),
  delete: (id: string) => apiClient.del(`/maintenance/orders/${id}`),
}
