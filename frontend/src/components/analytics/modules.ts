import { format, subDays } from 'date-fns'
import type { DashboardDateRange, DashboardLayout, ModuleKey, ModuleViz } from '@/types'

/** Dashboard module registry: title (spec §5.8 copy) and allowed visualizations, the first being the default. */
export const MODULES: Record<ModuleKey, { title: string; allowed: ModuleViz[] }> = {
  top_day: { title: 'Top day by engagements', allowed: ['scorecard'] },
  top_location: { title: 'Top location by engagements', allowed: ['scorecard'] },
  over_time: { title: 'Engagements over time', allowed: ['line', 'bar', 'table'] },
  devices: { title: 'Engagements by device', allowed: ['doughnut', 'bar', 'table'] },
  referrers: { title: 'Engagements by referrer', allowed: ['bar', 'doughnut', 'table'] },
  locations: { title: 'Engagements by location', allowed: ['table', 'bar'] },
  items: { title: 'Links & QR Codes by engagements', allowed: ['table', 'bar'] },
}

/** The layout of a new dashboard (same as the backend default). */
export const DEFAULT_LAYOUT: DashboardLayout = {
  modules: [
    { key: 'top_day', viz: 'scorecard' },
    { key: 'top_location', viz: 'scorecard' },
    { key: 'over_time', viz: 'line' },
    { key: 'devices', viz: 'doughnut' },
    { key: 'referrers', viz: 'bar' },
    { key: 'locations', viz: 'table' },
    { key: 'items', viz: 'table' },
  ],
}

const RANGE_DAYS: Record<DashboardDateRange, number> = {
  today: 1,
  last_7_days: 7,
  last_30_days: 30,
  last_60_days: 60,
  last_90_days: 90,
}

export const DATE_RANGE_OPTIONS: { value: DashboardDateRange; label: string }[] = [
  { value: 'today', label: 'Today' },
  { value: 'last_7_days', label: 'Last 7 days' },
  { value: 'last_30_days', label: 'Last 30 days' },
  { value: 'last_60_days', label: 'Last 60 days' },
  { value: 'last_90_days', label: 'Last 90 days' },
]

/** Inclusive `yyyy-MM-dd` window of a saved date range ending today (`now`'s calendar day). */
export function rangeFor(dateRange: DashboardDateRange, now: Date = new Date()): { from: string; to: string } {
  const days = RANGE_DAYS[dateRange] ?? RANGE_DAYS.last_7_days
  return {
    from: format(subDays(now, days - 1), 'yyyy-MM-dd'),
    to: format(now, 'yyyy-MM-dd'),
  }
}
