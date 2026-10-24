'use client'

import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import type { BlockType } from '@/types'
import { BLOCK_TYPE_LABEL } from './BlockSheet'
import { BLOCK_ICONS } from './BlockRow'

const TYPES: BlockType[] = ['link', 'text', 'image', 'social']

/** "+ Add" dropdown: Link, Text, Image, Social icons. */
export function AddBlockMenu({ onAdd }: { onAdd: (type: BlockType) => void }) {
  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild>
        <Button type="button">+ Add</Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-48">
        {TYPES.map((t) => {
          const Icon = BLOCK_ICONS[t]
          return (
            <DropdownMenuItem key={t} onSelect={() => onAdd(t)}>
              <Icon />
              {BLOCK_TYPE_LABEL[t]}
            </DropdownMenuItem>
          )
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
