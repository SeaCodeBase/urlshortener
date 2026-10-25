'use client'

import { useId, useState, type FormEvent } from 'react'
import { toast } from 'sonner'
import { CopyButton } from '@/components/common/CopyButton'
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
import { useWebhookMutations } from '@/lib/hooks'
import { useAuthStore } from '@/stores/auth'
import type { Webhook, WebhookEvent, WebhookInput } from '@/types'

const MAX_NAME = 120
const MAX_URL = 2048

export interface WebhookDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Edit mode when given. */
  webhook?: Webhook
}

/**
 * Create or edit a webhook; after a create it shows the signing secret, and the Edit form can reveal it again
 * ("Reveal secret", with copy). The secret is never put in a toast.
 */
export function WebhookDialog({ open, onOpenChange, webhook }: WebhookDialogProps) {
  const [saving, setSaving] = useState(false)
  return (
    <Dialog open={open} onOpenChange={(o) => !saving && onOpenChange(o)}>
      <DialogContent className="sm:max-w-[520px]">
        {/* Mounted only while open so the form (and the secret) reset every time. */}
        {open && (
          <WebhookBody
            key={webhook?.id ?? 'new'}
            webhook={webhook}
            onSavingChange={setSaving}
            onDone={() => onOpenChange(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}

function WebhookBody({
  webhook,
  onSavingChange,
  onDone,
}: {
  webhook?: Webhook
  onSavingChange: (saving: boolean) => void
  onDone: () => void
}) {
  const ids = { name: useId(), url: useId(), event: useId(), group: useId(), secret: useId() }
  const { create, update } = useWebhookMutations()
  const groupName = useAuthStore((s) => s.groups.find((g) => g.id === s.activeGroupId)?.name ?? '')
  const [name, setName] = useState(webhook?.name ?? '')
  const [url, setUrl] = useState(webhook?.url ?? '')
  const [event, setEvent] = useState<WebhookEvent>(webhook?.event ?? 'engagement')
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [secret, setSecret] = useState<string | null>(null)
  const [revealed, setRevealed] = useState(false)
  const editing = webhook != null
  const valid = name.trim().length > 0 && url.trim().length > 0

  function setBusy(b: boolean) {
    setSaving(b)
    onSavingChange(b)
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!valid || saving) return
    const input: WebhookInput = { name: name.trim(), url: url.trim(), event }
    setError(null)
    setBusy(true)
    try {
      if (webhook) {
        await update.mutateAsync({ id: webhook.id, input })
        toast.success('Webhook updated')
        setBusy(false)
        onDone()
      } else {
        const created = await create.mutateAsync(input)
        toast.success('Webhook created')
        setBusy(false)
        setSecret(created.secret)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : editing ? "Couldn't update the webhook" : "Couldn't create the webhook")
      setBusy(false)
    }
  }

  if (secret != null) {
    return (
      <div className="flex flex-col gap-6">
        <DialogHeader>
          <DialogTitle className="text-xl font-semibold">Webhook created</DialogTitle>
          <DialogDescription className="sr-only">Copy the webhook signing secret</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-2">
          <Label htmlFor={ids.secret}>Secret</Label>
          <div className="flex items-center gap-2">
            <Input id={ids.secret} value={secret} readOnly className="font-mono" onFocus={(e) => e.target.select()} />
            <CopyButton text={secret} />
          </div>
          <p className="text-sm text-muted-foreground">
            Use this to verify the X-Signature header (HMAC-SHA256 of the body).
          </p>
        </div>
        <DialogFooter>
          <Button type="button" onClick={onDone}>
            Done
          </Button>
        </DialogFooter>
      </div>
    )
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-6">
      <DialogHeader>
        <DialogTitle className="text-xl font-semibold">{editing ? 'Edit webhook' : 'Create webhook'}</DialogTitle>
        <DialogDescription className="sr-only">{editing ? 'Edit webhook' : 'Create webhook'}</DialogDescription>
      </DialogHeader>
      <div className="flex flex-col gap-2">
        <Label htmlFor={ids.name}>Name</Label>
        <Input
          id={ids.name}
          value={name}
          onChange={(e) => setName(e.target.value)}
          maxLength={MAX_NAME}
          required
          autoFocus
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor={ids.url}>URL</Label>
        <Input
          id={ids.url}
          type="url"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          maxLength={MAX_URL}
          placeholder="https://example.com/webhook"
          required
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor={ids.event}>Event</Label>
        <Select value={event} onValueChange={(v) => setEvent(v as WebhookEvent)} disabled={editing}>
          <SelectTrigger id={ids.event} aria-label="Event" className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="engagement">engagement</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor={ids.group}>Group</Label>
        <Input id={ids.group} value={groupName} readOnly disabled />
      </div>
      {webhook && (
        <div className="flex flex-col gap-2">
          {revealed ? (
            <>
              <Label htmlFor={ids.secret}>Secret</Label>
              <div className="flex items-center gap-2">
                <Input
                  id={ids.secret}
                  value={webhook.secret}
                  readOnly
                  className="font-mono"
                  onFocus={(e) => e.target.select()}
                />
                <CopyButton text={webhook.secret} />
              </div>
              <Button type="button" variant="ghost" size="sm" className="self-start" onClick={() => setRevealed(false)}>
                Hide secret
              </Button>
            </>
          ) : (
            <Button type="button" variant="secondary" className="self-start" onClick={() => setRevealed(true)}>
              Reveal secret
            </Button>
          )}
        </div>
      )}
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      <DialogFooter>
        <Button type="button" variant="secondary" onClick={onDone} disabled={saving}>
          Cancel
        </Button>
        <Button type="submit" disabled={saving || !valid}>
          {editing ? 'Save' : 'Create webhook'}
        </Button>
      </DialogFooter>
    </form>
  )
}
