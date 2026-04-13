import { useQuery } from '@tanstack/react-query'
import { activityApi } from '../lib/api/activity'

export function useActivityFeed(targetType: string, targetId: string) {
  return useQuery({
    queryKey: ['activityFeed', targetType, targetId],
    queryFn: () => activityApi.getFeed({ target_type: targetType, target_id: targetId }),
    enabled: !!targetType && !!targetId,
  })
}
