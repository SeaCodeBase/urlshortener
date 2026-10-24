import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, fireEvent, render, screen } from '@testing-library/react'
import type { Page } from '@/types'
import { DESIGN_PRESETS } from './designPresets'
import { DesignTab } from './DesignTab'

const update = { mutate: vi.fn(), isPending: false }
const upload = { mutateAsync: vi.fn(), isPending: false }
vi.mock('@/lib/hooks', () => ({
  usePageMutations: () => ({ update }),
  useUploadImage: () => upload,
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
  status: 'draft',
  published_at: null,
  has_unpublished_changes: true,
  archived: false,
  url: 'http://localhost:8080/m/jose',
  views: 0,
  created_at: '2026-10-01T00:00:00Z',
  updated_at: '2026-10-01T00:00:00Z',
  blocks: [],
}

describe('DesignTab', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    update.mutate.mockReset()
    upload.mutateAsync.mockReset()
  })
  afterEach(() => vi.useRealTimers())

  it('applies a whole preset immediately when its tile is clicked', () => {
    const onDesignChange = vi.fn()
    render(<DesignTab page={page} onDesignChange={onDesignChange} />)
    fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
    expect(update.mutate).toHaveBeenCalledTimes(1)
    expect(update.mutate.mock.calls[0][0]).toEqual({ id: 3, patch: { design: DESIGN_PRESETS.bold } })
    expect(onDesignChange).toHaveBeenCalledWith(DESIGN_PRESETS.bold)
    expect(screen.getByRole('button', { name: 'Bold' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('shows an inline error for an invalid hex and does not send it', () => {
    render(<DesignTab page={page} onDesignChange={vi.fn()} />)
    fireEvent.change(screen.getByLabelText('Text colour'), { target: { value: '#12' } })
    expect(screen.getByText('Enter a colour like #2A5BD7')).toBeInTheDocument()
    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(update.mutate).not.toHaveBeenCalled()
  })

  it('debounces a valid colour edit and sends the full design', () => {
    const onDesignChange = vi.fn()
    render(<DesignTab page={page} onDesignChange={onDesignChange} />)
    const input = screen.getByLabelText('Text colour')
    fireEvent.change(input, { target: { value: '#000000' } })
    fireEvent.change(input, { target: { value: '#111111' } })
    const expected = { ...DESIGN_PRESETS.classic, text_color: '#111111' }
    expect(onDesignChange).toHaveBeenLastCalledWith(expected)
    act(() => {
      vi.advanceTimersByTime(399)
    })
    expect(update.mutate).not.toHaveBeenCalled()
    act(() => {
      vi.advanceTimersByTime(1)
    })
    expect(update.mutate).toHaveBeenCalledTimes(1)
    expect(update.mutate.mock.calls[0][0]).toEqual({ id: 3, patch: { design: expected } })
  })

  it('switches the button style and font', () => {
    render(<DesignTab page={page} onDesignChange={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Outline' }))
    fireEvent.click(screen.getByRole('button', { name: 'Mono' }))
    act(() => {
      vi.advanceTimersByTime(400)
    })
    expect(update.mutate).toHaveBeenCalledTimes(1)
    expect(update.mutate.mock.calls[0][0].patch.design).toEqual({
      ...DESIGN_PRESETS.classic,
      button: { ...DESIGN_PRESETS.classic.button, style: 'outline' },
      font: 'mono',
    })
  })

  it('saves the display name and bio on blur', () => {
    render(<DesignTab page={page} onDesignChange={vi.fn()} />)
    const name = screen.getByLabelText('Display name')
    fireEvent.change(name, { target: { value: 'José' } })
    fireEvent.blur(name)
    expect(update.mutate.mock.calls[0][0]).toEqual({ id: 3, patch: { title: 'José' } })
    const bio = screen.getByLabelText('Bio')
    fireEvent.blur(bio)
    expect(update.mutate).toHaveBeenCalledTimes(1)
    fireEvent.change(bio, { target: { value: 'New bio' } })
    fireEvent.blur(bio)
    expect(update.mutate.mock.calls[1][0]).toEqual({ id: 3, patch: { description: 'New bio' } })
  })
})
