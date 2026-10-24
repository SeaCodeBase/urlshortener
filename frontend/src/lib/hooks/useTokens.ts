import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import type { CreateTokenInput } from '@/types';
import { queryKeys } from './keys';

export function useTokens() {
  return useQuery({
    queryKey: queryKeys.tokens,
    queryFn: async () => (await api.getTokens()).tokens,
  });
}

/** The response's `token` is the raw token, shown only once. */
export function useCreateToken() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateTokenInput) => api.createToken(input),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.tokens }),
  });
}

export function useRevokeToken() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api.revokeToken(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.tokens }),
  });
}
