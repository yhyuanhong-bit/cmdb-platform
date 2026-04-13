import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { syncApi } from '../lib/api/sync'

export function useSyncState() {
  return useQuery({
    queryKey: ['syncState'],
    queryFn: () => syncApi.getState(),
    refetchInterval: 30000,
  })
}

export function useSyncConflicts() {
  return useQuery({
    queryKey: ['syncConflicts'],
    queryFn: () => syncApi.getConflicts(),
  })
}

export function useResolveConflict() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, resolution }: { id: string; resolution: 'local_wins' | 'remote_wins' }) =>
      syncApi.resolveConflict(id, resolution),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['syncConflicts'] }),
  })
}

export function useSyncStats() {
  return useQuery({
    queryKey: ['syncStats'],
    queryFn: () => syncApi.getStats(),
    refetchInterval: 30000,
  })
}
