import { apiClient } from './client'
import type { ApiResponse, ApiListResponse } from './types'
import type { Asset } from './assets'
import type { Service } from './services'
import type { Problem } from './problems'

/** Change lifecycle status — matches the DB CHECK constraint. UI uses
 *  this to gate which lifecycle actions show in the action bar. */
export type ChangeStatus =
  | 'draft'
  | 'submitted'
  | 'approved'
  | 'rejected'
  | 'in_progress'
  | 'succeeded'
  | 'failed'
  | 'rolled_back'

export type ChangeType = 'standard' | 'normal' | 'emergency'
export type ChangeRisk = 'low' | 'medium' | 'high' | 'critical'
export type ChangeVote = 'approve' | 'reject' | 'abstain'

export interface Change {
  id: string
  title: string
  description?: string | null
  type: ChangeType
  risk: ChangeRisk
  status: ChangeStatus
  approval_threshold: number
  requested_by?: string | null
  assignee_user_id?: string | null
  planned_start?: string | null
  planned_end?: string | null
  actual_start?: string | null
  actual_end?: string | null
  rollback_plan?: string | null
  impact_summary?: string | null
  created_at: string
  updated_at?: string | null
  submitted_at?: string | null
  approved_at?: string | null
  rejected_at?: string | null
}

export interface ChangeApproval {
  id: string
  change_id: string
  voter_id: string
  voter_username?: string | null
  vote: ChangeVote
  note?: string | null
  voted_at: string
}

export interface ChangeComment {
  id: string
  change_id: string
  author_id?: string | null
  author_username?: string | null
  kind: 'human' | 'system'
  body: string
  created_at: string
}

export interface CreateChangeInput {
  title: string
  description?: string
  type?: ChangeType
  risk?: ChangeRisk
  approval_threshold?: number
  assignee_user_id?: string
  planned_start?: string
  planned_end?: string
  rollback_plan?: string
  impact_summary?: string
}

export interface UpdateChangeInput {
  title?: string
  description?: string
  type?: ChangeType
  risk?: ChangeRisk
  approval_threshold?: number
  assignee_user_id?: string
  planned_start?: string
  planned_end?: string
  rollback_plan?: string
  impact_summary?: string
}

export interface ListChangesParams {
  page?: number
  page_size?: number
  status?: ChangeStatus
  type?: ChangeType
  risk?: ChangeRisk
}

function stringParams(p?: ListChangesParams): Record<string, string> | undefined {
  if (!p) return undefined
  const out: Record<string, string> = {}
  if (p.page != null) out.page = String(p.page)
  if (p.page_size != null) out.page_size = String(p.page_size)
  if (p.status) out.status = p.status
  if (p.type) out.type = p.type
  if (p.risk) out.risk = p.risk
  return out
}

export const changesApi = {
  list: (params?: ListChangesParams) =>
    apiClient.get<ApiListResponse<Change>>('/changes', stringParams(params)),
  get: (id: string) =>
    apiClient.get<ApiResponse<Change>>(`/changes/${id}`),
  create: (body: CreateChangeInput) =>
    apiClient.post<ApiResponse<Change>>('/changes', body),
  update: (id: string, body: UpdateChangeInput) =>
    apiClient.put<ApiResponse<Change>>(`/changes/${id}`, body),

  // Lifecycle. 409 CHANGE_INVALID_TRANSITION on illegal source state.
  submit: (id: string) =>
    apiClient.post<ApiResponse<Change>>(`/changes/${id}/submit`),
  start: (id: string) =>
    apiClient.post<ApiResponse<Change>>(`/changes/${id}/start`),
  markSucceeded: (id: string, note?: string) =>
    apiClient.post<ApiResponse<Change>>(`/changes/${id}/succeeded`, { note: note ?? '' }),
  markFailed: (id: string, note?: string) =>
    apiClient.post<ApiResponse<Change>>(`/changes/${id}/failed`, { note: note ?? '' }),
  markRolledBack: (id: string, note?: string) =>
    apiClient.post<ApiResponse<Change>>(`/changes/${id}/rolled-back`, { note: note ?? '' }),

  // CAB voting.
  listApprovals: (id: string) =>
    apiClient.get<ApiResponse<ChangeApproval[]>>(`/changes/${id}/votes`),
  castVote: (id: string, vote: ChangeVote, note?: string) =>
    apiClient.post<ApiResponse<Change>>(`/changes/${id}/votes`, { vote, note: note ?? '' }),

  // Comments.
  listComments: (id: string) =>
    apiClient.get<ApiResponse<ChangeComment[]>>(`/changes/${id}/comments`),
  createComment: (id: string, body: string) =>
    apiClient.post<ApiResponse<ChangeComment>>(`/changes/${id}/comments`, { body }),

  // Linkage (M:N with assets / services / problems).
  listAssets: (id: string) =>
    apiClient.get<ApiResponse<Asset[]>>(`/changes/${id}/assets`),
  linkAsset: (id: string, assetId: string) =>
    apiClient.post<void>(`/changes/${id}/assets/${assetId}`),
  unlinkAsset: (id: string, assetId: string) =>
    apiClient.del(`/changes/${id}/assets/${assetId}`),

  listServices: (id: string) =>
    apiClient.get<ApiResponse<Service[]>>(`/changes/${id}/services`),
  linkService: (id: string, serviceId: string) =>
    apiClient.post<void>(`/changes/${id}/services/${serviceId}`),
  unlinkService: (id: string, serviceId: string) =>
    apiClient.del(`/changes/${id}/services/${serviceId}`),

  listProblems: (id: string) =>
    apiClient.get<ApiResponse<Problem[]>>(`/changes/${id}/problems`),
  linkProblem: (id: string, problemId: string) =>
    apiClient.post<void>(`/changes/${id}/problems/${problemId}`),
  unlinkProblem: (id: string, problemId: string) =>
    apiClient.del(`/changes/${id}/problems/${problemId}`),

  // Reverse lookup from Problem.
  listChangesForProblem: (problemId: string) =>
    apiClient.get<ApiResponse<Change[]>>(`/problems/${problemId}/changes`),
}
