'use client'

import { useState } from 'react'
import { Check, ChevronDown } from 'lucide-react'
import { useAuthStore } from '@/stores/auth'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { cn } from '@/lib/utils'

/** Top-bar group picker; the active group is sent as X-Group-ID by the API client. */
export function GroupSwitcher() {
  const groups = useAuthStore((s) => s.groups)
  const activeGroupId = useAuthStore((s) => s.activeGroupId)
  const setActiveGroup = useAuthStore((s) => s.setActiveGroup)
  const [open, setOpen] = useState(false)
  const [filter, setFilter] = useState('')

  const active = groups.find((g) => g.id === activeGroupId)
  const visible = groups.filter((g) => g.name.toLowerCase().includes(filter.trim().toLowerCase()))

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (!next) setFilter('')
      }}
    >
      <PopoverTrigger className="flex h-10 max-w-[240px] items-center gap-2 rounded-[4px] px-2 text-base font-medium text-foreground hover:bg-accent">
        <span className="truncate">{active?.name ?? 'Select group'}</span>
        <ChevronDown className="size-4 shrink-0" aria-hidden />
      </PopoverTrigger>
      <PopoverContent align="start" className="w-64 p-2">
        <Input
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="Search groups..."
          aria-label="Search groups"
          className="mb-2"
        />
        <ul role="listbox" aria-label="Groups" className="max-h-64 overflow-y-auto">
          {visible.map((g) => {
            const selected = g.id === activeGroupId
            return (
              <li key={g.id} role="option" aria-selected={selected}>
                <button
                  type="button"
                  onClick={() => {
                    setActiveGroup(g.id)
                    setOpen(false)
                    setFilter('')
                  }}
                  className={cn(
                    'flex h-9 w-full items-center gap-2 rounded-[4px] px-2 text-left text-sm text-foreground hover:bg-accent',
                    selected && 'font-medium'
                  )}
                >
                  <Check className={cn('size-4 shrink-0', !selected && 'invisible')} aria-hidden />
                  <span className="truncate">{g.name}</span>
                </button>
              </li>
            )
          })}
          {visible.length === 0 && <li className="px-2 py-2 text-sm text-muted-foreground">No groups found</li>}
        </ul>
      </PopoverContent>
    </Popover>
  )
}
