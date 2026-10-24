import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { queryKeys, useActiveGroupId } from './keys';

/** Performance overview of the active group: this week vs last week and top performing items. */
export function useAnalyticsOverview() {
  const groupId = useActiveGroupId();
  return useQuery({
    queryKey: queryKeys.overview(groupId),
    queryFn: () => api.getAnalyticsOverview(),
    enabled: groupId != null,
  });
}
