'use client'

import type { ButtonHTMLAttributes, ReactNode, Ref } from 'react'
import { format, parseISO } from 'date-fns'
import { Download, GripVertical, MoreHorizontal } from 'lucide-react'
import { BarBreakdownChart, type BarBreakdownRow } from '@/components/charts/BarBreakdownChart'
import { BreakdownTable, type BreakdownRow } from '@/components/charts/BreakdownTable'
import { DoughnutChart } from '@/components/charts/DoughnutChart'
import { NoData } from '@/components/charts/NoData'
import { TimeSeriesChart, formatBucket } from '@/components/charts/TimeSeriesChart'
import { StatCard } from '@/components/common/StatCard'
import { flagEmoji } from '@/components/links/LinkAnalytics'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { downloadBlob } from '@/lib/download'
import { formatNumber, formatShortUrl } from '@/lib/format'
import { cn } from '@/lib/utils'
import type { DashboardData, ModuleKey, ModuleViz, StatsUnit } from '@/types'
import { moduleCsv, slugify } from './dashboardCsv'
import { ItemsTable } from './ItemsTable'
import { MODULES } from './modules'

export const VIZ_LABELS: Record<ModuleViz, string> = {
  scorecard: 'Scorecard',
  line: 'Line chart',
  bar: 'Bar chart',
  doughnut: 'Doughnut chart',
  table: 'Table',
}

/** Props spread on the drag handle button (dnd-kit attributes, listeners and activator ref). */
export type DragHandleProps = ButtonHTMLAttributes<HTMLButtonElement> & { ref?: Ref<HTMLButtonElement> }

export interface ModuleCardProps {
  moduleKey: ModuleKey
  viz: ModuleViz
  data: DashboardData
  compare: boolean
  unit: StatsUnit
  onChangeViz: (viz: ModuleViz) => void
  /** Renders the drag handle when set. */
  dragHandleProps?: DragHandleProps
  className?: string
}

/** Day bucket → `Oct 20, 2026`; hour bucket (`2026-10-20T13:00`) → `Oct 20, 13:00`. */
export function bucketLabel(date: string, unit: StatsUnit): string {
  const day = parseISO(date.slice(0, 10))
  if (Number.isNaN(day.getTime())) return date
  return unit === 'hour' ? `${format(day, 'MMM d')}, ${formatBucket(date, 'hour')}` : format(day, 'MMM d, yyyy')
}

const cap = (s: string) => (s ? s.charAt(0).toUpperCase() + s.slice(1) : 'Unknown')

type Breakdown = { key: string; label: string; display?: ReactNode; clicks: number; percentage: number; prev_clicks?: number }

/** Chart views (bar/doughnut) show at most this many rows plus "Other"; tables and CSVs keep every row. */
export const CHART_MAX_ROWS = 8

/**
 * Keeps the first CHART_MAX_ROWS rows (already sorted by engagements) and
 * folds the rest into one "Other" row summing clicks, percentage and, when
 * comparing, prev_clicks.
 */
export function capChartRows(rows: Breakdown[], compare: boolean): Breakdown[] {
  if (rows.length <= CHART_MAX_ROWS) return rows
  const rest = rows.slice(CHART_MAX_ROWS)
  const other: Breakdown = {
    key: '__other',
    label: 'Other',
    clicks: rest.reduce((sum, r) => sum + r.clicks, 0),
    percentage: Math.round(rest.reduce((sum, r) => sum + r.percentage, 0) * 10) / 10,
  }
  if (compare) other.prev_clicks = rest.reduce((sum, r) => sum + (r.prev_clicks ?? 0), 0)
  return [...rows.slice(0, CHART_MAX_ROWS), other]
}

const toTable = (rows: Breakdown[]): BreakdownRow[] =>
  rows.map((r) => ({ key: r.key, label: r.display ?? r.label, clicks: r.clicks, percentage: r.percentage, prev_clicks: r.prev_clicks }))
/** Bar rows of a chart view, capped to the top rows plus "Other". */
const toBars = (rows: Breakdown[], compare: boolean): BarBreakdownRow[] =>
  capChartRows(rows, compare).map((r) => ({ label: r.label, value: r.clicks, prev: r.prev_clicks }))

function breakdownView(viz: ModuleViz, labelHeader: string, rows: Breakdown[], compare: boolean): ReactNode {
  if (viz === 'doughnut') return <DoughnutChart rows={capChartRows(rows, false).map((r) => ({ label: r.label, value: r.clicks }))} />
  if (viz === 'bar') return <BarBreakdownChart rows={toBars(rows, compare)} compare={compare} />
  return <BreakdownTable labelHeader={labelHeader} rows={toTable(rows)} compare={compare} />
}

function countryRows(data: DashboardData): Breakdown[] {
  return data.countries.map((c) => ({
    key: c.code || c.name,
    label: c.name || 'Unknown',
    display: (
      <span className="inline-flex items-center gap-2">
        <span aria-hidden>{flagEmoji(c.code)}</span>
        {c.name || 'Unknown'}
      </span>
    ),
    clicks: c.clicks,
    percentage: c.percentage,
    prev_clicks: c.prev_clicks,
  }))
}

function ModuleBody({ moduleKey, viz, data, compare, unit }: Omit<ModuleCardProps, 'onChangeViz' | 'dragHandleProps'>) {
  const change = (pct: number | null) => (compare ? { pct, label: 'vs previous period' } : undefined)
  switch (moduleKey) {
    case 'top_day': {
      const d = data.top_day
      if (!d) return <NoData className="min-h-24" />
      return (
        <StatCard
          className="bg-transparent p-0"
          label={`${formatNumber(d.count)} engagements`}
          value={bucketLabel(d.date, unit)}
          change={change(d.change_pct)}
        />
      )
    }
    case 'top_location': {
      const l = data.top_location
      if (!l) return <NoData className="min-h-24" />
      return (
        <StatCard
          className="bg-transparent p-0"
          label={`${formatNumber(l.count)} engagements`}
          value={
            <span className="inline-flex items-center gap-2">
              <span aria-hidden>{flagEmoji(l.code)}</span>
              {l.name || 'Unknown'}
            </span>
          }
        />
      )
    }
    case 'over_time': {
      if (viz === 'table') {
        const rows = data.series.map((p, i) => ({
          key: p.date,
          label: bucketLabel(p.date, unit),
          clicks: p.clicks,
          percentage: data.total ? (p.clicks / data.total) * 100 : 0,
          prev_clicks: data.prev_series[i]?.clicks,
        }))
        return <BreakdownTable labelHeader="Date" rows={rows} compare={compare} />
      }
      return (
        <TimeSeriesChart
          series={data.series}
          prevSeries={data.prev_series}
          unit={unit}
          compare={compare}
          variant={viz === 'bar' ? 'bar' : 'line'}
        />
      )
    }
    case 'devices':
      return breakdownView(
        viz,
        'Device',
        data.devices.map((d) => ({ key: d.key || '(unknown)', label: cap(d.key), clicks: d.clicks, percentage: d.percentage, prev_clicks: d.prev_clicks })),
        compare,
      )
    case 'referrers':
      return breakdownView(
        viz,
        'Referrer',
        data.referrers.map((r) => ({
          key: r.referrer || '(direct)',
          label: r.referrer || 'Direct',
          clicks: r.clicks,
          percentage: r.percentage,
          prev_clicks: r.prev_clicks,
        })),
        compare,
      )
    case 'locations': {
      if (viz === 'bar') return <BarBreakdownChart rows={toBars(countryRows(data), compare)} compare={compare} />
      const cities: BreakdownRow[] = data.cities.map((c) => ({
        key: `${c.name}\u0000${c.country}`,
        label: c.name ? `${c.name}${c.country ? `, ${c.country}` : ''}` : 'Unknown',
        clicks: c.clicks,
        percentage: c.percentage,
        prev_clicks: c.prev_clicks,
      }))
      return (
        <Tabs defaultValue="countries">
          <TabsList className="mb-2">
            <TabsTrigger value="countries">Countries</TabsTrigger>
            <TabsTrigger value="cities">Cities</TabsTrigger>
          </TabsList>
          <TabsContent value="countries">
            <BreakdownTable labelHeader="Country" rows={toTable(countryRows(data))} compare={compare} />
          </TabsContent>
          <TabsContent value="cities">
            <BreakdownTable labelHeader="City" rows={cities} compare={compare} />
          </TabsContent>
        </Tabs>
      )
    }
    case 'items':
      if (viz === 'bar') {
        return (
          <BarBreakdownChart
            rows={toBars(
              data.items.map((it) => ({
                key: `${it.type}:${it.id}`,
                label: it.title || formatShortUrl(it.short_url),
                clicks: it.clicks,
                percentage: it.percentage,
                prev_clicks: it.prev_clicks,
              })),
              compare,
            )}
            compare={compare}
          />
        )
      }
      return <ItemsTable items={data.items} compare={compare} />
  }
}

/** One dashboard module: title, drag handle, CSV download, "Change visualization" menu and the chart/table. */
export function ModuleCard({ moduleKey, viz, data, compare, unit, onChangeViz, dragHandleProps, className }: ModuleCardProps) {
  const { title, allowed } = MODULES[moduleKey]
  const download = () =>
    downloadBlob(`${slugify(title)}.csv`, new Blob([moduleCsv(moduleKey, data, compare)], { type: 'text/csv;charset=utf-8' }))

  return (
    <section className={cn('flex h-full flex-col rounded-xl bg-card p-6', className)}>
      <div className="mb-4 flex items-center gap-2">
        {dragHandleProps && (
          <button
            type="button"
            {...dragHandleProps}
            aria-label={`Reorder ${title}`}
            className="-ml-2 inline-flex size-8 cursor-grab touch-none items-center justify-center rounded-md text-muted-foreground hover:bg-muted active:cursor-grabbing"
          >
            <GripVertical className="size-4" />
          </button>
        )}
        <h3 className="min-w-0 flex-1 truncate text-lg font-semibold text-foreground">{title}</h3>
        <Button type="button" variant="ghost" size="icon-sm" aria-label={`Download ${title} CSV`} onClick={download}>
          <Download className="size-4" />
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button type="button" variant="ghost" size="icon-sm" aria-label={`${title} options`}>
              <MoreHorizontal className="size-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuSub>
              <DropdownMenuSubTrigger>Change visualization</DropdownMenuSubTrigger>
              <DropdownMenuSubContent>
                <DropdownMenuRadioGroup value={viz} onValueChange={(v) => v !== viz && onChangeViz(v as ModuleViz)}>
                  {allowed.map((v) => (
                    <DropdownMenuRadioItem key={v} value={v}>
                      {VIZ_LABELS[v]}
                    </DropdownMenuRadioItem>
                  ))}
                </DropdownMenuRadioGroup>
              </DropdownMenuSubContent>
            </DropdownMenuSub>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <div className="min-w-0 flex-1">
        <ModuleBody moduleKey={moduleKey} viz={viz} data={data} compare={compare} unit={unit} />
      </div>
    </section>
  )
}
