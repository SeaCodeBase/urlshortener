'use client'

import { useState } from 'react'
import type { BlockType, Page, PageBlock } from '@/types'
import { AddBlockMenu } from './AddBlockMenu'
import { BlockList } from './BlockList'
import { BlockSheet } from './BlockSheet'

interface SheetState {
  open: boolean
  type: BlockType
  block?: PageBlock
}

/** Build tab: "+ Add", the reorderable block list, and the add/edit sheet. */
export function BuildTab({ page }: { page: Page }) {
  // type/block stay set while the sheet animates closed.
  const [sheet, setSheet] = useState<SheetState>({ open: false, type: 'link' })
  const blocks = page.blocks ?? []

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h2 className="text-lg font-semibold text-foreground">Blocks</h2>
          <p className="text-sm text-muted-foreground">Drag to reorder. Pinned blocks show first on your page.</p>
        </div>
        <AddBlockMenu onAdd={(type) => setSheet({ open: true, type })} />
      </div>

      {blocks.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border bg-card p-8 text-center">
          <p className="font-semibold text-foreground">No blocks yet</p>
          <p className="mt-1 text-sm text-muted-foreground">Add links, text, images or social icons to your page.</p>
        </div>
      ) : (
        <BlockList pageId={page.id} blocks={blocks} onEdit={(block) => setSheet({ open: true, type: block.type, block })} />
      )}

      <BlockSheet
        open={sheet.open}
        onOpenChange={(open) => setSheet((s) => ({ ...s, open }))}
        pageId={page.id}
        type={sheet.type}
        block={sheet.block}
      />
    </div>
  )
}
