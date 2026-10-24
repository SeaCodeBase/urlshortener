'use client'

import type { ReactNode } from 'react'
import { Line, LineChart } from 'recharts'
import { cn } from '@/lib/utils'

/** `21.234` → `+21.2%`, `-5` → `-5.0%`. */
export function formatChangePct(pct: number): string {
  return `${pct >= 0 ? '+' : ''}${pct.toFixed(1)}%`
}

/** Scorecard: label, big value, optional ▲/▼ change badge and sparkline. */
export function StatCard({
  label,
  value,
  change,
  spark,
  className,
}: {
  label: string
  value: ReactNode
  change?: { pct: number | null; label: string }
  spark?: number[]
  className?: string
}) {
  const up = change?.pct != null && change.pct >= 0
  return (
    <div className={cn('rounded-xl bg-card p-6', className)}>
      <div className="text-sm font-medium tracking-wide text-muted-foreground uppercase">{label}</div>
      <div className="mt-2 flex items-end justify-between gap-4">
        <div className="text-[28px] leading-9 font-bold text-foreground">{value}</div>
        {spark && spark.length > 1 && (
          <LineChart width={80} height={32} data={spark.map((v, i) => ({ i, v }))} aria-hidden>
            <Line type="monotone" dataKey="v" stroke="#2A5BD7" strokeWidth={2} dot={false} isAnimationActive={false} />
          </LineChart>
        )}
      </div>
      {change && change.pct != null && (
        <div className="mt-2 flex items-center gap-2 text-sm text-muted-foreground">
          <span
            className={cn(
              'inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-semibold',
              up ? 'bg-success/10 text-success' : 'bg-destructive/10 text-destructive',
            )}
          >
            <span aria-hidden>{up ? '▲' : '▼'}</span>
            {formatChangePct(change.pct)}
          </span>
          <span>{change.label}</span>
        </div>
      )}
    </div>
  )
}
