import { useQuery } from '@tanstack/react-query'
import { schedulerHealthApi } from '../lib/api/schedulerHealth'

/** 30-second auto-refresh — operator dashboards stay fresh without
 *  manual reload. Slower than the metric-freshness page (60s) because
 *  scheduler ticks happen at higher frequency than agent heartbeats. */
export function useSchedulerHealth() {
  return useQuery({
    queryKey: ['scheduler-health'],
    queryFn: () => schedulerHealthApi.get(),
    refetchInterval: 30_000,
  })
}
