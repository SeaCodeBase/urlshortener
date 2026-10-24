import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import type { ActivityParams } from '@/types';
import { queryKeys, useActiveGroupId } from './keys';

export function useActivity(params: ActivityParams = {}) {
  const groupId = useActiveGroupId();
  return useQuery({
    queryKey: queryKeys.activity(groupId, params),
    queryFn: () => api.getActivity(params),
    enabled: groupId != null,
    placeholderData: keepPreviousData,
  });
}
