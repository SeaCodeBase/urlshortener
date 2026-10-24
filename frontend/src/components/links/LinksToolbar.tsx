'use client'

import { LayoutGrid, List, Rows3 } from 'lucide-react'
import { RadioGroup as RadioGroupPrimitive } from 'radix-ui'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { cn } from '@/lib/utils'
import type { LinksShow, LinksView } from './linksUrlState'

export type { LinksShow, LinksView } from './linksUrlState'

export interface LinksToolbarProps {
  selectedCount: number
  allSelected: boolean
  /** Disables the select-all checkbox (e.g. nothing loaded). */
  selectAllDisabled?: boolean
  onToggleAll: (selected: boolean) => void
  onExport: () => void
  exporting?: boolean
  /** Hide in the Active view, Unhide in the Hidden view. */
  onHide: () => void
  onTag: () => void
  show: LinksShow
  onShowChange: (show: LinksShow) => void
  view?: LinksView
  onViewChange?: (view: LinksView) => void
  /** What the list holds, for accessible labels (default "links"). */
  noun?: 'links' | 'codes'
  /** Shows the Compact/Default/Grid toggle (default true). */
  showViews?: boolean
}

const VIEWS: { value: LinksView; label: string; icon: typeof List }[] = [
  { value: 'compact', label: 'Compact view', icon: Rows3 },
  { value: 'default', label: 'Default view', icon: List },
  { value: 'grid', label: 'Grid view', icon: LayoutGrid },
]

/** Second toolbar row of the links (and QR Codes) list: selection + bulk actions, view toggle, Show select. */
export function LinksToolbar({
  selectedCount,
  allSelected,
  selectAllDisabled,
  onToggleAll,
  onExport,
  exporting,
  onHide,
  onTag,
  show,
  onShowChange,
  view,
  onViewChange,
  noun = 'links',
  showViews = true,
}: LinksToolbarProps) {
  const none = selectedCount === 0
  return (
    <div className="flex flex-wrap items-center justify-between gap-4">
      <div className="flex items-center gap-4">
        <Checkbox
          checked={allSelected}
          onCheckedChange={onToggleAll}
          disabled={selectAllDisabled}
          aria-label={`Select all ${noun}`}
        />
        <span className="text-sm text-foreground">{selectedCount} selected</span>
        <div className="flex items-center gap-1 border-l border-border pl-3">
          <Button type="button" variant="ghost" size="sm" onClick={onExport} disabled={exporting}>
            Export
          </Button>
          <Button type="button" variant="ghost" size="sm" onClick={onHide} disabled={none}>
            {show === 'hidden' ? 'Unhide' : 'Hide'}
          </Button>
          <Button type="button" variant="ghost" size="sm" onClick={onTag} disabled={none}>
            Tag
          </Button>
        </div>
      </div>

      <div className="flex items-center gap-3">
        {showViews && (
          <RadioGroupPrimitive.Root
            value={view}
            onValueChange={(v) => onViewChange?.(v as LinksView)}
            orientation="horizontal"
            aria-label="View"
            className="flex items-center rounded border border-border bg-white p-0.5 dark:bg-card"
          >
            {VIEWS.map(({ value, label, icon: Icon }) => (
              <RadioGroupPrimitive.Item
                key={value}
                value={value}
                aria-label={label}
                title={label}
                className={cn(
                  'inline-flex size-8 items-center justify-center rounded text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50',
                  'data-[state=checked]:bg-accent data-[state=checked]:text-primary',
                )}
              >
                <Icon className="size-4" />
              </RadioGroupPrimitive.Item>
            ))}
          </RadioGroupPrimitive.Root>
        )}

        <Select value={show} onValueChange={(v) => onShowChange(v as LinksShow)}>
          <SelectTrigger aria-label="Show" className="w-auto min-w-36 bg-white dark:bg-card">
            <span className="text-muted-foreground">{'Show: '}</span>
            <SelectValue />
          </SelectTrigger>
          <SelectContent align="end">
            <SelectItem value="active">Active</SelectItem>
            <SelectItem value="hidden">Hidden</SelectItem>
          </SelectContent>
        </Select>
      </div>
    </div>
  )
}
