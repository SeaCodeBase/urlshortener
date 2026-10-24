import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { queryKeys, useActiveGroupId } from './keys';

/** Match counts for q in the active group. Debounce q in the caller. */
export function useSearch(q: string) {
  const groupId = useActiveGroupId();
  const query = q.trim();
  return useQuery({
    queryKey: queryKeys.search(groupId, query),
    queryFn: () => api.search(query),
    enabled: groupId != null && query !== '',
  });
}
