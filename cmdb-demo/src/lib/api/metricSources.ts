import { apiClient } from './client'
import type { ApiResponse } from './types'

export type MetricSourceKind = 'snmp' | 'ipmi' | 'agent' | 'pipeline' | 'manual'
export type MetricSourceStatus = 'active' | 'disabled'

export interface MetricSource {
  id: string
  name: string
  kind: MetricSourceKind
  expected_interval_seconds: number
  status: MetricSourceStatus
  last_heartbeat_at?: string | null
  last_sample_count: number
  notes?: string | null
  created_at: string
  updated_at?: string | null
}

/** A source returned by the freshness check.
 *  seconds_since_heartbeat is null when the source has never heartbeated. */
export interface MetricSourceFreshness {
  id: string
  name: string
  kind: string
  expected_interval_seconds: number
  status: string
  last_heartbeat_at?: string | null
  seconds_since_heartbeat?: number | null
}

export interface CreateMetricSourceInput {
  name: string
  kind: MetricSourceKind
  expected_interval_seconds: number
  status?: MetricSourceStatus
  notes?: string
}

export interface UpdateMetricSourceInput {
  name?: string
  kind?: MetricSourceKind
  expected_interval_seconds?: number
  status?: MetricSourceStatus
  notes?: string
}

export interface ListMetricSourcesParams {
  status?: MetricSourceStatus
  kind?: MetricSourceKind
}

function listParams(p?: ListMetricSourcesParams): Record<string, string> | undefined {
  if (!p) return undefined
  const out: Record<string, string> = {}
  if (p.status) out.status = p.status
  if (p.kind) out.kind = p.kind
  return out
}

export const metricSourcesApi = {
  list: (params?: ListMetricSourcesParams) =>
    apiClient.get<ApiResponse<MetricSource[]>>('/metrics/sources', listParams(params)),
  get: (id: string) =>
    apiClient.get<ApiResponse<MetricSource>>(`/metrics/sources/${id}`),
  create: (body: CreateMetricSourceInput) =>
    apiClient.post<ApiResponse<MetricSource>>('/metrics/sources', body),
  update: (id: string, body: UpdateMetricSourceInput) =>
    apiClient.put<ApiResponse<MetricSource>>(`/metrics/sources/${id}`, body),
  remove: (id: string) =>
    apiClient.del(`/metrics/sources/${id}`),
  heartbeat: (id: string, sampleDelta = 0) =>
    apiClient.post<ApiResponse<MetricSource>>(
      `/metrics/sources/${id}/heartbeat`,
      { sample_delta: sampleDelta },
    ),
  freshness: () =>
    apiClient.get<ApiResponse<MetricSourceFreshness[]>>('/metrics/sources/freshness'),
}
