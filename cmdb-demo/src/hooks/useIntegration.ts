import { useQuery } from '@tanstack/react-query'
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
