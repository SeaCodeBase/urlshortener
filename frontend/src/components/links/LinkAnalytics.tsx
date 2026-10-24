'use client'

import { useState, type ReactNode } from 'react'
import { usePathname, useRouter, useSearchParams } from 'next/navigation'
import { differenceInCalendarDays, format, isValid, parseISO, subDays } from 'date-fns'
import { Calendar, ChevronDown, Download } from 'lucide-react'
import { BreakdownTable, type BreakdownRow } from '@/components/charts/BreakdownTable'
import { DeviceBarChart, capitalize } from '@/components/charts/DeviceBarChart'
import { TimeSeriesChart } from '@/components/charts/TimeSeriesChart'
import { DateRangeDialog } from '@/components/common/DateRangeDialog'
import { StatCard } from '@/components/common/StatCard'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { formatNumber } from '@/lib/format'
import { useLinkStats } from '@/lib/hooks'
import type { LinkStatsQuery, LinkStatsV2, StatsSource, StatsUnit } from '@/types'

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/
const DAY = 'yyyy-MM-dd'
const DEFAULT_LABEL = 'Last 30 days'
const SOURCES: { value: StatsSource; label: string }[] = [
  { value: 'all', label: 'All engagements' },
  { value: 'link', label: 'Clicks' },
  { value: 'qr', label: 'QR scans' },
  { value: 'page', label: 'Page clicks' },
]

type Compare = 'previous' | 'none'

/** Analytics controls as they live in the URL (`?from&to&compare&source`). */
export interface AnalyticsUrlState {
  /** YYYY-MM-DD, both null for the default "Last 30 days". */
  from: string | null
  to: string | null
  compare: Compare
  source: StatsSource
}

function validDay(v: string | null): string | null {
  return v && DATE_RE.test(v) && isValid(parseISO(v)) ? v : null
}

export function parseAnalyticsUrlState(sp: Pick<URLSearchParams, 'get'>): AnalyticsUrlState {
  let from = validDay(sp.get('from'))
  let to = validDay(sp.get('to'))
  if (!from || !to || from > to) from = to = null
  const source = sp.get('source')
  return {
    from,
    to,
    compare: sp.get('compare') === 'previous' ? 'previous' : 'none',
    source: SOURCES.some((s) => s.value === source) ? (source as StatsSource) : 'all',
  }
}

/** The effective range: the URL's, or the last 30 days ending today. */
export function effectiveRange(state: Pick<AnalyticsUrlState, 'from' | 'to'>, now = new Date()): { from: string; to: string } {
  if (state.from && state.to) return { from: state.from, to: state.to }
  return { from: format(subDays(now, 29), DAY), to: format(now, DAY) }
}

/** Hourly buckets for ranges of up to 2 days, daily otherwise (backend rule). */
export function unitFor(from: string, to: string): StatsUnit {
  return differenceInCalendarDays(parseISO(to), parseISO(from)) + 1 <= 2 ? 'hour' : 'day'
}

/** "Last 30 days" by default; "Today" / "Last N days" for the presets ending today; else the range. */
export function rangeLabel(state: Pick<AnalyticsUrlState, 'from' | 'to'>, now = new Date()): string {
  if (!state.from || !state.to) return DEFAULT_LABEL
  const from = parseISO(state.from)
  const to = parseISO(state.to)
  const today = format(now, DAY)
  if (state.to === today) {
    const days = differenceInCalendarDays(to, from) + 1
    if (days === 1) return 'Today'
    if ([7, 30, 60, 90].includes(days)) return `Last ${days} days`
  }
  if (state.from === state.to) return format(from, 'MMM d, yyyy')
  if (from.getFullYear() === to.getFullYear()) return `${format(from, 'MMM d')} – ${format(to, 'MMM d, yyyy')}`
  return `${format(from, 'MMM d, yyyy')} – ${format(to, 'MMM d, yyyy')}`
}

/** `US` → 🇺🇸 (empty for anything that isn't a 2-letter code). */
export function flagEmoji(code: string): string {
  const c = code.toUpperCase()
  if (!/^[A-Z]{2}$/.test(c)) return ''
  return String.fromCodePoint(...[...c].map((ch) => 0x1f1e6 + ch.charCodeAt(0) - 65))
}

function hostOf(url: string): string {
  try {
    return new URL(url).hostname
  } catch {
    return url
  }
}

function csvCell(v: string | number): string {
  const s = String(v)
  return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s
}

/** CSV of the loaded stats: one titled section per module, separated by blank lines. */
export function buildStatsCsv(stats: LinkStatsV2, label: string): string {
  const lines: string[] = []
  const section = (title: string, header: string[], rows: (string | number)[][]) => {
    if (lines.length) lines.push('')
    lines.push(csvCell(title), header.map(csvCell).join(','))
    for (const r of rows) lines.push(r.map(csvCell).join(','))
  }
  section('Summary', ['Metric', 'Value'], [
    ['Range', `${stats.from} to ${stats.to}`],
    ['Total engagements', stats.total_all_time],
    [`Engagements (${label})`, stats.total_range],
  ])
  section('Engagements over time', ['Date', 'Engagements'], stats.series.map((p) => [p.date, p.clicks]))
  section('Countries', ['Country', 'Code', 'Engagements', '%'], stats.countries.map((c) => [c.name, c.code, c.clicks, c.percentage.toFixed(1)]))
  section('Cities', ['City', 'Country', 'Engagements', '%'], stats.cities.map((c) => [c.name, c.country, c.clicks, c.percentage.toFixed(1)]))
  section('Referrers', ['Referrer', 'Engagements', '%'], stats.referrers.map((r) => [r.referrer || 'Direct', r.clicks, r.percentage.toFixed(1)]))
  section('Devices', ['Device', 'Engagements', '%'], stats.devices.map((d) => [capitalize(d.key), d.clicks, d.percentage.toFixed(1)]))
  section('Browsers', ['Browser', 'Engagements', '%'], stats.browsers.map((d) => [d.key || 'Unknown', d.clicks, d.percentage.toFixed(1)]))
  section('Operating systems', ['Operating system', 'Engagements', '%'], stats.os.map((d) => [d.key || 'Unknown', d.clicks, d.percentage.toFixed(1)]))
  return lines.join('\n') + '\n'
}

function downloadCsv(filename: string, csv: string) {
  const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }))
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

function Module({ title, children, className }: { title: string; children: ReactNode; className?: string }) {
  return (
    <section className={`rounded-xl bg-card p-6 ${className ?? ''}`}>
      <h3 className="mb-4 text-lg font-semibold text-foreground">{title}</h3>
      {children}
    </section>
  )
}

function ModuleSkeleton({ height = 160 }: { height?: number }) {
  return <Skeleton className="w-full" style={{ height }} />
}

const keyRows = (rows: LinkStatsV2['browsers'], fmt: (k: string) => string = (k) => k || 'Unknown'): BreakdownRow[] =>
  rows.map((r) => ({ key: r.key || '(unknown)', label: fmt(r.key), clicks: r.clicks, percentage: r.percentage, prev_clicks: r.prev_clicks }))

/** "Link analytics - {host}" section of the link details page (spec §5.4). */
export function LinkAnalytics({ linkId, destination }: { linkId: number; destination: string }) {
  const router = useRouter()
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const state = parseAnalyticsUrlState(searchParams)
  const [dateOpen, setDateOpen] = useState(false)

  const range = effectiveRange(state)
  const unit = unitFor(range.from, range.to)
  const comparing = state.compare === 'previous'
  const label = rangeLabel(state)
  const query: LinkStatsQuery = { ...range, unit, compare: state.compare, source: state.source }
  const { data: stats, isLoading } = useLinkStats(linkId, query)
  const loading = isLoading && !stats

  function update(patch: Partial<AnalyticsUrlState>) {
    const next = { ...state, ...patch }
    const sp = new URLSearchParams(searchParams.toString())
    const set = (k: string, v: string | null) => (v ? sp.set(k, v) : sp.delete(k))
    set('from', next.from)
    set('to', next.to)
    set('compare', next.compare === 'previous' ? 'previous' : null)
    set('source', next.source === 'all' ? null : next.source)
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
    <section aria-labelledby="link-analytics-heading" className="space-y-4">
      <h2 id="link-analytics-heading" className="truncate text-2xl font-semibold text-foreground">
        Link analytics - {hostOf(destination)}
      </h2>

      <div className="flex flex-wrap items-center gap-2">
        <Button type="button" variant="secondary" onClick={() => setDateOpen(true)}>
          <Calendar className="size-4" />
          {label}
          <ChevronDown className="size-4 text-muted-foreground" />
        </Button>
        <Select value={state.compare} onValueChange={(v) => update({ compare: v as Compare })}>
          <SelectTrigger aria-label="Comparison" className="h-10 w-48 bg-white">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="none">No comparison</SelectItem>
            <SelectItem value="previous">Previous period</SelectItem>
          </SelectContent>
        </Select>
        <Select value={state.source} onValueChange={(v) => update({ source: v as StatsSource })}>
          <SelectTrigger aria-label="Source" className="h-10 w-48 bg-white">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {SOURCES.map((s) => (
              <SelectItem key={s.value} value={s.value}>
                {s.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button
          type="button"
          variant="secondary"
          size="icon"
          aria-label="Download CSV"
          className="ml-auto"
          disabled={!stats}
          onClick={() => stats && downloadCsv(`link-${linkId}-analytics-${range.from}-${range.to}.csv`, buildStatsCsv(stats, label))}
        >
          <Download className="size-5" />
        </Button>
      </div>

      <DateRangeDialog
        open={dateOpen}
        onOpenChange={setDateOpen}
        title="Select date range"
        value={{ from: parseISO(range.from), to: parseISO(range.to) }}
        onApply={(r) => update({ from: format(r.from, DAY), to: format(r.to, DAY) })}
        onClear={state.from ? () => update({ from: null, to: null }) : undefined}
      />

      <div className="grid gap-4 md:grid-cols-2">
        {loading ? (
          <>
            <Skeleton className="h-[124px] rounded-xl" />
            <Skeleton className="h-[124px] rounded-xl" />
          </>
        ) : (
          <>
            <StatCard label="Total engagements" value={formatNumber(stats?.total_all_time ?? 0)} />
            <StatCard
              label={`Engagements (${label})`}
              value={formatNumber(stats?.total_range ?? 0)}
              change={comparing ? { pct: stats?.change_pct ?? null, label: 'vs previous period' } : undefined}
              spark={stats?.series.map((p) => p.clicks)}
            />
          </>
        )}
      </div>

      <Module title="Engagements over time">
        {loading ? (
          <ModuleSkeleton height={280} />
        ) : (
          <TimeSeriesChart
            series={stats?.series ?? []}
            prevSeries={stats?.prev_series ?? []}
            unit={stats?.unit ?? unit}
            compare={comparing}
          />
        )}
      </Module>

      <div className="grid gap-4 lg:grid-cols-2">
        <Module title="Locations">
          {loading ? (
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
        <Module title="Referrers">
          {loading ? <ModuleSkeleton /> : <BreakdownTable labelHeader="Referrer" rows={referrerRows} compare={comparing} />}
        </Module>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Module title="Devices">
          {loading ? <ModuleSkeleton /> : <DeviceBarChart devices={stats?.devices ?? []} compare={comparing} />}
        </Module>
        <Module title="Browsers">
          {loading ? (
            <ModuleSkeleton />
          ) : (
            <BreakdownTable labelHeader="Browser" rows={keyRows(stats?.browsers ?? [])} compare={comparing} />
          )}
        </Module>
      </div>

      <Module title="Operating systems">
        {loading ? (
          <ModuleSkeleton />
        ) : (
          <BreakdownTable labelHeader="Operating system" rows={keyRows(stats?.os ?? [])} compare={comparing} />
        )}
      </Module>
    </section>
  )
}
