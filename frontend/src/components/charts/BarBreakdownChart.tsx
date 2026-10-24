'use client'

import { Bar, BarChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { NoData } from './NoData'
import { CURRENT_COLOR, PREVIOUS_COLOR } from './palette'

export interface BarBreakdownRow {
  label: string
  value: number
  prev?: number
}

const ROW_HEIGHT = 32

/** Longest axis label drawn in full; longer ones end in "…". */
export const MAX_LABEL_CHARS = 24

/** `label` cut to MAX_LABEL_CHARS characters (the last being "…") when longer. */
export function truncateLabel(label: string): string {
  const chars = Array.from(label)
  return chars.length > MAX_LABEL_CHARS ? `${chars.slice(0, MAX_LABEL_CHARS - 1).join('')}…` : label
}

/** Y-axis category tick: the truncated label, with the full label as a hover `title`. */
export function CategoryTick({ x, y, payload }: { x?: number | string; y?: number | string; payload?: { value?: unknown } }) {
  const label = String(payload?.value ?? '')
  return (
    <text x={x} y={y} dy={4} textAnchor="end" fontSize={12} fill="#526281">
      <title>{label}</title>
      {truncateLabel(label)}
    </text>
  )
}

/** Horizontal bars per label: current engagements (and previous period when comparing). */
export function BarBreakdownChart({
  rows,
  compare = false,
  height,
}: {
  rows: BarBreakdownRow[]
  compare?: boolean
  /** Defaults to a height that fits every row. */
  height?: number
}) {
  const h = height ?? Math.max(160, rows.length * (compare ? ROW_HEIGHT * 1.5 : ROW_HEIGHT) + 40)
  if (!rows.length) return <NoData style={{ height: height ?? 240 }} />

  const data = rows.map((r) => ({
    label: r.label,
    current: r.value,
    ...(compare ? { previous: r.prev ?? 0 } : {}),
  }))

  return (
    <ResponsiveContainer width="100%" height={h} initialDimension={{ width: 400, height: h }}>
      <BarChart data={data} layout="vertical" margin={{ top: 8, right: 16, bottom: 0, left: 8 }}>
        <CartesianGrid horizontal={false} stroke="#E1E4EC" />
        <XAxis type="number" allowDecimals={false} tickLine={false} axisLine={false} tick={{ fontSize: 12, fill: '#526281' }} />
        <YAxis
          type="category"
          dataKey="label"
          width={160}
          tickLine={false}
          axisLine={false}
          tick={<CategoryTick />}
        />
        <Tooltip cursor={{ fill: '#F4F6FA' }} />
        {compare && <Legend verticalAlign="top" align="right" height={28} iconType="square" />}
        <Bar dataKey="current" name="Engagements" fill={CURRENT_COLOR} radius={[0, 4, 4, 0]} maxBarSize={20} isAnimationActive={false} />
        {compare && (
          <Bar dataKey="previous" name="Previous period" fill={PREVIOUS_COLOR} radius={[0, 4, 4, 0]} maxBarSize={20} isAnimationActive={false} />
        )}
      </BarChart>
    </ResponsiveContainer>
  )
}
