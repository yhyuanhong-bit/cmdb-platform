import { useQuery, useMutation, useQueryClient, type QueryClient } from '@tanstack/react-query'
import {
  changesApi,
  type ChangeVote,
  type CreateChangeInput,
  type ListChangesParams,
  type UpdateChangeInput,
} from '../lib/api/changes'

/** Invalidate every cached view of one change. Used by every mutation
 *  (CRUD, lifecycle, votes, comments, linkage) so callers don't have to
 *  remember the right key set. The linkage tree is included so a
 *  link/unlink in another page also refreshes the change detail's
 *  panels. */
function invalidateChangeTree(qc: QueryClient, id: string) {
  qc.invalidateQueries({ queryKey: ['changes'] })
  qc.invalidateQueries({ queryKey: ['changes', id] })
  qc.invalidateQueries({ queryKey: ['changes', id, 'comments'] })
  qc.invalidateQueries({ queryKey: ['changes', id, 'approvals'] })
  qc.invalidateQueries({ queryKey: ['changes', id, 'assets'] })
  qc.invalidateQueries({ queryKey: ['changes', id, 'services'] })
  qc.invalidateQueries({ queryKey: ['changes', id, 'problems'] })
}

export function useChanges(params?: ListChangesParams) {
  return useQuery({
    queryKey: ['changes', params],
    queryFn: () => changesApi.list(params),
  })
}

export function useChange(id: string | undefined) {
  return useQuery({
    queryKey: ['changes', id],
    queryFn: () => changesApi.get(id!),
    enabled: !!id,
  })
}

export function useChangeApprovals(id: string | undefined) {
  return useQuery({
    queryKey: ['changes', id, 'approvals'],
    queryFn: () => changesApi.listApprovals(id!),
    enabled: !!id,
  })
}

export function useChangeComments(id: string | undefined) {
  return useQuery({
    queryKey: ['changes', id, 'comments'],
    queryFn: () => changesApi.listComments(id!),
    enabled: !!id,
  })
}

export function useChangeAssets(id: string | undefined) {
  return useQuery({
    queryKey: ['changes', id, 'assets'],
    queryFn: () => changesApi.listAssets(id!),
    enabled: !!id,
  })
}

export function useChangeServices(id: string | undefined) {
  return useQuery({
    queryKey: ['changes', id, 'services'],
    queryFn: () => changesApi.listServices(id!),
    enabled: !!id,
  })
}

export function useChangeProblems(id: string | undefined) {
  return useQuery({
    queryKey: ['changes', id, 'problems'],
    queryFn: () => changesApi.listProblems(id!),
    enabled: !!id,
  })
}

export function useCreateChange() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: CreateChangeInput) => changesApi.create(body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['changes'] }),
  })
}

export function useUpdateChange() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: UpdateChangeInput }) =>
      changesApi.update(id, body),
    onSuccess: (_d, vars) => invalidateChangeTree(qc, vars.id),
  })
}

export function useSubmitChange() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => changesApi.submit(id),
    onSuccess: (_d, id) => invalidateChangeTree(qc, id),
  })
}

export function useStartChange() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => changesApi.start(id),
    onSuccess: (_d, id) => invalidateChangeTree(qc, id),
  })
}

export function useMarkChangeSucceeded() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, note }: { id: string; note?: string }) =>
      changesApi.markSucceeded(id, note),
    onSuccess: (_d, vars) => invalidateChangeTree(qc, vars.id),
  })
}

export function useMarkChangeFailed() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, note }: { id: string; note?: string }) =>
      changesApi.markFailed(id, note),
    onSuccess: (_d, vars) => invalidateChangeTree(qc, vars.id),
  })
}

export function useMarkChangeRolledBack() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, note }: { id: string; note?: string }) =>
      changesApi.markRolledBack(id, note),
    onSuccess: (_d, vars) => invalidateChangeTree(qc, vars.id),
  })
}

export function useCastChangeVote() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, vote, note }: { id: string; vote: ChangeVote; note?: string }) =>
      changesApi.castVote(id, vote, note),
    onSuccess: (_d, vars) => invalidateChangeTree(qc, vars.id),
  })
}

export function useAddChangeComment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: string }) =>
      changesApi.createComment(id, body),
    onSuccess: (_d, vars) => invalidateChangeTree(qc, vars.id),
  })
}

export function useLinkChangeAsset() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ changeId, assetId }: { changeId: string; assetId: string }) =>
      changesApi.linkAsset(changeId, assetId),
    onSuccess: (_d, vars) => invalidateChangeTree(qc, vars.changeId),
  })
}

export function useUnlinkChangeAsset() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ changeId, assetId }: { changeId: string; assetId: string }) =>
      changesApi.unlinkAsset(changeId, assetId),
    onSuccess: (_d, vars) => invalidateChangeTree(qc, vars.changeId),
  })
}

export function useLinkChangeService() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ changeId, serviceId }: { changeId: string; serviceId: string }) =>
      changesApi.linkService(changeId, serviceId),
    onSuccess: (_d, vars) => invalidateChangeTree(qc, vars.changeId),
  })
}

export function useUnlinkChangeService() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ changeId, serviceId }: { changeId: string; serviceId: string }) =>
      changesApi.unlinkService(changeId, serviceId),
    onSuccess: (_d, vars) => invalidateChangeTree(qc, vars.changeId),
  })
}

export function useLinkChangeProblem() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ changeId, problemId }: { changeId: string; problemId: string }) =>
      changesApi.linkProblem(changeId, problemId),
    onSuccess: (_d, vars) => invalidateChangeTree(qc, vars.changeId),
  })
}

export function useUnlinkChangeProblem() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ changeId, problemId }: { changeId: string; problemId: string }) =>
      changesApi.unlinkProblem(changeId, problemId),
    onSuccess: (_d, vars) => invalidateChangeTree(qc, vars.changeId),
  })
}
