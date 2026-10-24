import { csvRow } from '@/lib/csv'
import type { DashboardData, ModuleKey } from '@/types'

type Cell = string | number

const pct = (p: number) => Number(p.toFixed(1))
const cap = (s: string) => (s ? s.charAt(0).toUpperCase() + s.slice(1) : 'Unknown')

/** `Q4 Launch!` → `q4-launch` (file name part; `dashboard` when nothing is left). */
export function slugify(name: string): string {
  const s = name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
  return s || 'dashboard'
}

function table(header: string[], rows: Cell[][]): string {
  return [csvRow(header), ...rows.map(csvRow)].join('\n') + '\n'
}

/** `label · Engagements · %` (+ `Previous`) rows of a breakdown. */
function breakdown(
  labelHeaders: string[],
  rows: { labels: Cell[]; clicks: number; percentage: number; prev?: number }[],
  compare: boolean,
): string {
  const header = [...labelHeaders, 'Engagements', '%', ...(compare ? ['Previous'] : [])]
  return table(
    header,
    rows.map((r) => [...r.labels, r.clicks, pct(r.percentage), ...(compare ? [r.prev ?? 0] : [])]),
  )
}

/** CSV of one dashboard module's loaded data (the module's CSV icon). */
export function moduleCsv(key: ModuleKey, data: DashboardData, compare: boolean): string {
  switch (key) {
    case 'top_day': {
      const d = data.top_day
      const header = ['Date', 'Engagements', ...(compare ? ['Previous'] : [])]
      return table(header, d ? [[d.date, d.count, ...(compare ? [d.prev_count] : [])]] : [])
    }
    case 'top_location': {
      const l = data.top_location
      return table(['Country', 'Code', 'Engagements'], l ? [[l.name || 'Unknown', l.code, l.count]] : [])
    }
    case 'over_time':
      return table(
        ['Date', 'Engagements', ...(compare ? ['Previous'] : [])],
        data.series.map((p, i) => [p.date, p.clicks, ...(compare ? [data.prev_series[i]?.clicks ?? 0] : [])]),
      )
    case 'devices':
      return breakdown(
        ['Device'],
        data.devices.map((d) => ({ labels: [cap(d.key)], clicks: d.clicks, percentage: d.percentage, prev: d.prev_clicks })),
        compare,
      )
    case 'referrers':
      return breakdown(
        ['Referrer'],
        data.referrers.map((r) => ({ labels: [r.referrer || 'Direct'], clicks: r.clicks, percentage: r.percentage, prev: r.prev_clicks })),
        compare,
      )
    case 'locations': {
      const countries = breakdown(
        ['Country', 'Code'],
        data.countries.map((c) => ({ labels: [c.name || 'Unknown', c.code], clicks: c.clicks, percentage: c.percentage, prev: c.prev_clicks })),
        compare,
      )
      const cities = breakdown(
        ['City', 'Country'],
        data.cities.map((c) => ({ labels: [c.name || 'Unknown', c.country], clicks: c.clicks, percentage: c.percentage, prev: c.prev_clicks })),
        compare,
      )
      return `${countries}\n${cities}`
    }
    case 'items':
      return breakdown(
        ['Type', 'Title', 'Link'],
        data.items.map((it) => ({
          labels: [it.type === 'qr' ? 'QR Code' : 'Link', it.title, it.short_url],
          clicks: it.clicks,
          percentage: it.percentage,
          prev: it.prev_clicks,
        })),
        compare,
      )
  }
}
