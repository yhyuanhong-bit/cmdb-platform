import { apiClient } from './client'
import type { ApiListResponse } from './types'
import type { components } from '../../generated/api-types'

export type AuditEvent = components['schemas']['AuditEvent']

export const auditApi = {
  query: (params?: Record<string, string>) =>
    apiClient.get<ApiListResponse<AuditEvent>>('/audit/events', params),
}
