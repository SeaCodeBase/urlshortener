import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { DESIGN_PRESETS } from '@/components/pages/designPresets'
import { TooltipProvider } from '@/components/ui/tooltip'
import { api } from '@/lib/api'
import type { Page, PageDesign } from '@/types'
import PageEditorPage from './page'

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }),
  useParams: () => ({ id: '3' }),
  usePathname: () => '/dashboard/pages/3',
  useSearchParams: () => new URLSearchParams('tab=design'),
}))
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }))

const page: Page = {
  id: 3,
  group_id: 7,
  slug: 'jose',
  title: 'Jose',
  description: 'Hi',
  avatar_url: '',
  design: DESIGN_PRESETS.classic,
  status: 'published',
  published_at: '2026-10-01T00:00:00Z',
  has_unpublished_changes: false,
  archived: false,
  url: 'http://localhost:8080/m/jose',
  views: 0,
  created_at: '2026-10-01T00:00:00Z',
  updated_at: '2026-10-01T00:00:00Z',
  blocks: [],
}

/** The backend encodes Go maps: object keys come back sorted. */
function sortedKeys<T>(v: T): T {
  if (v == null || typeof v !== 'object' || Array.isArray(v)) return v
  const rec = v as Record<string, unknown>
  return Object.fromEntries(
    Object.keys(rec)
      .sort()
      .map((k) => [k, sortedKeys(rec[k])]),
  ) as T
}

const settle = (ms = 20) =>
  act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })

describe('PageEditorPage publish state', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('shows "Saving…" while a design edit is unsaved and re-enables publish once the server echoes it', async () => {
    let server: Page = page
    vi.spyOn(api, 'getPage').mockImplementation(async () => server)
    vi.spyOn(api, 'updatePage').mockImplementation(async (_id, patch) => {
      server = { ...server, design: sortedKeys(patch.design as PageDesign), has_unpublished_changes: true }
      return server
    })
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <TooltipProvider>
          <PageEditorPage />
        </TooltipProvider>
      </QueryClientProvider>,
    )
    await settle()
    const publish = () => screen.getByRole('button', { name: 'Publish changes' })
    expect(publish()).toBeDisabled() // nothing to publish yet

    fireEvent.change(screen.getByLabelText('Text colour'), { target: { value: '#111111' } })
    await settle()
    expect(publish()).toBeDisabled()
    fireEvent.focus(publish().parentElement as HTMLElement)
    await settle()
    expect(screen.getAllByText('Saving…').length).toBeGreaterThan(0)

    await settle(400) // debounce fires, save + refetch complete
    await settle()
    expect(api.updatePage).toHaveBeenCalledTimes(1)
    expect(publish()).toBeEnabled()
  })
})
