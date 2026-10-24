import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { useAuthStore } from '@/stores/auth';
import type { UpdateGroupInput } from '@/types';
import { queryKeys } from './keys';

export function useGroups() {
  return useQuery({
    queryKey: queryKeys.groups,
    queryFn: async () => (await api.getGroups()).groups,
  });
}

/** Refreshes both the query cache and the auth store's group list. */
function useGroupsChanged() {
  const qc = useQueryClient();
  return () => {
    qc.invalidateQueries({ queryKey: queryKeys.groups });
    useAuthStore.getState().loadGroups().catch(() => {});
  };
}

export function useCreateGroup() {
  const changed = useGroupsChanged();
  return useMutation({
    mutationFn: (name: string) => api.createGroup(name),
    onSuccess: changed,
  });
}

export function useUpdateGroup() {
  const changed = useGroupsChanged();
  return useMutation({
    mutationFn: ({ id, data }: { id: number; data: UpdateGroupInput }) => api.updateGroup(id, data),
    onSuccess: changed,
  });
}
