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

export const integrationApi = {
  listAdapters: () =>
    apiClient.get<ApiResponse<AdapterConfig[]>>('/integration/adapters'),
  createAdapter: (data: any) =>
    apiClient.post<ApiResponse<AdapterConfig>>('/integration/adapters', data),
  listWebhooks: () =>
    apiClient.get<ApiResponse<WebhookSubscription[]>>('/integration/webhooks'),
  createWebhook: (data: any) =>
    apiClient.post<ApiResponse<WebhookSubscription>>('/integration/webhooks', data),
  listDeliveries: (webhookID: string) =>
    apiClient.get<ApiResponse<any[]>>(`/integration/webhooks/${webhookID}/deliveries`),
}
