'use client'

import { useState } from 'react'
import NextLink from 'next/link'
import { BarChart2, Copy, Eye, EyeOff, MoreHorizontal, Trash2 } from 'lucide-react'
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
import { useBulkQrCodes, useDeleteQrCode } from '@/lib/hooks'
import type { QrCode } from '@/types'

/** "⋯" menu on a QR Code card. */
export function QrActionsMenu({
  qr,
  onDeleted,
  triggerClassName = 'size-8',
}: {
  qr: QrCode
  /** Called after a successful delete (e.g. to leave the details page). */
  onDeleted?: () => void
  triggerClassName?: string
}) {
  const [confirmDelete, setConfirmDelete] = useState(false)
  const bulk = useBulkQrCodes()
  const del = useDeleteQrCode()

  function toggleHidden() {
    const hiding = !qr.archived
    bulk.mutate(
      { action: hiding ? 'archive' : 'unarchive', ids: [qr.id] },
      {
        onSuccess: () => toast.success(hiding ? 'QR Code hidden' : 'QR Code shown'),
        onError: () => toast.error(hiding ? "Couldn't hide the QR Code" : "Couldn't show the QR Code"),
      },
    )
  }

  function onDelete() {
    return del.mutateAsync(qr.id).then(
      () => {
        toast.success('QR Code deleted')
        setConfirmDelete(false)
        onDeleted?.()
      },
      (err: unknown) => {
        toast.error(err instanceof ApiError ? err.message : "Couldn't delete the QR Code")
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
        <DropdownMenuContent align="end" className="w-56">
          <DropdownMenuItem asChild>
            <NextLink href={`/dashboard/qr-codes/${qr.id}`}>
              <BarChart2 />
              View details
            </NextLink>
          </DropdownMenuItem>
          <DropdownMenuItem asChild>
            <NextLink href={`/dashboard/qr-codes/create?design=${qr.id}`}>
              <Copy />
              Duplicate design
            </NextLink>
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem onSelect={toggleHidden}>
            {qr.archived ? <Eye /> : <EyeOff />}
            {qr.archived ? 'Show code' : 'Hide code'}
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
        title="Delete QR Code?"
        description={`This permanently deletes the QR Code${qr.link_source === 'qr' ? ' and its short link' : ''}.`}
        confirmLabel="Delete"
        destructive
        loading={del.isPending}
        onConfirm={onDelete}
      />
    </>
  )
}
