import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ingestionApi } from '../lib/api/ingestion'
import { useAuthStore } from '../stores/authStore'

function useTenantId() {
  return useAuthStore((s) => s.tenantId) ?? 'a0000000-0000-0000-0000-000000000001'
}

export function useScanTargets() {
  const tenantId = useTenantId()
  return useQuery({
    queryKey: ['scanTargets', tenantId],
    queryFn: () => ingestionApi.listScanTargets({ tenant_id: tenantId }),
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
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) =>
      ingestionApi.updateScanTarget(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['scanTargets'] }),
  })
}

export function useDeleteScanTarget() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => ingestionApi.deleteScanTarget(id),
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
    queryFn: () => ingestionApi.listTasks({ tenant_id: tenantId }),
    refetchInterval: 10000,
  })
}

export function useTestCollector() {
  return useMutation({
    mutationFn: ({ name, data }: { name: string; data: { credential_id: string; endpoint: string } }) =>
      ingestionApi.testCollector(name, data),
  })
}
