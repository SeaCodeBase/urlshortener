import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useCallback, useRef, type ReactNode } from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { TooltipProvider } from '@/components/ui/tooltip'
import { api } from '@/lib/api'
import type { Page } from '@/types'
import { DESIGN_PRESETS } from './designPresets'
import { DesignTab } from './DesignTab'
import { EditorTopBar } from './EditorTopBar'

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }),
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
  has_unpublished_changes: true,
  archived: false,
  url: 'http://localhost:8080/m/jose',
  views: 0,
  created_at: '2026-10-01T00:00:00Z',
  updated_at: '2026-10-01T00:00:00Z',
  blocks: [],
}

function deferred<T>() {
  let resolve!: (v: T) => void
  const promise = new Promise<T>((r) => (resolve = r))
  return { promise, resolve }
}

function wrapper(children: ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return (
    <QueryClientProvider client={qc}>
      <TooltipProvider>{children}</TooltipProvider>
    </QueryClientProvider>
  )
}

/** The editor page's wiring: the Design tab registers its flush, the top bar calls it. */
function Editor({ designPending }: { designPending?: boolean }) {
  const flush = useRef<(() => Promise<void>) | null>(null)
  const register = useCallback((f: (() => Promise<void>) | null) => {
    flush.current = f
  }, [])
  const flushPendingSaves = useCallback(async () => {
    await flush.current?.()
  }, [])
  return (
    <>
      <EditorTopBar page={page} flushPendingSaves={flushPendingSaves} designPending={designPending} />
      <DesignTab page={page} registerFlush={register} />
    </>
  )
}

const publishButton = () => screen.getByRole('button', { name: 'Publish changes' })

/** Lets TanStack's scheduled notifications and promise callbacks run. */
const settle = () =>
  act(async () => {
    await vi.advanceTimersByTimeAsync(20)
  })

describe('EditorTopBar publish', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.spyOn(api, 'getPage').mockResolvedValue(page)
    vi.spyOn(api, 'publishPage').mockResolvedValue({ ...page, has_unpublished_changes: false })
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('flushes a pending debounced design save and publishes only after it finishes', async () => {
    const save = deferred<Page>()
    const updatePage = vi.spyOn(api, 'updatePage').mockReturnValue(save.promise)
    render(wrapper(<Editor />))

    fireEvent.change(screen.getByLabelText('Text colour'), { target: { value: '#111111' } })
    expect(updatePage).not.toHaveBeenCalled() // still debounced

    fireEvent.click(publishButton())
    await settle()
    expect(updatePage).toHaveBeenCalledTimes(1)
    expect(updatePage).toHaveBeenCalledWith(3, { design: { ...DESIGN_PRESETS.classic, text_color: '#111111' } })
    expect(api.publishPage).not.toHaveBeenCalled()
    expect(publishButton()).toBeDisabled()

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })
    expect(api.publishPage).not.toHaveBeenCalled() // the save is still in flight

    save.resolve(page)
    await settle()
    expect(api.publishPage).toHaveBeenCalledTimes(1)
    expect(api.publishPage).toHaveBeenCalledWith(3)
    expect(updatePage.mock.invocationCallOrder[0]).toBeLessThan(vi.mocked(api.publishPage).mock.invocationCallOrder[0])
    // The debounce was consumed by the flush: no second save.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000)
    })
    expect(updatePage).toHaveBeenCalledTimes(1)
  })

  it('waits for an in-flight profile save before publishing', async () => {
    const save = deferred<Page>()
    const updatePage = vi.spyOn(api, 'updatePage').mockReturnValue(save.promise)
    render(wrapper(<Editor />))

    const name = screen.getByLabelText('Display name')
    fireEvent.change(name, { target: { value: 'José' } })
    fireEvent.blur(name)
    // Clicked in the same tick the save started, before the button re-rendered.
    fireEvent.click(publishButton())
    await settle()
    expect(updatePage).toHaveBeenCalledWith(3, { title: 'José' })
    expect(api.publishPage).not.toHaveBeenCalled()

    save.resolve(page)
    await settle()
    expect(api.publishPage).toHaveBeenCalledTimes(1)
  })

  it('disables "Publish changes" while a page save is pending or a design edit is unsaved', async () => {
    const save = deferred<Page>()
    vi.spyOn(api, 'updatePage').mockReturnValue(save.promise)
    const { unmount } = render(wrapper(<Editor />))
    expect(publishButton()).toBeEnabled()

    fireEvent.click(screen.getByRole('button', { name: 'Bold' })) // presets save at once
    await settle()
    expect(publishButton()).toBeDisabled()

    save.resolve(page)
    await settle()
    expect(publishButton()).toBeEnabled()
    unmount()

    render(wrapper(<Editor designPending />))
    expect(publishButton()).toBeDisabled()
    fireEvent.focus(publishButton().parentElement as HTMLElement)
    await settle()
    expect(screen.getAllByText('Saving…').length).toBeGreaterThan(0)
  })
})
