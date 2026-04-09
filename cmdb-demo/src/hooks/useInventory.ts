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
    mutationFn: ({ taskId, itemId, data }: { taskId: string; itemId: string; data: Record<string, unknown> }) =>
      inventoryApi.scanItem(taskId, itemId, data),
    onSuccess: (_data, variables) =>
      qc.invalidateQueries({ queryKey: ['inventoryTasks', variables.taskId, 'items'] }),
  })
}

export function useImportInventoryItems() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ taskId, items }: { taskId: string; items: Record<string, unknown>[] }) =>
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

export function useItemScanHistory(taskId: string, itemId: string) {
  return useQuery({
    queryKey: ['itemScanHistory', taskId, itemId],
    queryFn: () => inventoryApi.listScanHistory(taskId, itemId),
    enabled: !!taskId && !!itemId,
  })
}

export function useItemNotes(taskId: string, itemId: string) {
  return useQuery({
    queryKey: ['itemNotes', taskId, itemId],
    queryFn: () => inventoryApi.listItemNotes(taskId, itemId),
    enabled: !!taskId && !!itemId,
  })
}

export function useCreateItemScanRecord() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ taskId, itemId, data }: { taskId: string; itemId: string; data: Record<string, unknown> }) =>
      inventoryApi.createScanRecord(taskId, itemId, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['itemScanHistory'] }),
  })
}

export function useCreateItemNote() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ taskId, itemId, data }: { taskId: string; itemId: string; data: Record<string, unknown> }) =>
      inventoryApi.createItemNote(taskId, itemId, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['itemNotes'] }),
  })
}

export function useUpdateInventoryTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Record<string, unknown> }) =>
      inventoryApi.update(id, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['inventoryTasks'] }),
  })
}

export function useResolveDiscrepancy() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ taskId, itemId, data }: { taskId: string; itemId: string; data: { action: string; note?: string } }) =>
      inventoryApi.resolve(taskId, itemId, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['inventoryTasks'] })
      qc.invalidateQueries({ queryKey: ['inventoryDiscrepancies'] })
    },
  })
}

export function useDeleteInventoryTask() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => inventoryApi.delete(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['inventoryTasks'] }),
  })
}
