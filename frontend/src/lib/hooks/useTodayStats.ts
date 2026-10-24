import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import type { TodayRange } from '@/types';
import { queryKeys, useActiveGroupId } from './keys';

/** Engagements of the active group over the range. */
export function useTodayStats(range: TodayRange = 'today') {
  const groupId = useActiveGroupId();
  return useQuery({
    queryKey: queryKeys.today(groupId, range),
    queryFn: () => api.getTodayStats(range),
    enabled: groupId != null,
  });
}
