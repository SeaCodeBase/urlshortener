'use client'

import { useState } from 'react'
import { Copy, MoreHorizontal, Pencil, Trash2 } from 'lucide-react'
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
import { useDashboardMutations } from '@/lib/hooks'
import type { Dashboard } from '@/types'
import { DashboardNameDialog } from './DashboardNameDialog'

/** "⋯" menu on a dashboard row: Rename / Duplicate / Delete (the default dashboard cannot be deleted). */
export function DashboardActionsMenu({ dashboard }: { dashboard: Dashboard }) {
  const { update, duplicate, remove } = useDashboardMutations()
  const [renameOpen, setRenameOpen] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)

  function onDuplicate() {
    duplicate.mutateAsync(dashboard.id).then(
      () => toast.success('Dashboard duplicated'),
      (err: unknown) => toast.error(err instanceof ApiError ? err.message : "Couldn't duplicate the dashboard"),
    )
  }

  function onDelete() {
    return remove.mutateAsync(dashboard.id).then(
      () => {
        toast.success('Dashboard deleted')
        setConfirmDelete(false)
      },
      (err: unknown) => {
        toast.error(err instanceof ApiError ? err.message : "Couldn't delete the dashboard")
        setConfirmDelete(false)
      },
    )
  }

  const deleteItem = (
    <DropdownMenuItem
      variant="destructive"
      disabled={dashboard.is_default}
      onSelect={() => setConfirmDelete(true)}
    >
      <Trash2 />
      Delete
    </DropdownMenuItem>
  )

  return (
    <>
      <DropdownMenu modal={false}>
        <DropdownMenuTrigger asChild>
          <Button type="button" variant="secondary" size="icon-sm" aria-label="More" className="size-8">
            <MoreHorizontal className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-56">
          <DropdownMenuItem onSelect={() => setRenameOpen(true)}>
            <Pencil />
            Rename
          </DropdownMenuItem>
          <DropdownMenuItem disabled={duplicate.isPending} onSelect={onDuplicate}>
            <Copy />
            Duplicate
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          {dashboard.is_default ? (
            <Tooltip>
              {/* A disabled item ignores the pointer, so the tooltip hangs off a wrapper. */}
              <TooltipTrigger asChild>
                <div>{deleteItem}</div>
              </TooltipTrigger>
              <TooltipContent side="left">The default dashboard cannot be deleted</TooltipContent>
            </Tooltip>
          ) : (
            deleteItem
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      <DashboardNameDialog
        open={renameOpen}
        onOpenChange={setRenameOpen}
        mode="rename"
        initialName={dashboard.name}
        onSubmit={async (name) => {
          await update.mutateAsync({ id: dashboard.id, patch: { name } })
          toast.success('Dashboard renamed')
        }}
      />
      <ConfirmDialog
        open={confirmDelete}
        onOpenChange={setConfirmDelete}
        title="Delete dashboard?"
        description="This permanently deletes the dashboard."
        confirmLabel="Delete"
        destructive
        loading={remove.isPending}
        onConfirm={onDelete}
      />
    </>
  )
}
