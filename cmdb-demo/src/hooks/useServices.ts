import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  servicesApi,
  type ListServicesParams,
  type CreateServiceRequest,
  type UpdateServiceRequest,
  type AddServiceAssetRequest,
} from '../lib/api/services'

export function useServices(params?: ListServicesParams) {
  return useQuery({
    queryKey: ['services', params],
    queryFn: () => servicesApi.list(params),
  })
}

export function useService(id: string | undefined) {
  return useQuery({
    queryKey: ['services', id],
    queryFn: () => servicesApi.get(id!),
    enabled: !!id,
  })
}

export function useServiceAssets(id: string | undefined) {
  return useQuery({
    queryKey: ['services', id, 'assets'],
    queryFn: () => servicesApi.listAssets(id!),
    enabled: !!id,
  })
}

export function useServiceHealth(id: string | undefined) {
  return useQuery({
    queryKey: ['services', id, 'health'],
    queryFn: () => servicesApi.health(id!),
    enabled: !!id,
    // Health polls more frequently than static service metadata — 30s
    // matches the default alert-evaluator cadence, so a degraded
    // service shows up in the UI within one refresh cycle.
    refetchInterval: 30_000,
  })
}

export function useServicesForAsset(assetId: string | undefined) {
  return useQuery({
    queryKey: ['assets', assetId, 'services'],
    queryFn: () => servicesApi.listForAsset(assetId!),
    enabled: !!assetId,
  })
}

export function useCreateService() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: CreateServiceRequest) => servicesApi.create(body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['services'] })
    },
  })
}

export function useUpdateService(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: UpdateServiceRequest) => servicesApi.update(id, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['services'] })
      qc.invalidateQueries({ queryKey: ['services', id] })
    },
  })
}

export function useDeleteService() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => servicesApi.remove(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['services'] })
    },
  })
}

export function useAddServiceAsset(serviceId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: AddServiceAssetRequest) => servicesApi.addAsset(serviceId, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['services', serviceId, 'assets'] })
      qc.invalidateQueries({ queryKey: ['services', serviceId, 'health'] })
    },
  })
}

export function useRemoveServiceAsset(serviceId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (assetId: string) => servicesApi.removeAsset(serviceId, assetId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['services', serviceId, 'assets'] })
      qc.invalidateQueries({ queryKey: ['services', serviceId, 'health'] })
    },
  })
}
