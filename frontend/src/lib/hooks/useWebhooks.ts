import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import type { WebhookInput } from '@/types';
import { queryKeys, useActiveGroupId } from './keys';

/** Webhooks of the active group (secrets included). */
export function useWebhooks() {
  const groupId = useActiveGroupId();
  return useQuery({
    queryKey: queryKeys.webhooks(groupId),
    queryFn: async () => (await api.listWebhooks()).webhooks,
    enabled: groupId != null,
  });
}

/** The webhook's last 20 deliveries; fetched only while `enabled` (e.g. its panel is open). */
export function useWebhookDeliveries(id: number | null | undefined, enabled = true) {
  return useQuery({
    queryKey: queryKeys.webhookDeliveries(id ?? 0),
    queryFn: async () => (await api.getWebhookDeliveries(id as number)).deliveries,
    enabled: enabled && id != null && id > 0,
  });
}

/** Webhook mutations; each invalidates the list, and those touching one webhook its deliveries. */
export function useWebhookMutations() {
  const qc = useQueryClient();
  const invalidate = (id?: number) => {
    qc.invalidateQueries({ queryKey: ['webhooks'] });
    if (id != null) qc.invalidateQueries({ queryKey: queryKeys.webhookDeliveries(id) });
  };

  const create = useMutation({
    mutationFn: (input: WebhookInput) => api.createWebhook(input),
    onSuccess: () => invalidate(),
  });

  const update = useMutation({
    mutationFn: ({ id, input }: { id: number; input: WebhookInput }) => api.updateWebhook(id, input),
    onSuccess: (_w, { id }) => invalidate(id),
  });

  const remove = useMutation({
    mutationFn: (id: number) => api.deleteWebhook(id),
    onSuccess: (_data, id) => {
      qc.removeQueries({ queryKey: queryKeys.webhookDeliveries(id) });
      invalidate();
    },
  });

  const verify = useMutation({
    mutationFn: (id: number) => api.verifyWebhook(id),
    onSuccess: (_res, id) => invalidate(id),
  });

  const deactivate = useMutation({
    mutationFn: (id: number) => api.deactivateWebhook(id),
    onSuccess: (_w, id) => invalidate(id),
  });

  const activate = useMutation({
    mutationFn: (id: number) => api.activateWebhook(id),
    onSuccess: (_w, id) => invalidate(id),
  });

  return { create, update, remove, verify, deactivate, activate };
}
