'use client'

import { Suspense, useState } from 'react'
import NextLink from 'next/link'
import { usePathname, useRouter, useSearchParams } from 'next/navigation'
import { endOfDay, format, isValid, parseISO, startOfDay } from 'date-fns'
import { CalendarDays, X } from 'lucide-react'
import { DateRangeDialog, type DateRange } from '@/components/common/DateRangeDialog'
import { EmptyState } from '@/components/common/EmptyState'
import { PageHeader } from '@/components/common/PageHeader'
import { activityDetails, activityTarget, humanizeAction } from '@/components/settings/activity'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { formatDate, formatDateTime } from '@/lib/format'
import { useActivity } from '@/lib/hooks'

const PAGE_SIZE = 20
const YMD = 'yyyy-MM-dd'

function parseYmd(s: string | null): Date | null {
  if (!s) return null
  const d = parseISO(s)
  return isValid(d) ? d : null
}

function ActivityLog() {
  const router = useRouter()
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const [dateOpen, setDateOpen] = useState(false)

  const fromDate = parseYmd(searchParams.get('from'))
  const toDate = parseYmd(searchParams.get('to'))
  const range: DateRange | null = fromDate && toDate ? { from: startOfDay(fromDate), to: endOfDay(toDate) } : null
  const page = Math.max(1, Number(searchParams.get('page')) || 1)

  const { data, isLoading } = useActivity({
    from: range ? format(range.from, YMD) : undefined,
    to: range ? format(range.to, YMD) : undefined,
    page,
    limit: PAGE_SIZE,
  })

  function setParams(next: Record<string, string | null>) {
    const sp = new URLSearchParams(searchParams.toString())
    for (const [k, v] of Object.entries(next)) {
      if (v == null) sp.delete(k)
      else sp.set(k, v)
    }
    const qs = sp.toString()
    router.replace(qs ? `${pathname}?${qs}` : pathname)
  }

  const entries = data?.entries ?? []
  const totalPages = Math.max(1, data?.total_pages ?? 1)

  return (
    <>
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <Button variant="secondary" onClick={() => setDateOpen(true)}>
          <CalendarDays className="size-4" />
          {range ? `${formatDate(range.from.toISOString())} – ${formatDate(range.to.toISOString())}` : 'Filter by date'}
        </Button>
        {range && (
          <Button variant="ghost" onClick={() => setParams({ from: null, to: null, page: null })}>
            <X className="size-4" />
            Clear
          </Button>
        )}
      </div>
      <DateRangeDialog
        open={dateOpen}
        onOpenChange={setDateOpen}
        title="Filter by date"
        value={range}
        onApply={(r) => setParams({ from: format(r.from, YMD), to: format(r.to, YMD), page: null })}
        onClear={() => setParams({ from: null, to: null, page: null })}
      />

      {isLoading ? (
        <Skeleton className="h-64 w-full rounded-xl" />
      ) : entries.length === 0 ? (
        range ? (
          <EmptyState
            title="No activity in this date range"
            description="Try a different date range."
            action={
              <Button variant="secondary" onClick={() => setParams({ from: null, to: null, page: null })}>
                Clear date filter
              </Button>
            }
          />
        ) : (
          <EmptyState
            title="No activity yet"
            description="Changes to your links, QR Codes, domains and settings will appear here."
          />
        )
      ) : (
        <div className="rounded-xl bg-card p-4">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Time</TableHead>
                <TableHead>Action</TableHead>
                <TableHead>Target</TableHead>
                <TableHead>Details</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {entries.map((e) => {
                const target = activityTarget(e)
                const details = activityDetails(e.metadata)
                return (
                  <TableRow key={e.id}>
                    <TableCell className="text-sm whitespace-nowrap">{formatDateTime(e.created_at)}</TableCell>
                    <TableCell className="whitespace-nowrap">{humanizeAction(e.action)}</TableCell>
                    <TableCell>
                      {e.entity_type === 'link' && e.action !== 'link.delete' ? (
                        <NextLink href={`/dashboard/links/${e.entity_id}`} className="text-primary hover:underline">
                          {target}
                        </NextLink>
                      ) : (
                        target
                      )}
                    </TableCell>
                    <TableCell className="whitespace-normal">
                      <div className="flex flex-wrap gap-1">
                        {details.map((d) => (
                          <span
                            key={d}
                            className="max-w-[240px] truncate rounded bg-muted px-2 py-0.5 font-mono text-xs text-foreground"
                            title={d}
                          >
                            {d}
                          </span>
                        ))}
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
          <div className="mt-4 flex items-center justify-between">
            <span className="text-sm text-muted-foreground">
              Page {page} of {totalPages}
            </span>
            <div className="flex gap-2">
              <Button
                variant="secondary"
                size="sm"
                disabled={page <= 1}
                onClick={() => setParams({ page: page - 1 > 1 ? String(page - 1) : null })}
              >
                Previous
              </Button>
              <Button
                variant="secondary"
                size="sm"
                disabled={page >= totalPages}
                onClick={() => setParams({ page: String(page + 1) })}
              >
                Next
              </Button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}

export default function ActivityLogPage() {
  return (
    <>
      <PageHeader title="Activity log" />
      {/* useSearchParams needs a Suspense boundary for static rendering. */}
      <Suspense>
        <ActivityLog />
      </Suspense>
    </>
  )
}
