import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { integrationApi, type UpdateAdapterInput, type UpdateWebhookInput } from '../lib/api/integration'

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

export function useUpdateAdapter() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (vars: { id: string; data: UpdateAdapterInput }) =>
      integrationApi.updateAdapter(vars.id, vars.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['adapters'] }),
  })
}

export function useDeleteAdapter() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => integrationApi.deleteAdapter(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['adapters'] }),
  })
}

export function useUpdateWebhook() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (vars: { id: string; data: UpdateWebhookInput }) =>
      integrationApi.updateWebhook(vars.id, vars.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['webhooks'] }),
  })
}

export function useDeleteWebhook() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => integrationApi.deleteWebhook(id),
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
