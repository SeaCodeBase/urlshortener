'use client'

import { Bar, BarChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import type { KeyStat } from '@/types'
import { NoData } from './NoData'
import { CURRENT_COLOR, PREVIOUS_COLOR } from './palette'

/** `mobile` → `Mobile`. */
export function capitalize(s: string): string {
  return s ? s.charAt(0).toUpperCase() + s.slice(1) : 'Unknown'
}

/** Devices bar chart: current (and previous when comparing) engagements per device type. */
export function DeviceBarChart({ devices, compare = false, height = 240 }: { devices: KeyStat[]; compare?: boolean; height?: number }) {
  if (!devices.length) return <NoData style={{ height }} />

  const data = devices.map((d) => ({
    name: capitalize(d.key),
    current: d.clicks,
    ...(compare ? { previous: d.prev_clicks } : {}),
  }))

  return (
    <ResponsiveContainer width="100%" height={height} initialDimension={{ width: 400, height }}>
      <BarChart data={data} margin={{ top: 8, right: 16, bottom: 0, left: -16 }}>
        <CartesianGrid vertical={false} stroke="#E1E4EC" />
        <XAxis dataKey="name" tickLine={false} axisLine={false} tick={{ fontSize: 12, fill: '#526281' }} />
        <YAxis allowDecimals={false} tickLine={false} axisLine={false} tick={{ fontSize: 12, fill: '#526281' }} />
        <Tooltip cursor={{ fill: '#F4F6FA' }} />
        {compare && <Legend verticalAlign="top" align="right" height={28} iconType="square" />}
        <Bar dataKey="current" name="Engagements" fill={CURRENT_COLOR} radius={[4, 4, 0, 0]} maxBarSize={48} isAnimationActive={false} />
        {compare && (
          <Bar dataKey="previous" name="Previous period" fill={PREVIOUS_COLOR} radius={[4, 4, 0, 0]} maxBarSize={48} isAnimationActive={false} />
        )}
      </BarChart>
    </ResponsiveContainer>
  )
}
