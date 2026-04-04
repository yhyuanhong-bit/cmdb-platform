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
