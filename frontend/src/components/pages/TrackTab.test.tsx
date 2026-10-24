import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, within } from '@testing-library/react'
import type { PageStats, PageStatsQuery } from '@/types'
import { TrackTab } from './TrackTab'

let search = new URLSearchParams()
const replace = vi.fn()
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace, prefetch: vi.fn() }),
  usePathname: () => '/dashboard/pages/3',
  useSearchParams: () => search,
}))

const usePageStats = vi.fn()
vi.mock('@/lib/hooks', () => ({
  usePageStats: (id: number, q: PageStatsQuery) => usePageStats(id, q),
}))

const statsFixture: PageStats = {
  from: '2026-09-26',
  to: '2026-10-25',
  unit: 'day',
  total_views_all_time: 1234,
  total_views: 87,
  prev_total_views: 60,
  change_pct: 45,
  series: [
    { date: '2026-10-24', clicks: 40 },
    { date: '2026-10-25', clicks: 47 },
  ],
  prev_series: [],
  referrers: [{ referrer: 'instagram.com', clicks: 50, percentage: 57.5, prev_clicks: 0 }],
  devices: [{ key: 'mobile', clicks: 80, percentage: 92, prev_clicks: 0 }],
  countries: [{ code: 'US', name: 'United States', clicks: 87, percentage: 100, prev_clicks: 0 }],
  cities: [{ name: 'Austin', country: 'US', clicks: 87, percentage: 100, prev_clicks: 0 }],
  blocks: [
    { block_id: 5, title: 'Menu', link_id: 9, short_url: 'http://localhost:8080/abc', clicks: 12 },
    { block_id: 6, title: 'Shop', link_id: 10, short_url: 'http://localhost:8080/def', clicks: 3 },
  ],
}

describe('TrackTab', () => {
  beforeEach(() => {
    search = new URLSearchParams()
    usePageStats.mockReset()
    replace.mockReset()
  })

  it('renders totals and link click rows from the stats', () => {
    usePageStats.mockReturnValue({ data: statsFixture, isLoading: false, isError: false })
    render(<TrackTab pageId={3} />)
    expect(screen.getByRole('button', { name: /Last 30 days/ })).toBeInTheDocument()
    expect(screen.getByText('Total views')).toBeInTheDocument()
    expect(screen.getByText('1,234')).toBeInTheDocument()
    expect(screen.getByText('Views (Last 30 days)')).toBeInTheDocument()
    expect(screen.getByText('87', { selector: 'div' })).toBeInTheDocument()

    const table = screen.getByRole('table', { name: 'Link clicks' })
    const rows = within(table).getAllByRole('row').slice(1)
    expect(rows).toHaveLength(2)
    expect(rows[0]).toHaveTextContent('Menu')
    expect(rows[0]).toHaveTextContent('localhost:8080/abc')
    expect(rows[0]).toHaveTextContent('12')
    expect(rows[1]).toHaveTextContent('Shop')

    for (const h of ['Views over time', 'Link clicks', 'Views by referrer', 'Views by device', 'Views by location']) {
      expect(screen.getByRole('heading', { name: h })).toBeInTheDocument()
    }
    const [id, q] = usePageStats.mock.calls[0]
    expect(id).toBe(3)
    expect(q).toMatchObject({ unit: 'day', compare: 'none' })
  })

  it('shows the empty state when there are no link blocks', () => {
    usePageStats.mockReturnValue({ data: { ...statsFixture, blocks: [] }, isLoading: false, isError: false })
    render(<TrackTab pageId={3} />)
    expect(screen.getByText('No link blocks yet')).toBeInTheDocument()
  })

  it('uses the URL range and comparison', () => {
    search = new URLSearchParams('from=2026-10-01&to=2026-10-10&compare=previous')
    usePageStats.mockReturnValue({ data: statsFixture, isLoading: false, isError: false })
    render(<TrackTab pageId={3} />)
    expect(usePageStats.mock.calls[0][1]).toEqual({ from: '2026-10-01', to: '2026-10-10', unit: 'day', compare: 'previous' })
    expect(screen.getByText('vs previous period')).toBeInTheDocument()
  })

  it('shows skeletons while loading, never an empty state', () => {
    usePageStats.mockReturnValue({ data: undefined, isLoading: true, isError: false })
    render(<TrackTab pageId={3} />)
    expect(screen.queryByText('No link blocks yet')).not.toBeInTheDocument()
    expect(screen.queryByText('Total views')).not.toBeInTheDocument()
  })

  it('shows an inline error with Retry', () => {
    const refetch = vi.fn()
    usePageStats.mockReturnValue({ data: undefined, isLoading: false, isError: true, error: new Error('boom'), refetch })
    render(<TrackTab pageId={3} />)
    expect(screen.getByRole('alert')).toHaveTextContent('boom')
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(refetch).toHaveBeenCalled()
    expect(screen.queryByText('No link blocks yet')).not.toBeInTheDocument()
  })
})
