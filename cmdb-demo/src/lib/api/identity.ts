import { apiClient } from './client'
import type { ApiResponse, ApiListResponse } from './types'
import type { components } from '../../generated/api-types'

export type User = components['schemas']['User']
export type Role = components['schemas']['Role']

export const identityApi = {
  listUsers: (params?: Record<string, string>) =>
    apiClient.get<ApiListResponse<User>>('/users', params),
  getUser: (id: string) =>
    apiClient.get<ApiResponse<User>>(`/users/${id}`),
  createUser: (data: any) =>
    apiClient.post<ApiResponse<User>>('/users', data),
  updateUser: (id: string, data: any) =>
    apiClient.put<ApiResponse<User>>(`/users/${id}`, data),
  listRoles: () =>
    apiClient.get<ApiResponse<Role[]>>('/roles'),
  createRole: (data: any) =>
    apiClient.post<ApiResponse<Role>>('/roles', data),
  updateRole: (id: string, data: any) =>
    apiClient.put<ApiResponse<Role>>(`/roles/${id}`, data),
  deleteRole: (id: string) =>
    apiClient.del(`/roles/${id}`),
  assignRole: (userId: string, roleId: string) =>
    apiClient.post(`/users/${userId}/roles`, { role_id: roleId }),
  removeRole: (userId: string, roleId: string) =>
    apiClient.del(`/users/${userId}/roles/${roleId}`),
  listUserRoles: (userId: string) =>
    apiClient.get(`/users/${userId}/roles`),
  deleteUser: (userId: string) =>
    apiClient.del(`/users/${userId}`),
  listSessions: (userId: string) => apiClient.get(`/users/${userId}/sessions`),
  changePassword: (data: { current_password: string; new_password: string }) =>
    apiClient.post('/auth/change-password', data),
}
