import { apiClient } from './client'
import type { ApiResponse, ApiListResponse } from './types'
import type { components } from '../../generated/api-types'

export type User = components['schemas']['User']
export type Role = components['schemas']['Role']

export interface CreateUserInput {
  username: string
  display_name: string
  email: string
  phone?: string
  password: string
  status?: string
}

export interface UpdateUserInput {
  display_name?: string
  email?: string
  phone?: string
  status?: string
}

export interface CreateRoleInput {
  name: string
  description: string
  permissions?: Record<string, string[]>
}

export interface UpdateRoleInput {
  name?: string
  description?: string
  permissions?: Record<string, string[]>
}

export const identityApi = {
  listUsers: (params?: Record<string, string>) =>
    apiClient.get<ApiListResponse<User>>('/users', params),
  getUser: (id: string) =>
    apiClient.get<ApiResponse<User>>(`/users/${id}`),
  createUser: (data: CreateUserInput) =>
    apiClient.post<ApiResponse<User>>('/users', data),
  updateUser: (id: string, data: UpdateUserInput) =>
    apiClient.put<ApiResponse<User>>(`/users/${id}`, data),
  listRoles: () =>
    apiClient.get<ApiResponse<Role[]>>('/roles'),
  createRole: (data: CreateRoleInput) =>
    apiClient.post<ApiResponse<Role>>('/roles', data),
  updateRole: (id: string, data: UpdateRoleInput) =>
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
