import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import type { CreateDomainInput, UpdateDomainInput } from '@/types';
import { queryKeys } from './keys';

export function useDomains() {
  return useQuery({
    queryKey: queryKeys.domains,
    queryFn: async () => (await api.getDomains()).domains,
  });
}

function useInvalidateDomains() {
  const qc = useQueryClient();
  return () => qc.invalidateQueries({ queryKey: queryKeys.domains });
}

export function useCreateDomain() {
  const onSuccess = useInvalidateDomains();
  return useMutation({
    mutationFn: (input: CreateDomainInput) => api.createDomain(input),
    onSuccess,
  });
}

export function useVerifyDomain() {
  const onSuccess = useInvalidateDomains();
  return useMutation({
    mutationFn: (id: number) => api.verifyDomain(id),
    onSuccess,
  });
}

export function useUpdateDomain() {
  const onSuccess = useInvalidateDomains();
  return useMutation({
    mutationFn: ({ id, data }: { id: number; data: UpdateDomainInput }) => api.updateDomain(id, data),
    onSuccess,
  });
}

export function useDeleteDomain() {
  const onSuccess = useInvalidateDomains();
  return useMutation({
    mutationFn: (id: number) => api.deleteDomain(id),
    onSuccess,
  });
}
