import { useQuery } from '@tanstack/react-query'
import { identityApi } from '../lib/api/identity'

export function useUsers(params?: Record<string, string>) {
  return useQuery({
    queryKey: ['users', params],
    queryFn: () => identityApi.listUsers(params),
  })
}

export function useUser(id: string) {
  return useQuery({
    queryKey: ['users', id],
    queryFn: () => identityApi.getUser(id),
    enabled: !!id,
  })
}

export function useRoles() {
  return useQuery({
    queryKey: ['roles'],
    queryFn: () => identityApi.listRoles(),
  })
}
