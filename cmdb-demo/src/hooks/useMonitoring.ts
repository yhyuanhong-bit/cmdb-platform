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
    mutationFn: (data: Parameters<typeof monitoringApi.createAlertRule>[0]) => monitoringApi.createAlertRule(data),
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

export function useFleetMetrics() {
  return useQuery({
    queryKey: ['fleetMetrics'],
    queryFn: () => monitoringApi.getFleetMetrics(),
    refetchInterval: 60000,
  })
}

// ---------------------------------------------------------------------------
// Wave 5.1: incident detail + lifecycle.
// ---------------------------------------------------------------------------

export function useIncident(id: string | undefined) {
  return useQuery({
    queryKey: ['incidents', id],
    queryFn: () => monitoringApi.getIncident(id!),
    enabled: !!id,
  })
}

export function useIncidentComments(id: string | undefined) {
  return useQuery({
    queryKey: ['incidents', id, 'comments'],
    queryFn: () => monitoringApi.listIncidentComments(id!),
    enabled: !!id,
  })
}

// All five lifecycle mutations invalidate the same set: list view + detail
// + timeline. Centralise that so each call site doesn't have to remember.
function invalidateIncidentTree(qc: ReturnType<typeof useQueryClient>, id: string) {
  qc.invalidateQueries({ queryKey: ['incidents'] })
  qc.invalidateQueries({ queryKey: ['incidents', id] })
  qc.invalidateQueries({ queryKey: ['incidents', id, 'comments'] })
}

export function useAcknowledgeIncident() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, note }: { id: string; note?: string }) =>
      monitoringApi.acknowledgeIncident(id, note),
    onSuccess: (_data, vars) => invalidateIncidentTree(qc, vars.id),
  })
}

export function useStartInvestigatingIncident() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => monitoringApi.startInvestigatingIncident(id),
    onSuccess: (_data, id) => invalidateIncidentTree(qc, id),
  })
}

export function useResolveIncident() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, rootCause, note }: { id: string; rootCause?: string; note?: string }) =>
      monitoringApi.resolveIncident(id, rootCause, note),
    onSuccess: (_data, vars) => invalidateIncidentTree(qc, vars.id),
  })
}

export function useCloseIncident() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => monitoringApi.closeIncident(id),
    onSuccess: (_data, id) => invalidateIncidentTree(qc, id),
  })
}

export function useReopenIncident() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, reason }: { id: string; reason?: string }) =>
      monitoringApi.reopenIncident(id, reason),
    onSuccess: (_data, vars) => invalidateIncidentTree(qc, vars.id),
  })
}

export function useAddIncidentComment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: string }) =>
      monitoringApi.createIncidentComment(id, body),
    onSuccess: (_data, vars) => invalidateIncidentTree(qc, vars.id),
  })
}
