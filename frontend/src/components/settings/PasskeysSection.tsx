'use client'

import { useCallback, useEffect, useState } from 'react'
import { startRegistration } from '@simplewebauthn/browser'
import { Pencil, Plus, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { ConfirmDialog } from '@/components/common/ConfirmDialog'
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
import { api } from '@/lib/api'
import { formatDate } from '@/lib/format'
import type { Passkey } from '@/types'

/** Registers a passkey via WebAuthn; resolves with the stored passkey. */
async function registerPasskey(name: string): Promise<Passkey> {
  // 1. Begin registration - get options from server
  const { options, session_data } = await api.beginPasskeyRegistration()

  // 2. Create credential using browser WebAuthn API
  // The server returns options wrapped in publicKey, extract it for SimpleWebAuthn
  const credential = await startRegistration({ optionsJSON: options.publicKey })

  // 3. Finish registration - send credential to server
  const authStorage = localStorage.getItem('auth-storage')
  const token = authStorage ? JSON.parse(authStorage).state?.token : ''

  const response = await fetch('/api/auth/passkeys/register/finish', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ session_data, name, ...credential }),
  })

  if (!response.ok) {
    throw new Error('Failed to register passkey')
  }
  return response.json()
}

function registrationErrorMessage(error: unknown): string {
  // Handle WebAuthn-specific errors with user-friendly messages
  const errorName = error instanceof Error ? error.name : ''
  const errorMessage = error instanceof Error ? error.message : ''
  // InvalidStateError - authenticator already registered (from excludeCredentials)
  if (errorName === 'InvalidStateError' || errorMessage.includes('previously registered')) {
    return 'This passkey is already registered to your account'
  }
  // NotAllowedError - user cancelled or timeout
  if (errorName === 'NotAllowedError') {
    return 'Registration was cancelled or timed out'
  }
  // Log unexpected errors for debugging
  console.error('Passkey registration error:', error)
  return 'Failed to add passkey'
}

/** Passkey list with inline rename, delete and an "Add passkey" dialog. */
export function PasskeysSection() {
  const [passkeys, setPasskeys] = useState<Passkey[]>([])
  const [loading, setLoading] = useState(true)
  const [addOpen, setAddOpen] = useState(false)
  const [newName, setNewName] = useState('')
  const [adding, setAdding] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editName, setEditName] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<Passkey | null>(null)
  const [deleting, setDeleting] = useState(false)

  const load = useCallback(async () => {
    try {
      setPasskeys(await api.getPasskeys())
    } catch {
      toast.error('Failed to load passkeys')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  async function add(e: React.FormEvent) {
    e.preventDefault()
    if (!newName.trim()) {
      toast.error('Please enter a name for the passkey')
      return
    }
    setAdding(true)
    try {
      const passkey = await registerPasskey(newName.trim())
      setPasskeys((list) => [passkey, ...list])
      setNewName('')
      setAddOpen(false)
      toast.success('Passkey added')
    } catch (error) {
      toast.error(registrationErrorMessage(error))
    } finally {
      setAdding(false)
    }
  }

  async function saveRename(id: number) {
    const name = editName.trim()
    if (!name) return
    try {
      await api.renamePasskey(id, name)
      setPasskeys((list) => list.map((p) => (p.id === id ? { ...p, name } : p)))
      setEditingId(null)
      toast.success('Passkey renamed')
    } catch {
      toast.error('Failed to rename passkey')
    }
  }

  async function confirmDelete() {
    if (!deleteTarget) return
    setDeleting(true)
    try {
      await api.deletePasskey(deleteTarget.id)
      setPasskeys((list) => list.filter((p) => p.id !== deleteTarget.id))
      setDeleteTarget(null)
      toast.success('Passkey deleted')
    } catch {
      toast.error('Failed to delete passkey')
    } finally {
      setDeleting(false)
    }
  }

  return (
    <div className="space-y-3">
      {loading ? (
        <Skeleton className="h-16 w-full" />
      ) : passkeys.length === 0 ? (
        <p className="text-sm text-muted-foreground">No passkeys registered yet.</p>
      ) : (
        <ul className="divide-y divide-border rounded border border-border">
          {passkeys.map((p) => (
            <li key={p.id} className="flex items-center justify-between gap-4 px-4 py-3">
              {editingId === p.id ? (
                <form
                  className="flex flex-1 items-center gap-2"
                  onSubmit={(e) => {
                    e.preventDefault()
                    void saveRename(p.id)
                  }}
                >
                  <Input
                    aria-label="Passkey name"
                    value={editName}
                    onChange={(e) => setEditName(e.target.value)}
                    autoFocus
                  />
                  <Button type="submit" size="sm" disabled={!editName.trim()}>
                    Save
                  </Button>
                  <Button type="button" size="sm" variant="secondary" onClick={() => setEditingId(null)}>
                    Cancel
                  </Button>
                </form>
              ) : (
                <>
                  <div className="min-w-0">
                    <p className="truncate font-medium text-foreground">{p.name}</p>
                    <p className="text-sm text-muted-foreground">
                      Created {formatDate(p.created_at)} · Last used {p.last_used_at ? formatDate(p.last_used_at) : 'Never'}
                    </p>
                  </div>
                  <div className="flex shrink-0 gap-1">
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label={`Rename ${p.name}`}
                      onClick={() => {
                        setEditingId(p.id)
                        setEditName(p.name)
                      }}
                    >
                      <Pencil className="size-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label={`Delete ${p.name}`}
                      className="text-destructive"
                      onClick={() => setDeleteTarget(p)}
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </div>
                </>
              )}
            </li>
          ))}
        </ul>
      )}
      <Button type="button" variant="secondary" onClick={() => setAddOpen(true)}>
        <Plus className="size-4" />
        Add passkey
      </Button>

      <Dialog open={addOpen} onOpenChange={(o) => !adding && setAddOpen(o)}>
        <DialogContent className="sm:max-w-[480px]">
          <form onSubmit={add} className="grid gap-4">
            <DialogHeader>
              <DialogTitle>Add passkey</DialogTitle>
              <DialogDescription>Give this passkey a name so you can recognise it later.</DialogDescription>
            </DialogHeader>
            <div className="grid gap-1.5">
              <Label htmlFor="passkey-name">Passkey name</Label>
              <Input
                id="passkey-name"
                placeholder="e.g. MacBook Pro"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                autoFocus
              />
            </div>
            <DialogFooter>
              <Button type="button" variant="secondary" disabled={adding} onClick={() => setAddOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={adding || !newName.trim()}>
                {adding ? 'Adding...' : 'Add passkey'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(null)}
        title="Delete passkey?"
        description={deleteTarget ? `"${deleteTarget.name}" will no longer be usable to sign in.` : undefined}
        confirmLabel="Delete"
        destructive
        loading={deleting}
        onConfirm={confirmDelete}
      />
    </div>
  )
}
