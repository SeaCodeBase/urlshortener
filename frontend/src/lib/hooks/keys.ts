import type { ActivityParams, LinkListParams, LinkStatsQuery, QrListParams, TagType, TodayRange } from '@/types';
import { useAuthStore } from '@/stores/auth';

/**
 * Query keys. Group-scoped data carries the active group id so switching
 * groups never shows another group's cache. Prefixes (`['links']`,
 * `['link', id]`, …) are what mutations invalidate.
 */
export const queryKeys = {
  links: (groupId: number | null, params: LinkListParams) => ['links', groupId, params] as const,
  link: (id: number) => ['link', id] as const,
  routingRules: (id: number) => ['link', id, 'routing-rules'] as const,
  linkStats: (id: number, query: LinkStatsQuery) => ['link', id, 'stats', query] as const,
  qrCodes: (groupId: number | null, params: QrListParams) => ['qr-codes', groupId, params] as const,
  qrCode: (id: number) => ['qr-code', id] as const,
  qrStats: (id: number, query: LinkStatsQuery) => ['qr-code', id, 'stats', query] as const,
  tags: (groupId: number | null, type: TagType) => ['tags', groupId, type] as const,
  groups: ['groups'] as const,
  domains: ['domains'] as const,
  tokens: ['tokens'] as const,
  me: ['me'] as const,
  activity: (groupId: number | null, params: ActivityParams) => ['activity', groupId, params] as const,
  search: (groupId: number | null, q: string) => ['search', groupId, q] as const,
  today: (groupId: number | null, range: TodayRange) => ['analytics', 'today', groupId, range] as const,
  meta: ['meta'] as const,
};

/** The active group id (null until the auth store has loaded groups). */
export function useActiveGroupId(): number | null {
  return useAuthStore((s) => s.activeGroupId);
}
