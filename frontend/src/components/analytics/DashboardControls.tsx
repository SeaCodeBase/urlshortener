'use client'

import { useState } from 'react'
import { format, parseISO } from 'date-fns'
import { Calendar, Check, ChevronDown, Filter } from 'lucide-react'
import { DateRangeDialog } from '@/components/common/DateRangeDialog'
import { MAX_RANGE_DAYS, rangeLabel } from '@/components/links/LinkAnalytics'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { Dashboard, DashboardDateRange, DashboardFilter } from '@/types'
import { DashboardFilterDialog } from './DashboardFilterDialog'
import { filterSummary } from './DashboardRow'
import { DATE_RANGE_OPTIONS } from './modules'

export type DashboardCompare = 'previous' | 'none'
export interface DayRange {
  /** yyyy-MM-dd */
  from: string
  to: string
}

const DAY = 'yyyy-MM-dd'

export interface DashboardControlsProps {
  dashboard: Dashboard
  /** The URL's custom range (`?from&to`), null when the saved preset applies. */
  customRange: DayRange | null
  /** The effective range (custom or the saved preset's window). */
  range: DayRange
  compare: DashboardCompare
  /** A preset was picked: persist it (and drop the custom range). */
  onPreset: (dateRange: DashboardDateRange) => void
  /** A custom range was applied (URL only); null clears it. */
  onCustomRange: (range: DayRange | null) => void
  onCompareChange: (compare: DashboardCompare) => void
  onFilterApply: (filter: DashboardFilter) => Promise<void>
}

/** Date range menu, comparison select, "Filter" dialog and the filter summary of a dashboard. */
export function DashboardControls({
  dashboard,
  customRange,
  range,
  compare,
  onPreset,
  onCustomRange,
  onCompareChange,
  onFilterApply,
}: DashboardControlsProps) {
  const [dateOpen, setDateOpen] = useState(false)
  const [filterOpen, setFilterOpen] = useState(false)
  const label = customRange
    ? rangeLabel(customRange)
    : (DATE_RANGE_OPTIONS.find((o) => o.value === dashboard.date_range)?.label ?? 'Last 7 days')

  return (
    <div className="flex flex-wrap items-center gap-2">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button type="button" variant="secondary" aria-label={`Date range: ${label}`}>
            <Calendar className="size-4" />
            {label}
            <ChevronDown className="size-4 text-muted-foreground" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-48">
          {DATE_RANGE_OPTIONS.map((o) => {
            const active = !customRange && o.value === dashboard.date_range
            return (
              <DropdownMenuItem key={o.value} onSelect={() => onPreset(o.value)}>
                <Check className={active ? 'size-4' : 'size-4 opacity-0'} aria-hidden />
                {o.label}
              </DropdownMenuItem>
            )
          })}
          <DropdownMenuSeparator />
          <DropdownMenuItem onSelect={() => setDateOpen(true)}>
            <Check className={customRange ? 'size-4' : 'size-4 opacity-0'} aria-hidden />
            Custom…
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Select value={compare} onValueChange={(v) => onCompareChange(v as DashboardCompare)}>
        <SelectTrigger aria-label="Comparison" className="h-10 w-48 bg-white">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="previous">Previous period</SelectItem>
          <SelectItem value="none">No comparison</SelectItem>
        </SelectContent>
      </Select>

      <Button type="button" variant="secondary" onClick={() => setFilterOpen(true)}>
        <Filter className="size-4" />
        Filter
      </Button>
      <span className="text-sm text-muted-foreground">{filterSummary(dashboard.filter)}</span>

      <DateRangeDialog
        open={dateOpen}
        onOpenChange={setDateOpen}
        title="Select date range"
        maxDays={MAX_RANGE_DAYS}
        value={{ from: parseISO(range.from), to: parseISO(range.to) }}
        onApply={(r) => onCustomRange({ from: format(r.from, DAY), to: format(r.to, DAY) })}
        onClear={customRange ? () => onCustomRange(null) : undefined}
      />
      <DashboardFilterDialog open={filterOpen} onOpenChange={setFilterOpen} value={dashboard.filter} onApply={onFilterApply} />
    </div>
  )
}
