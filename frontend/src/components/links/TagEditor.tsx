'use client'

import { useState } from 'react'
import { PlusCircle } from 'lucide-react'
import { toast } from 'sonner'
import { TagPicker } from '@/components/common/TagPicker'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { useSetLinkTags, useTags } from '@/lib/hooks'

/** Inline tag list for a link: chips (or "No tags") + "Add tag" popover that saves via setLinkTags. */
export function TagEditor({ linkId, tags }: { linkId: number; tags: string[] }) {
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState<string[]>(tags)
  const { data: suggestions = [] } = useTags()
  const setLinkTags = useSetLinkTags()

  function onOpenChange(o: boolean) {
    if (o) setDraft(tags)
    setOpen(o)
  }

  function save() {
    setLinkTags.mutate(
      { id: linkId, tags: draft },
      {
        onSuccess: () => {
          toast.success('Tags updated')
          setOpen(false)
        },
        onError: () => toast.error("Couldn't update tags"),
      },
    )
  }

  return (
    <div className="flex flex-wrap items-center gap-2 text-sm">
      {tags.length ? (
        tags.map((t) => (
          <span key={t} className="rounded bg-muted px-2 py-0.5 text-sm text-foreground">
            {t}
          </span>
        ))
      ) : (
        <span className="text-muted-foreground">No tags</span>
      )}
      <Popover open={open} onOpenChange={onOpenChange}>
        <PopoverTrigger asChild>
          <button type="button" className="inline-flex items-center gap-1 text-sm font-medium text-primary hover:underline">
            <PlusCircle className="size-3.5" />
            Add tag
          </button>
        </PopoverTrigger>
        <PopoverContent align="start" className="w-80">
          <div className="grid gap-3">
            <TagPicker value={draft} onChange={setDraft} suggestions={suggestions} />
            <div className="flex justify-end gap-2">
              <Button type="button" variant="secondary" size="sm" onClick={() => setOpen(false)}>
                Cancel
              </Button>
              <Button type="button" size="sm" disabled={setLinkTags.isPending} onClick={save}>
                Save
              </Button>
            </div>
          </div>
        </PopoverContent>
      </Popover>
    </div>
  )
}
