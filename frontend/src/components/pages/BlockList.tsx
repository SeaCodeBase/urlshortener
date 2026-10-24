'use client'

import { useId, useRef, useState } from 'react'
import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import { SortableContext, arrayMove, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/common/ConfirmDialog'
import { ApiError } from '@/lib/api'
import { usePageMutations } from '@/lib/hooks'
import { cn } from '@/lib/utils'
import type { PageBlock } from '@/types'
import { BlockRow } from './BlockRow'

interface RowHandlers {
  pageId: number
  onEdit: (block: PageBlock) => void
  onDelete: (block: PageBlock) => void
}

function SortableBlock({ block, pageId, onEdit, onDelete }: RowHandlers & { block: PageBlock }) {
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({ id: block.id })
  return (
    <li
      ref={setNodeRef}
      style={{ transform: CSS.Translate.toString(transform), transition }}
      className={cn(isDragging && 'relative z-10 opacity-90')}
    >
      <BlockRow
        pageId={pageId}
        block={block}
        dragging={isDragging}
        onEdit={() => onEdit(block)}
        onDelete={() => onDelete(block)}
        dragHandleProps={{ ref: setActivatorNodeRef, ...attributes, ...listeners }}
      />
    </li>
  )
}

const idsKey = (blocks: PageBlock[]) => blocks.map((b) => b.id).join(',')

/**
 * The page's blocks in order. Drag handles (pointer or keyboard) reorder them: the new order shows
 * at once and persists through `reorder`, reverting with a toast when it fails.
 */
export function BlockList({ pageId, blocks, onEdit }: { pageId: number; blocks: PageBlock[]; onEdit: (block: PageBlock) => void }) {
  const dndId = useId()
  const { reorder, deleteBlock } = usePageMutations()
  // Pending order, valid while the server's order is still the one it was made from.
  const [pending, setPending] = useState<{ ids: number[]; base: string } | null>(null)
  const latest = useRef(0)
  const [toDelete, setToDelete] = useState<PageBlock | null>(null)

  const serverKey = idsKey(blocks)
  const byId = new Map(blocks.map((b) => [b.id, b]))
  const shown =
    pending && pending.base === serverKey
      ? pending.ids.map((id) => byId.get(id)).filter((b): b is PageBlock => b != null)
      : blocks

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  function onDragEnd({ active, over }: DragEndEvent) {
    if (!over || active.id === over.id) return
    const ids = shown.map((b) => b.id)
    const from = ids.indexOf(Number(active.id))
    const to = ids.indexOf(Number(over.id))
    if (from < 0 || to < 0) return
    const next = arrayMove(ids, from, to)
    const call = ++latest.current
    setPending({ ids: next, base: serverKey })
    reorder.mutateAsync({ pageId, ids: next }).catch(() => {
      if (call === latest.current) setPending(null)
      toast.error("Couldn't reorder the blocks")
    })
  }

  function confirmDelete() {
    if (!toDelete) return
    return deleteBlock.mutateAsync({ pageId, blockId: toDelete.id }).then(
      () => {
        toast.success('Block deleted')
        setToDelete(null)
      },
      (err: unknown) => {
        toast.error(err instanceof ApiError ? err.message : "Couldn't delete the block")
        setToDelete(null)
      },
    )
  }

  return (
    <>
      <DndContext id={dndId} sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
        <SortableContext items={shown.map((b) => b.id)} strategy={verticalListSortingStrategy}>
          <ul className="space-y-2" aria-label="Blocks">
            {shown.map((b) => (
              <SortableBlock key={b.id} block={b} pageId={pageId} onEdit={onEdit} onDelete={setToDelete} />
            ))}
          </ul>
        </SortableContext>
      </DndContext>
      <ConfirmDialog
        open={toDelete != null}
        onOpenChange={(o) => !o && setToDelete(null)}
        title="Delete block?"
        description="This removes the block from the page. Its link stays in your links."
        confirmLabel="Delete"
        destructive
        loading={deleteBlock.isPending}
        onConfirm={confirmDelete}
      />
    </>
  )
}
