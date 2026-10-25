import { useEffect, useRef } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import type { BulkUpload, BulkUploadStatus } from '@/types';
import { queryKeys, useActiveGroupId } from './keys';

const BULK_POLL_MS = 3000;

/** 3 s while any job is queued or processing, else no polling. */
export function bulkPollInterval(uploads: BulkUpload[] | undefined): number | false {
  return uploads?.some((u) => u.status === 'queued' || u.status === 'processing') ? BULK_POLL_MS : false;
}

const isRunning = (s: BulkUploadStatus | undefined) => s === 'queued' || s === 'processing';
const isFinished = (s: BulkUploadStatus) => s === 'done' || s === 'failed';

/**
 * True when a job seen running in `prev` (or one that appeared after the first load) is now done or failed.
 * `prev` is null before the first load.
 */
export function bulkJobFinished(prev: Map<number, BulkUploadStatus> | null, uploads: BulkUpload[]): boolean {
  if (prev == null) return false;
  return uploads.some((u) => isFinished(u.status) && (isRunning(prev.get(u.id)) || !prev.has(u.id)));
}

/**
 * Bulk upload jobs of the active group, newest first; polls while a job is running. When polling sees a job
 * finish, the links lists are refetched so the new links show up.
 */
export function useBulkUploads() {
  const groupId = useActiveGroupId();
  const qc = useQueryClient();
  const query = useQuery({
    queryKey: queryKeys.bulkUploads(groupId),
    queryFn: async () => (await api.listBulkUploads()).uploads,
    enabled: groupId != null,
    refetchInterval: (query) => bulkPollInterval(query.state.data),
  });
  const seen = useRef<{ groupId: number | null; statuses: Map<number, BulkUploadStatus> } | null>(null);
  const data = query.data;
  useEffect(() => {
    if (!data) return;
    const prev = seen.current?.groupId === groupId ? seen.current.statuses : null;
    if (bulkJobFinished(prev, data)) void qc.invalidateQueries({ queryKey: ['links'] });
    seen.current = { groupId, statuses: new Map(data.map((u) => [u.id, u.status])) };
  }, [data, groupId, qc]);
  return query;
}

/** Queues a CSV; the list refetches (and starts polling) on success. */
export function useUploadBulkCsv() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (file: File) => api.uploadBulkCsv(file),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['bulk-uploads'] }),
  });
}
