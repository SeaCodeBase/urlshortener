'use client'

import { useId, useMemo, useState } from 'react'
import { LoadError } from '@/components/analytics/LoadError'
import { DATE_RANGE_OPTIONS, rangeFor } from '@/components/analytics/modules'
import { TimeSeriesChart } from '@/components/charts/TimeSeriesChart'
import { EmptyState } from '@/components/common/EmptyState'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { formatNumber } from '@/lib/format'
import { useCampaignStats } from '@/lib/hooks'
import type { CampaignStatsQuery, DashboardDateRange } from '@/types'

/** Preset options (no custom range on this card). */
export const CAMPAIGN_RANGE_OPTIONS = DATE_RANGE_OPTIONS.filter((o) => (o.value as string) !== 'custom')

export const DEFAULT_CAMPAIGN_RANGE: DashboardDateRange = 'last_7_days'

/** `{from, to, unit: 'day'}` of a preset, in UTC calendar days. */
export function campaignStatsQuery(range: DashboardDateRange): CampaignStatsQuery {
  return { ...rangeFor(range), unit: 'day' }
}

/**
 * "Performance": preset select, one line per channel, and a Channel · Links · Engagements summary with a Total row.
 * Controlled when `range`/`onRangeChange` are given (the page shares the preset with the channel cards).
 */
export function CampaignPerformance({
  campaignId,
  range: controlledRange,
  onRangeChange,
}: {
  campaignId: number
  range?: DashboardDateRange
  onRangeChange?: (range: DashboardDateRange) => void
}) {
  const headingId = useId()
  const [localRange, setLocalRange] = useState<DashboardDateRange>(DEFAULT_CAMPAIGN_RANGE)
  const range = controlledRange ?? localRange
  const query = useMemo(() => campaignStatsQuery(range), [range])
  const { data, isLoading, isError, refetch } = useCampaignStats(campaignId, query)

  function setRange(v: string) {
    const next = v as DashboardDateRange
    setLocalRange(next)
    onRangeChange?.(next)
  }

  return (
    <section aria-labelledby={headingId} className="rounded-xl bg-card p-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h2 id={headingId} className="text-xl font-semibold text-foreground">
          Performance
        </h2>
        <Select value={range} onValueChange={setRange}>
          <SelectTrigger aria-label="Date range" className="w-44">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {CAMPAIGN_RANGE_OPTIONS.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="mt-6">
        {isError && !data ? (
          <LoadError message="Failed to load performance" onRetry={() => void refetch()} />
        ) : isLoading || !data ? (
          <div className="h-[280px] animate-pulse rounded-lg bg-muted" aria-hidden />
        ) : data.channels.length === 0 ? (
          <EmptyState
            title="Add a channel to see performance"
            description="Engagements are charted per channel once the campaign has one."
          />
        ) : (
          <>
            <TimeSeriesChart
              series={[]}
              unit="day"
              variant="line"
              multiSeries={data.channels.map((c) => ({ name: c.name, points: c.series }))}
            />
            <Table className="mt-6">
              <TableHeader>
                <TableRow>
                  <TableHead>Channel</TableHead>
                  <TableHead className="text-right">Links</TableHead>
                  <TableHead className="text-right">Engagements</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.channels.map((c) => (
                  <TableRow key={c.channel_id}>
                    <TableCell>{c.name}</TableCell>
                    <TableCell className="text-right">{formatNumber(c.links)}</TableCell>
                    <TableCell className="text-right">{formatNumber(c.engagements)}</TableCell>
                  </TableRow>
                ))}
                <TableRow className="font-semibold">
                  <TableCell>Total</TableCell>
                  <TableCell className="text-right">{formatNumber(data.totals.links)}</TableCell>
                  <TableCell className="text-right">{formatNumber(data.totals.engagements)}</TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </>
        )}
      </div>
    </section>
  )
}
