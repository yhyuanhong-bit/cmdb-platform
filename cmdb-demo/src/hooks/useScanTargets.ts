import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ingestionApi } from '../lib/api/ingestion'
import { useAuthStore } from '../stores/authStore'

// Audit E3 (2026-04-28): the previous DEFAULT_TENANT constant routed every
// scan-target / discovery-task call to the demo tenant regardless of the
// signed-in user. tenant_id now flows from the auth store. If the user is
// not signed in (`tenant_id` undefined) the queries are skipped via the
// `enabled` flag so we never send the literal string "undefined" upstream.
function useTenantId(): string | undefined {
  return useAuthStore((s) => s.user?.tenant_id)
}

export function useScanTargets() {
  const tenantId = useTenantId()
  return useQuery({
    queryKey: ['scanTargets', tenantId],
    queryFn: () => ingestionApi.listScanTargets({ tenant_id: tenantId! }),
    enabled: !!tenantId,
  })
}

export function useCreateScanTarget() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ingestionApi.createScanTarget,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['scanTargets'] }),
  })
}

export function useUpdateScanTarget() {
  const qc = useQueryClient()
  const tenantId = useTenantId()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Record<string, unknown> }) => {
      if (!tenantId) throw new Error('not signed in')
      return ingestionApi.updateScanTarget(id, data, tenantId)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['scanTargets'] }),
  })
}

export function useDeleteScanTarget() {
  const qc = useQueryClient()
  const tenantId = useTenantId()
  return useMutation({
    mutationFn: (id: string) => {
      if (!tenantId) throw new Error('not signed in')
      return ingestionApi.deleteScanTarget(id, tenantId)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['scanTargets'] }),
  })
}

export function useTriggerScan() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ingestionApi.triggerScan,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['discoveryTasks'] }),
  })
}

export function useDiscoveryTasks() {
  const tenantId = useTenantId()
  return useQuery({
    queryKey: ['discoveryTasks', tenantId],
    queryFn: () => ingestionApi.listTasks({ tenant_id: tenantId! }),
    refetchInterval: 10000,
    enabled: !!tenantId,
  })
}

export function useTestCollector() {
  return useMutation({
    mutationFn: ({ name, data }: { name: string; data: { credential_id: string; endpoint: string } }) =>
      ingestionApi.testCollector(name, data),
  })
}
