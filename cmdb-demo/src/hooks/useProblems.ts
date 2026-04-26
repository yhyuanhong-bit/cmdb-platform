import { useQuery, useMutation, useQueryClient, type QueryClient } from '@tanstack/react-query'
import {
  problemsApi,
  type CreateProblemInput,
  type ListProblemsParams,
  type UpdateProblemInput,
} from '../lib/api/problems'

/** Invalidate every cache entry that depends on a single problem. Used by
 *  every mutation (CRUD, lifecycle, comments) so consumers don't have to
 *  remember the right key triplet each time. */
function invalidateProblemTree(qc: QueryClient, id: string) {
  qc.invalidateQueries({ queryKey: ['problems'] })
  qc.invalidateQueries({ queryKey: ['problems', id] })
  qc.invalidateQueries({ queryKey: ['problems', id, 'comments'] })
  qc.invalidateQueries({ queryKey: ['problems', id, 'incidents'] })
}

/** Invalidate caches that depend on the link between an incident and any
 *  problem. Both forward (incident → problems) and reverse (problem →
 *  incidents) lookups must refresh. */
function invalidateLinkTree(qc: QueryClient, incidentId: string, problemId: string) {
  qc.invalidateQueries({ queryKey: ['incidents', incidentId, 'problems'] })
  qc.invalidateQueries({ queryKey: ['problems', problemId, 'incidents'] })
}

export function useProblems(params?: ListProblemsParams) {
  return useQuery({
    queryKey: ['problems', params],
    queryFn: () => problemsApi.list(params),
  })
}

export function useProblem(id: string | undefined) {
  return useQuery({
    queryKey: ['problems', id],
    queryFn: () => problemsApi.get(id!),
    enabled: !!id,
  })
}

export function useProblemComments(id: string | undefined) {
  return useQuery({
    queryKey: ['problems', id, 'comments'],
    queryFn: () => problemsApi.listComments(id!),
    enabled: !!id,
  })
}

export function useIncidentsForProblem(problemId: string | undefined) {
  return useQuery({
    queryKey: ['problems', problemId, 'incidents'],
    queryFn: () => problemsApi.listIncidentsForProblem(problemId!),
    enabled: !!problemId,
  })
}

export function useProblemsForIncident(incidentId: string | undefined) {
  return useQuery({
    queryKey: ['incidents', incidentId, 'problems'],
    queryFn: () => problemsApi.listProblemsForIncident(incidentId!),
    enabled: !!incidentId,
  })
}

export function useCreateProblem() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (body: CreateProblemInput) => problemsApi.create(body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['problems'] }),
  })
}

export function useUpdateProblem() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: UpdateProblemInput }) =>
      problemsApi.update(id, body),
    onSuccess: (_data, vars) => invalidateProblemTree(qc, vars.id),
  })
}

export function useStartInvestigatingProblem() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, note }: { id: string; note?: string }) =>
      problemsApi.startInvestigating(id, note),
    onSuccess: (_data, vars) => invalidateProblemTree(qc, vars.id),
  })
}

export function useMarkProblemKnownError() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, workaround, note }: { id: string; workaround: string; note?: string }) =>
      problemsApi.markKnownError(id, workaround, note),
    onSuccess: (_data, vars) => invalidateProblemTree(qc, vars.id),
  })
}

export function useResolveProblem() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({
      id,
      rootCause,
      resolution,
      note,
    }: {
      id: string
      rootCause?: string
      resolution?: string
      note?: string
    }) => problemsApi.resolve(id, rootCause, resolution, note),
    onSuccess: (_data, vars) => invalidateProblemTree(qc, vars.id),
  })
}

export function useCloseProblem() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => problemsApi.close(id),
    onSuccess: (_data, id) => invalidateProblemTree(qc, id),
  })
}

export function useReopenProblem() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, reason }: { id: string; reason?: string }) =>
      problemsApi.reopen(id, reason),
    onSuccess: (_data, vars) => invalidateProblemTree(qc, vars.id),
  })
}

export function useAddProblemComment() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: string }) =>
      problemsApi.createComment(id, body),
    onSuccess: (_data, vars) => invalidateProblemTree(qc, vars.id),
  })
}

export function useLinkIncidentToProblem() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ incidentId, problemId }: { incidentId: string; problemId: string }) =>
      problemsApi.linkIncidentToProblem(incidentId, problemId),
    onSuccess: (_data, vars) => invalidateLinkTree(qc, vars.incidentId, vars.problemId),
  })
}

export function useUnlinkIncidentFromProblem() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ incidentId, problemId }: { incidentId: string; problemId: string }) =>
      problemsApi.unlinkIncidentFromProblem(incidentId, problemId),
    onSuccess: (_data, vars) => invalidateLinkTree(qc, vars.incidentId, vars.problemId),
  })
}
