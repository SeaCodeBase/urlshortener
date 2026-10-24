import type { CSSProperties } from 'react'
import { cn } from '@/lib/utils'

export const NO_DATA_MESSAGE = 'No data for this period.'

/** Centred muted "No data for this period." placeholder for an empty analytics module. */
export function NoData({ className, style }: { className?: string; style?: CSSProperties }) {
  return (
    <div
      className={cn('flex min-h-40 items-center justify-center text-sm text-muted-foreground', className)}
      style={style}
    >
      {NO_DATA_MESSAGE}
    </div>
  )
}
