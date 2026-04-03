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

export function useCreateLocation() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: topologyApi.createLocation,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['locations'] }),
  })
}
