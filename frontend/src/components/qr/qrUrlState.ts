import type { QrFilters } from '@/types'
import type { LinksShow } from '@/components/links/linksUrlState'

/** Filters of the QR Codes "Filters" dialog. */
export interface QrFilterValues {
  tags: string[]
  expiration: 'expired' | 'expiring' | 'none' | null
  hasLink: 'any' | 'with' | 'without'
}

export const DEFAULT_QR_FILTERS: QrFilterValues = { tags: [], expiration: null, hasLink: 'any' }

/** QR Codes list state as it lives in the URL (same shape as the links list). */
export interface QrUrlState {
  q: string
  show: LinksShow
  filters: QrFilterValues
  /** YYYY-MM-DD */
  from: string | null
  /** YYYY-MM-DD */
  to: string | null
}

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/

function oneOf<T extends string>(value: string | null, allowed: readonly T[], fallback: T): T {
  return allowed.includes(value as T) ? (value as T) : fallback
}

export function parseQrUrlState(sp: Pick<URLSearchParams, 'get'>): QrUrlState {
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
      expiration: exp === 'expired' || exp === 'expiring' || exp === 'none' ? exp : null,
      hasLink: oneOf(sp.get('link'), ['any', 'with', 'without'] as const, 'any'),
    },
    from: from && DATE_RE.test(from) ? from : null,
    to: to && DATE_RE.test(to) ? to : null,
  }
}

/** Serialises the state, omitting defaults so URLs stay short. */
export function qrUrlStateToSearch(s: QrUrlState): string {
  const p = new URLSearchParams()
  if (s.q) p.set('q', s.q)
  if (s.show !== 'active') p.set('show', s.show)
  if (s.filters.tags.length) p.set('tags', s.filters.tags.join(','))
  if (s.filters.expiration) p.set('exp', s.filters.expiration)
  if (s.filters.hasLink !== DEFAULT_QR_FILTERS.hasLink) p.set('link', s.filters.hasLink)
  if (s.from) p.set('from', s.from)
  if (s.to) p.set('to', s.to)
  return p.toString()
}

const HAS_LINK = { any: 'both', with: 'on', without: 'off' } as const

/** Maps URL state to backend list/export params. */
export function toQrApiParams(s: QrUrlState): QrFilters {
  const params: QrFilters = {
    archived: s.show === 'hidden' ? 'on' : 'off',
    has_link: HAS_LINK[s.filters.hasLink],
  }
  if (s.q) params.q = s.q
  if (s.filters.tags.length) params.tags = s.filters.tags
  if (s.filters.expiration) params.expiration = s.filters.expiration
  if (s.from) params.created_after = s.from
  if (s.to) params.created_before = s.to
  return params
}

/** Number of filters that differ from the defaults (for the "Add filters" badge). */
export function countActiveQrFilters(f: QrFilterValues): number {
  return (f.tags.length ? 1 : 0) + (f.expiration != null ? 1 : 0) + (f.hasLink !== 'any' ? 1 : 0)
}

/** True when anything narrows the list beyond the Active/Hidden view. */
export function hasActiveQrQuery(s: QrUrlState): boolean {
  return !!(s.q || s.from || s.to || countActiveQrFilters(s.filters))
}
