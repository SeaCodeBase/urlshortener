import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import type { DashboardDataQuery, DashboardPatch, DashboardSort } from '@/types';
import { queryKeys, useActiveGroupId } from './keys';

/** Dashboards of the active group. */
export function useDashboards(sort: DashboardSort = 'created') {
  const groupId = useActiveGroupId();
  return useQuery({
    queryKey: queryKeys.dashboards(groupId, sort),
    queryFn: () => api.listDashboards(sort),
    enabled: groupId != null,
  });
}

export function useDashboard(id: number | null | undefined) {
  return useQuery({
    queryKey: queryKeys.dashboard(id ?? 0),
    queryFn: () => api.getDashboard(id as number),
    enabled: id != null && id > 0,
  });
}

/** Module data of a dashboard over the window; keeps the previous data while a new window loads. */
export function useDashboardData(id: number | null | undefined, query: DashboardDataQuery = {}) {
  return useQuery({
    queryKey: queryKeys.dashboardData(id ?? 0, query),
    queryFn: () => api.getDashboardData(id as number, query),
    enabled: id != null && id > 0,
    placeholderData: keepPreviousData,
  });
}

/** Create / update / duplicate / remove; each invalidates the list and (when known) the dashboard and its data. */
export function useDashboardMutations() {
  const qc = useQueryClient();
  const invalidateList = () => qc.invalidateQueries({ queryKey: ['dashboards'] });

  const create = useMutation({
    mutationFn: (name: string) => api.createDashboard(name),
    onSuccess: invalidateList,
  });

  const update = useMutation({
    mutationFn: ({ id, patch }: { id: number; patch: DashboardPatch }) => api.updateDashboard(id, patch),
    onSuccess: (dashboard, { id }) => {
      qc.setQueryData(queryKeys.dashboard(id), dashboard);
      qc.invalidateQueries({ queryKey: queryKeys.dashboard(id) });
      invalidateList();
    },
  });

  const duplicate = useMutation({
    mutationFn: (id: number) => api.duplicateDashboard(id),
    onSuccess: invalidateList,
  });

  const remove = useMutation({
    mutationFn: (id: number) => api.deleteDashboard(id),
    onSuccess: (_data, id) => {
      qc.removeQueries({ queryKey: queryKeys.dashboard(id) });
      invalidateList();
    },
  });

  return { create, update, duplicate, remove };
}
