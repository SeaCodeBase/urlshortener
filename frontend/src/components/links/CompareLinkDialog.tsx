'use client'

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { useLinks } from '@/lib/hooks'
import { useDebouncedValue } from '@/lib/hooks/useDebouncedValue'

const PAGE_SIZE = 50
const SEARCH_DEBOUNCE_MS = 300

export interface CompareLinkDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The link whose analytics are shown; it is left out of the list. */
  currentId: number
  onCompare: (id: number) => void
}

/** "Compare to a link" (spec §5.4): pick another link of the group to chart next to this one. */
export function CompareLinkDialog({ open, onOpenChange, currentId, onCompare }: CompareLinkDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle className="text-xl font-semibold">Compare to a link</DialogTitle>
          <DialogDescription>Chart another link&apos;s engagements next to this one.</DialogDescription>
        </DialogHeader>
        {/* Mounted only while open so search and choice start empty each time. */}
        {open && <CompareLinkBody currentId={currentId} onCancel={() => onOpenChange(false)} onCompare={onCompare} />}
      </DialogContent>
    </Dialog>
  )
}

function CompareLinkBody({
  currentId,
  onCancel,
  onCompare,
}: {
  currentId: number
  onCancel: () => void
  onCompare: (id: number) => void
}) {
  const [search, setSearch] = useState('')
  const q = useDebouncedValue(search.trim(), SEARCH_DEBOUNCE_MS)
  const [choice, setChoice] = useState('')

  const { data, isLoading, hasNextPage, fetchNextPage, isFetchingNextPage } = useLinks({
    ...(q ? { q } : {}),
    limit: PAGE_SIZE,
  })
  const links = (data?.pages ?? []).flatMap((p) => p.links).filter((l) => l.id !== currentId)

  return (
    <>
      <Input
        type="search"
        aria-label="Search links"
        placeholder="Search links"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
      />
      <div className="max-h-80 overflow-y-auto">
        {isLoading ? (
          <p className="py-6 text-center text-sm text-muted-foreground">Loading links…</p>
        ) : links.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">No links found.</p>
        ) : (
          <RadioGroup value={choice} onValueChange={setChoice} aria-label="Links" className="gap-1">
            {links.map((l) => (
              <div key={l.id} className="flex items-center gap-3 rounded-md px-2 py-2 hover:bg-muted">
                <RadioGroupItem id={`compare-link-${l.id}`} value={String(l.id)} />
                <Label htmlFor={`compare-link-${l.id}`} className="flex min-w-0 flex-1 flex-col items-start gap-0.5 font-normal">
                  <span className="w-full truncate font-medium text-foreground">{l.title || l.short_url}</span>
                  <span className="w-full truncate text-xs text-muted-foreground">{l.short_url}</span>
                </Label>
              </div>
            ))}
          </RadioGroup>
        )}
        {hasNextPage && (
          <Button
            type="button"
            variant="ghost"
            className="mt-2 w-full"
            disabled={isFetchingNextPage}
            onClick={() => fetchNextPage()}
          >
            Load more
          </Button>
        )}
      </div>
      <DialogFooter>
        <Button type="button" variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
        <Button type="button" disabled={!choice} onClick={() => onCompare(Number(choice))}>
          Compare
        </Button>
      </DialogFooter>
    </>
  )
}
