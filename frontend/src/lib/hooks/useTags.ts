import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import type { TagType } from '@/types';
import { queryKeys, useActiveGroupId } from './keys';

/** Tag names used in the active group. */
export function useTags(type: TagType = 'link') {
  const groupId = useActiveGroupId();
  return useQuery({
    queryKey: queryKeys.tags(groupId, type),
    queryFn: async () => (await api.getTags(type)).tags,
    enabled: groupId != null,
  });
}
