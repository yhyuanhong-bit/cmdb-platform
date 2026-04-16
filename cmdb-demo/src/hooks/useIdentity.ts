import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
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

export function useCreateUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: identityApi.createUser,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['users'] }),
  })
}

export function useUpdateUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Record<string, unknown> }) =>
      identityApi.updateUser(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['users'] }),
  })
}

export function useCreateRole() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: identityApi.createRole,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['roles'] }),
  })
}

export function useUpdateRole() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Parameters<typeof identityApi.updateRole>[1] }) =>
      identityApi.updateRole(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['roles'] }),
  })
}

export function useDeleteRole() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: identityApi.deleteRole,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['roles'] }),
  })
}

export function useAssignRole() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ userId, roleId }: { userId: string; roleId: string }) =>
      identityApi.assignRole(userId, roleId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['users'] }),
  })
}

export function useRemoveRole() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ userId, roleId }: { userId: string; roleId: string }) =>
      identityApi.removeRole(userId, roleId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['users'] }),
  })
}

export function useUserRoles(userId: string) {
  return useQuery({
    queryKey: ['userRoles', userId],
    queryFn: () => identityApi.listUserRoles(userId),
    enabled: !!userId,
  })
}

export function useDeleteUser() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (userId: string) => identityApi.deleteUser(userId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['users'] }),
  })
}

export function useUserSessions(userId: string) {
  return useQuery({
    queryKey: ['userSessions', userId],
    queryFn: () => identityApi.listSessions(userId),
    enabled: !!userId,
  })
}

export function useChangePassword() {
  return useMutation({
    mutationFn: identityApi.changePassword,
  })
}
