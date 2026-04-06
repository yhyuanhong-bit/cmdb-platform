import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { inventoryApi } from '../lib/api/inventory'

export function useInventoryTasks(params?: Record<string, string>) {
  return useQuery({
    queryKey: ['inventoryTasks', params],
    queryFn: () => inventoryApi.list(params),
  })
}

export function useInventoryTask(id: string) {
  return useQuery({
    queryKey: ['inventoryTasks', id],
    queryFn: () => inventoryApi.getById(id),
    enabled: !!id,
  })
}

export function useInventoryItems(taskId: string) {
  return useQuery({
    queryKey: ['inventoryTasks', taskId, 'items'],
    queryFn: () => inventoryApi.listItems(taskId),
    enabled: !!taskId,
  })
}

export function useCreateInventoryTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: inventoryApi.create,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['inventoryTasks'] }),
  })
}

export function useCompleteTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (taskId: string) => inventoryApi.complete(taskId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['inventoryTasks'] }),
  })
}

export function useScanItem() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ taskId, itemId, data }: { taskId: string; itemId: string; data: any }) =>
      inventoryApi.scanItem(taskId, itemId, data),
    onSuccess: (_data, variables) =>
      qc.invalidateQueries({ queryKey: ['inventoryTasks', variables.taskId, 'items'] }),
  })
}

export function useImportInventoryItems() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ taskId, items }: { taskId: string; items: any[] }) =>
      inventoryApi.importItems(taskId, items),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['inventoryTasks'] }) }
  })
}

export function useTaskSummary(taskId: string) {
  return useQuery({
    queryKey: ['inventoryTasks', taskId, 'summary'],
    queryFn: () => inventoryApi.getSummary(taskId),
    enabled: !!taskId,
  })
}
