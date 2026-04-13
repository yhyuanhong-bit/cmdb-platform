import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { discoveryApi } from '../lib/api/discovery'

export function useDiscoveredAssets(params?: Record<string, string>) {
  return useQuery({
    queryKey: ['discoveredAssets', params],
    queryFn: () => discoveryApi.list(params),
  })
}

export function useDiscoveryStats() {
  return useQuery({
    queryKey: ['discoveryStats'],
    queryFn: () => discoveryApi.getStats(),
  })
}

export function useApproveAsset() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => discoveryApi.approve(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['discoveredAssets'] })
      qc.invalidateQueries({ queryKey: ['discoveryStats'] })
    },
  })
}

export function useIgnoreAsset() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => discoveryApi.ignore(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['discoveredAssets'] })
      qc.invalidateQueries({ queryKey: ['discoveryStats'] })
    },
  })
}
