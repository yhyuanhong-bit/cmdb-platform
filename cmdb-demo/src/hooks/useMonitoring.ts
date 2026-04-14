import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { monitoringApi, type AlertRule } from '../lib/api/monitoring'

export function useAlerts(params?: Record<string, string>) {
  return useQuery({
    queryKey: ['alerts', params],
    queryFn: () => monitoringApi.listAlerts(params),
  })
}

export function useAlertRules() {
  return useQuery({
    queryKey: ['alertRules'],
    queryFn: () => monitoringApi.listAlertRules(),
  })
}

export function useIncidents(params?: Record<string, string>) {
  return useQuery({
    queryKey: ['incidents', params],
    queryFn: () => monitoringApi.listIncidents(params),
  })
}

export function useAcknowledgeAlert() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: monitoringApi.acknowledgeAlert,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alerts'] }),
  })
}

export function useResolveAlert() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: monitoringApi.resolveAlert,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alerts'] }),
  })
}

export function useUpdateAlertRule() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<AlertRule> }) =>
      monitoringApi.updateAlertRule(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alertRules'] }),
  })
}

export function useCreateAlertRule() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: Record<string, unknown>) => monitoringApi.createAlertRule(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alertRules'] }),
  })
}

export function useDeleteAlertRule() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => monitoringApi.deleteAlertRule(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alertRules'] }),
  })
}
