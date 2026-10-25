import type { ReactNode } from 'react'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

export type StatusTone = 'success' | 'warning' | 'info' | 'muted' | 'destructive'

const TONES: Record<StatusTone, string> = {
  success: 'bg-success/15 text-success',
  warning: 'bg-amber-500/15 text-amber-700 dark:text-amber-400',
  info: 'bg-primary/10 text-primary',
  muted: 'bg-muted text-muted-foreground',
  destructive: 'bg-destructive/10 text-destructive',
}

/** Status pill shared by domains, webhooks, deliveries and bulk jobs. */
export function StatusChip({
  tone,
  children,
  className,
  title,
}: {
  tone: StatusTone
  children: ReactNode
  className?: string
  title?: string
}) {
  return (
    <Badge className={cn(TONES[tone], className)} title={title}>
      {children}
    </Badge>
  )
}
