import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render as rtlRender, screen, within } from '@testing-library/react'
import type { ReactElement } from 'react'
import { TooltipProvider } from '@/components/ui/tooltip'
import userEvent from '@testing-library/user-event'
import { DESIGN_PRESETS } from './designPresets'
import type { Page } from '@/types'
import { PageCard } from './PageCard'

const setArchived = { mutate: vi.fn(), isPending: false }
const remove = { mutateAsync: vi.fn(), isPending: false }

vi.mock('@/lib/hooks', () => ({
  usePageMutations: () => ({ setArchived, remove }),
}))

const base: Page = {
  id: 4,
  group_id: 7,
  slug: 'jose',
  title: 'Jose',
  description: '',
  avatar_url: '',
  design: DESIGN_PRESETS.classic,
  status: 'draft',
  published_at: null,
  has_unpublished_changes: true,
  archived: false,
  url: 'http://localhost:8080/m/jose',
  views: 12,
  created_at: '2026-10-20T10:00:00Z',
  updated_at: '2026-10-20T10:00:00Z',
}

const render = (ui: ReactElement) => rtlRender(ui, { wrapper: TooltipProvider })

beforeEach(() => {
  setArchived.mutate.mockReset()
  remove.mutateAsync.mockReset().mockResolvedValue(undefined)
})

describe('PageCard', () => {
  it('renders title, URL, views, Draft chip and Edit', () => {
    render(<PageCard page={base} />)
    expect(screen.getByRole('link', { name: 'Jose' })).toHaveAttribute('href', '/dashboard/pages/4')
    expect(screen.getByText('localhost:8080/m/jose')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Copy' })).toBeInTheDocument()
    expect(screen.getByText('12 views')).toBeInTheDocument()
    expect(screen.getByText('Draft')).toBeInTheDocument()
    expect(screen.queryByText('Published')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Unpublished changes')).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Edit' })).toHaveAttribute('href', '/dashboard/pages/4')
    expect(screen.getByText('J')).toBeInTheDocument()
  })

  it('shows Published, and the unpublished-changes dot only when changes are pending', () => {
    const { rerender } = render(
      <PageCard page={{ ...base, status: 'published', published_at: '2026-10-21T10:00:00Z', has_unpublished_changes: false }} />,
    )
    expect(screen.getByText('Published')).toBeInTheDocument()
    expect(screen.queryByText('Draft')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Unpublished changes')).not.toBeInTheDocument()
    rerender(<PageCard page={{ ...base, status: 'published', published_at: '2026-10-21T10:00:00Z', has_unpublished_changes: true }} />)
    expect(screen.getByLabelText('Unpublished changes')).toBeInTheDocument()
  })

  it('uses the singular for one view', () => {
    render(<PageCard page={{ ...base, views: 1 }} />)
    expect(screen.getByText('1 view')).toBeInTheDocument()
  })

  it('Share opens the share dialog with the page URL', async () => {
    const user = userEvent.setup()
    render(<PageCard page={base} />)
    await user.click(screen.getByRole('button', { name: 'Share' }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText('localhost:8080/m/jose')).toBeInTheDocument()
  })

  it('menu Hide calls setArchived(true); Show calls setArchived(false)', async () => {
    const user = userEvent.setup()
    const { rerender } = render(<PageCard page={base} />)
    await user.click(screen.getByRole('button', { name: 'More' }))
    const menu = await screen.findByRole('menu')
    expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent?.trim())).toEqual(['Hide', 'Delete'])
    await user.click(within(menu).getByRole('menuitem', { name: 'Hide' }))
    expect(setArchived.mutate).toHaveBeenCalledWith({ id: 4, archived: true }, expect.anything())

    rerender(<PageCard page={{ ...base, archived: true }} />)
    await user.click(screen.getByRole('button', { name: 'More' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Show' }))
    expect(setArchived.mutate).toHaveBeenLastCalledWith({ id: 4, archived: false }, expect.anything())
  })

  it('menu Delete confirms then calls remove', async () => {
    const user = userEvent.setup()
    render(<PageCard page={base} />)
    await user.click(screen.getByRole('button', { name: 'More' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Delete' }))
    const dialog = await screen.findByRole('alertdialog')
    expect(within(dialog).getByText('Delete page?')).toBeInTheDocument()
    expect(within(dialog).getByText('This permanently deletes the page and its blocks. Links stay.')).toBeInTheDocument()
    expect(remove.mutateAsync).not.toHaveBeenCalled()
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }))
    expect(remove.mutateAsync).toHaveBeenCalledWith(4)
  })
})
