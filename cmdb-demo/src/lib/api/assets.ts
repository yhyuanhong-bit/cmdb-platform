import { apiClient } from './client'
import type { ApiResponse, ApiListResponse } from './types'
import type { components } from '../../generated/api-types'

export type Asset = components['schemas']['Asset']

export const assetApi = {
  list: (params?: Record<string, string>) =>
    apiClient.get<ApiListResponse<Asset>>('/assets', params),
  getById: (id: string) =>
    apiClient.get<ApiResponse<Asset>>(`/assets/${id}`),
  create: (data: Partial<Asset>) =>
    apiClient.post<ApiResponse<Asset>>('/assets', data),
  update: (id: string, data: Partial<Asset>) =>
    apiClient.put<ApiResponse<Asset>>(`/assets/${id}`, data),
  delete: (id: string) =>
    apiClient.del(`/assets/${id}`),
}
