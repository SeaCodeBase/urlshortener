'use client'

import { useId, useState, type KeyboardEvent } from 'react'
import { Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { ApiError } from '@/lib/api'
import { useCampaignMutations, useChannels } from '@/lib/hooks'

const MAX_CHANNEL_NAME = 80

/**
 * Multi-select of the group's channels (checkboxes) with an inline
 * "Create new channel" input: the channel is created right away and then selected.
 */
export function ChannelPicker({ value, onChange }: { value: number[]; onChange: (ids: number[]) => void }) {
  const inputId = useId()
  const channels = useChannels()
  const { createChannel } = useCampaignMutations()
  const [name, setName] = useState('')
  const [error, setError] = useState<string | null>(null)
  const list = channels.data ?? []
  const trimmed = name.trim()

  function toggle(id: number, checked: boolean) {
    if (checked) {
      if (!value.includes(id)) onChange([...value, id])
    } else {
      onChange(value.filter((v) => v !== id))
    }
  }

  async function add() {
    if (!trimmed || createChannel.isPending) return
    setError(null)
    const existing = list.find((c) => c.name.toLowerCase() === trimmed.toLowerCase())
    if (existing) {
      toggle(existing.id, true)
      setName('')
      return
    }
    try {
      const created = await createChannel.mutateAsync(trimmed)
      if (!value.includes(created.id)) onChange([...value, created.id])
      setName('')
    } catch (err) {
      setError(err instanceof ApiError || err instanceof Error ? err.message : "Couldn't create the channel")
    }
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    // Enter creates the channel instead of submitting the surrounding form.
    if (e.key === 'Enter') {
      e.preventDefault()
      void add()
    }
  }

  return (
    <div className="flex flex-col gap-3">
      {channels.isPending ? (
        <div className="flex flex-col gap-2" aria-hidden>
          <Skeleton className="h-5 w-1/3" />
          <Skeleton className="h-5 w-1/4" />
        </div>
      ) : list.length === 0 ? (
        <p className="text-sm text-muted-foreground">No channels yet. Create one below.</p>
      ) : (
        <ul className="flex max-h-48 flex-col gap-2 overflow-y-auto">
          {list.map((c) => {
            const id = `${inputId}-ch-${c.id}`
            return (
              <li key={c.id} className="flex items-center gap-2">
                <Checkbox
                  id={id}
                  aria-label={c.name}
                  checked={value.includes(c.id)}
                  onCheckedChange={(checked) => toggle(c.id, checked)}
                />
                <label htmlFor={id} className="cursor-pointer text-sm text-foreground">
                  {c.name}
                </label>
              </li>
            )
          })}
        </ul>
      )}
      <div className="flex items-center gap-2">
        <Input
          id={inputId}
          aria-label="New channel name"
          placeholder="New channel name"
          value={name}
          maxLength={MAX_CHANNEL_NAME}
          onChange={(e) => {
            setName(e.target.value)
            setError(null)
          }}
          onKeyDown={onKeyDown}
        />
        <Button
          type="button"
          variant="secondary"
          onClick={() => void add()}
          disabled={!trimmed || createChannel.isPending}
          className="shrink-0"
        >
          <Plus />
          Create new channel
        </Button>
      </div>
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
    </div>
  )
}
