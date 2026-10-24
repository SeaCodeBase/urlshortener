'use client'

import { format, parseISO } from 'date-fns'
import { CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import type { SeriesPoint, StatsUnit } from '@/types'
import { NoData } from './NoData'
import { CURRENT_COLOR, PREVIOUS_COLOR } from './palette'

/** `2026-10-24` → `Oct 24`; `2026-10-24T13:00` → `13:00`. */
export function formatBucket(date: string, unit: StatsUnit): string {
  if (unit === 'hour') {
    const m = /T(\d{2})/.exec(date)
    return m ? `${m[1]}:00` : date
  }
  const d = parseISO(date)
  return Number.isNaN(d.getTime()) ? date : format(d, 'MMM d')
}

interface Row {
  label: string
  current: number
  previous?: number
}

/** Engagements over time: current period solid blue, previous period dashed light blue (aligned by bucket index). */
export function TimeSeriesChart({
  series,
  prevSeries = [],
  unit,
  compare = false,
  height = 280,
  seriesName = 'Engagements',
}: {
  series: SeriesPoint[]
  prevSeries?: SeriesPoint[]
  unit: StatsUnit
  compare?: boolean
  height?: number
  /** Legend/tooltip name of the current period's line. */
  seriesName?: string
}) {
  if (!series.length) return <NoData style={{ height }} />

  const data: Row[] = series.map((p, i) => ({
    label: formatBucket(p.date, unit),
    current: p.clicks,
    ...(compare ? { previous: prevSeries[i]?.clicks ?? 0 } : {}),
  }))

  return (
    <ResponsiveContainer width="100%" height={height} initialDimension={{ width: 600, height }}>
      <LineChart data={data} margin={{ top: 8, right: 16, bottom: 0, left: -16 }}>
        <CartesianGrid vertical={false} stroke="#E1E4EC" />
        <XAxis dataKey="label" tickLine={false} axisLine={false} tick={{ fontSize: 12, fill: '#526281' }} minTickGap={16} />
        <YAxis allowDecimals={false} tickLine={false} axisLine={false} tick={{ fontSize: 12, fill: '#526281' }} />
        <Tooltip />
        {compare && <Legend verticalAlign="top" align="right" height={28} iconType="plainline" />}
        <Line
          type="monotone"
          dataKey="current"
          name={seriesName}
          stroke={CURRENT_COLOR}
          strokeWidth={2}
          dot={false}
          isAnimationActive={false}
        />
        {compare && (
          <Line
            type="monotone"
            dataKey="previous"
            name="Previous period"
            stroke={PREVIOUS_COLOR}
            strokeWidth={2}
            strokeDasharray="5 4"
            dot={false}
            isAnimationActive={false}
          />
        )}
      </LineChart>
    </ResponsiveContainer>
  )
}
