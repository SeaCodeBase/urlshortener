'use client'

import { toast } from 'sonner'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { ApiError } from '@/lib/api'
import { usePageMutations } from '@/lib/hooks'
import type { BlockInput, BlockType, PageBlock } from '@/types'
import type { BlockFormValues } from './blockFormShared'
import { ImageBlockForm } from './ImageBlockForm'
import { LinkBlockForm } from './LinkBlockForm'
import { SocialBlockForm } from './SocialBlockForm'
import { TextBlockForm } from './TextBlockForm'

export const BLOCK_TYPE_LABEL: Record<BlockType, string> = {
  link: 'Link',
  text: 'Text',
  image: 'Image',
  social: 'Social icons',
}

const DESCRIPTION: Record<BlockType, string> = {
  link: 'A button that opens one of your short links.',
  text: 'A heading and a paragraph.',
  image: 'An image from a URL or an upload.',
  social: 'Icons that link to your profiles.',
}

export interface BlockSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  pageId: number
  /** The block to edit; undefined creates a new block of `type`. */
  block?: PageBlock
  type: BlockType
}

/** Right-side sheet to add or edit a block. */
export function BlockSheet({ open, onOpenChange, pageId, block, type }: BlockSheetProps) {
  const { addBlock, updateBlock } = usePageMutations()
  const submitting = addBlock.isPending || updateBlock.isPending
  const verb = block ? 'Edit' : 'Add'
  const title = `${verb} ${BLOCK_TYPE_LABEL[type].toLowerCase()}`

  async function save(values: BlockFormValues<BlockType>) {
    try {
      if (block) {
        await updateBlock.mutateAsync({ pageId, blockId: block.id, patch: { content: values.content } })
        toast.success('Block updated')
      } else {
        await addBlock.mutateAsync({ pageId, input: { type, ...values } as BlockInput })
        toast.success('Block added')
      }
      onOpenChange(false)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : block ? "Couldn't update the block" : "Couldn't add the block")
    }
  }

  const formProps = { submitting, onSubmit: save, onCancel: () => onOpenChange(false) }
  // Keyed by block so reopening on another block starts from its content.
  const key = block ? `edit-${block.id}` : `new-${type}`

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-md">
        <SheetHeader>
          <SheetTitle className="text-xl font-semibold">{title}</SheetTitle>
          <SheetDescription>{DESCRIPTION[type]}</SheetDescription>
        </SheetHeader>
        {/* Mounted only while open so each opening starts from the block (or empty). */}
        {open && (
          <div className="flex flex-1 flex-col px-4 pb-4">
            {type === 'link' && <LinkBlockForm key={key} block={block?.type === 'link' ? block : undefined} {...formProps} />}
            {type === 'text' && <TextBlockForm key={key} block={block?.type === 'text' ? block : undefined} {...formProps} />}
            {type === 'image' && <ImageBlockForm key={key} block={block?.type === 'image' ? block : undefined} {...formProps} />}
            {type === 'social' && <SocialBlockForm key={key} block={block?.type === 'social' ? block : undefined} {...formProps} />}
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}
