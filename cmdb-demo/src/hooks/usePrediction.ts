import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { predictionApi } from '../lib/api/prediction'

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
    mutationFn: ({ id, data }: { id: string; data: any }) => predictionApi.verifyRCA(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['predictions'] }),
  })
}
