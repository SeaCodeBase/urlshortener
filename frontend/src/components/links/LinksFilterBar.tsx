'use client'

import { useEffect, useRef, useState } from 'react'
import { format, parse } from 'date-fns'
import { Calendar, Search, SlidersHorizontal, X } from 'lucide-react'
import { DateRangeDialog, type DateRange } from '@/components/common/DateRangeDialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useTags } from '@/lib/hooks'
import { countActiveFilters, DEFAULT_LINK_FILTERS, FiltersDialog, type LinkFilters } from './FiltersDialog'
import type { LinksUrlState } from './linksUrlState'

export const API_DATE = 'yyyy-MM-dd'

export function toRange(from: string | null, to: string | null): DateRange | null {
  if (!from && !to) return null
  const f = parse(from ?? to!, API_DATE, new Date())
  const t = parse(to ?? from!, API_DATE, new Date())
  return { from: f, to: t }
}

export function rangeLabel(r: DateRange): string {
  return `${format(r.from, 'MMM d, yyyy')} – ${format(r.to, 'MMM d, yyyy')}`
}

interface Chip {
  key: string
  label: string
  remove: Partial<LinksUrlState>
}

function chipsFor(state: LinksUrlState): Chip[] {
  const f = state.filters
  const chips: Chip[] = []
  const withFilters = (patch: Partial<LinkFilters>) => ({ filters: { ...f, ...patch } })
  if (f.tags.length) chips.push({ key: 'tags', label: `Tags: ${f.tags.join(', ')}`, remove: withFilters({ tags: [] }) })
  if (f.linkType !== 'all')
    chips.push({
      key: 'type',
      label: f.linkType === 'custom' ? 'Custom back-halves' : 'No custom back-halves',
      remove: withFilters({ linkType: 'all' }),
    })
  if (f.expiration)
    chips.push({
      key: 'exp',
      label: { expired: 'Expired', expiring: 'Expiring', none: 'No expiration' }[f.expiration],
      remove: withFilters({ expiration: null }),
    })
  if (f.hasQr !== 'any')
    chips.push({
      key: 'qr',
      label: f.hasQr === 'with' ? 'With QR Code' : 'Without QR Code',
      remove: withFilters({ hasQr: 'any' }),
    })
  return chips
}

/** First toolbar row: search, created-date filter, "Add filters", and active-filter chips. */
export function LinksFilterBar({
  state,
  onChange,
}: {
  state: LinksUrlState
  onChange: (patch: Partial<LinksUrlState>) => void
}) {
  const [text, setText] = useState(state.q)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [dateOpen, setDateOpen] = useState(false)
  const [filtersOpen, setFiltersOpen] = useState(false)
  const { data: tags = [] } = useTags()

  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current)
  }, [])

  function onSearch(value: string) {
    setText(value)
    if (timer.current) clearTimeout(timer.current)
    timer.current = setTimeout(() => onChange({ q: value.trim() }), 300)
  }

  const range = toRange(state.from, state.to)
  const filterCount = countActiveFilters(state.filters)
  const chips = chipsFor(state)

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative w-full sm:w-[300px]">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            type="search"
            value={text}
            onChange={(e) => onSearch(e.target.value)}
            placeholder="Search links"
            aria-label="Search links"
            className="bg-white pl-9 dark:bg-card"
          />
        </div>

        {range ? (
          <div className="inline-flex h-10 items-center rounded border border-border bg-white dark:bg-card">
            <button
              type="button"
              onClick={() => setDateOpen(true)}
              className="inline-flex h-full items-center gap-2 pr-2 pl-3 text-base font-medium text-foreground"
            >
              <Calendar className="size-4" />
              {rangeLabel(range)}
            </button>
            <button
              type="button"
              aria-label="Clear created date filter"
              onClick={() => onChange({ from: null, to: null })}
              className="inline-flex h-full items-center pr-3 pl-1 text-muted-foreground hover:text-foreground"
            >
              <X className="size-4" />
            </button>
          </div>
        ) : (
          <Button type="button" variant="secondary" onClick={() => setDateOpen(true)}>
            <Calendar />
            Filter by created date
          </Button>
        )}

        <Button type="button" variant="secondary" onClick={() => setFiltersOpen(true)}>
          <SlidersHorizontal />
          Add filters
          {filterCount > 0 && (
            <span className="ml-1 inline-flex size-5 items-center justify-center rounded-full bg-primary text-xs text-primary-foreground">
              {filterCount}
            </span>
          )}
        </Button>
      </div>

      {chips.length > 0 && (
        <div className="flex flex-wrap items-center gap-2">
          {chips.map((c) => (
            <span
              key={c.key}
              className="inline-flex items-center gap-1 rounded-full border border-border bg-white px-3 py-1 text-sm text-foreground dark:bg-card"
            >
              {c.label}
              <button
                type="button"
                aria-label={`Remove filter ${c.label}`}
                onClick={() => onChange(c.remove)}
                className="text-muted-foreground hover:text-foreground"
              >
                <X className="size-3.5" />
              </button>
            </span>
          ))}
          <Button
            type="button"
            variant="link"
            size="sm"
            onClick={() => onChange({ filters: DEFAULT_LINK_FILTERS, from: null, to: null })}
          >
            Clear all filters
          </Button>
        </div>
      )}

      <DateRangeDialog
        open={dateOpen}
        onOpenChange={setDateOpen}
        value={range}
        onApply={(r) => onChange({ from: format(r.from, API_DATE), to: format(r.to, API_DATE) })}
        onClear={() => onChange({ from: null, to: null })}
      />
      <FiltersDialog
        open={filtersOpen}
        onOpenChange={setFiltersOpen}
        value={state.filters}
        tags={tags}
        onApply={(filters) => onChange({ filters })}
      />
    </div>
  )
}
