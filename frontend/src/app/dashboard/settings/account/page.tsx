'use client'

import { useState } from 'react'
import Link from 'next/link'
import { toast } from 'sonner'
import { PageHeader } from '@/components/common/PageHeader'
import { SettingsSection } from '@/components/settings/SettingsSection'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { formatDate } from '@/lib/format'
import { useUpdateMe } from '@/lib/hooks'
import { useAuthStore } from '@/stores/auth'

export default function AccountPage() {
  const user = useAuthStore((s) => s.user)
  const updateMe = useUpdateMe()
  const accountName = user?.account_name || user?.email.split('@')[0] || ''
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')

  async function save(e: React.FormEvent) {
    e.preventDefault()
    try {
      await updateMe.mutateAsync({ account_name: draft.trim() })
      toast.success('Account name updated')
      setEditing(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update account name')
    }
  }

  return (
    <>
      <PageHeader title="Account details" />
      <div className="space-y-6">
        <SettingsSection title="Preferences">
          <div className="grid gap-1.5">
            {editing ? <Label htmlFor="account-name">Account name</Label> : <div className="text-sm font-medium">Account name</div>}
            {editing ? (
              <form onSubmit={save} className="flex gap-2">
                <Input
                  id="account-name"
                  value={draft}
                  onChange={(e) => setDraft(e.target.value)}
                  maxLength={100}
                  autoFocus
                />
                <Button type="submit" disabled={!draft.trim() || updateMe.isPending}>
                  Save
                </Button>
                <Button type="button" variant="secondary" onClick={() => setEditing(false)}>
                  Cancel
                </Button>
              </form>
            ) : (
              <div className="flex items-center justify-between gap-4">
                <span className="text-base text-foreground">
                  {accountName}
                </span>
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() => {
                    setDraft(accountName)
                    setEditing(true)
                  }}
                >
                  Edit name
                </Button>
              </div>
            )}
          </div>
          {user?.created_at && (
            <p className="mt-4 text-sm text-muted-foreground">Member since {formatDate(user.created_at)}</p>
          )}
        </SettingsSection>

        <SettingsSection title="Contact details">
          <div className="grid gap-1.5">
            <Label htmlFor="notification-email">Notification email</Label>
            <Input id="notification-email" value={user?.email ?? ''} readOnly disabled />
            <p className="text-sm text-muted-foreground">
              Notifications are sent to your primary email. To change it, go to{' '}
              <Link href="/dashboard/settings/profile" className="text-primary hover:underline">
                Profile
              </Link>
              .
            </p>
          </div>
        </SettingsSection>
      </div>
    </>
  )
}
