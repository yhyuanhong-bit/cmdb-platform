import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  metricSourcesApi,
  type CreateMetricSourceInput,
  type ListMetricSourcesParams,
  type UpdateMetricSourceInput,
} from '../lib/api/metricSources'

export function useMetricSources(params?: ListMetricSourcesParams) {
  return useQuery({
    queryKey: ['metric-sources', params],
    queryFn: () => metricSourcesApi.list(params),
  })
}

export function useMetricSource(id: string | undefined) {
  return useQuery({
    queryKey: ['metric-sources', id],
    queryFn: () => metricSourcesApi.get(id!),
    enabled: !!id,
  })
}

/** Freshness auto-refreshes every 60s. The data-plane health view is the
 *  one place an on-call operator stares at; auto-refresh means a stale
 *  source becomes visible without a manual reload. */
export function useMetricSourceFreshness() {
  return useQuery({
    queryKey: ['metric-sources', 'freshness'],
    queryFn: () => metricSourcesApi.freshness(),
    refetchInterval: 60_000,
  })
}

export function useCreateMetricSource() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: CreateMetricSourceInput) => metricSourcesApi.create(body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['metric-sources'] }),
  })
}

export function useUpdateMetricSource() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: UpdateMetricSourceInput }) =>
      metricSourcesApi.update(id, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['metric-sources'] }),
  })
}

export function useDeleteMetricSource() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => metricSourcesApi.remove(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['metric-sources'] }),
  })
}
