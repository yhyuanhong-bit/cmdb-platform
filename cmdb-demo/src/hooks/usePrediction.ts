import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { predictionApi } from '../lib/api/prediction'
import type { VerifyRCAData } from '../lib/api/prediction'

export function usePredictionModels() {
  return useQuery({
    queryKey: ['predictionModels'],
    queryFn: () => predictionApi.listModels(),
  })
}

export function usePredictionsByAsset(ciId: string) {
  return useQuery({
    queryKey: ['predictions', ciId],
    queryFn: () => predictionApi.listByCI(ciId),
    enabled: !!ciId,
  })
}

export function useCreateRCA() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: predictionApi.createRCA,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['predictions'] }),
  })
}

export function useVerifyRCA() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: VerifyRCAData }) => predictionApi.verifyRCA(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['predictions'] }),
  })
}

export function useAssetRUL(assetId: string) {
  return useQuery({
    queryKey: ['assetRUL', assetId],
    queryFn: () => predictionApi.getRUL(assetId),
    enabled: !!assetId,
  })
}

export function useFailureDistribution() {
  return useQuery({
    queryKey: ['failureDistribution'],
    queryFn: () => predictionApi.getFailureDistribution(),
  })
}
