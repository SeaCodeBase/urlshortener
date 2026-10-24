import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import type { RoutingRuleInput } from '@/types';
import { queryKeys } from './keys';

/** A link's dynamic-routing rules, in priority order. */
export function useRoutingRules(id: number | null | undefined) {
  return useQuery({
    queryKey: queryKeys.routingRules(id ?? 0),
    queryFn: () => api.getRoutingRules(id as number).then((r) => r.routing_rules),
    enabled: id != null && id > 0,
  });
}

export function useSetRoutingRules() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, rules }: { id: number; rules: RoutingRuleInput[] }) =>
      api.setRoutingRules(id, rules).then((r) => r.routing_rules),
    onSuccess: (rules, { id }) => {
      qc.setQueryData(queryKeys.routingRules(id), rules);
      // The link payload embeds routing_rules too.
      qc.invalidateQueries({ queryKey: queryKeys.link(id) });
    },
  });
}
