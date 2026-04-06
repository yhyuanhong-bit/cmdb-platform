import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { qualityApi } from '../lib/api/quality'

export function useQualityDashboard() {
  return useQuery({ queryKey: ['qualityDashboard'], queryFn: () => qualityApi.getDashboard() })
}

export function useQualityRules() {
  return useQuery({ queryKey: ['qualityRules'], queryFn: () => qualityApi.listRules() })
}

export function useQualityHistory(assetId: string) {
  return useQuery({
    queryKey: ['qualityHistory', assetId],
    queryFn: () => qualityApi.getAssetHistory(assetId),
    enabled: !!assetId,
  })
}

export function useTriggerQualityScan() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => qualityApi.triggerScan(),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['qualityDashboard'] }) }
  })
}

export function useCreateQualityRule() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: any) => qualityApi.createRule(data),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['qualityRules'] }) }
  })
}
