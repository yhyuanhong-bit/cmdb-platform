import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  predictiveRefreshApi,
  type AggregatePredictiveRefreshParams,
  type ListPredictiveRefreshParams,
  type PredictiveRefreshKind,
  type PredictiveRefreshStatus,
} from '../lib/api/predictiveRefresh'

export function usePredictiveRefresh(params?: ListPredictiveRefreshParams) {
  return useQuery({
    queryKey: ['predictive', 'refresh', params],
    queryFn: () => predictiveRefreshApi.list(params),
  })
}

export function usePredictiveRefreshAggregate(params?: AggregatePredictiveRefreshParams) {
  return useQuery({
    queryKey: ['predictive', 'refresh', 'aggregate', params],
    queryFn: () => predictiveRefreshApi.aggregate(params),
  })
}

export function useRunPredictiveRefreshScan() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => predictiveRefreshApi.runScan(),
    onSuccess: () => {
      // A scan can change every list — drop them all and let the
      // visible page refetch.
      qc.invalidateQueries({ queryKey: ['predictive', 'refresh'] })
    },
  })
}

export function useTransitionPredictiveRefresh() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({
      assetId,
      kind,
      status,
      note,
    }: {
      assetId: string
      kind: PredictiveRefreshKind
      status: PredictiveRefreshStatus
      note?: string
    }) => predictiveRefreshApi.transition(assetId, kind, status, note),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['predictive', 'refresh'] })
    },
  })
}
