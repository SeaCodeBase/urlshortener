'use client'

import { useEffect, useRef, useState } from 'react'
import { Search } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { ApiError } from '@/lib/api'
import { formatShortUrl } from '@/lib/format'
import { useLinks, useQrCodes } from '@/lib/hooks'
import { cn } from '@/lib/utils'
import type { DashboardFilter } from '@/types'

/** Most ids a dashboard filter may hold (backend limit). */
export const MAX_FILTER_IDS = 200
const PAGE_SIZE = 50

type FilterType = DashboardFilter['type']
type PickType = Exclude<FilterType, 'all'>

const TYPES: { value: FilterType; label: string }[] = [
  { value: 'links', label: 'Links' },
  { value: 'qr', label: 'QR Codes' },
  { value: 'all', label: 'All' },
]

export interface DashboardFilterDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  value: DashboardFilter
  /** Dialog closes when it resolves; a rejection is toasted and the dialog stays open. */
  onApply: (filter: DashboardFilter) => Promise<void>
}

/** "Filter" dialog of a dashboard: Links | QR Codes | All, searchable checklist of up to 200 items. */
export function DashboardFilterDialog({ open, onOpenChange, value, onApply }: DashboardFilterDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[calc(100%-2rem)] sm:max-w-[560px]">
        <DialogHeader>
          <DialogTitle className="text-xl font-semibold">Filter</DialogTitle>
          <DialogDescription className="sr-only">Choose the links or QR Codes this dashboard shows</DialogDescription>
        </DialogHeader>
        {/* Mounted only while open, so the draft resets from `value` each time. */}
        {open && <FilterBody value={value} onCancel={() => onOpenChange(false)} onApply={onApply} onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function FilterBody({
  value,
  onCancel,
  onApply,
  onDone,
}: {
  value: DashboardFilter
  onCancel: () => void
  onApply: (filter: DashboardFilter) => Promise<void>
  onDone: () => void
}) {
  const [type, setType] = useState<FilterType>(value.type)
  const [selected, setSelected] = useState<Record<PickType, number[]>>({
    // Deleted links/QR codes are pruned by the API, which omits an empty ids array.
    links: value.type === 'links' ? (value.ids ?? []) : [],
    qr: value.type === 'qr' ? (value.ids ?? []) : [],
  })
  const [saving, setSaving] = useState(false)

  const ids = type === 'all' ? [] : selected[type]
  const canApply = !saving && (type === 'all' || ids.length > 0)

  function toggle(id: number, checked: boolean) {
    if (type === 'all') return
    setSelected((s) => {
      const cur = s[type]
      if (checked) return cur.includes(id) || cur.length >= MAX_FILTER_IDS ? s : { ...s, [type]: [...cur, id] }
      return { ...s, [type]: cur.filter((x) => x !== id) }
    })
  }

  async function apply() {
    const filter: DashboardFilter = type === 'all' ? { type: 'all' } : { type, ids }
    setSaving(true)
    try {
      await onApply(filter)
      onDone()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Couldn't update the filter")
    } finally {
      setSaving(false)
    }
  }

  return (
    <>
      <div role="radiogroup" aria-label="Type" className="inline-flex w-fit rounded-lg bg-muted p-1">
        {TYPES.map((t) => (
          <button
            key={t.value}
            type="button"
            role="radio"
            aria-checked={type === t.value}
            onClick={() => setType(t.value)}
            className={cn(
              'rounded-md px-4 py-1.5 text-sm font-medium transition-colors',
              type === t.value ? 'bg-white text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground',
            )}
          >
            {t.label}
          </button>
        ))}
      </div>

      {type === 'all' ? (
        <p className="py-6 text-sm text-muted-foreground">Showing data for all links and QR Codes</p>
      ) : (
        <ItemPicker key={type} type={type} selected={ids} onToggle={toggle} />
      )}

      <DialogFooter className="items-center sm:justify-between">
        <span className="text-sm text-muted-foreground">
          {type === 'all' ? '' : `${ids.length}/${MAX_FILTER_IDS} selected`}
        </span>
        <div className="flex gap-2">
          <Button type="button" variant="secondary" onClick={onCancel}>
            Cancel
          </Button>
          <Button type="button" disabled={!canApply} onClick={apply}>
            Apply filters
          </Button>
        </div>
      </DialogFooter>
    </>
  )
}

interface PickRow {
  id: number
  title: string
  shortUrl: string
}

interface PickerProps {
  selected: number[]
  onToggle: (id: number, checked: boolean) => void
}

/** Debounced search text (300 ms, like the list filter bars). */
function useDebounced(value: string): string {
  const [debounced, setDebounced] = useState(value)
  const first = useRef(true)
  useEffect(() => {
    if (first.current) {
      first.current = false
      return
    }
    const t = setTimeout(() => setDebounced(value.trim()), 300)
    return () => clearTimeout(t)
  }, [value])
  return debounced
}

function ItemPicker({ type, ...props }: PickerProps & { type: PickType }) {
  const [q, setQ] = useState('')
  const debounced = useDebounced(q)
  const placeholder = type === 'links' ? 'Search links' : 'Search QR Codes'
  return (
    <div className="space-y-2">
      <div className="relative">
        <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden />
        <Input value={q} onChange={(e) => setQ(e.target.value)} placeholder={placeholder} aria-label={placeholder} className="pl-9" />
      </div>
      {type === 'links' ? <LinkList q={debounced} {...props} /> : <QrList q={debounced} {...props} />}
    </div>
  )
}

function LinkList({ q, ...props }: PickerProps & { q: string }) {
  const res = useLinks({ q: q || undefined, limit: PAGE_SIZE })
  const rows = (res.data?.pages ?? []).flatMap((p) =>
    p.links.map((l) => ({ id: l.id, title: l.title || formatShortUrl(l.short_url), shortUrl: l.short_url })),
  )
  return <Checklist rows={rows} {...props} {...res} empty="No links found" />
}

function QrList({ q, ...props }: PickerProps & { q: string }) {
  const res = useQrCodes({ q: q || undefined, limit: PAGE_SIZE })
  const rows = (res.data?.pages ?? []).flatMap((p) =>
    p.qr_codes.map((c) => ({ id: c.id, title: c.title || formatShortUrl(c.short_url), shortUrl: c.short_url })),
  )
  return <Checklist rows={rows} {...props} {...res} empty="No QR Codes found" />
}

function Checklist({
  rows,
  selected,
  onToggle,
  isLoading,
  hasNextPage,
  isFetchingNextPage,
  fetchNextPage,
  empty,
}: PickerProps & {
  rows: PickRow[]
  isLoading: boolean
  hasNextPage: boolean
  isFetchingNextPage: boolean
  fetchNextPage: () => unknown
  empty: string
}) {
  const full = selected.length >= MAX_FILTER_IDS
  return (
    <div className="max-h-80 overflow-y-auto rounded-lg border border-border">
      {isLoading ? (
        <p className="p-4 text-sm text-muted-foreground">Loading…</p>
      ) : rows.length === 0 ? (
        <p className="p-4 text-sm text-muted-foreground">{empty}</p>
      ) : (
        <ul className="divide-y divide-border">
          {rows.map((r) => {
            const checked = selected.includes(r.id)
            const id = `filter-item-${r.id}`
            return (
              <li key={r.id} className="flex items-center gap-3 px-4 py-2.5">
                <Checkbox
                  id={id}
                  aria-label={r.title}
                  aria-describedby={`${id}-url`}
                  checked={checked}
                  disabled={!checked && full}
                  onCheckedChange={(c) => onToggle(r.id, c === true)}
                />
                <label htmlFor={id} className="min-w-0 flex-1 cursor-pointer">
                  <span className="block truncate text-sm font-medium text-foreground">{r.title}</span>
                  <span id={`${id}-url`} className="block truncate text-xs text-muted-foreground">
                    {formatShortUrl(r.shortUrl)}
                  </span>
                </label>
              </li>
            )
          })}
        </ul>
      )}
      {hasNextPage && (
        <div className="border-t border-border p-2 text-center">
          <Button type="button" variant="ghost" size="sm" disabled={isFetchingNextPage} onClick={() => fetchNextPage()}>
            {isFetchingNextPage ? 'Loading…' : 'Load more'}
          </Button>
        </div>
      )}
    </div>
  )
}
