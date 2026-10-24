import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Link, PageBlock } from '@/types'
import { LinkBlockForm } from './LinkBlockForm'

const link = (id: number, title: string): Link =>
  ({ id, title, short_url: `https://sho.rt/l${id}`, original_url: `https://example.com/${id}` }) as Link

const useLinks = vi.fn()
vi.mock('@/lib/hooks', () => ({
  useLinks: (params: unknown) => useLinks(params),
}))

beforeEach(() => {
  useLinks.mockReset().mockReturnValue({
    data: { pages: [{ links: [link(8, 'Spring sale'), link(9, '')], page: 1, total_pages: 1, total: 2 }] },
    isLoading: false,
    hasNextPage: false,
    fetchNextPage: vi.fn(),
  })
})

describe('LinkBlockForm', () => {
  it('shows validation messages when nothing is filled in', async () => {
    const onSubmit = vi.fn()
    const user = userEvent.setup()
    render(<LinkBlockForm submitting={false} onSubmit={onSubmit} onCancel={() => {}} />)
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(screen.getByText('Pick a link or enter a destination URL')).toBeInTheDocument()
    expect(screen.getByText('Button title is required')).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('existing-link mode submits link_id and the prefilled button title', async () => {
    const onSubmit = vi.fn()
    const user = userEvent.setup()
    render(<LinkBlockForm submitting={false} onSubmit={onSubmit} onCancel={() => {}} />)
    await user.click(screen.getByRole('radio', { name: /Spring sale/ }))
    expect(screen.getByLabelText('Button title')).toHaveValue('Spring sale')
    await user.type(screen.getByLabelText('Description'), 'Up to 50% off')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(onSubmit).toHaveBeenCalledWith({
      content: { title: 'Spring sale', description: 'Up to 50% off', thumbnail_url: '' },
      link_id: 8,
    })
    expect(useLinks.mock.calls[0][0]).toMatchObject({ limit: 50 })
  })

  it('new-destination mode submits new_link', async () => {
    const onSubmit = vi.fn()
    const user = userEvent.setup()
    render(<LinkBlockForm submitting={false} onSubmit={onSubmit} onCancel={() => {}} />)
    await user.click(screen.getByRole('button', { name: 'New destination' }))
    await user.type(screen.getByLabelText('Destination URL'), 'example.com/shop')
    await user.type(screen.getByLabelText('Link title (optional)'), 'Shop')
    await user.type(screen.getByLabelText('Button title'), 'Visit my shop')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(onSubmit).toHaveBeenCalledWith({
      content: { title: 'Visit my shop', description: '', thumbnail_url: '' },
      new_link: { original_url: 'https://example.com/shop', title: 'Shop' },
    })
  })

  it('rejects an invalid thumbnail URL and an over-long title', async () => {
    const onSubmit = vi.fn()
    const user = userEvent.setup()
    render(<LinkBlockForm submitting={false} onSubmit={onSubmit} onCancel={() => {}} />)
    await user.click(screen.getByRole('radio', { name: /Spring sale/ }))
    await user.clear(screen.getByLabelText('Button title'))
    await user.type(screen.getByLabelText('Button title'), 'x'.repeat(81))
    await user.type(screen.getByLabelText('Thumbnail URL'), 'ftp://nope')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(screen.getByText('Button title must be 80 characters or fewer')).toBeInTheDocument()
    expect(screen.getByText('Enter a valid URL')).toBeInTheDocument()
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('edit mode keeps the link and submits only content', async () => {
    const block: PageBlock = {
      id: 5,
      type: 'link',
      position: 0,
      content: { title: 'Menu', description: '', thumbnail_url: '' },
      is_active: true,
      is_pinned: false,
      link_id: 9,
      link: { id: 9, title: 'Menu', short_url: 'http://localhost:8080/abc', original_url: 'https://example.com' },
    }
    const onSubmit = vi.fn()
    const user = userEvent.setup()
    render(<LinkBlockForm block={block} submitting={false} onSubmit={onSubmit} onCancel={() => {}} />)
    expect(screen.getByText('Delete the block to change its link')).toBeInTheDocument()
    expect(screen.queryByRole('radio')).not.toBeInTheDocument()
    await user.clear(screen.getByLabelText('Button title'))
    await user.type(screen.getByLabelText('Button title'), 'Our menu')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(onSubmit).toHaveBeenCalledWith({ content: { title: 'Our menu', description: '', thumbnail_url: '' } })
  })
})
