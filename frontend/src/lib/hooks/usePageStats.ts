import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import type { PageStatsQuery } from '@/types';
import { queryKeys } from './keys';

/** Views/engagements of a page over the window; keeps the previous data while a new window loads. */
export function usePageStats(id: number | null | undefined, query: PageStatsQuery = {}) {
  return useQuery({
    queryKey: queryKeys.pageStats(id ?? 0, query),
    queryFn: () => api.getPageStats(id as number, query),
    enabled: id != null && id > 0,
    placeholderData: keepPreviousData,
  });
}
