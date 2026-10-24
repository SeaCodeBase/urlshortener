'use client'

import Link from 'next/link'
import { Link2, QrCode } from 'lucide-react'
import { EmptyState } from '@/components/common/EmptyState'
import { Favicon } from '@/components/common/Favicon'
import { formatNumber, formatShortUrl } from '@/lib/format'
import { useAnalyticsOverview } from '@/lib/hooks'
import type { AnalyticsItem } from '@/types'

function itemHref(item: AnalyticsItem): string {
  return item.type === 'qr' ? `/dashboard/qr-codes/${item.id}` : `/dashboard/links/${item.id}`
}

/** "Top performing": the 5 links / QR Codes with the most engagements over the last 30 days. */
export function TopPerforming() {
  const { data, isLoading } = useAnalyticsOverview()
  const items = data?.top_performing ?? []

  return (
    <section aria-labelledby="overview-top">
      <h2 id="overview-top" className="text-xl font-semibold text-foreground">
        Top performing
      </h2>
      <div className="mt-4">
        {isLoading ? (
          <p className="rounded-xl bg-card p-6 text-sm text-muted-foreground">Loading…</p>
        ) : items.length === 0 ? (
          <EmptyState title="No engagements in the last 30 days" />
        ) : (
          <ul className="divide-y divide-border rounded-xl border border-border bg-card px-6">
            {items.map((item) => {
              const TypeIcon = item.type === 'qr' ? QrCode : Link2
              return (
                <li key={`${item.type}-${item.id}`} className="flex items-center gap-3 py-4">
                  <TypeIcon className="size-5 shrink-0 text-muted-foreground" aria-label={item.type === 'qr' ? 'QR Code' : 'Link'} />
                  {/* Items carry no destination URL, so this is the generic globe. */}
                  <Favicon url="" />
                  <div className="min-w-0 flex-1">
                    <Link href={itemHref(item)} className="block truncate font-semibold text-foreground hover:underline">
                      {item.title || formatShortUrl(item.short_url)}
                    </Link>
                    <a href={item.short_url} target="_blank" rel="noreferrer" className="block truncate text-sm text-primary hover:underline">
                      {formatShortUrl(item.short_url)}
                    </a>
                  </div>
                  <span className="shrink-0 text-sm text-muted-foreground">{formatNumber(item.clicks)} engagements</span>
                </li>
              )
            })}
          </ul>
        )}
      </div>
    </section>
  )
}
