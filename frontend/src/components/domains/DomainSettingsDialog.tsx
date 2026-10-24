'use client'

import { useState } from 'react'
import { toast } from 'sonner'
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
import { useUpdateDomain } from '@/lib/hooks'
import type { Domain } from '@/types'
import { GroupSelect, groupIdToValue, groupValueToId } from './GroupSelect'

/** Root / 404 redirects and group of a custom domain. */
export function DomainSettingsDialog({
  domain,
  open,
  onOpenChange,
}: {
  domain: Domain
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[520px]">
        {open && <SettingsForm domain={domain} onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function isValidUrl(value: string): boolean {
  if (!value) return true
  try {
    const u = new URL(value)
    return u.protocol === 'http:' || u.protocol === 'https:'
  } catch {
    return false
  }
}

function SettingsForm({ domain, onDone }: { domain: Domain; onDone: () => void }) {
  const update = useUpdateDomain()
  const [rootRedirect, setRootRedirect] = useState(domain.root_redirect ?? '')
  const [notFound, setNotFound] = useState(domain.not_found_redirect ?? '')
  const [group, setGroup] = useState(groupIdToValue(domain.group_id))
  const [error, setError] = useState<string | null>(null)

  async function save(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    const root = rootRedirect.trim()
    const nf = notFound.trim()
    if (!isValidUrl(root) || !isValidUrl(nf)) {
      setError('Enter a valid URL starting with http:// or https://')
      return
    }
    try {
      await update.mutateAsync({
        id: domain.id,
        data: { root_redirect: root, not_found_redirect: nf, group_id: groupValueToId(group) },
      })
      toast.success('Domain settings saved')
      onDone()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save settings')
    }
  }

  return (
    <form onSubmit={save} className="grid gap-4" noValidate>
      <DialogHeader>
        <DialogTitle>Domain settings</DialogTitle>
        <DialogDescription>{domain.domain}</DialogDescription>
      </DialogHeader>
      <div className="grid gap-2">
        <Label htmlFor="root-redirect">Root redirect</Label>
        <Input
          id="root-redirect"
          type="url"
          value={rootRedirect}
          onChange={(e) => setRootRedirect(e.target.value)}
          placeholder="https://example.com"
          aria-describedby="root-redirect-help"
        />
        <p id="root-redirect-help" className="text-sm text-muted-foreground">
          Where visitors land when they open the domain itself
        </p>
      </div>
      <div className="grid gap-2">
        <Label htmlFor="not-found-redirect">404 redirect</Label>
        <Input
          id="not-found-redirect"
          type="url"
          value={notFound}
          onChange={(e) => setNotFound(e.target.value)}
          placeholder="https://example.com/not-found"
          aria-describedby="not-found-redirect-help"
        />
        <p id="not-found-redirect-help" className="text-sm text-muted-foreground">
          Where visitors land for expired or unknown links
        </p>
      </div>
      <div className="grid gap-2">
        <Label htmlFor="domain-settings-group">Group</Label>
        <GroupSelect id="domain-settings-group" value={group} onChange={setGroup} />
      </div>
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      <DialogFooter>
        <Button type="button" variant="secondary" onClick={onDone}>
          Cancel
        </Button>
        <Button type="submit" disabled={update.isPending}>
          {update.isPending ? 'Saving...' : 'Save'}
        </Button>
      </DialogFooter>
    </form>
  )
}
