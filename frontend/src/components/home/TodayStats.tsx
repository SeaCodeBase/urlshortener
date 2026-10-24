'use client'

import { useState } from 'react'
import Link from 'next/link'
import { BarChart2 } from 'lucide-react'
import { formatNumber } from '@/lib/format'
import { useTodayStats } from '@/lib/hooks'
import { cn } from '@/lib/utils'
import type { TodayRange } from '@/types'

const RANGES: { value: TodayRange; label: string }[] = [
  { value: 'today', label: 'Today' },
  { value: '7d', label: 'Last 7 days' },
  { value: '30d', label: 'Last 30 days' },
]

/** "Today's stats" card: engagements of the active group over the chosen range. */
export function TodayStats() {
  const [range, setRange] = useState<TodayRange>('today')
  const { data, isLoading } = useTodayStats(range)

  return (
    <div className="flex flex-col rounded-xl border border-border bg-card p-6">
      <Link href="/dashboard/analytics" className="group block rounded outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50">
        <h2 className="text-lg font-semibold text-foreground group-hover:underline">Today&apos;s stats</h2>
        <div className="mt-4 flex items-center gap-4">
          <span className="inline-flex size-12 items-center justify-center rounded-lg bg-sidebar-active-bg text-primary">
            <BarChart2 className="size-6" />
          </span>
          <div>
            <div className="text-sm font-medium tracking-wide text-muted-foreground uppercase">Engagements</div>
            <div className="text-[28px] leading-9 font-bold text-foreground">
              {isLoading || !data ? '–' : formatNumber(data.engagements)}
            </div>
          </div>
        </div>
      </Link>
      <div role="radiogroup" aria-label="Time range" className="mt-6 flex flex-wrap gap-2">
        {RANGES.map((r) => (
          <button
            key={r.value}
            type="button"
            role="radio"
            aria-checked={range === r.value}
            onClick={() => setRange(r.value)}
            className={cn(
              'rounded-full border px-3 py-1 text-sm font-medium transition-colors',
              range === r.value
                ? 'border-primary bg-sidebar-active-bg text-primary'
                : 'border-border text-muted-foreground hover:text-foreground',
            )}
          >
            {r.label}
          </button>
        ))}
      </div>
    </div>
  )
}
