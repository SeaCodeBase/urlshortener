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
import { useCampaigns } from '@/lib/hooks'

export interface LinkFilters {
  tags: string[]
  linkType: 'all' | 'custom' | 'nocustom'
  expiration: 'expired' | 'expiring' | 'none' | null
  hasQr: 'any' | 'with' | 'without'
  /** Campaign id as a string; '' (or 'all') = any campaign. */
  campaignId: string
}

export const DEFAULT_LINK_FILTERS: LinkFilters = { tags: [], linkType: 'all', expiration: null, hasQr: 'any', campaignId: '' }

/** True when a campaign narrows the list ('' and 'all' mean none). */
export function hasCampaignFilter(f: Pick<LinkFilters, 'campaignId'>): boolean {
  return f.campaignId !== '' && f.campaignId !== 'all'
}

const LINK_TYPES: { value: LinkFilters['linkType']; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'custom', label: 'Links with custom back-halves' },
  { value: 'nocustom', label: 'Links without custom back-halves' },
]

const EXPIRATIONS: { value: NonNullable<LinkFilters['expiration']> | 'any'; label: string }[] = [
  { value: 'any', label: 'Any' },
  { value: 'expired', label: 'Expired' },
  { value: 'expiring', label: 'Expiring' },
  { value: 'none', label: 'No expiration' },
]

const QR_OPTIONS: { value: LinkFilters['hasQr']; label: string }[] = [
  { value: 'any', label: 'Links with or without attached QR Codes' },
  { value: 'with', label: 'With attached QR Codes' },
  { value: 'without', label: 'Without attached QR Codes' },
]

/** Number of filters that differ from the defaults (for "Add filters (n)" badges). */
export function countActiveFilters(f: LinkFilters): number {
  return (
    (f.tags.length ? 1 : 0) +
    (f.linkType !== 'all' ? 1 : 0) +
    (f.expiration != null ? 1 : 0) +
    (f.hasQr !== 'any' ? 1 : 0) +
    (hasCampaignFilter(f) ? 1 : 0)
  )
}

export interface FiltersDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  value: LinkFilters
  tags: string[]
  onApply: (value: LinkFilters) => void
}

/** "Filters" dialog for the links list (spec §5.2). Edits a draft; Apply commits it. */
export function FiltersDialog({ open, onOpenChange, value, tags, onApply }: FiltersDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle className="text-xl font-semibold">Filters</DialogTitle>
          <DialogDescription className="sr-only">Narrow down the links shown</DialogDescription>
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
  value: LinkFilters
  tags: string[]
  onCancel: () => void
  onApply: (v: LinkFilters) => void
}) {
  const [draft, setDraft] = useState<LinkFilters>(value)
  const { data: campaigns } = useCampaigns()
  const set = <K extends keyof LinkFilters>(key: K, v: LinkFilters[K]) => setDraft((d) => ({ ...d, [key]: v }))

  return (
    <>
      <div className="grid gap-6">
        <div className="grid gap-2">
          <Label>Tags</Label>
          <TagPicker value={draft.tags} onChange={(t) => set('tags', t)} suggestions={tags} />
        </div>

        <div className="grid gap-2">
          <Label htmlFor="filters-link-type">Link type</Label>
          <Select value={draft.linkType} onValueChange={(v) => set('linkType', v as LinkFilters['linkType'])}>
            <SelectTrigger id="filters-link-type" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {LINK_TYPES.map((o) => (
                <SelectItem key={o.value} value={o.value}>
                  {o.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <fieldset className="grid gap-2">
          <legend className="mb-2 text-sm font-medium text-foreground">Link expiration</legend>
          <RadioGroup
            value={draft.expiration ?? 'any'}
            onValueChange={(v) => set('expiration', v === 'any' ? null : (v as LinkFilters['expiration']))}
          >
            {EXPIRATIONS.map((o) => (
              <div key={o.value} className="flex items-center gap-2">
                <RadioGroupItem id={`filters-exp-${o.value}`} value={o.value} />
                <Label htmlFor={`filters-exp-${o.value}`} className="font-normal">
                  {o.label}
                </Label>
              </div>
            ))}
          </RadioGroup>
        </fieldset>

        <div className="grid gap-2">
          <Label htmlFor="filters-qr">Attached QR Code</Label>
          <Select value={draft.hasQr} onValueChange={(v) => set('hasQr', v as LinkFilters['hasQr'])}>
            <SelectTrigger id="filters-qr" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {QR_OPTIONS.map((o) => (
                <SelectItem key={o.value} value={o.value}>
                  {o.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="grid gap-2">
          <Label htmlFor="filters-campaign">Campaign</Label>
          <Select
            value={hasCampaignFilter(draft) ? draft.campaignId : 'all'}
            onValueChange={(v) => set('campaignId', v === 'all' ? '' : v)}
          >
            <SelectTrigger id="filters-campaign" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All campaigns</SelectItem>
              {(campaigns ?? []).map((c) => (
                <SelectItem key={c.id} value={String(c.id)}>
                  {c.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      <DialogFooter className="items-center sm:justify-between">
        <Button type="button" variant="link" className="px-0" onClick={() => setDraft(DEFAULT_LINK_FILTERS)}>
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
