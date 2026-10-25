import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import type { BulkUpload } from '@/types';
import { queryKeys, useActiveGroupId } from './keys';

const BULK_POLL_MS = 3000;

/** 3 s while any job is queued or processing, else no polling. */
export function bulkPollInterval(uploads: BulkUpload[] | undefined): number | false {
  return uploads?.some((u) => u.status === 'queued' || u.status === 'processing') ? BULK_POLL_MS : false;
}

/** Bulk upload jobs of the active group, newest first; polls while a job is running. */
export function useBulkUploads() {
  const groupId = useActiveGroupId();
  return useQuery({
    queryKey: queryKeys.bulkUploads(groupId),
    queryFn: async () => (await api.listBulkUploads()).uploads,
    enabled: groupId != null,
    refetchInterval: (query) => bulkPollInterval(query.state.data),
  });
}

/** Queues a CSV; the list refetches (and starts polling) on success. */
export function useUploadBulkCsv() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (file: File) => api.uploadBulkCsv(file),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['bulk-uploads'] }),
  });
}
