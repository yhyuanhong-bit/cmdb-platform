import { useQuery } from '@tanstack/react-query'
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
