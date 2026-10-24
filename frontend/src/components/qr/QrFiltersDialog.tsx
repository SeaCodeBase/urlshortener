'use client'

import { useState } from 'react'
import { TagPicker } from '@/components/common/TagPicker'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { DEFAULT_QR_FILTERS, type QrFilterValues } from './qrUrlState'

const EXPIRATIONS: { value: NonNullable<QrFilterValues['expiration']> | 'any'; label: string }[] = [
  { value: 'any', label: 'Any' },
  { value: 'expired', label: 'Expired' },
  { value: 'expiring', label: 'Expiring' },
  { value: 'none', label: 'No expiration' },
]

const LINK_OPTIONS: { value: QrFilterValues['hasLink']; label: string }[] = [
  { value: 'any', label: 'Codes with or without a short link' },
  { value: 'with', label: 'With a short link' },
  { value: 'without', label: 'Without a short link' },
]

export interface QrFiltersDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  value: QrFilterValues
  /** Tag suggestions (from `useTags('qr')`). */
  tags: string[]
  onApply: (value: QrFilterValues) => void
}

/** "Filters" dialog for the QR Codes list. Edits a draft; Apply commits it. */
export function QrFiltersDialog({ open, onOpenChange, value, tags, onApply }: QrFiltersDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle className="text-xl font-semibold">Filters</DialogTitle>
          <DialogDescription className="sr-only">Narrow down the QR Codes shown</DialogDescription>
        </DialogHeader>
        <FiltersBody
          value={value}
          tags={tags}
          onCancel={() => onOpenChange(false)}
          onApply={(v) => {
            onApply(v)
            onOpenChange(false)
          }}
        />
      </DialogContent>
    </Dialog>
  )
}

function FiltersBody({
  value,
  tags,
  onCancel,
  onApply,
}: {
  value: QrFilterValues
  tags: string[]
  onCancel: () => void
  onApply: (v: QrFilterValues) => void
}) {
  const [draft, setDraft] = useState<QrFilterValues>(value)
  const set = <K extends keyof QrFilterValues>(key: K, v: QrFilterValues[K]) => setDraft((d) => ({ ...d, [key]: v }))

  return (
    <>
      <div className="grid gap-6">
        <div className="grid gap-2">
          <Label>Tags</Label>
          <TagPicker value={draft.tags} onChange={(t) => set('tags', t)} suggestions={tags} />
        </div>

        <fieldset className="grid gap-2">
          <legend className="mb-2 text-sm font-medium text-foreground">Code expiration</legend>
          <RadioGroup
            value={draft.expiration ?? 'any'}
            onValueChange={(v) => set('expiration', v === 'any' ? null : (v as QrFilterValues['expiration']))}
          >
            {EXPIRATIONS.map((o) => (
              <div key={o.value} className="flex items-center gap-2">
                <RadioGroupItem id={`qr-filters-exp-${o.value}`} value={o.value} />
                <Label htmlFor={`qr-filters-exp-${o.value}`} className="font-normal">
                  {o.label}
                </Label>
              </div>
            ))}
          </RadioGroup>
        </fieldset>

        <div className="grid gap-2">
          <Label htmlFor="qr-filters-link">Attached link</Label>
          <Select value={draft.hasLink} onValueChange={(v) => set('hasLink', v as QrFilterValues['hasLink'])}>
            <SelectTrigger id="qr-filters-link" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {LINK_OPTIONS.map((o) => (
                <SelectItem key={o.value} value={o.value}>
                  {o.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      <DialogFooter className="items-center sm:justify-between">
        <Button type="button" variant="link" className="px-0" onClick={() => setDraft(DEFAULT_QR_FILTERS)}>
          Clear all filters
        </Button>
        <div className="flex gap-2">
          <Button type="button" variant="secondary" onClick={onCancel}>
            Cancel
          </Button>
          <Button type="button" onClick={() => onApply(draft)}>
            Apply
          </Button>
        </div>
      </DialogFooter>
    </>
  )
}
