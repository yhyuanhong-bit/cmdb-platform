import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { maintenanceApi } from '../lib/api/maintenance'

export function useWorkOrders(params?: Record<string, string>) {
  return useQuery({
    queryKey: ['workOrders', params],
    queryFn: () => maintenanceApi.list(params),
  })
}

export function useWorkOrder(id: string) {
  return useQuery({
    queryKey: ['workOrders', id],
    queryFn: () => maintenanceApi.getById(id),
    enabled: !!id,
  })
}

export function useCreateWorkOrder() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: maintenanceApi.create,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['workOrders'] }),
  })
}

export function useUpdateWorkOrder() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) =>
      maintenanceApi.update(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['workOrders'] }),
  })
}

export function useTransitionWorkOrder() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: { status: string; comment: string } }) =>
      maintenanceApi.transition(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['workOrders'] }),
  })
}

export function useWorkOrderLogs(id: string) {
  return useQuery({
    queryKey: ['workOrders', id, 'logs'],
    queryFn: () => maintenanceApi.listLogs(id),
    enabled: !!id,
  })
}
