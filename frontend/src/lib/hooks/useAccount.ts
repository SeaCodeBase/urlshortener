import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { useAuthStore } from '@/stores/auth';
import type { UpdateEmailInput, UpdateMeInput, User } from '@/types';
import { queryKeys } from './keys';

function useUserChanged() {
  const qc = useQueryClient();
  return (user: User) => {
    useAuthStore.setState({ user });
    qc.invalidateQueries({ queryKey: queryKeys.me });
  };
}

export function useUpdateMe() {
  const onSuccess = useUserChanged();
  return useMutation({
    mutationFn: (input: UpdateMeInput) => api.updateMe(input),
    onSuccess,
  });
}

export function useUpdateEmail() {
  const onSuccess = useUserChanged();
  return useMutation({
    mutationFn: (input: UpdateEmailInput) => api.updateEmail(input),
    onSuccess,
  });
}
