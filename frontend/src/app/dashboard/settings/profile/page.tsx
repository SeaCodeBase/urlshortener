'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { toast } from 'sonner'
import { PageHeader } from '@/components/common/PageHeader'
import { PasskeysSection } from '@/components/settings/PasskeysSection'
import { SettingsSection } from '@/components/settings/SettingsSection'
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
import { Separator } from '@/components/ui/separator'
import { api, ApiError } from '@/lib/api'
import { useUpdateEmail, useUpdateMe } from '@/lib/hooks'
import { useAuthStore } from '@/stores/auth'

function errorMessage(err: unknown, fallback: string) {
  return err instanceof Error && err.message ? err.message : fallback
}

function DisplayNameForm({ initial }: { initial: string }) {
  const checkAuth = useAuthStore((s) => s.checkAuth)
  const updateMe = useUpdateMe()
  const [value, setValue] = useState(initial)
  const changed = value.trim() !== initial

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    try {
      await updateMe.mutateAsync({ display_name: value.trim() })
      await checkAuth()
      toast.success('Display name updated')
    } catch (err) {
      toast.error(errorMessage(err, 'Failed to update display name'))
    }
  }

  return (
    <form onSubmit={submit} className="grid gap-3">
      <div className="grid gap-1.5">
        <Label htmlFor="display-name">Display name</Label>
        <Input id="display-name" value={value} onChange={(e) => setValue(e.target.value)} maxLength={100} />
      </div>
      <div>
        <Button type="submit" disabled={!changed || updateMe.isPending}>
          Update display name
        </Button>
      </div>
    </form>
  )
}

function ChangeEmailDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const updateEmail = useUpdateEmail()
  const checkAuth = useAuthStore((s) => s.checkAuth)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)

  function close(o: boolean) {
    if (updateEmail.isPending) return
    if (!o) {
      setEmail('')
      setPassword('')
      setError(null)
    }
    onOpenChange(o)
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      await updateEmail.mutateAsync({ new_email: email.trim(), password })
      await checkAuth()
      toast.success('Email updated')
      close(false)
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) setError('Incorrect password')
      else setError(errorMessage(err, 'Failed to update email'))
    }
  }

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent className="sm:max-w-[480px]">
        <form onSubmit={submit} className="grid gap-4">
          <DialogHeader>
            <DialogTitle>Change email</DialogTitle>
            <DialogDescription>Enter your new email address and confirm with your password.</DialogDescription>
          </DialogHeader>
          <div className="grid gap-1.5">
            <Label htmlFor="new-email">New email</Label>
            <Input id="new-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} autoFocus />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="email-password">Current password</Label>
            <Input
              id="email-password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}
          <DialogFooter>
            <Button type="button" variant="secondary" onClick={() => close(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={!email.trim() || !password || updateEmail.isPending}>
              Change email
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function ChangePasswordForm() {
  const router = useRouter()
  const logout = useAuthStore((s) => s.logout)
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    if (next.length < 8) return setError('Password must be at least 8 characters')
    if (next !== confirm) return setError('Passwords do not match')
    setSaving(true)
    try {
      await api.changePassword(current, next)
      toast.success('Password changed')
      logout()
      router.push('/login')
    } catch (err) {
      setError(err instanceof ApiError && err.status === 401 ? 'Current password is incorrect' : errorMessage(err, 'Failed to change password'))
      setSaving(false)
    }
  }

  return (
    <form onSubmit={submit} className="grid gap-3">
      <div className="grid gap-1.5">
        <Label htmlFor="current-password">Current password</Label>
        <Input
          id="current-password"
          type="password"
          autoComplete="current-password"
          value={current}
          onChange={(e) => setCurrent(e.target.value)}
          required
        />
      </div>
      <div className="grid gap-1.5">
        <Label htmlFor="new-password">New password</Label>
        <Input
          id="new-password"
          type="password"
          autoComplete="new-password"
          value={next}
          onChange={(e) => setNext(e.target.value)}
          required
        />
      </div>
      <div className="grid gap-1.5">
        <Label htmlFor="confirm-password">Confirm new password</Label>
        <Input
          id="confirm-password"
          type="password"
          autoComplete="new-password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          required
        />
      </div>
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      <div>
        <Button type="submit" disabled={saving || !current || !next || !confirm}>
          Change password
        </Button>
      </div>
    </form>
  )
}

export default function ProfilePage() {
  const user = useAuthStore((s) => s.user)
  const [emailOpen, setEmailOpen] = useState(false)

  return (
    <>
      <PageHeader title="Profile" />
      <div className="space-y-6">
        <SettingsSection title="Preferences">
          {/* Keyed so the field resets when the stored name changes (e.g. after checkAuth). */}
          <DisplayNameForm key={user?.display_name ?? ''} initial={user?.display_name ?? ''} />
        </SettingsSection>

        <SettingsSection title="Email address">
          <div className="flex items-end gap-3">
            <div className="grid flex-1 gap-1.5">
              <Label htmlFor="current-email">Email</Label>
              <Input id="current-email" value={user?.email ?? ''} readOnly disabled />
            </div>
            <Button type="button" variant="secondary" onClick={() => setEmailOpen(true)}>
              Change email
            </Button>
          </div>
          <ChangeEmailDialog open={emailOpen} onOpenChange={setEmailOpen} />
        </SettingsSection>

        <SettingsSection title="Security & authentication">
          <h3 className="text-base font-semibold text-foreground">Change password</h3>
          <p className="mb-4 text-sm text-muted-foreground">You will be required to log in after changing your password</p>
          <ChangePasswordForm />
          <Separator className="my-6" />
          <h3 className="text-base font-semibold text-foreground">Passkeys</h3>
          <p className="mb-4 text-sm text-muted-foreground">Use passkeys as a second factor when signing in.</p>
          <PasskeysSection />
        </SettingsSection>
      </div>
    </>
  )
}
