'use client'

import { useState, useEffect } from 'react'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/lib/api'
import { Passkey } from '@/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { toast } from 'sonner'
import { Trash2, Plus, Key } from 'lucide-react'
import { startRegistration } from '@simplewebauthn/browser'

type Tab = 'profile' | 'security' | 'passkeys'

export default function SettingsPage() {
  const { user, checkAuth } = useAuthStore()
  const [activeTab, setActiveTab] = useState<Tab>('profile')

  // Profile state
  const [displayName, setDisplayName] = useState(user?.display_name || '')
  const [savingProfile, setSavingProfile] = useState(false)

  // Password state
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [savingPassword, setSavingPassword] = useState(false)

  // Passkeys state
  const [passkeys, setPasskeys] = useState<Passkey[]>([])
  const [loadingPasskeys, setLoadingPasskeys] = useState(false)
  const [addingPasskey, setAddingPasskey] = useState(false)
  const [newPasskeyName, setNewPasskeyName] = useState('')

  useEffect(() => {
    if (activeTab === 'passkeys') {
      loadPasskeys()
    }
  }, [activeTab])

  const loadPasskeys = async () => {
    setLoadingPasskeys(true)
    try {
      const data = await api.getPasskeys()
      setPasskeys(data)
    } catch {
      toast.error('Failed to load passkeys')
    } finally {
      setLoadingPasskeys(false)
    }
  }

  const handleSaveProfile = async (e: React.FormEvent) => {
    e.preventDefault()
    setSavingProfile(true)
    try {
      await api.updateProfile(displayName)
      await checkAuth()
      toast.success('Profile updated successfully')
    } catch {
      toast.error('Failed to update profile')
    } finally {
      setSavingProfile(false)
    }
  }

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    if (newPassword !== confirmPassword) {
      toast.error('Passwords do not match')
      return
    }
    if (newPassword.length < 8) {
      toast.error('Password must be at least 8 characters')
      return
    }
    setSavingPassword(true)
    try {
      await api.changePassword(currentPassword, newPassword)
      toast.success('Password changed successfully')
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } catch {
      toast.error('Failed to change password')
    } finally {
      setSavingPassword(false)
    }
  }

  const handleDeletePasskey = async (id: number) => {
    if (!confirm('Are you sure you want to delete this passkey?')) return
    try {
      await api.deletePasskey(id)
      setPasskeys(passkeys.filter(p => p.id !== id))
      toast.success('Passkey deleted')
    } catch {
      toast.error('Failed to delete passkey')
    }
  }

  const handleAddPasskey = async () => {
    if (!newPasskeyName.trim()) {
      toast.error('Please enter a name for the passkey')
      return
    }
    setAddingPasskey(true)
    try {
      // 1. Begin registration - get options from server
      const { options, session_data } = await api.beginPasskeyRegistration()

      // 2. Create credential using browser WebAuthn API
      // The server returns options wrapped in publicKey, extract it for SimpleWebAuthn
      const credential = await startRegistration({ optionsJSON: options.publicKey })

      // 3. Finish registration - send credential to server
      const authStorage = localStorage.getItem('auth-storage')
      const token = authStorage ? JSON.parse(authStorage).state?.token : ''

      const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8081'}/api/auth/passkeys/register/finish`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          session_data,
          name: newPasskeyName,
          ...credential,
        }),
      })

      if (!response.ok) {
        throw new Error('Failed to register passkey')
      }

      const passkey = await response.json()
      setPasskeys([passkey, ...passkeys])
      setNewPasskeyName('')
      toast.success('Passkey added successfully!')
    } catch (error: unknown) {
      console.error('Passkey registration error:', error)

      // Check for InvalidStateError - thrown when authenticator already has a credential
      // from excludeCredentials list (i.e., this YubiKey is already registered)
      if (error instanceof Error && error.name === 'InvalidStateError') {
        toast.error('This security key is already registered to your account')
        return
      }

      // Check for NotAllowedError - user cancelled or timeout
      if (error instanceof Error && error.name === 'NotAllowedError') {
        toast.error('Registration was cancelled or timed out')
        return
      }

      const message = error instanceof Error ? error.message : 'Failed to add passkey'
      toast.error(message)
    } finally {
      setAddingPasskey(false)
    }
  }

  const tabs = [
    { id: 'profile' as Tab, label: 'Profile' },
    { id: 'security' as Tab, label: 'Security' },
    { id: 'passkeys' as Tab, label: 'Passkeys' },
  ]

  return (
    <div className="max-w-2xl">
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Settings</h1>

      <div className="border-b border-gray-200 mb-6">
        <nav className="flex gap-8">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`pb-3 text-sm font-medium border-b-2 transition-colors ${
                activeTab === tab.id
                  ? 'border-primary text-primary'
                  : 'border-transparent text-gray-500 hover:text-gray-700'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </nav>
      </div>

      {activeTab === 'profile' && (
        <Card>
          <CardHeader>
            <CardTitle>Profile Information</CardTitle>
            <CardDescription>Update your display name and profile details.</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSaveProfile} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="email">Email</Label>
                <Input id="email" value={user?.email || ''} disabled className="bg-gray-50" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="displayName">Display Name</Label>
                <Input
                  id="displayName"
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  placeholder="Your display name"
                />
              </div>
              <Button type="submit" disabled={savingProfile} className="bg-primary hover:bg-primary/90">
                {savingProfile ? 'Saving...' : 'Save changes'}
              </Button>
            </form>
          </CardContent>
        </Card>
      )}

      {activeTab === 'security' && (
        <Card>
          <CardHeader>
            <CardTitle>Change Password</CardTitle>
            <CardDescription>Update your password to keep your account secure.</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleChangePassword} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="currentPassword">Current Password</Label>
                <Input
                  id="currentPassword"
                  type="password"
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="newPassword">New Password</Label>
                <Input
                  id="newPassword"
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  required
                  minLength={8}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="confirmPassword">Confirm New Password</Label>
                <Input
                  id="confirmPassword"
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  required
                />
              </div>
              <Button type="submit" disabled={savingPassword} className="bg-primary hover:bg-primary/90">
                {savingPassword ? 'Changing...' : 'Change password'}
              </Button>
            </form>
          </CardContent>
        </Card>
      )}

      {activeTab === 'passkeys' && (
        <Card>
          <CardHeader>
            <CardTitle>Passkeys</CardTitle>
            <CardDescription>
              Passkeys provide secure, passwordless authentication using biometrics or security keys.
            </CardDescription>
          </CardHeader>
          <CardContent>
            {loadingPasskeys ? (
              <div className="flex justify-center py-8">
                <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-primary"></div>
              </div>
            ) : passkeys.length === 0 ? (
              <div className="text-center py-8">
                <Key className="h-12 w-12 text-gray-300 mx-auto mb-4" />
                <p className="text-gray-500 mb-4">No passkeys registered yet.</p>
                <div className="space-y-4">
                  <Input
                    placeholder="Passkey name (e.g., MacBook Pro)"
                    value={newPasskeyName}
                    onChange={(e) => setNewPasskeyName(e.target.value)}
                  />
                  <Button
                    onClick={handleAddPasskey}
                    disabled={addingPasskey}
                    className="bg-primary hover:bg-primary/90"
                  >
                    {addingPasskey ? 'Adding...' : (
                      <>
                        <Plus className="h-4 w-4 mr-2" />
                        Add Passkey
                      </>
                    )}
                  </Button>
                </div>
              </div>
            ) : (
              <div className="space-y-4">
                {passkeys.map((passkey) => (
                  <div key={passkey.id} className="flex items-center justify-between p-4 border rounded-lg">
                    <div>
                      <p className="font-medium text-gray-900">{passkey.name}</p>
                      <p className="text-sm text-gray-500">
                        Added {new Date(passkey.created_at).toLocaleDateString()}
                        {passkey.last_used_at && ` · Last used ${new Date(passkey.last_used_at).toLocaleDateString()}`}
                      </p>
                    </div>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleDeletePasskey(passkey.id)}
                      className="text-red-500 hover:text-red-600 hover:bg-red-50"
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                ))}
                <div className="mt-4 pt-4 border-t space-y-2">
                  <Input
                    placeholder="Passkey name (e.g., iPhone)"
                    value={newPasskeyName}
                    onChange={(e) => setNewPasskeyName(e.target.value)}
                  />
                  <Button
                    onClick={handleAddPasskey}
                    disabled={addingPasskey}
                    className="w-full bg-primary hover:bg-primary/90"
                  >
                    {addingPasskey ? 'Adding...' : (
                      <>
                        <Plus className="h-4 w-4 mr-2" />
                        Add another Passkey
                      </>
                    )}
                  </Button>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  )
}
