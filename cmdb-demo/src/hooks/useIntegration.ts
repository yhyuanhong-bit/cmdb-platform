import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { integrationApi } from '../lib/api/integration'

export function useAdapters() {
  return useQuery({
    queryKey: ['adapters'],
    queryFn: () => integrationApi.listAdapters(),
  })
}

export function useWebhooks() {
  return useQuery({
    queryKey: ['webhooks'],
    queryFn: () => integrationApi.listWebhooks(),
  })
}

export function useCreateAdapter() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: integrationApi.createAdapter,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['adapters'] }),
  })
}

export function useCreateWebhook() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: integrationApi.createWebhook,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['webhooks'] }),
  })
}

export function useWebhookDeliveries(webhookId: string) {
  return useQuery({
    queryKey: ['webhooks', webhookId, 'deliveries'],
    queryFn: () => integrationApi.listDeliveries(webhookId),
    enabled: !!webhookId,
  })
}
