'use client'

import { format, parseISO } from 'date-fns'
import { Bar, BarChart, CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import type { SeriesPoint, StatsUnit } from '@/types'
import { NoData } from './NoData'
import { CURRENT_COLOR, DOUGHNUT_PALETTE, EXTRA_COLOR, PREVIOUS_COLOR } from './palette'

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
  extra?: number
}

const AXIS_TICK = { fontSize: 12, fill: '#526281' }

/** Colour of the `i`-th entry of `multiSeries`, cycling through the chart palette. */
export function multiSeriesColor(i: number): string {
  return DOUGHNUT_PALETTE[i % DOUGHNUT_PALETTE.length]
}

type MultiRow = { label: string } & Record<string, number | string>

/**
 * Engagements over time: current period solid blue, previous period dashed light blue (aligned by bucket index),
 * optional extra series solid teal. `variant="bar"` draws grouped bars in the same colours.
 */
export function TimeSeriesChart({
  series,
  prevSeries = [],
  unit,
  compare = false,
  height = 280,
  seriesName = 'Engagements',
  prevSeriesName = 'Previous period',
  extraSeries,
  multiSeries,
  variant = 'line',
}: {
  series: SeriesPoint[]
  prevSeries?: SeriesPoint[]
  unit: StatsUnit
  compare?: boolean
  height?: number
  /** Legend/tooltip name of the current period's line. */
  seriesName?: string
  /** Legend/tooltip name of the previous period's line. */
  prevSeriesName?: string
  /** A third series aligned by bucket index (e.g. a compared link). */
  extraSeries?: { name: string; points: SeriesPoint[] }
  /**
   * N named series aligned by bucket index (e.g. one per campaign channel). When given,
   * `series`/`prevSeries`/`extraSeries` are ignored and the legend lists each name.
   */
  multiSeries?: { name: string; points: SeriesPoint[] }[]
  variant?: 'line' | 'bar'
}) {
  if (multiSeries) return <MultiSeriesChart multiSeries={multiSeries} unit={unit} height={height} variant={variant} />
  if (!series.length) return <NoData style={{ height }} />

  const data: Row[] = series.map((p, i) => ({
    label: formatBucket(p.date, unit),
    current: p.clicks,
    ...(compare ? { previous: prevSeries[i]?.clicks ?? 0 } : {}),
    ...(extraSeries ? { extra: extraSeries.points[i]?.clicks ?? 0 } : {}),
  }))
  const showLegend = compare || extraSeries != null
  const margin = { top: 8, right: 16, bottom: 0, left: -16 }
  const axes = [
    <CartesianGrid key="grid" vertical={false} stroke="#E1E4EC" />,
    <XAxis key="x" dataKey="label" tickLine={false} axisLine={false} tick={AXIS_TICK} minTickGap={16} />,
    <YAxis key="y" allowDecimals={false} tickLine={false} axisLine={false} tick={AXIS_TICK} />,
  ]

  if (variant === 'bar') {
    return (
      <ResponsiveContainer width="100%" height={height} initialDimension={{ width: 600, height }}>
        <BarChart data={data} margin={margin}>
          {axes}
          <Tooltip cursor={{ fill: '#F4F6FA' }} />
          {showLegend && <Legend verticalAlign="top" align="right" height={28} iconType="square" />}
          <Bar dataKey="current" name={seriesName} fill={CURRENT_COLOR} radius={[4, 4, 0, 0]} maxBarSize={32} isAnimationActive={false} />
          {compare && (
            <Bar dataKey="previous" name={prevSeriesName} fill={PREVIOUS_COLOR} radius={[4, 4, 0, 0]} maxBarSize={32} isAnimationActive={false} />
          )}
          {extraSeries && (
            <Bar dataKey="extra" name={extraSeries.name} fill={EXTRA_COLOR} radius={[4, 4, 0, 0]} maxBarSize={32} isAnimationActive={false} />
          )}
        </BarChart>
      </ResponsiveContainer>
    )
  }

  return (
    <ResponsiveContainer width="100%" height={height} initialDimension={{ width: 600, height }}>
      <LineChart data={data} margin={margin}>
        {axes}
        <Tooltip />
        {showLegend && <Legend verticalAlign="top" align="right" height={28} iconType="plainline" />}
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
            name={prevSeriesName}
            stroke={PREVIOUS_COLOR}
            strokeWidth={2}
            strokeDasharray="5 4"
            dot={false}
            isAnimationActive={false}
          />
        )}
        {extraSeries && (
          <Line
            type="monotone"
            dataKey="extra"
            name={extraSeries.name}
            stroke={EXTRA_COLOR}
            strokeWidth={2}
            dot={false}
            isAnimationActive={false}
          />
        )}
      </LineChart>
    </ResponsiveContainer>
  )
}

function MultiSeriesChart({
  multiSeries,
  unit,
  height,
  variant,
}: {
  multiSeries: { name: string; points: SeriesPoint[] }[]
  unit: StatsUnit
  height: number
  variant: 'line' | 'bar'
}) {
  const longest = multiSeries.reduce<SeriesPoint[]>((acc, s) => (s.points.length > acc.length ? s.points : acc), [])
  if (!longest.length) return <NoData style={{ height }} />

  const data: MultiRow[] = longest.map((p, i) => {
    const row: MultiRow = { label: formatBucket(p.date, unit) }
    multiSeries.forEach((s, j) => {
      row[`s${j}`] = s.points[i]?.clicks ?? 0
    })
    return row
  })
  const margin = { top: 8, right: 16, bottom: 0, left: -16 }
  const axes = [
    <CartesianGrid key="grid" vertical={false} stroke="#E1E4EC" />,
    <XAxis key="x" dataKey="label" tickLine={false} axisLine={false} tick={AXIS_TICK} minTickGap={16} />,
    <YAxis key="y" allowDecimals={false} tickLine={false} axisLine={false} tick={AXIS_TICK} />,
  ]

  if (variant === 'bar') {
    return (
      <ResponsiveContainer width="100%" height={height} initialDimension={{ width: 600, height }}>
        <BarChart data={data} margin={margin}>
          {axes}
          <Tooltip cursor={{ fill: '#F4F6FA' }} />
          <Legend verticalAlign="top" align="right" height={28} iconType="square" itemSorter={null} />
          {multiSeries.map((s, j) => (
            <Bar
              key={j}
              dataKey={`s${j}`}
              name={s.name}
              fill={multiSeriesColor(j)}
              radius={[4, 4, 0, 0]}
              maxBarSize={32}
              isAnimationActive={false}
            />
          ))}
        </BarChart>
      </ResponsiveContainer>
    )
  }

  return (
    <ResponsiveContainer width="100%" height={height} initialDimension={{ width: 600, height }}>
      <LineChart data={data} margin={margin}>
        {axes}
        <Tooltip />
        {/* Keep the caller's order (recharts sorts legend items by name by default). */}
        <Legend verticalAlign="top" align="right" height={28} iconType="plainline" itemSorter={null} />
        {multiSeries.map((s, j) => (
          <Line
            key={j}
            type="monotone"
            dataKey={`s${j}`}
            name={s.name}
            stroke={multiSeriesColor(j)}
            strokeWidth={2}
            dot={false}
            isAnimationActive={false}
          />
        ))}
      </LineChart>
    </ResponsiveContainer>
  )
}
