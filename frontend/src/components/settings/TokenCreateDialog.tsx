'use client'

import { useState } from 'react'
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
import { ApiError } from '@/lib/api'
import { useCreateToken } from '@/lib/hooks'

/** "Generate token" dialog: name + password, then shows the raw token exactly once. */
export function TokenCreateDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[520px]">
        {/* Mounted only while open so the form (and the raw token) reset every time. */}
        {open && <TokenCreateBody onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function TokenCreateBody({ onDone }: { onDone: () => void }) {
  const create = useCreateToken()
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [token, setToken] = useState<string | null>(null)

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      const res = await create.mutateAsync({ name: name.trim(), password })
      setToken(res.token)
      setPassword('')
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) setError('Incorrect password')
      else toast.error(err instanceof Error ? err.message : 'Failed to generate token')
    }
  }

  if (token) {
    return (
      <>
        <DialogHeader>
          <DialogTitle>Token generated</DialogTitle>
          <DialogDescription>Make sure to copy your token now. You won&apos;t be able to see it again.</DialogDescription>
        </DialogHeader>
        <div className="flex items-center gap-2 rounded border border-border bg-muted px-3 py-2">
          <code className="min-w-0 flex-1 font-mono text-sm break-all text-foreground">{token}</code>
          <CopyButton text={token} />
        </div>
        <DialogFooter>
          <Button type="button" onClick={onDone}>
            Done
          </Button>
        </DialogFooter>
      </>
    )
  }

  return (
    <form onSubmit={submit} className="grid gap-4">
      <DialogHeader>
        <DialogTitle>Generate token</DialogTitle>
        <DialogDescription className="sr-only">Name the token and confirm your password</DialogDescription>
      </DialogHeader>
      <div className="grid gap-1.5">
        <Label htmlFor="token-name">Token name</Label>
        <Input id="token-name" value={name} onChange={(e) => setName(e.target.value)} maxLength={100} autoFocus />
      </div>
      <div className="grid gap-1.5">
        <Label htmlFor="token-password">Confirm your password</Label>
        <Input
          id="token-password"
          type="password"
          autoComplete="current-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          aria-invalid={!!error}
        />
        {error && <p className="text-sm text-destructive">{error}</p>}
      </div>
      <DialogFooter>
        <Button type="button" variant="secondary" onClick={onDone}>
          Cancel
        </Button>
        <Button type="submit" disabled={!name.trim() || !password || create.isPending}>
          Generate
        </Button>
      </DialogFooter>
    </form>
  )
}
