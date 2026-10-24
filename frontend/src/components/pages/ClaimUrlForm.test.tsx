import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ApiError } from '@/lib/api'
import type { SlugCheck } from '@/types'
import NewPage from '@/app/dashboard/pages/new/page'
import { ClaimUrlForm } from './ClaimUrlForm'

const push = vi.fn()
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push, replace: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => '/dashboard/pages/new',
  useSearchParams: () => new URLSearchParams(),
}))

type CheckFn = (slug: string) => { slug: string; data: SlugCheck | null; isChecking: boolean; error: Error | null }
const available: CheckFn = (slug) => ({
  slug,
  data: slug ? { slug, available: true, reason: '' } : null,
  isChecking: false,
  error: null,
})
const slugCheck = vi.fn<CheckFn>()
const create = { mutateAsync: vi.fn(), isPending: false }

vi.mock('@/lib/hooks', () => ({
  useSlugCheck: (slug: string) => slugCheck(slug),
  useMeta: () => ({ data: { base_url: 'http://localhost:8080' } }),
  usePageMutations: () => ({ create }),
}))

beforeEach(() => {
  push.mockReset()
  slugCheck.mockReset().mockImplementation(available)
  create.mutateAsync.mockReset().mockResolvedValue({ id: 42 })
})

describe('ClaimUrlForm', () => {
  it('shows the host prefix and lowercases the slug as you type', async () => {
    const user = userEvent.setup()
    render(<ClaimUrlForm onSubmit={vi.fn()} />)
    expect(screen.getByText('Claim your URL')).toBeInTheDocument()
    expect(screen.getByText('localhost:8080/m/')).toBeInTheDocument()
    const input = screen.getByRole('textbox', { name: 'Page URL' })
    await user.type(input, 'JoSe Doe!')
    expect(input).toHaveValue('josedoe')
    expect(slugCheck).toHaveBeenLastCalledWith('josedoe')
  })

  it('keeps "Create your page" disabled until the slug is available', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn()
    slugCheck.mockImplementation((slug) => ({
      slug,
      data: slug === 'taken' ? { slug, available: false, reason: 'taken' } : slug ? { slug, available: true, reason: '' } : null,
      isChecking: false,
      error: null,
    }))
    render(<ClaimUrlForm onSubmit={onSubmit} />)
    const button = screen.getByRole('button', { name: 'Create your page' })
    expect(button).toBeDisabled()
    const input = screen.getByRole('textbox', { name: 'Page URL' })
    await user.type(input, 'taken')
    expect(screen.getByText('Already taken')).toBeInTheDocument()
    expect(button).toBeDisabled()
    await user.type(input, 'x')
    expect(screen.getByText('Available')).toBeInTheDocument()
    expect(button).toBeEnabled()
    await user.click(button)
    expect(onSubmit).toHaveBeenCalledWith('takenx')
  })

  it('is disabled while checking and shows Invalid / Reserved', async () => {
    const user = userEvent.setup()
    slugCheck.mockImplementation((slug) => ({
      slug,
      data:
        slug === 'admin'
          ? { slug, available: false, reason: 'reserved' }
          : slug === 'ab'
            ? { slug, available: false, reason: 'invalid' }
            : null,
      isChecking: slug === 'abc',
      error: null,
    }))
    render(<ClaimUrlForm onSubmit={vi.fn()} />)
    const input = screen.getByRole('textbox', { name: 'Page URL' })
    await user.type(input, 'ab')
    expect(screen.getByText('Invalid')).toBeInTheDocument()
    await user.type(input, 'c')
    expect(screen.getByRole('button', { name: 'Create your page' })).toBeDisabled()
    await user.clear(input)
    await user.type(input, 'admin')
    expect(screen.getByText('Reserved')).toBeInTheDocument()
  })
})

describe('New page flow', () => {
  it('claims the slug, picks a template and creates the page', async () => {
    const user = userEvent.setup()
    render(<NewPage />)
    await user.type(screen.getByRole('textbox', { name: 'Page URL' }), 'jose')
    await user.click(screen.getByRole('button', { name: 'Create your page' }))
    expect(screen.getByRole('button', { name: 'Start from scratch' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Bold' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Minimal' })).toBeInTheDocument()
    expect(screen.getAllByText('Welcome').length).toBe(3)
    expect(screen.getAllByText('My website').length).toBe(3)
    await user.click(screen.getByRole('button', { name: 'Classic' }))
    expect(create.mutateAsync).toHaveBeenCalledWith({ slug: 'jose', template: 'classic' })
    expect(push).toHaveBeenCalledWith('/dashboard/pages/42')
  })

  it('creates from scratch', async () => {
    const user = userEvent.setup()
    render(<NewPage />)
    await user.type(screen.getByRole('textbox', { name: 'Page URL' }), 'jose')
    await user.click(screen.getByRole('button', { name: 'Create your page' }))
    await user.click(screen.getByRole('button', { name: 'Start from scratch' }))
    expect(create.mutateAsync).toHaveBeenCalledWith({ slug: 'jose', template: 'scratch' })
  })

  it('a 409 returns to the slug step with "Already taken"', async () => {
    const user = userEvent.setup()
    create.mutateAsync.mockRejectedValue(new ApiError('slug already taken', 409))
    render(<NewPage />)
    await user.type(screen.getByRole('textbox', { name: 'Page URL' }), 'jose')
    await user.click(screen.getByRole('button', { name: 'Create your page' }))
    await user.click(screen.getByRole('button', { name: 'Minimal' }))
    expect(await screen.findByText('Already taken')).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Page URL' })).toHaveValue('jose')
    expect(screen.getByRole('button', { name: 'Create your page' })).toBeDisabled()
    expect(push).not.toHaveBeenCalled()
  })
})
