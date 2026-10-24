'use client'

import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from 'recharts'
import { formatNumber, pct } from '@/lib/format'
import { NoData } from './NoData'
import { DOUGHNUT_PALETTE } from './palette'

export interface DoughnutRow {
  label: string
  value: number
}

function sliceColor(i: number): string {
  return DOUGHNUT_PALETTE[i % DOUGHNUT_PALETTE.length]
}

/** Doughnut of shares with a `label · value · %` legend on the right; NoData when every value is 0. */
export function DoughnutChart({ rows, height = 240 }: { rows: DoughnutRow[]; height?: number }) {
  const total = rows.reduce((sum, r) => sum + r.value, 0)
  if (!rows.length || total <= 0) return <NoData style={{ height }} />

  return (
    <div className="flex flex-col items-center gap-6 sm:flex-row">
      <div className="w-full sm:w-1/2" style={{ height }}>
        <ResponsiveContainer width="100%" height={height} initialDimension={{ width: 240, height }}>
          <PieChart>
            <Pie
              data={rows}
              dataKey="value"
              nameKey="label"
              innerRadius="60%"
              outerRadius="90%"
              paddingAngle={1}
              stroke="none"
              isAnimationActive={false}
            >
              {rows.map((r, i) => (
                <Cell key={r.label} fill={sliceColor(i)} />
              ))}
            </Pie>
            <Tooltip />
          </PieChart>
        </ResponsiveContainer>
      </div>
      <ul aria-label="Legend" className="w-full space-y-2 text-sm sm:w-1/2">
        {rows.map((r, i) => (
          <li key={r.label} className="flex items-center gap-2">
            <span aria-hidden className="size-2.5 shrink-0 rounded-full" style={{ backgroundColor: sliceColor(i) }} />
            <span className="min-w-0 flex-1 truncate">{r.label}</span>
            <span className="tabular-nums">{formatNumber(r.value)}</span>
            <span className="w-12 text-right tabular-nums text-muted-foreground">{pct(r.value, total)}</span>
          </li>
        ))}
      </ul>
    </div>
  )
}
