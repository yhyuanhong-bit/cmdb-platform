import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ingestionApi } from '../lib/api/ingestion'
const DEFAULT_TENANT = 'a0000000-0000-0000-0000-000000000001'

function useTenantId() {
  return DEFAULT_TENANT
}

export function useCredentials() {
  const tenantId = useTenantId()
  return useQuery({
    queryKey: ['credentials', tenantId],
    queryFn: () => ingestionApi.listCredentials({ tenant_id: tenantId }),
  })
}

export function useCreateCredential() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ingestionApi.createCredential,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['credentials'] }),
  })
}

export function useUpdateCredential() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Record<string, unknown> }) =>
      ingestionApi.updateCredential(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['credentials'] }),
  })
}

export function useDeleteCredential() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => ingestionApi.deleteCredential(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['credentials'] }),
  })
}
