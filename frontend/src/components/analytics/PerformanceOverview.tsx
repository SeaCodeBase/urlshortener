'use client'

import { StatCard } from '@/components/common/StatCard'
import { TimeSeriesChart } from '@/components/charts/TimeSeriesChart'
import { formatNumber } from '@/lib/format'
import { useAnalyticsOverview } from '@/lib/hooks'
import { LoadError } from './LoadError'

/** "Performance overview": engagements this week vs last week, with both weeks charted. */
export function PerformanceOverview() {
  const { data, isLoading, isError, refetch } = useAnalyticsOverview()

  return (
    <section aria-labelledby="overview-performance">
      <h2 id="overview-performance" className="text-xl font-semibold text-foreground">
        Performance overview
      </h2>
      <div className="mt-4 rounded-xl border border-border bg-card">
        <StatCard
          label="TOTAL ENGAGEMENTS · THIS WEEK"
          value={isLoading || !data ? '–' : formatNumber(data.this_week.total)}
          change={data ? { pct: data.change_pct, label: 'vs last week' } : undefined}
          className="pb-0"
        />
        <div className="p-6">
          {isError && !data ? (
            <LoadError message="Couldn't load the performance overview." onRetry={() => void refetch()} />
          ) : isLoading || !data ? (
            <div className="h-[280px] animate-pulse rounded-lg bg-muted" aria-hidden />
          ) : (
            <TimeSeriesChart
              series={data.this_week.series}
              prevSeries={data.last_week.series}
              unit="day"
              compare
              seriesName="This week"
              prevSeriesName="Last week"
            />
          )}
        </div>
      </div>
    </section>
  )
}
