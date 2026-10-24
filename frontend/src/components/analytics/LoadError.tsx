'use client'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

/** Inline "couldn't load" line with a Retry button, used instead of a skeleton or an empty state when a query fails. */
export function LoadError({ message, onRetry, className }: { message: string; onRetry: () => void; className?: string }) {
  return (
    <div role="alert" className={cn('flex items-center justify-between gap-4 text-sm text-muted-foreground', className)}>
      <span>{message}</span>
      <Button type="button" variant="secondary" size="sm" onClick={onRetry}>
        Retry
      </Button>
    </div>
  )
}
