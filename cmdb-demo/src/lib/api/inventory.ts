import { apiClient } from './client'
import type { ApiResponse, ApiListResponse } from './types'
import type { components } from '../../generated/api-types'

export type InventoryTask = components['schemas']['InventoryTask']

export const inventoryApi = {
  list: (params?: Record<string, string>) =>
    apiClient.get<ApiListResponse<InventoryTask>>('/inventory/tasks', params),
  getById: (id: string) =>
    apiClient.get<ApiResponse<InventoryTask>>(`/inventory/tasks/${id}`),
  create: (data: any) =>
    apiClient.post<ApiResponse<InventoryTask>>('/inventory/tasks', data),
  complete: (id: string) =>
    apiClient.post<ApiResponse<InventoryTask>>(`/inventory/tasks/${id}/complete`),
  listItems: (taskID: string) =>
    apiClient.get<ApiResponse<any[]>>(`/inventory/tasks/${taskID}/items`),
  scanItem: (taskID: string, itemID: string, data: any) =>
    apiClient.post<ApiResponse<any>>(`/inventory/tasks/${taskID}/items/${itemID}/scan`, data),
  getSummary: (taskID: string) =>
    apiClient.get<ApiResponse<any>>(`/inventory/tasks/${taskID}/summary`),
  importItems: (taskId: string, items: any[]) =>
    apiClient.post<ApiResponse<any>>(`/inventory/tasks/${taskId}/import`, { items }),
}
