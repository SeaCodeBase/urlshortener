import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import type { LinkStatsQuery } from '@/types';
import { queryKeys } from './keys';

export function useLinkStats(id: number | null | undefined, query: LinkStatsQuery = {}) {
  return useQuery({
    queryKey: queryKeys.linkStats(id ?? 0, query),
    queryFn: () => api.getLinkStats(id as number, query),
    enabled: id != null && id > 0,
    placeholderData: keepPreviousData,
  });
}
