'use client'

import { useState, type ReactNode } from 'react'
import { usePathname, useRouter, useSearchParams } from 'next/navigation'
import { format, parseISO } from 'date-fns'
import { AlertCircle, Calendar, ChevronDown, Download } from 'lucide-react'
import { BreakdownTable, type BreakdownRow } from '@/components/charts/BreakdownTable'
import { DeviceBarChart } from '@/components/charts/DeviceBarChart'
import { TimeSeriesChart } from '@/components/charts/TimeSeriesChart'
import { DateRangeDialog } from '@/components/common/DateRangeDialog'
import { EmptyState } from '@/components/common/EmptyState'
import { StatCard } from '@/components/common/StatCard'
import {
  MAX_RANGE_DAYS,
  effectiveRange,
  flagEmoji,
  parseAnalyticsUrlState,
  rangeLabel,
  unitFor,
} from '@/components/links/LinkAnalytics'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { buildPageStatsCsv } from '@/lib/csv'
import { downloadBlob } from '@/lib/download'
import { formatNumber, formatShortUrl } from '@/lib/format'
import { usePageStats } from '@/lib/hooks'
import type { PageStatsQuery } from '@/types'

const DAY = 'yyyy-MM-dd'
type Compare = 'previous' | 'none'

/**
 * A local Date whose calendar day is today's UTC day: the default window and the
 * preset labels then follow the backend's UTC days (Phase 3 `rangeFor` convention).
 */
function utcToday(now = new Date()): Date {
  return new Date(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate())
}

function Module({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="rounded-xl bg-card p-6">
      <h3 className="mb-4 text-lg font-semibold text-foreground">{title}</h3>
      {children}
    </section>
  )
}

const ModuleSkeleton = ({ height = 160 }: { height?: number }) => <Skeleton className="w-full" style={{ height }} />

/** Track tab of the page editor: views, link-block clicks and breakdowns over a date range. */
export function TrackTab({ pageId }: { pageId: number }) {
  const router = useRouter()
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const state = parseAnalyticsUrlState(searchParams)
  const [dateOpen, setDateOpen] = useState(false)

  const today = utcToday()
  const range = effectiveRange(state, today)
  const unit = unitFor(range.from, range.to)
  const comparing = state.compare === 'previous'
  const label = rangeLabel(state, today)
  const query: PageStatsQuery = { ...range, unit, compare: state.compare }
  const { data: stats, isLoading, isError, error, refetch, isFetching } = usePageStats(pageId, query)
  const loading = isLoading && !stats

  function setUrl(patch: { from?: string | null; to?: string | null; compare?: Compare }) {
    const next = { from: state.from, to: state.to, compare: state.compare, ...patch }
    const sp = new URLSearchParams(searchParams.toString())
    const set = (k: string, v: string | null) => (v ? sp.set(k, v) : sp.delete(k))
    set('from', next.from)
    set('to', next.to)
    set('compare', next.compare === 'previous' ? 'previous' : null)
    const qs = sp.toString()
    router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false })
  }

  const countryRows: BreakdownRow[] = (stats?.countries ?? []).map((c) => ({
    key: c.code || c.name,
    label: (
      <span className="inline-flex items-center gap-2">
        <span aria-hidden>{flagEmoji(c.code)}</span>
        {c.name || 'Unknown'}
      </span>
    ),
    clicks: c.clicks,
    percentage: c.percentage,
    prev_clicks: c.prev_clicks,
  }))
  const cityRows: BreakdownRow[] = (stats?.cities ?? []).map((c) => ({
    key: `${c.name}\u0000${c.country}`,
    label: c.name ? `${c.name}${c.country ? `, ${c.country}` : ''}` : 'Unknown',
    clicks: c.clicks,
    percentage: c.percentage,
    prev_clicks: c.prev_clicks,
  }))
  const referrerRows: BreakdownRow[] = (stats?.referrers ?? []).map((r) => ({
    key: r.referrer || '(direct)',
    label: r.referrer || 'Direct',
    clicks: r.clicks,
    percentage: r.percentage,
    prev_clicks: r.prev_clicks,
  }))

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <Button type="button" variant="secondary" onClick={() => setDateOpen(true)}>
          <Calendar className="size-4" />
          {label}
          <ChevronDown className="size-4 text-muted-foreground" />
        </Button>
        <Select value={state.compare} onValueChange={(v) => setUrl({ compare: v as Compare })}>
          <SelectTrigger aria-label="Comparison" className="h-10 w-48 bg-white">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="none">No comparison</SelectItem>
            <SelectItem value="previous">Previous period</SelectItem>
          </SelectContent>
        </Select>
        <Button
          type="button"
          variant="secondary"
          size="icon"
          aria-label="Download CSV"
          className="ml-auto"
          disabled={!stats}
          onClick={() =>
            stats &&
            downloadBlob(
              `page-${pageId}-stats-${range.from}-${range.to}.csv`,
              new Blob([buildPageStatsCsv(stats, { rangeLabel: label })], { type: 'text/csv;charset=utf-8' }),
            )
          }
        >
          <Download className="size-5" />
        </Button>
      </div>

      <DateRangeDialog
        open={dateOpen}
        onOpenChange={setDateOpen}
        title="Select date range"
        maxDays={MAX_RANGE_DAYS}
        value={{ from: parseISO(range.from), to: parseISO(range.to) }}
        onApply={(r) => setUrl({ from: format(r.from, DAY), to: format(r.to, DAY) })}
        onClear={state.from ? () => setUrl({ from: null, to: null }) : undefined}
      />

      {isError && !stats ? (
        <div role="alert" className="flex flex-col items-center gap-3 rounded-xl bg-card px-8 py-12 text-center">
          <AlertCircle className="size-8 text-destructive" aria-hidden />
          <h3 className="text-lg font-semibold text-foreground">Couldn&apos;t load page stats</h3>
          <p className="text-sm text-muted-foreground">{error instanceof Error ? error.message : 'Something went wrong.'}</p>
          <Button type="button" variant="secondary" disabled={isFetching} onClick={() => refetch()}>
            Retry
          </Button>
        </div>
      ) : (
        <>
          <div className="grid gap-4 md:grid-cols-2">
            {loading || !stats ? (
              <>
                <Skeleton className="h-[124px] rounded-xl" />
                <Skeleton className="h-[124px] rounded-xl" />
              </>
            ) : (
              <>
                <StatCard label="Total views" value={formatNumber(stats.total_views_all_time)} />
                <StatCard
                  label={`Views (${label})`}
                  value={formatNumber(stats.total_views)}
                  change={comparing ? { pct: stats.change_pct, label: 'vs previous period' } : undefined}
                  spark={stats.series.map((p) => p.clicks)}
                />
              </>
            )}
          </div>

          <Module title="Views over time">
            {loading || !stats ? (
              <ModuleSkeleton height={280} />
            ) : (
              <TimeSeriesChart
                series={stats.series}
                prevSeries={stats.prev_series}
                unit={stats.unit ?? unit}
                compare={comparing}
                seriesName="Views"
              />
            )}
          </Module>

          <Module title="Link clicks">
            {loading || !stats ? (
              <ModuleSkeleton />
            ) : stats.blocks.length === 0 ? (
              <EmptyState title="No link blocks yet" description="Add a link block on the Build tab to track its clicks." />
            ) : (
              <Table aria-label="Link clicks">
                <TableHeader>
                  <TableRow>
                    <TableHead>Block</TableHead>
                    <TableHead>Short link</TableHead>
                    <TableHead className="text-right">Clicks</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {stats.blocks.map((b) => (
                    <TableRow key={b.block_id}>
                      <TableCell className="max-w-64 truncate font-medium">{b.title || 'Untitled'}</TableCell>
                      <TableCell className="max-w-64 truncate text-primary">
                        {b.short_url ? formatShortUrl(b.short_url) : '—'}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">{formatNumber(b.clicks)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </Module>

          <div className="grid gap-4 lg:grid-cols-2">
            <Module title="Views by referrer">
              {loading || !stats ? (
                <ModuleSkeleton />
              ) : (
                <BreakdownTable labelHeader="Referrer" rows={referrerRows} compare={comparing} />
              )}
            </Module>
            <Module title="Views by device">
              {loading || !stats ? <ModuleSkeleton /> : <DeviceBarChart devices={stats.devices} compare={comparing} />}
            </Module>
          </div>

          <Module title="Views by location">
            {loading || !stats ? (
              <ModuleSkeleton />
            ) : (
              <Tabs defaultValue="countries">
                <TabsList className="mb-2">
                  <TabsTrigger value="countries">Countries</TabsTrigger>
                  <TabsTrigger value="cities">Cities</TabsTrigger>
                </TabsList>
                <TabsContent value="countries">
                  <BreakdownTable labelHeader="Country" rows={countryRows} compare={comparing} />
                </TabsContent>
                <TabsContent value="cities">
                  <BreakdownTable labelHeader="City" rows={cityRows} compare={comparing} />
                </TabsContent>
              </Tabs>
            )}
          </Module>
        </>
      )}
    </div>
  )
}
