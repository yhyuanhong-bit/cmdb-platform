import { apiClient } from './client'
import type { ApiResponse } from './types'
import type { components } from '../../generated/api-types'

export type Location = components['schemas']['Location']
export type Rack = components['schemas']['Rack']
export type Asset = components['schemas']['Asset']
export type LocationStats = components['schemas']['LocationStats']

export interface RackSlot {
  id: string
  rack_id: string
  asset_id: string
  start_u: number
  height_u: number
  side: string
}

export interface CreateRackSlotInput {
  asset_id: string
  start_u: number
  end_u: number
  height_u?: number
  side?: string
}

export interface Dependency {
  id: string
  source_id: string
  target_id: string
  type: string
}

export interface CreateDependencyInput {
  source_id: string
  target_id: string
  type: string
}

export interface NetworkConnection {
  id: string
  source_port: string
  speed: string
  status: string
  vlans: number[]
  connection_type: string
  connected_asset_id?: string
  external_device?: string
}

export interface CreateNetworkConnectionInput {
  source_port: string
  speed: string
  status: string
  vlans: number[]
  connection_type: string
  connected_asset_id?: string
  external_device?: string
}

export const topologyApi = {
  listRootLocations: () =>
    apiClient.get<ApiResponse<Location[]>>('/locations'),
  listAllLocations: () =>
    apiClient.get<ApiResponse<Location[]>>('/locations', { all: 'true' }),
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
    apiClient.get<ApiResponse<LocationStats>>(`/locations/${id}/stats`),
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
    apiClient.get<ApiResponse<Asset[]>>(`/racks/${rackId}/assets`),
  listRackSlots: (rackId: string) =>
    apiClient.get<ApiResponse<RackSlot[]>>(`/racks/${rackId}/slots`),
  createRackSlot: (rackId: string, data: CreateRackSlotInput) =>
    apiClient.post<ApiResponse<RackSlot>>(`/racks/${rackId}/slots`, data),
  getRackStats: () =>
    apiClient.get<RackStats>('/racks/stats'),
  getLocationAssetCounts: () =>
    apiClient.get<ApiResponse<{ counts: Record<string, number>; alerts: Record<string, number> }>>('/locations/asset-counts'),
  listDependencies: (params: Record<string, string>) => apiClient.get<ApiResponse<Dependency[]>>('/topology/dependencies', params),
  createDependency: (data: CreateDependencyInput) => apiClient.post<ApiResponse<Dependency>>('/topology/dependencies', data),
  deleteDependency: (id: string) => apiClient.del(`/topology/dependencies/${id}`),
  getTopologyGraph: (params: Record<string, string>) => apiClient.get('/topology/graph', params),
  listNetworkConnections: (rackId: string) => apiClient.get<ApiResponse<NetworkConnection[]>>(`/racks/${rackId}/network-connections`),
  createNetworkConnection: (rackId: string, data: CreateNetworkConnectionInput) => apiClient.post<ApiResponse<NetworkConnection>>(`/racks/${rackId}/network-connections`, data),
  deleteNetworkConnection: (rackId: string, connId: string) => apiClient.del(`/racks/${rackId}/network-connections/${connId}`),
}

export interface RackStats {
  total_racks: number
  total_u: number
  used_u: number
  occupancy_pct: number
}
