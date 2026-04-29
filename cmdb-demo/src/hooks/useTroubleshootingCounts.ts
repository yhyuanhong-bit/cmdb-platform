import { useQueries } from '@tanstack/react-query'
import { auditApi } from '../lib/api/audit'

/**
 * Troubleshooting category → backend `audit_events.module` value(s).
 *
 * Modules emitted by `recordAudit` (cmdb-core/internal/api):
 *   asset, identity, monitoring, maintenance, topology,
 *   integration, discovery, quality, inventory, prediction, bia
 *
 * The page surface is "Troubleshooting", so we treat each category's count
 * as the volume of audit activity in the underlying module (a reasonable
 * proxy for "how many issues have been touched here lately"). Multi-module
 * categories (Data) sum across their members.
 */
const CATEGORY_MODULES: Record<string, readonly string[]> = {
  Auth: ['identity'],
  Assets: ['asset'],
  Monitoring: ['monitoring'],
  Maintenance: ['maintenance'],
  Network: ['topology'],
  Data: ['integration', 'discovery', 'quality'],
}

export type TroubleshootingFilterKey = keyof typeof CATEGORY_MODULES

interface AuditListWithPagination {
  pagination?: { total?: number }
}

/**
 * Returns a map of `filterKey → count`. Counts come from
 * `pagination.total` of an `/audit/events?module=…&page_size=1` query so we
 * only ship a single envelope per module rather than the full event list.
 *
 * On error or pending state, the corresponding key resolves to `null` so the
 * caller can render a placeholder (— / 0) instead of stale or fake data.
 */
export function useTroubleshootingCounts(): {
  counts: Record<string, number | null>
  isLoading: boolean
} {
  const moduleList = Array.from(
    new Set(Object.values(CATEGORY_MODULES).flat())
  )

  const queries = useQueries({
    queries: moduleList.map((module) => ({
      queryKey: ['troubleshootingCount', module] as const,
      queryFn: () =>
        auditApi.query({ module, page: '1', page_size: '1' }) as Promise<AuditListWithPagination>,
      staleTime: 60_000,
    })),
  })

  const moduleCount: Record<string, number | null> = {}
  moduleList.forEach((module, idx) => {
    const q = queries[idx]
    if (q.isSuccess) {
      moduleCount[module] = q.data?.pagination?.total ?? 0
    } else if (q.isError) {
      moduleCount[module] = null
    } else {
      moduleCount[module] = null
    }
  })

  const counts: Record<string, number | null> = {}
  for (const [filterKey, modules] of Object.entries(CATEGORY_MODULES)) {
    let sum = 0
    let allReady = true
    for (const m of modules) {
      const v = moduleCount[m]
      if (v == null) {
        allReady = false
        break
      }
      sum += v
    }
    counts[filterKey] = allReady ? sum : null
  }

  return {
    counts,
    isLoading: queries.some((q) => q.isPending),
  }
}
