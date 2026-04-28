import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ingestionApi } from '../lib/api/ingestion'
import { useAuthStore } from '../stores/authStore'

// Audit E3 (2026-04-28): replaced the `DEFAULT_TENANT` constant with the
// signed-in user's tenant from the auth store so every credential call
// scopes correctly. Queries are gated on a non-empty tenant_id so
// unauthenticated state doesn't send the literal "undefined".
function useTenantId(): string | undefined {
  return useAuthStore((s) => s.user?.tenant_id)
}

export function useCredentials() {
  const tenantId = useTenantId()
  return useQuery({
    queryKey: ['credentials', tenantId],
    queryFn: () => ingestionApi.listCredentials({ tenant_id: tenantId! }),
    enabled: !!tenantId,
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
  const tenantId = useTenantId()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Record<string, unknown> }) => {
      if (!tenantId) throw new Error('not signed in')
      return ingestionApi.updateCredential(id, data, tenantId)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['credentials'] }),
  })
}

export function useDeleteCredential() {
  const qc = useQueryClient()
  const tenantId = useTenantId()
  return useMutation({
    mutationFn: (id: string) => {
      if (!tenantId) throw new Error('not signed in')
      return ingestionApi.deleteCredential(id, tenantId)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['credentials'] }),
  })
}
