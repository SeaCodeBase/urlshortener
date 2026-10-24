import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import type { BulkLinksInput, CreateLinkInput, LinkListParams } from '@/types';
import { queryKeys, useActiveGroupId } from './keys';

export type UseLinksParams = Omit<LinkListParams, 'page'>;

/** Paged link list of the active group; `fetchNextPage` loads the next page. */
export function useLinks(params: UseLinksParams = {}) {
  const groupId = useActiveGroupId();
  return useInfiniteQuery({
    queryKey: queryKeys.links(groupId, params),
    queryFn: ({ pageParam }) => api.getLinks({ ...params, page: pageParam }),
    initialPageParam: 1,
    getNextPageParam: (last) => (last.page < last.total_pages ? last.page + 1 : undefined),
    enabled: groupId != null,
  });
}

export function useCreateLink() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateLinkInput) => api.createLink(input),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['links'] });
      qc.invalidateQueries({ queryKey: ['tags'] });
    },
  });
}

/** Rejects with ApiError status 409 when the link can't be deleted. */
export function useDeleteLink() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api.deleteLink(id),
    onSuccess: (_data, id) => {
      qc.invalidateQueries({ queryKey: ['links'] });
      qc.removeQueries({ queryKey: queryKeys.link(id) });
      qc.invalidateQueries({ queryKey: ['qr-codes'] });
      qc.invalidateQueries({ queryKey: ['qr-code'] });
    },
  });
}

export function useBulkLinks() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: BulkLinksInput) => api.bulkLinks(input),
    onSuccess: (_data, input) => {
      qc.invalidateQueries({ queryKey: ['links'] });
      for (const id of input.ids) qc.invalidateQueries({ queryKey: queryKeys.link(id) });
      if (input.action === 'edit_tags') qc.invalidateQueries({ queryKey: ['tags'] });
    },
  });
}
