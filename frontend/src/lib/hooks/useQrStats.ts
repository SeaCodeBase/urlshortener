import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import type { LinkStatsQuery } from '@/types';
import { queryKeys } from './keys';

/** Scan analytics of one QR code (same shape as link stats). */
export function useQrStats(id: number | null | undefined, query: LinkStatsQuery = {}) {
  return useQuery({
    queryKey: queryKeys.qrStats(id ?? 0, query),
    queryFn: () => api.getQrStats(id as number, query),
    enabled: id != null && id > 0,
    placeholderData: keepPreviousData,
  });
}
