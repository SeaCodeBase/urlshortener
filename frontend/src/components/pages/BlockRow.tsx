'use client'

import { useRef, useState, type ComponentProps } from 'react'
import { GripVertical, Image as ImageIcon, Link2, Pencil, Pin, Share2, Trash2, Type, type LucideIcon } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { formatShortUrl } from '@/lib/format'
import { usePageMutations } from '@/lib/hooks'
import { cn } from '@/lib/utils'
import type { BlockPatch, BlockType, PageBlock } from '@/types'

export const BLOCK_ICONS: Record<BlockType, LucideIcon> = { link: Link2, text: Type, image: ImageIcon, social: Share2 }

const EXCERPT = 80
const excerpt = (s: string) => (s.length > EXCERPT ? `${s.slice(0, EXCERPT).trimEnd()}…` : s)

/** One-line title and secondary line for a block row. */
export function blockSummary(block: PageBlock): { title: string; detail?: string } {
  switch (block.type) {
    case 'link':
      return { title: block.content.title, detail: block.link ? formatShortUrl(block.link.short_url) : undefined }
    case 'text':
      return block.content.heading
        ? { title: excerpt(block.content.heading), detail: block.content.body ? excerpt(block.content.body) : undefined }
        : { title: excerpt(block.content.body) }
    case 'image':
      return { title: block.content.url, detail: block.content.alt || undefined }
    case 'social': {
      const n = block.content.items.length
      return { title: `${n} ${n === 1 ? 'platform' : 'platforms'}` }
    }
  }
}

type Flag = 'is_active' | 'is_pinned'

/**
 * Optimistic boolean: shows the toggled value until the server's value catches up (or the call
 * fails, which reverts and toasts). Only the latest call settles the state.
 */
function useOptimisticFlag(pageId: number, block: PageBlock, flag: Flag, failMessage: string) {
  const { updateBlock } = usePageMutations()
  const server = block[flag]
  const [override, setOverride] = useState<{ value: boolean; base: boolean } | null>(null)
  const latest = useRef(0)
  // Drop the override once the server value moves away from what it was when we toggled.
  const value = override && override.base === server ? override.value : server

  function toggle(next: boolean) {
    const call = ++latest.current
    setOverride({ value: next, base: server })
    const patch: BlockPatch = { [flag]: next }
    updateBlock.mutateAsync({ pageId, blockId: block.id, patch }).catch(() => {
      if (call === latest.current) setOverride(null)
      toast.error(failMessage)
    })
  }

  return [value, toggle] as const
}

export interface BlockRowProps {
  pageId: number
  block: PageBlock
  onEdit: () => void
  onDelete: () => void
  /** Ref, attributes and listeners for the drag handle (from `useSortable`). */
  dragHandleProps?: ComponentProps<'button'>
  dragging?: boolean
}

/** A block in the Build tab: handle, type icon, summary, Active switch, pin, Edit and Delete. */
export function BlockRow({ pageId, block, onEdit, onDelete, dragHandleProps, dragging }: BlockRowProps) {
  const [active, setActive] = useOptimisticFlag(pageId, block, 'is_active', "Couldn't update the block")
  const [pinned, setPinned] = useOptimisticFlag(pageId, block, 'is_pinned', "Couldn't update the block")
  const Icon = BLOCK_ICONS[block.type]
  const { title, detail } = blockSummary(block)

  return (
    <div
      data-block-row={block.id}
      className={cn(
        'flex items-center gap-3 rounded-xl border border-border bg-card px-3 py-3',
        dragging && 'shadow-lg',
      )}
    >
      <button
        type="button"
        aria-label="Reorder block"
        className="flex size-8 shrink-0 cursor-grab touch-none items-center justify-center rounded text-muted-foreground hover:bg-muted active:cursor-grabbing"
        {...dragHandleProps}
      >
        <GripVertical className="size-4" aria-hidden />
      </button>
      <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">
        <Icon className="size-4" aria-hidden />
      </span>
      <div className={cn('min-w-0 flex-1', !active && 'opacity-60')}>
        <p className="truncate text-sm font-semibold text-foreground">{title || 'Untitled'}</p>
        {detail && <p className="truncate text-xs text-muted-foreground">{detail}</p>}
      </div>
      <div className="flex shrink-0 items-center gap-1">
        <Switch aria-label="Active" checked={active} onCheckedChange={setActive} className="mr-1" />
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label="Pin"
          aria-pressed={pinned}
          title={pinned ? 'Unpin' : 'Pin to top'}
          onClick={() => setPinned(!pinned)}
          className={cn(pinned && 'text-primary')}
        >
          <Pin className={cn(pinned && 'fill-current')} />
        </Button>
        <Button type="button" variant="ghost" size="icon-sm" aria-label="Edit" title="Edit" onClick={onEdit}>
          <Pencil />
        </Button>
        <Button type="button" variant="ghost" size="icon-sm" aria-label="Delete" title="Delete" onClick={onDelete}>
          <Trash2 />
        </Button>
      </div>
    </div>
  )
}
