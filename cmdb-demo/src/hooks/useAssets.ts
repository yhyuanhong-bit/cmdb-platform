import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { assetApi } from '../lib/api/assets'

export function useAssets(params?: Record<string, string>) {
  return useQuery({
    queryKey: ['assets', params],
    queryFn: () => assetApi.list(params),
  })
}

export function useAsset(id: string) {
  return useQuery({
    queryKey: ['assets', id],
    queryFn: () => assetApi.getById(id),
    enabled: !!id,
  })
}

export function useCreateAsset() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: assetApi.create,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['assets'] }),
  })
}

export function useUpdateAsset() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<any> }) =>
      assetApi.update(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['assets'] }),
  })
}

export function useDeleteAsset() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => assetApi.delete(id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['assets'] }) },
  })
}

export function useUpgradeRecommendations(assetId: string) {
  return useQuery({
    queryKey: ['upgradeRecommendations', assetId],
    queryFn: () => assetApi.getUpgradeRecommendations(assetId),
    enabled: !!assetId,
  })
}

export function useAcceptUpgradeRecommendation() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ assetId, category, data }: { assetId: string; category: string; data?: any }) =>
      assetApi.acceptUpgradeRecommendation(assetId, category, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['upgradeRecommendations'] })
      qc.invalidateQueries({ queryKey: ['workOrders'] })
    },
  })
}
