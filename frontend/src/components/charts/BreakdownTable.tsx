'use client'

import { useState, type ReactNode } from 'react'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { formatChangePct } from '@/components/common/StatCard'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'
import { NoData } from './NoData'

export interface BreakdownRow {
  key: string
  label: ReactNode
  clicks: number
  /** 0–100. */
  percentage: number
  prev_clicks?: number
}

const PAGE_SIZE = 10

/** Change vs the previous period in %, or null when the previous count is 0. */
export function changeVsPrev(clicks: number, prev: number | undefined): number | null {
  if (!prev) return null
  return ((clicks - prev) / prev) * 100
}

function ChangeCell({ clicks, prev }: { clicks: number; prev?: number }) {
  const change = changeVsPrev(clicks, prev)
  if (change == null) return <span className="text-muted-foreground">—</span>
  const up = change >= 0
  return (
    <span className={cn('inline-flex items-center gap-1 font-medium', up ? 'text-success' : 'text-destructive')}>
      <span aria-hidden>{up ? '▲' : '▼'}</span>
      {formatChangePct(change)}
    </span>
  )
}

/** `{label} · Engagements · %` (+ `vs prev.`) table, 10 rows per page (spec §5.4). */
export function BreakdownTable({
  labelHeader,
  rows,
  compare = false,
  pageSize = PAGE_SIZE,
}: {
  labelHeader: string
  rows: BreakdownRow[]
  compare?: boolean
  pageSize?: number
}) {
  const [pageState, setPage] = useState(0)
  if (!rows.length) return <NoData />

  const pages = Math.ceil(rows.length / pageSize)
  // Clamp: the rows may shrink (new range) while a later page is selected.
  const page = Math.min(pageState, pages - 1)
  const start = page * pageSize
  const visible = rows.slice(start, start + pageSize)

  return (
    <div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{labelHeader}</TableHead>
            <TableHead className="text-right">Engagements</TableHead>
            <TableHead className="text-right">%</TableHead>
            {compare && <TableHead className="text-right">vs prev.</TableHead>}
          </TableRow>
        </TableHeader>
        <TableBody>
          {visible.map((r) => (
            <TableRow key={r.key}>
              <TableCell className="max-w-0 truncate text-foreground">{r.label}</TableCell>
              <TableCell className="w-28 text-right tabular-nums">{formatNumber(r.clicks)}</TableCell>
              <TableCell className="w-16 text-right tabular-nums">{Math.round(r.percentage)}%</TableCell>
              {compare && (
                <TableCell className="w-24 text-right tabular-nums">
                  <ChangeCell clicks={r.clicks} prev={r.prev_clicks} />
                </TableCell>
              )}
            </TableRow>
          ))}
        </TableBody>
      </Table>
      {pages > 1 && (
        <div className="mt-3 flex items-center justify-end gap-2 text-sm text-muted-foreground">
          <span>
            {start + 1}–{start + visible.length} of {rows.length}
          </span>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label="Previous page"
            disabled={page === 0}
            onClick={() => setPage(page - 1)}
          >
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
