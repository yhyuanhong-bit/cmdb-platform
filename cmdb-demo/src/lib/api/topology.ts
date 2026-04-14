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
  listRackAssets: (rackId: string) =>
    apiClient.get<ApiResponse<any[]>>(`/racks/${rackId}/assets`),
  listRackSlots: (rackId: string) =>
    apiClient.get<ApiResponse<any[]>>(`/racks/${rackId}/slots`),
  createRackSlot: (rackId: string, data: any) =>
    apiClient.post<ApiResponse<any>>(`/racks/${rackId}/slots`, data),
  getRackStats: () =>
    apiClient.get<RackStats>('/racks/stats'),
  getLocationAssetCounts: () =>
    apiClient.get<ApiResponse<{ counts: Record<string, number>; alerts: Record<string, number> }>>('/locations/asset-counts'),
  listDependencies: (params: Record<string, string>) => apiClient.get('/topology/dependencies', params),
  createDependency: (data: any) => apiClient.post('/topology/dependencies', data),
  deleteDependency: (id: string) => apiClient.del(`/topology/dependencies/${id}`),
  getTopologyGraph: (params: Record<string, string>) => apiClient.get('/topology/graph', params),
  listNetworkConnections: (rackId: string) => apiClient.get(`/racks/${rackId}/network-connections`),
  createNetworkConnection: (rackId: string, data: any) => apiClient.post(`/racks/${rackId}/network-connections`, data),
  deleteNetworkConnection: (rackId: string, connId: string) => apiClient.del(`/racks/${rackId}/network-connections/${connId}`),
}

export interface RackStats {
  total_racks: number
  total_u: number
  used_u: number
  occupancy_pct: number
}
