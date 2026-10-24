'use client'

import { useState } from 'react'
import { TagPicker } from '@/components/common/TagPicker'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

export interface BulkTagDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  count: number
  suggestions: string[]
  saving?: boolean
  onSave: (tags: string[]) => void
  /** What is being tagged (default "links"). */
  noun?: 'links' | 'codes'
}

const NOUNS = {
  links: { title: 'Tag links', one: 'link', many: 'links' },
  codes: { title: 'Tag QR Codes', one: 'QR Code', many: 'QR Codes' },
} as const

/** "Tag links" dialog: pick or create tags to add to the selected links (or QR Codes). */
export function BulkTagDialog({ open, onOpenChange, count, suggestions, saving, onSave, noun = 'links' }: BulkTagDialogProps) {
  const words = NOUNS[noun]
  return (
    <Dialog open={open} onOpenChange={(o) => !saving && onOpenChange(o)}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle className="text-xl font-semibold">{words.title}</DialogTitle>
          <DialogDescription>
            Add tags to {count} selected {count === 1 ? words.one : words.many}.
          </DialogDescription>
        </DialogHeader>
        {/* Mounted only while open so the draft starts empty each time. */}
        {open && <BulkTagBody suggestions={suggestions} saving={saving} onCancel={() => onOpenChange(false)} onSave={onSave} />}
      </DialogContent>
    </Dialog>
  )
}

function BulkTagBody({
  suggestions,
  saving,
  onCancel,
  onSave,
}: {
  suggestions: string[]
  saving?: boolean
  onCancel: () => void
  onSave: (tags: string[]) => void
}) {
  const [tags, setTags] = useState<string[]>([])
  return (
    <>
      <TagPicker value={tags} onChange={setTags} suggestions={suggestions} />
      <DialogFooter>
        <Button type="button" variant="secondary" onClick={onCancel} disabled={saving}>
          Cancel
        </Button>
        <Button type="button" onClick={() => onSave(tags)} disabled={saving || tags.length === 0}>
          Save
        </Button>
      </DialogFooter>
    </>
  )
}
