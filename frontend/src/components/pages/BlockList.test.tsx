import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { PageBlock } from '@/types'
import { BlockList } from './BlockList'

const updateBlock = { mutateAsync: vi.fn(), isPending: false }
const deleteBlock = { mutateAsync: vi.fn(), isPending: false }
const reorder = { mutateAsync: vi.fn(), isPending: false }

vi.mock('@/lib/hooks', () => ({
  usePageMutations: () => ({ updateBlock, deleteBlock, reorder }),
}))

const toastError = vi.fn()
vi.mock('sonner', () => ({ toast: { error: (m: string) => toastError(m), success: vi.fn() } }))

const blocks: PageBlock[] = [
  {
    id: 5,
    type: 'link',
    position: 0,
    content: { title: 'Menu', description: '', thumbnail_url: '' },
    is_active: true,
    is_pinned: false,
    link_id: 9,
    link: { id: 9, title: 'Menu', short_url: 'http://localhost:8080/abc', original_url: 'https://example.com' },
  },
  {
    id: 6,
    type: 'text',
    position: 1,
    content: { heading: '', body: 'Welcome to my page' },
    is_active: true,
    is_pinned: true,
    link_id: null,
  },
  { id: 7, type: 'image', position: 2, content: { url: 'https://img.test/a.png', alt: '' }, is_active: false, is_pinned: false, link_id: null },
  {
    id: 8,
    type: 'social',
    position: 3,
    content: {
      items: [
        { platform: 'instagram', url: 'https://instagram.com/x' },
        { platform: 'x', url: 'https://x.com/x' },
      ],
    },
    is_active: true,
    is_pinned: false,
    link_id: null,
  },
]

const rowOf = (id: number) => document.querySelector(`[data-block-row="${id}"]`) as HTMLElement

beforeEach(() => {
  updateBlock.mutateAsync.mockReset().mockResolvedValue({})
  deleteBlock.mutateAsync.mockReset().mockResolvedValue(undefined)
  reorder.mutateAsync.mockReset().mockResolvedValue({ blocks })
  toastError.mockReset()
})

describe('BlockList', () => {
  it('renders a row per block with a summary and controls', () => {
    render(<BlockList pageId={1} blocks={blocks} onEdit={() => {}} />)
    expect(within(rowOf(5)).getByText('Menu')).toBeInTheDocument()
    expect(within(rowOf(5)).getByText('localhost:8080/abc')).toBeInTheDocument()
    expect(within(rowOf(6)).getByText('Welcome to my page')).toBeInTheDocument()
    expect(within(rowOf(7)).getByText('https://img.test/a.png')).toBeInTheDocument()
    expect(within(rowOf(8)).getByText('2 platforms')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Reorder block' })).toHaveLength(4)
    expect(within(rowOf(7)).getByRole('switch', { name: 'Active' })).not.toBeChecked()
    expect(within(rowOf(6)).getByRole('button', { name: 'Pin' })).toHaveAttribute('aria-pressed', 'true')
    expect(within(rowOf(5)).getByRole('button', { name: 'Pin' })).toHaveAttribute('aria-pressed', 'false')
  })

  it('toggling Active calls updateBlock and shows the new state at once', async () => {
    const user = userEvent.setup()
    render(<BlockList pageId={1} blocks={blocks} onEdit={() => {}} />)
    const sw = within(rowOf(5)).getByRole('switch', { name: 'Active' })
    await user.click(sw)
    expect(updateBlock.mutateAsync).toHaveBeenCalledWith({ pageId: 1, blockId: 5, patch: { is_active: false } })
    expect(sw).not.toBeChecked()
  })

  it('reverts a toggle and toasts when the update fails', async () => {
    updateBlock.mutateAsync.mockRejectedValue(new Error('boom'))
    const user = userEvent.setup()
    render(<BlockList pageId={1} blocks={blocks} onEdit={() => {}} />)
    const pin = within(rowOf(5)).getByRole('button', { name: 'Pin' })
    await user.click(pin)
    expect(updateBlock.mutateAsync).toHaveBeenCalledWith({ pageId: 1, blockId: 5, patch: { is_pinned: true } })
    await waitFor(() => expect(toastError).toHaveBeenCalled())
    expect(pin).toHaveAttribute('aria-pressed', 'false')
  })

  it('Edit hands the block to onEdit', async () => {
    const onEdit = vi.fn()
    const user = userEvent.setup()
    render(<BlockList pageId={1} blocks={blocks} onEdit={onEdit} />)
    await user.click(within(rowOf(6)).getByRole('button', { name: 'Edit' }))
    expect(onEdit).toHaveBeenCalledWith(blocks[1])
  })

  it('Delete asks for confirmation, then calls deleteBlock', async () => {
    const user = userEvent.setup()
    render(<BlockList pageId={1} blocks={blocks} onEdit={() => {}} />)
    await user.click(within(rowOf(7)).getByRole('button', { name: 'Delete' }))
    expect(deleteBlock.mutateAsync).not.toHaveBeenCalled()
    const dialog = await screen.findByRole('alertdialog')
    expect(within(dialog).getByText('Delete block?')).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Delete' }))
    expect(deleteBlock.mutateAsync).toHaveBeenCalledWith({ pageId: 1, blockId: 7 })
  })
})
