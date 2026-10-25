'use client'

import { useState } from 'react'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/common/ConfirmDialog'
import { EmptyState } from '@/components/common/EmptyState'
import { PageHeader } from '@/components/common/PageHeader'
import { WebhookDialog } from '@/components/settings/WebhookDialog'
import { WebhookRow } from '@/components/settings/WebhookRow'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useWebhookMutations, useWebhooks } from '@/lib/hooks'
import type { Webhook } from '@/types'

export default function WebhooksPage() {
  const { data: webhooks, isLoading, isError } = useWebhooks()
  const { remove } = useWebhookMutations()
  // `null` = closed, `undefined` webhook = create.
  const [dialog, setDialog] = useState<{ webhook?: Webhook } | null>(null)
  const [deleting, setDeleting] = useState<Webhook | null>(null)
  const [openId, setOpenId] = useState<number | null>(null)

  async function confirmDelete() {
    if (!deleting) return
    try {
      await remove.mutateAsync(deleting.id)
      toast.success('Webhook deleted')
      if (openId === deleting.id) setOpenId(null)
      setDeleting(null)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Couldn't delete the webhook")
    }
  }

  const create = <Button onClick={() => setDialog({})}>Create webhook</Button>

  return (
    <>
      <PageHeader title="Webhooks" actions={create} />
      {isLoading ? (
        <Skeleton className="h-64 w-full rounded-xl" aria-label="Loading webhooks" />
      ) : isError ? (
        <p role="alert" className="rounded-xl bg-card p-6 text-sm text-destructive">
          Failed to load webhooks
        </p>
      ) : !webhooks?.length ? (
        <EmptyState
          title="No webhooks yet"
          description="Get notified with a signed POST each time one of your links is engaged."
          action={<Button onClick={() => setDialog({})}>Create webhook</Button>}
        />
      ) : (
        <div className="rounded-xl bg-card p-4">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>URL</TableHead>
                <TableHead>Event</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Last delivery</TableHead>
                <TableHead>
                  <span className="sr-only">Actions</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {webhooks.map((w) => (
                <WebhookRow
                  key={w.id}
                  webhook={w}
                  open={openId === w.id}
                  onToggle={() => setOpenId((cur) => (cur === w.id ? null : w.id))}
                  onEdit={(hook) => setDialog({ webhook: hook })}
                  onDelete={setDeleting}
                />
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      <WebhookDialog
        open={dialog != null}
        onOpenChange={(o) => !o && setDialog(null)}
        webhook={dialog?.webhook}
      />
      <ConfirmDialog
        open={deleting != null}
        onOpenChange={(o) => !o && setDeleting(null)}
        title={deleting ? `Delete ${deleting.name}?` : 'Delete webhook?'}
        description="Deliveries to this URL stop immediately."
        confirmLabel="Delete"
        destructive
        loading={remove.isPending}
        onConfirm={confirmDelete}
      />
    </>
  )
}
