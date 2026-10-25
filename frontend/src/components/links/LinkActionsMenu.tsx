'use client'

import { useState, type ReactNode } from 'react'
import NextLink from 'next/link'
import { BarChart2, Eye, EyeOff, FileText, Megaphone, MoreHorizontal, QrCode, Trash2 } from 'lucide-react'
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
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { ApiError } from '@/lib/api'
import { useBulkLinks, useDeleteLink } from '@/lib/hooks'
import { cn } from '@/lib/utils'
import type { Link } from '@/types'
import { AddToCampaignDialog } from './AddToCampaignDialog'
import { AddToPageDialog } from './AddToPageDialog'

export const CANT_DELETE_MESSAGE = "This link can't be deleted. You can hide it instead."

/** Bitly rule: links that were ever redirected or have a custom back-half can only be hidden. */
export function isDeletable(link: Pick<Link, 'redirect_count' | 'has_custom_back_half'>): boolean {
  return !(link.redirect_count > 0 || link.has_custom_back_half)
}

/** A disabled menu item with a hover tooltip (the wrapper takes the pointer since the item ignores it). */
function DisabledItem({ tooltip, children }: { tooltip: string; children: ReactNode }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div>
          <DropdownMenuItem disabled>{children}</DropdownMenuItem>
        </div>
      </TooltipTrigger>
      <TooltipContent side="left">{tooltip}</TooltipContent>
    </Tooltip>
  )
}

/** "⋯" menu on a link card (spec §5.2). */
export function LinkActionsMenu({
  link,
  triggerClassName,
  onDeleted,
}: {
  link: Link
  triggerClassName?: string
  /** Called after a successful delete (e.g. to leave the details page). */
  onDeleted?: () => void
}) {
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [addToPage, setAddToPage] = useState(false)
  const [addToCampaign, setAddToCampaign] = useState(false)
  const bulk = useBulkLinks()
  const del = useDeleteLink()
  const deletable = isDeletable(link)

  function toggleHidden() {
    const hiding = !link.archived
    bulk.mutate(
      { action: hiding ? 'archive' : 'unarchive', ids: [link.id] },
      {
        onSuccess: () => toast.success(hiding ? 'Link hidden' : 'Link shown'),
        onError: () => toast.error(hiding ? "Couldn't hide the link" : "Couldn't show the link"),
      },
    )
  }

  function onDelete() {
    return del.mutateAsync(link.id).then(
      () => {
        toast.success('Link deleted')
        setConfirmDelete(false)
        onDeleted?.()
      },
      (err: unknown) => {
        toast.error(err instanceof ApiError ? err.message : "Couldn't delete the link")
        setConfirmDelete(false)
      },
    )
  }

  return (
    <>
      <DropdownMenu modal={false}>
        <DropdownMenuTrigger asChild>
          <Button type="button" variant="secondary" size="icon-sm" aria-label="More" className={cn('size-8', triggerClassName)}>
            <MoreHorizontal className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-56">
          <DropdownMenuItem asChild>
            <NextLink href={`/dashboard/links/${link.id}`}>
              <BarChart2 />
              View link details
            </NextLink>
          </DropdownMenuItem>
          <DropdownMenuItem asChild>
            {link.qr_code_id ? (
              <NextLink href={`/dashboard/qr-codes/${link.qr_code_id}`}>
                <QrCode />
                View QR Code
              </NextLink>
            ) : (
              <NextLink href={`/dashboard/qr-codes/create?link=${link.id}`}>
                <QrCode />
                Create QR Code
              </NextLink>
            )}
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={() => setAddToPage(true)}>
            <FileText />
            Add to a page
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={() => setAddToCampaign(true)}>
            <Megaphone />
            Add to campaign
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem onSelect={toggleHidden}>
            {link.archived ? <Eye /> : <EyeOff />}
            {link.archived ? 'Show link' : 'Hide link'}
          </DropdownMenuItem>
          {deletable ? (
            <DropdownMenuItem variant="destructive" onSelect={() => setConfirmDelete(true)}>
              <Trash2 />
              Delete
            </DropdownMenuItem>
          ) : (
            <DisabledItem tooltip={CANT_DELETE_MESSAGE}>
              <Trash2 />
              Delete
            </DisabledItem>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      <AddToPageDialog open={addToPage} onOpenChange={setAddToPage} link={link} />
      <AddToCampaignDialog open={addToCampaign} onOpenChange={setAddToCampaign} link={link} />
      <ConfirmDialog
        open={confirmDelete}
        onOpenChange={setConfirmDelete}
        title="Delete link?"
        description={
          link.qr_code_id
            ? 'This permanently deletes the link, its click data and its QR Code.'
            : 'This permanently deletes the link and its click data.'
        }
        confirmLabel="Delete"
        destructive
        loading={del.isPending}
        onConfirm={onDelete}
      />
    </>
  )
}
