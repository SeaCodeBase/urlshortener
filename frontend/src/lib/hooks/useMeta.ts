import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { queryKeys } from './keys';

/** Public server settings (base URL, redirect host/IP, feature flags). */
export function useMeta() {
  return useQuery({
    queryKey: queryKeys.meta,
    queryFn: () => api.getMeta(),
    staleTime: Infinity,
  });
}
