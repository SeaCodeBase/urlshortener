import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import type { UpdateQrInput } from '@/types';
import { queryKeys } from './keys';

/** One QR code with its link (incl. routing_rules). */
export function useQrCode(id: number | null | undefined) {
  return useQuery({
    queryKey: queryKeys.qrCode(id ?? 0),
    queryFn: () => api.getQrCode(id as number),
    enabled: id != null && id > 0,
  });
}

export function useUpdateQrCode() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: number; data: UpdateQrInput }) => api.updateQrCode(id, data),
    onSuccess: (qr, { id, data }) => {
      qc.setQueryData(queryKeys.qrCode(id), qr);
      qc.invalidateQueries({ queryKey: ['qr-codes'] });
      qc.invalidateQueries({ queryKey: queryKeys.qrCode(id) });
      // The destination, expiration and routing live on the QR code's link.
      qc.invalidateQueries({ queryKey: ['links'] });
      qc.invalidateQueries({ queryKey: queryKeys.link(qr.link_id) });
      if (data.tags) qc.invalidateQueries({ queryKey: ['tags'] });
    },
  });
}

export function useSetQrTags() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, tags }: { id: number; tags: string[] }) => api.setQrTags(id, tags),
    onSuccess: (_res, { id }) => {
      qc.invalidateQueries({ queryKey: ['qr-codes'] });
      qc.invalidateQueries({ queryKey: queryKeys.qrCode(id) });
      qc.invalidateQueries({ queryKey: ['tags'] });
    },
  });
}
