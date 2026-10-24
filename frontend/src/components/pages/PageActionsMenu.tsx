'use client'

import { useState } from 'react'
import { ExternalLink, Eye, EyeOff, MoreHorizontal, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/common/ConfirmDialog'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ApiError } from '@/lib/api'
import { usePageMutations } from '@/lib/hooks'
import type { Page } from '@/types'

/** "⋯" menu on a page card (and the editor): optional "View page", Hide/Show and Delete (with confirmation). */
export function PageActionsMenu({
  page,
  onDeleted,
  triggerClassName = 'size-8',
  showViewPage = false,
}: {
  page: Page
  /** Adds "View page" (opens `page.url` in a new tab; disabled until the page is published). */
  showViewPage?: boolean
  /** Called after a successful delete (e.g. to leave the editor). */
  onDeleted?: () => void
  triggerClassName?: string
}) {
  const [confirmDelete, setConfirmDelete] = useState(false)
  const { setArchived, remove } = usePageMutations()

  function toggleHidden() {
    const hiding = !page.archived
    setArchived.mutate(
      { id: page.id, archived: hiding },
      {
        onSuccess: () => toast.success(hiding ? 'Page hidden' : 'Page shown'),
        onError: () => toast.error(hiding ? "Couldn't hide the page" : "Couldn't show the page"),
      },
    )
  }

  function onDelete() {
    return remove.mutateAsync(page.id).then(
      () => {
        toast.success('Page deleted')
        setConfirmDelete(false)
        onDeleted?.()
      },
      (err: unknown) => {
        toast.error(err instanceof ApiError ? err.message : "Couldn't delete the page")
        setConfirmDelete(false)
      },
    )
  }

  return (
    <>
      <DropdownMenu modal={false}>
        <DropdownMenuTrigger asChild>
          <Button type="button" variant="secondary" size="icon-sm" aria-label="More" className={triggerClassName}>
            <MoreHorizontal className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-48">
          {showViewPage && (
            <>
              <DropdownMenuItem
                disabled={page.status !== 'published'}
                onSelect={() => window.open(page.url, '_blank', 'noopener,noreferrer')}
              >
                <ExternalLink />
                View page
              </DropdownMenuItem>
              <DropdownMenuSeparator />
            </>
          )}
          <DropdownMenuItem onSelect={toggleHidden}>
            {page.archived ? <Eye /> : <EyeOff />}
            {page.archived ? 'Show' : 'Hide'}
          </DropdownMenuItem>
          <DropdownMenuItem variant="destructive" onSelect={() => setConfirmDelete(true)}>
            <Trash2 />
            Delete
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <ConfirmDialog
        open={confirmDelete}
        onOpenChange={setConfirmDelete}
        title="Delete page?"
        description="This permanently deletes the page and its blocks. Links stay."
        confirmLabel="Delete"
        destructive
        loading={remove.isPending}
        onConfirm={onDelete}
      />
    </>
  )
}
