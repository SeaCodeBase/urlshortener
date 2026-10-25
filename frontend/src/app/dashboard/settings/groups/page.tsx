'use client'

import { useState } from 'react'
import Link from 'next/link'
import { ChevronRight, Plus } from 'lucide-react'
import { toast } from 'sonner'
import { EmptyState } from '@/components/common/EmptyState'
import { PageHeader } from '@/components/common/PageHeader'
import { Badge } from '@/components/ui/badge'
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
import { Skeleton } from '@/components/ui/skeleton'
import { useCreateGroup, useGroups } from '@/lib/hooks'
import { useAuthStore } from '@/stores/auth'

function AddGroupDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const create = useCreateGroup()
  const [name, setName] = useState('')

  function close(o: boolean) {
    if (create.isPending) return
    if (!o) setName('')
    onOpenChange(o)
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    try {
      await create.mutateAsync(name.trim())
      toast.success('Group created')
      close(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to create group')
    }
  }

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent className="sm:max-w-[480px]">
        <form onSubmit={submit} className="grid gap-4">
          <DialogHeader>
            <DialogTitle>Add new group</DialogTitle>
            <DialogDescription className="sr-only">Name the new group</DialogDescription>
          </DialogHeader>
          <div className="grid gap-1.5">
            <Label htmlFor="group-name">Group name</Label>
            <Input id="group-name" value={name} onChange={(e) => setName(e.target.value)} maxLength={100} autoFocus />
          </div>
          <DialogFooter>
            <Button type="button" variant="secondary" onClick={() => close(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={!name.trim() || create.isPending}>
              Create
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

export default function GroupsPage() {
  const { data: groups, isLoading } = useGroups()
  const defaultGroupId = useAuthStore((s) => s.user?.default_group_id ?? null)
  const [addOpen, setAddOpen] = useState(false)

  return (
    <>
      <PageHeader
        title="Groups"
        actions={
          <Button onClick={() => setAddOpen(true)}>
            <Plus className="size-4" />
            Add new group
          </Button>
        }
      />
      {isLoading ? (
        <Skeleton className="h-32 w-full rounded-xl" />
      ) : !groups?.length ? (
        // Every account has a default group, so this is a safety net; "Add new group" sits in the header.
        <EmptyState title="No groups yet" description="Add a group to organise links, QR Codes and domains." />
      ) : (
        <>
          <p className="mb-3 text-sm text-muted-foreground">Using {groups?.length ?? 0} groups</p>
          <ul className="divide-y divide-border overflow-hidden rounded-xl bg-card">
            {groups?.map((g) => (
              <li key={g.id}>
                <Link
                  href={`/dashboard/settings/groups/${g.id}`}
                  className="flex items-center justify-between gap-4 px-6 py-4 hover:bg-muted"
                >
                  <span className="flex min-w-0 items-center gap-2">
                    <span className="truncate font-medium text-foreground">{g.name}</span>
                    {g.id === defaultGroupId && <Badge variant="secondary">Default</Badge>}
                  </span>
                  <ChevronRight className="size-5 shrink-0 text-muted-foreground" aria-hidden />
                </Link>
              </li>
            ))}
          </ul>
        </>
      )}
      <AddGroupDialog open={addOpen} onOpenChange={setAddOpen} />
    </>
  )
}
