'use client'

import { Fragment, type KeyboardEvent } from 'react'
import { MoreHorizontal, Pencil, Power, Send, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { TableCell, TableRow } from '@/components/ui/table'
import { formatDateTime } from '@/lib/format'
import { useWebhookMutations } from '@/lib/hooks'
import type { Webhook, WebhookStatus } from '@/types'
import { DeliveriesPanel } from './DeliveriesPanel'

/** Same colouring as the domain verification chip. */
const STATUS: Record<WebhookStatus, { label: string; className: string }> = {
  active: { label: 'Active', className: 'bg-success/15 text-success' },
  alert: { label: 'Alert', className: 'bg-amber-500/15 text-amber-700 dark:text-amber-400' },
  deactivated: { label: 'Disabled', className: 'bg-muted text-muted-foreground' },
}

export const WEBHOOK_COLUMNS = 6

function message(err: unknown, fallback: string): string {
  return err instanceof Error && err.message ? err.message : fallback
}

/** One webhook table row; clicking it toggles its deliveries panel in a row beneath. */
export function WebhookRow({
  webhook,
  open,
  onToggle,
  onEdit,
  onDelete,
}: {
  webhook: Webhook
  open: boolean
  onToggle: () => void
  onEdit: (w: Webhook) => void
  onDelete: (w: Webhook) => void
}) {
  const { verify, deactivate, activate } = useWebhookMutations()
  const status = STATUS[webhook.status] ?? STATUS.active
  const disabled = webhook.status === 'deactivated'
  const panelId = `webhook-${webhook.id}-deliveries`

  async function runVerify() {
    try {
      const { delivery } = await verify.mutateAsync(webhook.id)
      const code = delivery.status_code
      if (code >= 200 && code < 300) toast.success(`Ping delivered (${code})`)
      else toast.error(delivery.error || `Ping failed (${code})`)
    } catch (err) {
      toast.error(message(err, 'Ping failed'))
    }
  }

  async function toggleActive() {
    try {
      if (disabled) {
        await activate.mutateAsync(webhook.id)
        toast.success('Webhook activated')
      } else {
        await deactivate.mutateAsync(webhook.id)
        toast.success('Webhook deactivated')
      }
    } catch (err) {
      toast.error(message(err, "Couldn't update the webhook"))
    }
  }

  function onKeyDown(e: KeyboardEvent<HTMLTableRowElement>) {
    if (e.target !== e.currentTarget) return
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      onToggle()
    }
  }

  return (
    <Fragment>
      <TableRow
        className="cursor-pointer"
        tabIndex={0}
        aria-expanded={open}
        aria-controls={open ? panelId : undefined}
        onClick={onToggle}
        onKeyDown={onKeyDown}
      >
        <TableCell className="font-medium">{webhook.name}</TableCell>
        <TableCell className="max-w-[280px] truncate font-mono text-sm" title={webhook.url}>
          {webhook.url}
        </TableCell>
        <TableCell>{webhook.event}</TableCell>
        <TableCell>
          <Badge className={status.className}>{status.label}</Badge>
        </TableCell>
        <TableCell className="text-sm whitespace-nowrap">
          {webhook.last_delivery_at ? formatDateTime(webhook.last_delivery_at) : '—'}
        </TableCell>
        <TableCell className="text-right" onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.stopPropagation()}>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon-sm" aria-label={`More actions for ${webhook.name}`}>
                <MoreHorizontal className="size-5" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onSelect={() => void runVerify()} disabled={verify.isPending}>
                <Send className="size-4" />
                Verify
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={() => onEdit(webhook)}>
                <Pencil className="size-4" />
                Edit
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={() => void toggleActive()}>
                <Power className="size-4" />
                {disabled ? 'Activate' : 'Deactivate'}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem variant="destructive" onSelect={() => onDelete(webhook)}>
                <Trash2 className="size-4" />
                Delete
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </TableCell>
      </TableRow>
      {open && (
        <TableRow id={panelId} className="hover:bg-transparent">
          <TableCell colSpan={WEBHOOK_COLUMNS} className="bg-muted/40 px-6 py-3 whitespace-normal">
            <DeliveriesPanel webhookId={webhook.id} open={open} />
          </TableCell>
        </TableRow>
      )}
    </Fragment>
  )
}
