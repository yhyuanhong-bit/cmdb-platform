import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { topologyApi } from '../lib/api/topology'

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
    mutationFn: ({ id, data }: { id: string; data: Partial<any> }) =>
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
    mutationFn: ({ id, data }: { id: string; data: Partial<any> }) =>
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
    mutationFn: ({ rackId, data }: { rackId: string; data: any }) => topologyApi.createRackSlot(rackId, data),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['rackSlots'] }) }
  })
}
