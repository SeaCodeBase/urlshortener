'use client'

import { useId, useState, type FormEvent } from 'react'
import NextLink from 'next/link'
import { Info } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { hostOf } from '@/lib/format'
import { usePageMutations, usePages } from '@/lib/hooks'
import type { Link, Page } from '@/types'

/** Link block button titles are 1–80 characters. */
export const BUTTON_TITLE_MAX = 80

/** A page's display name: its title, or its slug when untitled. */
export function pageLabel(page: Pick<Page, 'title' | 'slug'>): string {
  return page.title.trim() || page.slug
}

/** Default button title for a link: its title, else its destination host. */
export function defaultButtonTitle(link: Pick<Link, 'title' | 'original_url'>): string {
  return (link.title?.trim() || hostOf(link.original_url)).slice(0, BUTTON_TITLE_MAX)
}

/** Page picker shared by the dialog and the create-link form. */
export function PageSelect({
  pages,
  value,
  onChange,
  id,
}: {
  pages: Page[]
  value: string
  onChange: (v: string) => void
  id?: string
}) {
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger id={id} aria-label="Page" className="w-full">
        <SelectValue placeholder="Choose a page" />
      </SelectTrigger>
      <SelectContent>
        {pages.map((p) => (
          <SelectItem key={p.id} value={String(p.id)}>
            {pageLabel(p)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

/** "You don't have any pages yet" + "Create a page". */
export function NoPagesYet() {
  return (
    <div className="space-y-1 text-sm">
      <p className="text-muted-foreground">You don&apos;t have any pages yet</p>
      <NextLink href="/dashboard/pages/new" className="font-semibold text-primary hover:underline">
        Create a page
      </NextLink>
    </div>
  )
}

export interface AddToPageDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  link: Link
  onAdded?: (page: Page) => void
}

/** "Add to a page": adds the link as a link block on one of the group's pages. */
export function AddToPageDialog({ open, onOpenChange, link, onAdded }: AddToPageDialogProps) {
  const { addBlock } = usePageMutations()
  const saving = addBlock.isPending
  return (
    <Dialog open={open} onOpenChange={(o) => !saving && onOpenChange(o)}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle className="text-xl font-semibold">Add to a page</DialogTitle>
          <DialogDescription>Add this link as a button on one of your pages.</DialogDescription>
        </DialogHeader>
        {/* Mounted only while open so the draft resets each time. */}
        {open && <AddToPageBody link={link} onClose={() => onOpenChange(false)} onAdded={onAdded} />}
      </DialogContent>
    </Dialog>
  )
}

function AddToPageBody({ link, onClose, onAdded }: { link: Link; onClose: () => void; onAdded?: (page: Page) => void }) {
  const id = useId()
  const { data, isLoading } = usePages({ archived: 'off' })
  const { addBlock } = usePageMutations()
  const pages = data?.pages ?? []
  const [pageChoice, setPageChoice] = useState<string | null>(null)
  const [title, setTitle] = useState(() => defaultButtonTitle(link))
  const pageId = pageChoice ?? (pages[0] ? String(pages[0].id) : '')
  const page = pages.find((p) => String(p.id) === pageId)
  const trimmed = title.trim()
  const valid = !!page && trimmed.length >= 1 && trimmed.length <= BUTTON_TITLE_MAX

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!valid || !page || addBlock.isPending) return
    try {
      await addBlock.mutateAsync({
        pageId: page.id,
        input: { type: 'link', link_id: link.id, content: { title: trimmed, description: '', thumbnail_url: '' } },
      })
      toast.success(`Added to ${pageLabel(page)}`)
      onAdded?.(page)
      onClose()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Couldn't add the link to the page")
    }
  }

  if (isLoading) return <p className="text-sm text-muted-foreground">Loading pages…</p>

  if (pages.length === 0) {
    return (
      <>
        <NoPagesYet />
        <DialogFooter>
          <Button type="button" variant="secondary" onClick={onClose}>
            Cancel
          </Button>
        </DialogFooter>
      </>
    )
  }

  return (
    <form onSubmit={submit} noValidate className="space-y-4">
      <div className="space-y-1.5">
        <Label htmlFor={`${id}-page`}>Page</Label>
        <PageSelect id={`${id}-page`} pages={pages} value={pageId} onChange={setPageChoice} />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor={`${id}-title`}>Button title</Label>
        <Input
          id={`${id}-title`}
          value={title}
          maxLength={BUTTON_TITLE_MAX}
          aria-invalid={trimmed.length === 0 ? true : undefined}
          onChange={(e) => setTitle(e.target.value)}
        />
      </div>
      <p className="flex items-start gap-2 rounded-lg bg-muted px-3 py-2 text-sm text-muted-foreground">
        <Info className="mt-0.5 size-4 shrink-0" aria-hidden />
        Make sure you publish your page&apos;s updates after adding this link
      </p>
      <DialogFooter>
        <Button type="button" variant="secondary" onClick={onClose} disabled={addBlock.isPending}>
          Cancel
        </Button>
        <Button type="submit" disabled={!valid || addBlock.isPending}>
          Add to page
        </Button>
      </DialogFooter>
    </form>
  )
}
