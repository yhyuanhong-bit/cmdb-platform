import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { assetApi, type Asset, type CapacityForecast } from '../lib/api/assets'

export function useAssetLifecycle(assetId: string) {
  return useQuery({
    queryKey: ['assetLifecycle', assetId],
    queryFn: () => assetApi.getLifecycle(assetId),
    enabled: !!assetId,
  })
}

// Surfaces the most recent scan-style audit_events for an asset (discovery /
// integration today). Backend returns last_scan_at=null with an empty events
// array when nothing matches — the page renders an empty state in that case.
export function useAssetComplianceScan(assetId: string) {
  return useQuery({
    queryKey: ['assetComplianceScan', assetId],
    queryFn: () => assetApi.getComplianceScan(assetId),
    enabled: !!assetId,
  })
}

export function useLifecycleStats() {
  return useQuery({
    queryKey: ['lifecycleStats'],
    queryFn: () => assetApi.getLifecycleStats(),
  })
}

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
    mutationFn: ({ id, data }: { id: string; data: Partial<Asset> }) =>
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

export function useCapacityPlanning() {
  return useQuery({
    queryKey: ['capacityPlanning'],
    queryFn: () => assetApi.getCapacityPlanning(),
    refetchInterval: 60000,
    select: (res) => (Array.isArray(res?.data) ? res.data : []) as CapacityForecast[],
  })
}

export function useAcceptUpgradeRecommendation() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ assetId, category, data }: { assetId: string; category: string; data?: Record<string, unknown> }) =>
      assetApi.acceptUpgradeRecommendation(assetId, category, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['upgradeRecommendations'] })
      qc.invalidateQueries({ queryKey: ['workOrders'] })
    },
  })
}
