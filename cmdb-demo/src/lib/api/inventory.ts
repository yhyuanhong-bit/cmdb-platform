import { apiClient } from './client'
import type { ApiResponse, ApiListResponse } from './types'
import type { components } from '../../generated/api-types'

export type InventoryTask = components['schemas']['InventoryTask']

export interface CreateInventoryTaskData {
  name: string
  location_id?: string
  assignee_id?: string
  scheduled_date?: string
  description?: string
}

export interface ScanItemData {
  scanned_serial?: string
  scanned_asset_tag?: string
  condition?: string
  note?: string
}

export interface ScanRecord {
  scanned_serial?: string
  scanned_asset_tag?: string
  condition?: string
  note?: string
  scanned_at?: string
  scanned_by?: string
}

export interface ItemNote {
  content?: string
  text?: string
  severity?: string
  [key: string]: unknown
}

export interface InventoryItem {
  id: string
  asset_id: string
  asset_tag?: string
  serial?: string
  name?: string
  status: string
  scanned: boolean
  discrepancy?: string
  expected?: Record<string, unknown>
  actual?: Record<string, unknown>
}

export interface InventorySummary {
  total: number
  scanned: number
  discrepancies: number
  completion_pct: number
}

export const inventoryApi = {
  list: (params?: Record<string, string>) =>
    apiClient.get<ApiListResponse<InventoryTask>>('/inventory/tasks', params),
  getById: (id: string) =>
    apiClient.get<ApiResponse<InventoryTask>>(`/inventory/tasks/${id}`),
  create: (data: CreateInventoryTaskData) =>
    apiClient.post<ApiResponse<InventoryTask>>('/inventory/tasks', data),
  complete: (id: string) =>
    apiClient.post<ApiResponse<InventoryTask>>(`/inventory/tasks/${id}/complete`),
  listItems: (taskID: string) =>
    apiClient.get<ApiResponse<InventoryItem[]>>(`/inventory/tasks/${taskID}/items`),
  scanItem: (taskID: string, itemID: string, data: ScanItemData) =>
    apiClient.post<ApiResponse<InventoryItem>>(`/inventory/tasks/${taskID}/items/${itemID}/scan`, data),
  getSummary: (taskID: string) =>
    apiClient.get<ApiResponse<InventorySummary>>(`/inventory/tasks/${taskID}/summary`),
  importItems: (taskId: string, items: Record<string, unknown>[]) =>
    apiClient.post<ApiResponse<{ imported: number }>>(`/inventory/tasks/${taskId}/import`, { items }),
  listScanHistory: (taskId: string, itemId: string) => apiClient.get<ApiResponse<ScanRecord[]>>(`/inventory/tasks/${taskId}/items/${itemId}/scan-history`),
  createScanRecord: (taskId: string, itemId: string, data: ScanRecord) => apiClient.post(`/inventory/tasks/${taskId}/items/${itemId}/scan-history`, data),
  listItemNotes: (taskId: string, itemId: string) => apiClient.get<ApiResponse<ItemNote[]>>(`/inventory/tasks/${taskId}/items/${itemId}/notes`),
  createItemNote: (taskId: string, itemId: string, data: ItemNote) => apiClient.post(`/inventory/tasks/${taskId}/items/${itemId}/notes`, data),
  update: (id: string, data: Record<string, unknown>) =>
    apiClient.put<ApiResponse<InventoryTask>>(`/inventory/tasks/${id}`, data),
  delete: (id: string) => apiClient.del(`/inventory/tasks/${id}`),
  resolve: (taskId: string, itemId: string, data: { action: string; note?: string }) =>
    apiClient.post(`/inventory/tasks/${taskId}/items/${itemId}/resolve`, data),
}
