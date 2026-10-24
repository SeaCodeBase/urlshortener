'use client'

import { useState } from 'react'
import Link from 'next/link'
import { ChevronLeft, ChevronRight, Link2, QrCode } from 'lucide-react'
import { ChangeCell } from '@/components/charts/BreakdownTable'
import { NoData } from '@/components/charts/NoData'
import { Favicon } from '@/components/common/Favicon'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { formatNumber, formatShortUrl } from '@/lib/format'
import type { AnalyticsItem } from '@/types'

const PAGE_SIZE = 10

export function itemHref(item: AnalyticsItem): string {
  return item.type === 'qr' ? `/dashboard/qr-codes/${item.id}` : `/dashboard/links/${item.id}`
}

/** "Links & QR Codes by engagements": Type · Link · Engagements · % (· vs prev.), 10 per page. */
export function ItemsTable({ items, compare }: { items: AnalyticsItem[]; compare: boolean }) {
  const [pageState, setPage] = useState(0)
  if (!items.length) return <NoData />

  const pages = Math.ceil(items.length / PAGE_SIZE)
  const page = Math.min(pageState, pages - 1)
  const start = page * PAGE_SIZE
  const visible = items.slice(start, start + PAGE_SIZE)

  return (
    <div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-12">Type</TableHead>
            <TableHead>Link</TableHead>
            <TableHead className="text-right">Engagements</TableHead>
            <TableHead className="text-right">%</TableHead>
            {compare && <TableHead className="text-right">vs prev.</TableHead>}
          </TableRow>
        </TableHeader>
        <TableBody>
          {visible.map((item) => {
            const TypeIcon = item.type === 'qr' ? QrCode : Link2
            return (
              <TableRow key={`${item.type}-${item.id}`}>
                <TableCell>
                  <TypeIcon
                    className="size-5 text-muted-foreground"
                    role="img"
                    aria-label={item.type === 'qr' ? 'QR Code' : 'Link'}
                  />
                </TableCell>
                <TableCell className="max-w-0">
                  <div className="flex min-w-0 items-center gap-3">
                    <Favicon url={item.original_url} size={28} />
                    <div className="min-w-0">
                      <Link href={itemHref(item)} className="block truncate font-semibold text-foreground hover:underline">
                        {item.title || formatShortUrl(item.short_url)}
                      </Link>
                      <span className="block truncate text-sm text-primary">{formatShortUrl(item.short_url)}</span>
                    </div>
                  </div>
                </TableCell>
                <TableCell className="w-28 text-right tabular-nums">{formatNumber(item.clicks)}</TableCell>
                <TableCell className="w-16 text-right tabular-nums">{Math.round(item.percentage)}%</TableCell>
                {compare && (
                  <TableCell className="w-24 text-right tabular-nums">
                    <ChangeCell clicks={item.clicks} prev={item.prev_clicks} />
                  </TableCell>
                )}
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
      {pages > 1 && (
        <div className="mt-3 flex items-center justify-end gap-2 text-sm text-muted-foreground">
          <span>
            {start + 1}–{start + visible.length} of {items.length}
          </span>
          <Button type="button" variant="ghost" size="icon-sm" aria-label="Previous page" disabled={page === 0} onClick={() => setPage(page - 1)}>
            <ChevronLeft className="size-4" />
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label="Next page"
            disabled={page >= pages - 1}
            onClick={() => setPage(page + 1)}
          >
            <ChevronRight className="size-4" />
          </Button>
        </div>
      )}
    </div>
  )
}
