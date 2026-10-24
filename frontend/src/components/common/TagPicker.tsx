'use client'

import { useState, type KeyboardEvent } from 'react'
import { Check, ChevronDown, Plus, X } from 'lucide-react'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import { Popover, PopoverAnchor, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { cn } from '@/lib/utils'

export const MAX_TAG_LENGTH = 100
const CREATE_VALUE = '__create_tag__'

/** Trims and caps a typed tag name at 100 characters. */
export function normalizeTag(raw: string): string {
  return raw.trim().slice(0, MAX_TAG_LENGTH).trim()
}

/** Multi-select tag combobox: chips with ✕, filterable suggestions, "Create '{x}'". */
export function TagPicker({
  value,
  onChange,
  suggestions,
  placeholder = 'Select tags',
  disabled,
}: {
  value: string[]
  onChange: (tags: string[]) => void
  suggestions: string[]
  placeholder?: string
  disabled?: boolean
}) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [highlighted, setHighlighted] = useState('')

  const typed = normalizeTag(query)
  const lower = typed.toLowerCase()
  const options = Array.from(new Set([...value, ...suggestions])).filter((t) =>
    t.toLowerCase().includes(lower),
  )
  const exists = [...value, ...suggestions].some((t) => t.toLowerCase() === lower)
  const canCreate = typed.length > 0 && !exists

  function toggle(tag: string) {
    onChange(value.includes(tag) ? value.filter((t) => t !== tag) : [...value, tag])
  }

  function create() {
    if (!canCreate) return
    onChange([...value, typed])
    setQuery('')
  }

  function onInputKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key !== 'Enter' || !canCreate) return
    // Let cmdk select an existing option the user arrowed to; otherwise Enter creates.
    if (highlighted && highlighted !== CREATE_VALUE) return
    e.preventDefault()
    create()
  }

  return (
    <Popover open={open} onOpenChange={(o) => !disabled && setOpen(o)}>
      <PopoverAnchor asChild>
        <div
          className={cn(
            'flex min-h-10 w-full flex-wrap items-center gap-1 rounded border border-border bg-background py-1 pr-1 pl-2',
            disabled && 'cursor-not-allowed bg-disabled-bg',
          )}
        >
          {value.map((tag) => (
            <span
              key={tag}
              className="inline-flex max-w-full items-center gap-1 rounded bg-muted px-2 py-0.5 text-sm text-foreground"
            >
              <span className="truncate">{tag}</span>
              <button
                type="button"
                aria-label={`Remove tag ${tag}`}
                disabled={disabled}
                onClick={() => onChange(value.filter((t) => t !== tag))}
                className="rounded text-muted-foreground hover:text-foreground"
              >
                <X className="size-3.5" />
              </button>
            </span>
          ))}
          <PopoverTrigger asChild>
            <button
              type="button"
              aria-label={placeholder}
              disabled={disabled}
              className="flex h-8 min-w-24 flex-1 items-center justify-between gap-2 px-1 text-left text-base text-muted-foreground outline-none disabled:cursor-not-allowed"
            >
              <span className="truncate">{value.length ? '' : placeholder}</span>
              <ChevronDown className="size-4 shrink-0" />
            </button>
          </PopoverTrigger>
        </div>
      </PopoverAnchor>
      <PopoverContent align="start" className="w-(--radix-popover-trigger-width) min-w-64 p-0">
        <Command shouldFilter={false} value={highlighted} onValueChange={setHighlighted}>
          <CommandInput
            placeholder="Search or create a tag"
            value={query}
            maxLength={MAX_TAG_LENGTH + 20}
            onValueChange={setQuery}
            onKeyDown={onInputKeyDown}
          />
          <CommandList>
            {!canCreate && <CommandEmpty>No tags found</CommandEmpty>}
            <CommandGroup>
              {canCreate && (
                <CommandItem value={CREATE_VALUE} onSelect={create}>
                  <Plus className="size-4" />
                  {`Create '${typed}'`}
                </CommandItem>
              )}
              {options.map((tag) => (
                <CommandItem key={tag} value={tag} onSelect={() => toggle(tag)}>
                  <Check className={cn('size-4', value.includes(tag) ? 'opacity-100' : 'opacity-0')} />
                  <span className="truncate">{tag}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
