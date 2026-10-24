import type { LinkStatsV2 } from '@/types'

/** Leading characters that make spreadsheets treat a cell as a formula. */
const FORMULA_START = /^[=+\-@\t\r]/

/**
 * One CSV cell. Strings starting with a formula character get a `'` prefix
 * (CSV injection), then are quoted when they contain `"`, `,` or newlines.
 * Numbers are written as-is, so negatives stay numeric.
 */
export function csvCell(v: string | number): string {
  if (typeof v === 'number') return String(v)
  const s = FORMULA_START.test(v) ? `'${v}` : v
  return /[",\r\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s
}

export function csvRow(cells: (string | number)[]): string {
  return cells.map(csvCell).join(',')
}

const cap = (s: string) => (s ? s.charAt(0).toUpperCase() + s.slice(1) : 'Unknown')
const pct = (p: number) => Number(p.toFixed(1))

/** CSV of loaded stats: one titled section per module, separated by blank lines. */
export function buildStatsCsv(
  stats: LinkStatsV2,
  opts: {
    rangeLabel: string
    /** Plural noun of the metric (default "Engagements"; "Scans" for QR codes). */
    noun?: string
    /** Leave out the Referrers and Devices sections (QR scan stats). */
    scansOnly?: boolean
  },
): string {
  const n = opts.noun ?? 'Engagements'
  const lines: string[] = []
  const section = (title: string, header: string[], rows: (string | number)[][]) => {
    if (lines.length) lines.push('')
    lines.push(csvCell(title), csvRow(header))
    for (const r of rows) lines.push(csvRow(r))
  }
  section('Summary', ['Metric', 'Value'], [
    ['Range', `${stats.from} to ${stats.to}`],
    [`Total ${n.toLowerCase()}`, stats.total_all_time],
    [`${n} (${opts.rangeLabel})`, stats.total_range],
  ])
  section(`${n} over time`, ['Date', n], stats.series.map((p) => [p.date, p.clicks]))
  if (stats.compare_link) section('Compared link', ['Date', n], stats.compare_link.series.map((p) => [p.date, p.clicks]))
  section('Countries', ['Country', 'Code', n, '%'], stats.countries.map((c) => [c.name, c.code, c.clicks, pct(c.percentage)]))
  section('Cities', ['City', 'Country', n, '%'], stats.cities.map((c) => [c.name, c.country, c.clicks, pct(c.percentage)]))
  if (!opts.scansOnly) {
    section('Referrers', ['Referrer', n, '%'], stats.referrers.map((r) => [r.referrer || 'Direct', r.clicks, pct(r.percentage)]))
    section('Devices', ['Device', n, '%'], stats.devices.map((d) => [cap(d.key), d.clicks, pct(d.percentage)]))
  }
  section('Browsers', ['Browser', n, '%'], stats.browsers.map((d) => [d.key || 'Unknown', d.clicks, pct(d.percentage)]))
  section('Operating systems', ['Operating system', n, '%'], stats.os.map((d) => [d.key || 'Unknown', d.clicks, pct(d.percentage)]))
  return lines.join('\n') + '\n'
}
