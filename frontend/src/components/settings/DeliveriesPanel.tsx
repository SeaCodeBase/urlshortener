'use client'

import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDateTime } from '@/lib/format'
import { useWebhookDeliveries } from '@/lib/hooks'

function codeClass(code: number): string {
  if (code >= 200 && code < 300) return 'bg-success/15 text-success'
  if (code === 0 || code >= 400) return 'bg-destructive/10 text-destructive'
  return 'bg-muted text-muted-foreground'
}

/** A webhook's last 20 deliveries, newest first; fetched only while `open`. */
export function DeliveriesPanel({ webhookId, open = true }: { webhookId: number; open?: boolean }) {
  const { data, isLoading, isError } = useWebhookDeliveries(webhookId, open)

  if (isLoading) {
    return (
      <div className="flex flex-col gap-2" aria-busy="true" aria-label="Loading deliveries">
        <Skeleton className="h-6 w-full" />
        <Skeleton className="h-6 w-full" />
      </div>
    )
  }
  if (isError) {
    return (
      <p role="alert" className="text-sm text-destructive">
        Failed to load deliveries
      </p>
    )
  }
  if (!data?.length) {
    return <p className="py-2 text-sm text-muted-foreground">No deliveries yet</p>
  }
  return (
    <ul aria-label="Recent deliveries" className="flex flex-col divide-y divide-border">
      {data.map((d) => (
        <li key={d.id} className="flex items-center gap-4 py-2 text-sm">
          <Badge className={`w-12 justify-center font-mono ${codeClass(d.status_code)}`}>{d.status_code}</Badge>
          <span className="w-20 shrink-0 text-muted-foreground">Attempt {d.attempt}</span>
          <span className="shrink-0 whitespace-nowrap text-foreground">{formatDateTime(d.delivered_at)}</span>
          {d.error && (
            <span className="min-w-0 flex-1 truncate text-destructive" title={d.error}>
              {d.error}
            </span>
          )}
        </li>
      ))}
    </ul>
  )
}
