import { apiClient } from './client'
import type { ApiResponse, ApiListResponse } from './types'
import type { Incident } from './monitoring'

/** Problem lifecycle status — matches the DB CHECK constraint and the
 *  state machine enforced in problem.Service. */
export type ProblemStatus =
  | 'open'
  | 'investigating'
  | 'known_error'
  | 'resolved'
  | 'closed'

export type ProblemPriority = 'p1' | 'p2' | 'p3' | 'p4'

export interface Problem {
  id: string
  title: string
  description?: string | null
  status: ProblemStatus
  priority?: ProblemPriority | null
  severity: string
  root_cause?: string | null
  workaround?: string | null
  resolution?: string | null
  assignee_user_id?: string | null
  created_by?: string | null
  created_at: string
  updated_at?: string | null
  resolved_at?: string | null
  resolved_by?: string | null
  closed_at?: string | null
}

export interface ProblemComment {
  id: string
  problem_id: string
  author_id?: string | null
  author_username?: string | null
  kind: 'human' | 'system'
  body: string
  created_at: string
}

export interface CreateProblemInput {
  title: string
  description?: string
  severity?: string
  priority?: ProblemPriority
  workaround?: string
  assignee_user_id?: string
}

export interface UpdateProblemInput {
  title?: string
  description?: string
  severity?: string
  priority?: ProblemPriority
  workaround?: string
  root_cause?: string
  resolution?: string
  assignee_user_id?: string
}

export interface ListProblemsParams {
  page?: number
  page_size?: number
  status?: ProblemStatus
  priority?: ProblemPriority
}

function stringParams(p?: ListProblemsParams): Record<string, string> | undefined {
  if (!p) return undefined
  const out: Record<string, string> = {}
  if (p.page != null) out.page = String(p.page)
  if (p.page_size != null) out.page_size = String(p.page_size)
  if (p.status) out.status = p.status
  if (p.priority) out.priority = p.priority
  return out
}

export const problemsApi = {
  list: (params?: ListProblemsParams) =>
    apiClient.get<ApiListResponse<Problem>>('/problems', stringParams(params)),
  get: (id: string) =>
    apiClient.get<ApiResponse<Problem>>(`/problems/${id}`),
  create: (body: CreateProblemInput) =>
    apiClient.post<ApiResponse<Problem>>('/problems', body),
  update: (id: string, body: UpdateProblemInput) =>
    apiClient.put<ApiResponse<Problem>>(`/problems/${id}`, body),

  // Lifecycle. 409 PROBLEM_INVALID_TRANSITION on illegal source state.
  startInvestigating: (id: string, note?: string) =>
    apiClient.post<ApiResponse<Problem>>(`/problems/${id}/start-investigating`, { note: note ?? '' }),
  markKnownError: (id: string, workaround: string, note?: string) =>
    apiClient.post<ApiResponse<Problem>>(`/problems/${id}/known-error`, { workaround, note: note ?? '' }),
  resolve: (id: string, rootCause?: string, resolution?: string, note?: string) =>
    apiClient.post<ApiResponse<Problem>>(`/problems/${id}/resolve`, {
      root_cause: rootCause ?? '',
      resolution: resolution ?? '',
      note: note ?? '',
    }),
  close: (id: string) =>
    apiClient.post<ApiResponse<Problem>>(`/problems/${id}/close`),
  reopen: (id: string, reason?: string) =>
    apiClient.post<ApiResponse<Problem>>(`/problems/${id}/reopen`, { reason: reason ?? '' }),

  // Activity timeline.
  listComments: (id: string) =>
    apiClient.get<ApiResponse<ProblemComment[]>>(`/problems/${id}/comments`),
  createComment: (id: string, body: string) =>
    apiClient.post<ApiResponse<ProblemComment>>(`/problems/${id}/comments`, { body }),

  // Linkage (M:N with incidents). Idempotent on the link side.
  listIncidentsForProblem: (problemId: string) =>
    apiClient.get<ApiResponse<Incident[]>>(`/problems/${problemId}/incidents`),
  listProblemsForIncident: (incidentId: string) =>
    apiClient.get<ApiResponse<Problem[]>>(`/monitoring/incidents/${incidentId}/problems`),
  linkIncidentToProblem: (incidentId: string, problemId: string) =>
    apiClient.post<void>(`/monitoring/incidents/${incidentId}/problems/${problemId}`),
  unlinkIncidentFromProblem: (incidentId: string, problemId: string) =>
    apiClient.del(`/monitoring/incidents/${incidentId}/problems/${problemId}`),
}
