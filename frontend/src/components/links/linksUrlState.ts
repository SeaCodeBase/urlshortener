import type { LinkFilters as ApiLinkFilters } from '@/types'
import { DEFAULT_LINK_FILTERS, type LinkFilters } from './FiltersDialog'

export type LinksShow = 'active' | 'hidden'
export type LinksView = 'compact' | 'default' | 'grid'

/** Links-list state as it lives in the URL (spec §5.2). */
export interface LinksUrlState {
  q: string
  show: LinksShow
  filters: LinkFilters
  /** YYYY-MM-DD */
  from: string | null
  /** YYYY-MM-DD */
  to: string | null
}

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/

function oneOf<T extends string>(value: string | null, allowed: readonly T[], fallback: T): T {
  return allowed.includes(value as T) ? (value as T) : fallback
}

export function parseLinksUrlState(sp: Pick<URLSearchParams, 'get'>): LinksUrlState {
  const exp = sp.get('exp')
  const from = sp.get('from')
  const to = sp.get('to')
  return {
    q: sp.get('q') ?? '',
    show: oneOf(sp.get('show'), ['active', 'hidden'] as const, 'active'),
    filters: {
      tags: (sp.get('tags') ?? '')
        .split(',')
        .map((t) => t.trim())
        .filter(Boolean),
      linkType: oneOf(sp.get('type'), ['all', 'custom', 'nocustom'] as const, 'all'),
      expiration: exp === 'expired' || exp === 'expiring' || exp === 'none' ? exp : null,
      hasQr: oneOf(sp.get('qr'), ['any', 'with', 'without'] as const, 'any'),
    },
    from: from && DATE_RE.test(from) ? from : null,
    to: to && DATE_RE.test(to) ? to : null,
  }
}

/** Serialises the state, omitting defaults so URLs stay short. */
export function linksUrlStateToSearch(s: LinksUrlState): string {
  const p = new URLSearchParams()
  if (s.q) p.set('q', s.q)
  if (s.show !== 'active') p.set('show', s.show)
  if (s.filters.tags.length) p.set('tags', s.filters.tags.join(','))
  if (s.filters.linkType !== DEFAULT_LINK_FILTERS.linkType) p.set('type', s.filters.linkType)
  if (s.filters.expiration) p.set('exp', s.filters.expiration)
  if (s.filters.hasQr !== DEFAULT_LINK_FILTERS.hasQr) p.set('qr', s.filters.hasQr)
  if (s.from) p.set('from', s.from)
  if (s.to) p.set('to', s.to)
  return p.toString()
}

const BACK_HALF = { all: 'both', custom: 'on', nocustom: 'off' } as const
const QR = { any: 'both', with: 'on', without: 'off' } as const

/** Maps URL state to backend list/export params. */
export function toApiParams(s: LinksUrlState): ApiLinkFilters {
  const params: ApiLinkFilters = {
    archived: s.show === 'hidden' ? 'on' : 'off',
    custom_back_half: BACK_HALF[s.filters.linkType],
    has_qr: QR[s.filters.hasQr],
  }
  if (s.q) params.q = s.q
  if (s.filters.tags.length) params.tags = s.filters.tags
  if (s.filters.expiration) params.expiration = s.filters.expiration
  if (s.from) params.created_after = s.from
  if (s.to) params.created_before = s.to
  return params
}

/** True when anything narrows the list beyond the Active/Hidden view. */
export function hasActiveQuery(s: LinksUrlState): boolean {
  const f = s.filters
  return !!(s.q || s.from || s.to || f.tags.length || f.linkType !== 'all' || f.expiration || f.hasQr !== 'any')
}
