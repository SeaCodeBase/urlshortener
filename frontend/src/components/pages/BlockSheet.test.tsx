import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { PageBlock } from '@/types'
import { BlockSheet } from './BlockSheet'

const addBlock = { mutateAsync: vi.fn(), isPending: false }
const updateBlock = { mutateAsync: vi.fn(), isPending: false }
const upload = { mutateAsync: vi.fn(), isPending: false }

vi.mock('@/lib/hooks', () => ({
  usePageMutations: () => ({ addBlock, updateBlock }),
  useUploadImage: () => upload,
  useLinks: () => ({ data: { pages: [] }, isLoading: false, hasNextPage: false, fetchNextPage: vi.fn() }),
}))

const toastError = vi.fn()
vi.mock('sonner', () => ({ toast: { error: (m: string) => toastError(m), success: vi.fn() } }))

beforeEach(() => {
  addBlock.mutateAsync.mockReset().mockResolvedValue({ id: 10 })
  updateBlock.mutateAsync.mockReset().mockResolvedValue({ id: 6 })
  upload.mutateAsync.mockReset()
  toastError.mockReset()
})

describe('BlockSheet', () => {
  it('text form requires a heading or body', async () => {
    const user = userEvent.setup()
    render(<BlockSheet open onOpenChange={() => {}} pageId={1} type="text" />)
    expect(screen.getByRole('heading', { name: 'Add text' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(screen.getByText('Add a heading or body')).toBeInTheDocument()
    expect(addBlock.mutateAsync).not.toHaveBeenCalled()
  })

  it('saving a new text block calls addBlock with its content and closes', async () => {
    const onOpenChange = vi.fn()
    const user = userEvent.setup()
    render(<BlockSheet open onOpenChange={onOpenChange} pageId={1} type="text" />)
    await user.type(screen.getByLabelText('Heading'), 'Hello')
    await user.type(screen.getByLabelText('Body'), 'World')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(addBlock.mutateAsync).toHaveBeenCalledWith({
      pageId: 1,
      input: { type: 'text', content: { heading: 'Hello', body: 'World' } },
    })
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
  })

  it('editing a block calls updateBlock with the new content', async () => {
    const block: PageBlock = {
      id: 6,
      type: 'text',
      position: 0,
      content: { heading: 'Hi', body: '' },
      is_active: true,
      is_pinned: false,
      link_id: null,
    }
    const user = userEvent.setup()
    render(<BlockSheet open onOpenChange={() => {}} pageId={1} type="text" block={block} />)
    expect(screen.getByRole('heading', { name: 'Edit text' })).toBeInTheDocument()
    expect(screen.getByLabelText('Heading')).toHaveValue('Hi')
    await user.type(screen.getByLabelText('Heading'), ' there')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(updateBlock.mutateAsync).toHaveBeenCalledWith({
      pageId: 1,
      blockId: 6,
      patch: { content: { heading: 'Hi there', body: '' } },
    })
  })

  it('toasts an API error and stays open', async () => {
    addBlock.mutateAsync.mockRejectedValue(new Error('nope'))
    const onOpenChange = vi.fn()
    const user = userEvent.setup()
    render(<BlockSheet open onOpenChange={onOpenChange} pageId={1} type="text" />)
    await user.type(screen.getByLabelText('Heading'), 'Hello')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(toastError).toHaveBeenCalled())
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
  })

  it('image form requires a URL and accepts an uploaded path', async () => {
    upload.mutateAsync.mockResolvedValue({ url: '/uploads/a.png', name: 'a.png', size: 1, content_type: 'image/png' })
    const user = userEvent.setup()
    render(<BlockSheet open onOpenChange={() => {}} pageId={1} type="image" />)
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(screen.getByText('Enter an image URL')).toBeInTheDocument()
    await user.upload(screen.getByLabelText('Upload'), new File(['x'], 'a.png', { type: 'image/png' }))
    await waitFor(() => expect(screen.getByLabelText('Image URL')).toHaveValue('/uploads/a.png'))
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(addBlock.mutateAsync).toHaveBeenCalledWith({
      pageId: 1,
      input: { type: 'image', content: { url: '/uploads/a.png', alt: '' } },
    })
  })

  it('social form needs a profile and validates URLs', async () => {
    const user = userEvent.setup()
    render(<BlockSheet open onOpenChange={() => {}} pageId={1} type="social" />)
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(screen.getByText('Add at least one profile')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Add profile' }))
    await user.type(screen.getByLabelText('Profile URL 1'), 'not a url')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(screen.getByText('Enter a valid URL')).toBeInTheDocument()
    await user.clear(screen.getByLabelText('Profile URL 1'))
    await user.type(screen.getByLabelText('Profile URL 1'), 'https://instagram.com/jose')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(addBlock.mutateAsync).toHaveBeenCalledWith({
      pageId: 1,
      input: { type: 'social', content: { items: [{ platform: 'instagram', url: 'https://instagram.com/jose' }] } },
    })
  })
})
