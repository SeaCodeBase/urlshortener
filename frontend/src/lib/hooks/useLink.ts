import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import type { EditBackHalfInput, UpdateLinkInput } from '@/types';
import { queryKeys } from './keys';

/** One link with routing_rules and history. */
export function useLink(id: number | null | undefined) {
  return useQuery({
    queryKey: queryKeys.link(id ?? 0),
    queryFn: () => api.getLink(id as number),
    enabled: id != null && id > 0,
  });
}

export function useUpdateLink() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: number; data: UpdateLinkInput }) => api.updateLink(id, data),
    onSuccess: (link, { id, data }) => {
      qc.setQueryData(queryKeys.link(id), link);
      qc.invalidateQueries({ queryKey: ['links'] });
      qc.invalidateQueries({ queryKey: queryKeys.link(id) });
      if (data.tags) qc.invalidateQueries({ queryKey: ['tags'] });
    },
  });
}

/** Creates a new link with the back-half; the old link stays. */
export function useEditBackHalf() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, ...input }: EditBackHalfInput & { id: number }) => api.editBackHalf(id, input),
    onSuccess: (_link, { id }) => {
      qc.invalidateQueries({ queryKey: ['links'] });
      qc.invalidateQueries({ queryKey: queryKeys.link(id) });
    },
  });
}

export function useSetLinkTags() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, tags }: { id: number; tags: string[] }) => api.setLinkTags(id, tags),
    onSuccess: (_res, { id }) => {
      qc.invalidateQueries({ queryKey: ['links'] });
      qc.invalidateQueries({ queryKey: queryKeys.link(id) });
      qc.invalidateQueries({ queryKey: ['tags'] });
    },
  });
}
