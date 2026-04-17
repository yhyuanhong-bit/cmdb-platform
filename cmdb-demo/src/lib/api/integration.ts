import { apiClient } from './client'
import type { ApiResponse } from './types'

export interface AdapterConfig {
  id: string
  name: string
  type: string
  direction: string
  endpoint: string
  enabled: boolean
}

export interface WebhookSubscription {
  id: string
  name: string
  url: string
  events: string[]
  enabled: boolean
}

export interface WebhookDelivery {
  id: string
  webhook_id: string
  event: string
  status: string
  response_code: number
  delivered_at: string
}

export interface CreateAdapterInput {
  name: string
  type: string
  direction: string
  endpoint: string
  enabled?: boolean
  config?: Record<string, unknown>
}

export interface CreateWebhookInput {
  name: string
  url: string
  events: string[]
  enabled?: boolean
}

// All fields are optional; server treats omitted keys as "leave unchanged".
export interface UpdateAdapterInput {
  name?: string
  type?: string
  direction?: string
  endpoint?: string
  enabled?: boolean
  config?: Record<string, unknown>
}

export interface UpdateWebhookInput {
  name?: string
  url?: string
  secret?: string
  events?: string[]
  enabled?: boolean
}

export const integrationApi = {
  listAdapters: () =>
    apiClient.get<ApiResponse<AdapterConfig[]>>('/integration/adapters'),
  createAdapter: (data: CreateAdapterInput) =>
    apiClient.post<ApiResponse<AdapterConfig>>('/integration/adapters', data),
  updateAdapter: (id: string, data: UpdateAdapterInput) =>
    apiClient.patch<ApiResponse<AdapterConfig>>(`/integration/adapters/${id}`, data),
  deleteAdapter: (id: string) =>
    apiClient.del(`/integration/adapters/${id}`),
  listWebhooks: () =>
    apiClient.get<ApiResponse<WebhookSubscription[]>>('/integration/webhooks'),
  createWebhook: (data: CreateWebhookInput) =>
    apiClient.post<ApiResponse<WebhookSubscription>>('/integration/webhooks', data),
  updateWebhook: (id: string, data: UpdateWebhookInput) =>
    apiClient.patch<ApiResponse<WebhookSubscription>>(`/integration/webhooks/${id}`, data),
  deleteWebhook: (id: string) =>
    apiClient.del(`/integration/webhooks/${id}`),
  listDeliveries: (webhookID: string) =>
    apiClient.get<ApiResponse<WebhookDelivery[]>>(`/integration/webhooks/${webhookID}/deliveries`),
}
