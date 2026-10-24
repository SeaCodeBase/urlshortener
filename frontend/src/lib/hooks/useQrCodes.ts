import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import type { BulkQrInput, CreateQrInput, QrListParams } from '@/types';
import { queryKeys, useActiveGroupId } from './keys';

/** List params; the caller picks the page size with `limit`. */
export type UseQrCodesParams = Omit<QrListParams, 'page'>;

/** Paged QR code list of the active group; `fetchNextPage` loads the next page. */
export function useQrCodes(params: UseQrCodesParams = {}) {
  const groupId = useActiveGroupId();
  return useInfiniteQuery({
    queryKey: queryKeys.qrCodes(groupId, params),
    queryFn: ({ pageParam }) => api.getQrCodes({ ...params, page: pageParam }),
    initialPageParam: 1,
    getNextPageParam: (last) => (last.page < last.total_pages ? last.page + 1 : undefined),
    enabled: groupId != null,
  });
}

/** Creates a QR code (and its link, unless `link_id` attaches an existing one). */
export function useCreateQrCode() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateQrInput) => api.createQrCode(input),
    onSuccess: (qr) => {
      qc.invalidateQueries({ queryKey: ['qr-codes'] });
      qc.invalidateQueries({ queryKey: ['links'] });
      qc.invalidateQueries({ queryKey: queryKeys.link(qr.link_id) });
      qc.invalidateQueries({ queryKey: ['tags'] });
      qc.invalidateQueries({ queryKey: ['search'] });
    },
  });
}

export function useDeleteQrCode() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api.deleteQrCode(id),
    onSuccess: (_data, id) => {
      qc.invalidateQueries({ queryKey: ['qr-codes'] });
      qc.invalidateQueries({ queryKey: ['links'] });
      qc.invalidateQueries({ queryKey: ['link'] });
      qc.invalidateQueries({ queryKey: ['search'] });
      qc.removeQueries({ queryKey: queryKeys.qrCode(id) });
    },
  });
}

export function useBulkQrCodes() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: BulkQrInput) => api.bulkQrCodes(input),
    onSuccess: (_data, input) => {
      qc.invalidateQueries({ queryKey: ['qr-codes'] });
      for (const id of input.ids) qc.invalidateQueries({ queryKey: queryKeys.qrCode(id) });
      if (input.action === 'edit_tags') qc.invalidateQueries({ queryKey: ['tags'] });
    },
  });
}
