'use client'

import { useRef, useState } from 'react'
import NextLink from 'next/link'
import { useRouter } from 'next/navigation'
import { Eye, Pencil, Share2 } from 'lucide-react'
import { toast } from 'sonner'
import { ShareDialog } from '@/components/links/ShareDialog'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { ApiError } from '@/lib/api'
import { usePageMutations, usePageSavesPending, useWaitForPageSaves } from '@/lib/hooks'
import type { Page } from '@/types'
import { PageActionsMenu } from './PageActionsMenu'
import { PageStatusChip } from './PageCard'
import { PhonePreview } from './PhonePreview'

export const PAGES_HREF = '/dashboard/pages'
const TITLE_MAX = 255

const errorMessage = (err: unknown, fallback: string) => (err instanceof ApiError ? err.message : fallback)

/** Click to edit; Enter or blur saves, Escape cancels; an empty title is not saved. */
function EditableTitle({ title, onSave }: { title: string; onSave: (title: string) => Promise<void> }) {
  const [draft, setDraft] = useState<string | null>(null)
  const done = useRef(false)

  if (draft == null) {
    return (
      <h1 className="min-w-0">
        <button
          type="button"
          onClick={() => {
            done.current = false
            setDraft(title)
          }}
          aria-label={`Rename page ${title}`}
          className="group inline-flex max-w-full items-center gap-2 rounded text-left text-2xl leading-8 font-semibold text-foreground hover:bg-accent"
        >
          <span className="truncate">{title}</span>
          <Pencil className="size-4 shrink-0 text-muted-foreground opacity-0 group-hover:opacity-100" aria-hidden />
        </button>
      </h1>
    )
  }

  const finish = async (save: boolean) => {
    if (done.current) return
    done.current = true
    const next = draft.trim()
    setDraft(null)
    if (save && next && next !== title) await onSave(next)
  }

  return (
    <Input
      autoFocus
      aria-label="Page title"
      maxLength={TITLE_MAX}
      value={draft}
      onChange={(e) => setDraft(e.target.value)}
      onBlur={() => finish(true)}
      onKeyDown={(e) => {
        if (e.key === 'Enter') {
          e.preventDefault()
          finish(true)
        } else if (e.key === 'Escape') {
          e.preventDefault()
          finish(false)
        }
      }}
      className="h-10 max-w-md text-xl font-semibold"
    />
  )
}

export interface EditorTopBarProps {
  page: Page
  /** Sends edits still waiting in the editor (the Design tab's debounce); resolves when they settle. */
  flushPendingSaves?: () => Promise<void>
  /** An edit is shown in the editor but not saved yet (the Design tab's preview override). */
  designPending?: boolean
}

/**
 * Editor header: "← Pages", inline title, status, Preview (dialog), Share, Publish changes and ⋯
 * (View page / Hide / Delete; deleting returns to the Pages list). Publishing first flushes pending
 * edits and waits for in-flight saves, so the snapshot never misses them; while anything is saving
 * the button is disabled ("Saving…").
 */
export function EditorTopBar({ page, flushPendingSaves, designPending = false }: EditorTopBarProps) {
  const router = useRouter()
  const { update, publish } = usePageMutations()
  const savesPending = usePageSavesPending(page.id)
  const waitForSaves = useWaitForPageSaves(page.id)
  const [preparing, setPreparing] = useState(false)
  const [previewOpen, setPreviewOpen] = useState(false)
  const [shareOpen, setShareOpen] = useState(false)
  const title = page.title || page.slug
  const saving = savesPending > 0 || designPending
  const canPublish = page.has_unpublished_changes && !saving && !preparing && !publish.isPending

  async function rename(next: string) {
    try {
      await update.mutateAsync({ id: page.id, patch: { title: next } })
    } catch (err) {
      toast.error(errorMessage(err, "Couldn't rename the page"))
    }
  }

  async function onPublish() {
    if (preparing) return
    setPreparing(true)
    try {
      await flushPendingSaves?.()
      await waitForSaves()
    } finally {
      setPreparing(false)
    }
    publish.mutate(page.id, {
      onSuccess: () => toast.success('Page published'),
      onError: (err) => toast.error(errorMessage(err, "Couldn't publish the page")),
    })
  }

  const publishButton = (
    <Button type="button" disabled={!canPublish} onClick={onPublish}>
      Publish changes
    </Button>
  )

  return (
    <div className="space-y-3 pt-8">
      <NextLink href={PAGES_HREF} className="text-sm font-semibold text-primary hover:underline">
        ← Pages
      </NextLink>
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex min-w-0 items-center gap-3">
          <EditableTitle title={title} onSave={rename} />
          <PageStatusChip page={page} />
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <Button type="button" variant="secondary" onClick={() => setPreviewOpen(true)}>
            <Eye />
            Preview
          </Button>
          <Button type="button" variant="secondary" onClick={() => setShareOpen(true)}>
            <Share2 />
            Share
          </Button>
          {page.has_unpublished_changes && !saving ? (
            publishButton
          ) : (
            <Tooltip>
              <TooltipTrigger asChild>
                {/* A disabled button gets no pointer events; the wrapper carries the tooltip. */}
                <span tabIndex={0} className="inline-flex rounded">
                  {publishButton}
                </span>
              </TooltipTrigger>
              <TooltipContent>{saving ? 'Saving…' : 'No unpublished changes'}</TooltipContent>
            </Tooltip>
          )}
          <PageActionsMenu page={page} showViewPage triggerClassName="size-10" onDeleted={() => router.push(PAGES_HREF)} />
        </div>
      </div>

      <ShareDialog open={shareOpen} onOpenChange={setShareOpen} shortUrl={page.url} title={title} />
      <Dialog open={previewOpen} onOpenChange={setPreviewOpen}>
        <DialogContent className="w-auto max-w-none sm:max-w-none">
          <DialogHeader>
            <DialogTitle>Preview</DialogTitle>
            <DialogDescription>How your page looks with the latest changes.</DialogDescription>
          </DialogHeader>
          <PhonePreview page={page} className="mx-auto h-[min(700px,75vh)]" />
        </DialogContent>
      </Dialog>
    </div>
  )
}
