import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { topologyApi, type Location, type Rack } from '../lib/api/topology'

export function useRootLocations() {
  return useQuery({
    queryKey: ['locations', 'root'],
    queryFn: () => topologyApi.listRootLocations(),
  })
}

export function useLocation(id: string) {
  return useQuery({
    queryKey: ['locations', id],
    queryFn: () => topologyApi.getLocation(id),
    enabled: !!id,
  })
}

export function useLocationChildren(id: string) {
  return useQuery({
    queryKey: ['locations', id, 'children'],
    queryFn: () => topologyApi.listChildren(id),
    enabled: !!id,
  })
}

export function useRacks(locationID: string) {
  return useQuery({
    queryKey: ['racks', locationID],
    queryFn: () => topologyApi.listRacks(locationID),
    enabled: !!locationID,
  })
}

export function useRack(id: string) {
  return useQuery({
    queryKey: ['racks', 'detail', id],
    queryFn: () => topologyApi.getRack(id),
    enabled: !!id,
  })
}

export function useCreateRack() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: topologyApi.createRack,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['racks'] }),
  })
}

export function useLocationStats(id: string) {
  return useQuery({
    queryKey: ['locations', id, 'stats'],
    queryFn: () => topologyApi.getLocationStats(id),
    enabled: !!id,
  })
}

export function useLocationDescendants(id: string) {
  return useQuery({
    queryKey: ['locations', id, 'descendants'],
    queryFn: () => topologyApi.listDescendants(id),
    enabled: !!id,
  })
}

export function useRackAssets(rackId: string) {
  return useQuery({
    queryKey: ['racks', rackId, 'assets'],
    queryFn: () => topologyApi.listRackAssets(rackId),
    enabled: !!rackId,
  })
}

export function useCreateLocation() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: topologyApi.createLocation,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['locations'] }),
  })
}

export function useUpdateLocation() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<Location> }) =>
      topologyApi.updateLocation(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['locations'] }),
  })
}

export function useDeleteLocation() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: topologyApi.deleteLocation,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['locations'] }),
  })
}

export function useUpdateRack() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<Rack> }) =>
      topologyApi.updateRack(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['racks'] }),
  })
}

export function useDeleteRack() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: topologyApi.deleteRack,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['racks'] }),
  })
}

export function useRackSlots(rackId: string) {
  return useQuery({
    queryKey: ['rackSlots', rackId],
    queryFn: () => topologyApi.listRackSlots(rackId),
    enabled: !!rackId,
  })
}

export function useCreateRackSlot() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ rackId, data }: { rackId: string; data: Record<string, unknown> }) => topologyApi.createRackSlot(rackId, data),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['rackSlots'] }) }
  })
}

export function useTopologyGraph(locationId: string) {
  return useQuery({
    queryKey: ['topologyGraph', locationId],
    queryFn: () => topologyApi.getTopologyGraph({ location_id: locationId }),
    enabled: !!locationId,
  })
}

export function useAssetDependencies(assetId: string) {
  return useQuery({
    queryKey: ['assetDependencies', assetId],
    queryFn: () => topologyApi.listDependencies({ asset_id: assetId }),
    enabled: !!assetId,
  })
}

export function useCreateDependency() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: Record<string, unknown>) => topologyApi.createDependency(data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['assetDependencies'] })
      qc.invalidateQueries({ queryKey: ['topologyGraph'] })
    },
  })
}

export function useDeleteDependency() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => topologyApi.deleteDependency(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['assetDependencies'] })
      qc.invalidateQueries({ queryKey: ['topologyGraph'] })
    },
  })
}

export function useRackNetworkConnections(rackId: string) {
  return useQuery({
    queryKey: ['rackNetworkConnections', rackId],
    queryFn: () => topologyApi.listNetworkConnections(rackId),
    enabled: !!rackId,
  })
}

export function useCreateNetworkConnection() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ rackId, data }: { rackId: string; data: Record<string, unknown> }) =>
      topologyApi.createNetworkConnection(rackId, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['rackNetworkConnections'] }),
  })
}

export function useDeleteNetworkConnection() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ rackId, connId }: { rackId: string; connId: string }) =>
      topologyApi.deleteNetworkConnection(rackId, connId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['rackNetworkConnections'] }),
  })
}
