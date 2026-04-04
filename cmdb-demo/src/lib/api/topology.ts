import { apiClient } from './client'
import type { ApiResponse } from './types'
import type { components } from '../../generated/api-types'

export type Location = components['schemas']['Location']
export type Rack = components['schemas']['Rack']

export const topologyApi = {
  listRootLocations: () =>
    apiClient.get<ApiResponse<Location[]>>('/locations'),
  getLocation: (id: string) =>
    apiClient.get<ApiResponse<Location>>(`/locations/${id}`),
  listChildren: (id: string) =>
    apiClient.get<ApiResponse<Location[]>>(`/locations/${id}/children`),
  listDescendants: (id: string) =>
    apiClient.get<ApiResponse<Location[]>>(`/locations/${id}/descendants`),
  listAncestors: (id: string) =>
    apiClient.get<ApiResponse<Location[]>>(`/locations/${id}/ancestors`),
  createLocation: (data: Partial<Location>) =>
    apiClient.post<ApiResponse<Location>>('/locations', data),
  updateLocation: (id: string, data: Partial<Location>) =>
    apiClient.put<ApiResponse<Location>>(`/locations/${id}`, data),
  deleteLocation: (id: string) =>
    apiClient.del(`/locations/${id}`),
  getLocationStats: (id: string) =>
    apiClient.get<ApiResponse<any>>(`/locations/${id}/stats`),
  listRacks: (locationID: string) =>
    apiClient.get<ApiResponse<Rack[]>>(`/locations/${locationID}/racks`),
  createRack: (data: Partial<Rack>) =>
    apiClient.post<ApiResponse<Rack>>('/racks', data),
  getRack: (id: string) =>
    apiClient.get<ApiResponse<Rack>>(`/racks/${id}`),
  updateRack: (id: string, data: Partial<Rack>) =>
    apiClient.put<ApiResponse<Rack>>(`/racks/${id}`, data),
  deleteRack: (id: string) =>
    apiClient.del(`/racks/${id}`),
}
