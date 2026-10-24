'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { startAuthentication } from '@simplewebauthn/browser'
import { KeyRound } from 'lucide-react'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/lib/api'
import { useMeta } from '@/lib/hooks'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

function FormError({ message }: { message: string }) {
  if (!message) return null
  return (
    <p role="alert" className="rounded bg-destructive/10 px-3 py-2 text-sm text-destructive">
      {message}
    </p>
  )
}

export default function LoginPage() {
  const router = useRouter()
  const { setAuth } = useAuthStore()
  const { data: meta } = useMeta()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  // Passkey verification state
  const [requiresPasskey, setRequiresPasskey] = useState(false)
  const [userId, setUserId] = useState<number | null>(null)
  const [verifyingPasskey, setVerifyingPasskey] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError('')

    try {
      // First try normal login
      const response = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      })

      const data = await response.json()

      if (!response.ok) {
        throw new Error(data.error || 'Login failed')
      }

      if (data.requires_passkey) {
        // User has passkeys, need to verify
        setRequiresPasskey(true)
        setUserId(data.user_id)
      } else {
        // No passkeys, login successful
        setAuth(data.token, data.user)
        api.setToken(data.token)
        router.push('/dashboard')
      }
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Login failed'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  const handlePasskeyVerify = async () => {
    if (!userId) return
    setVerifyingPasskey(true)
    setError('')

    try {
      // 1. Begin passkey verification
      const { options, session_data } = await api.beginPasskeyVerify(userId)

      // 2. Perform WebAuthn authentication
      // The server returns options wrapped in publicKey, extract it for SimpleWebAuthn
      const credential = await startAuthentication({ optionsJSON: options.publicKey })

      // 3. Complete verification
      const response = await fetch('/api/auth/passkeys/verify/finish', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          user_id: userId,
          session_data,
          ...credential,
        }),
      })

      if (!response.ok) {
        throw new Error('Passkey verification failed')
      }

      const data = await response.json()

      // Store auth data and redirect
      setAuth(data.token, data.user)
      api.setToken(data.token)
      router.push('/dashboard')
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Passkey verification failed'
      setError(message)
    } finally {
      setVerifyingPasskey(false)
    }
  }

  if (requiresPasskey) {
    return (
      <div className="space-y-6">
        <div className="flex size-12 items-center justify-center rounded-full bg-primary/10">
          <KeyRound className="size-6 text-primary" aria-hidden />
        </div>
        <div className="space-y-2">
          <h1 className="text-[32px] leading-10 font-semibold text-foreground">Verify it&apos;s you</h1>
          <p className="text-base text-muted-foreground">Use your passkey to finish signing in.</p>
        </div>
        <FormError message={error} />
        <Button onClick={handlePasskeyVerify} disabled={verifyingPasskey} className="w-full">
          {verifyingPasskey ? 'Verifying...' : 'Continue with passkey'}
        </Button>
        <Button
          variant="link"
          className="w-full"
          onClick={() => {
            setRequiresPasskey(false)
            setUserId(null)
            setError('')
          }}
        >
          Back
        </Button>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <h1 className="text-[32px] leading-10 font-semibold text-foreground">Log in and start sharing</h1>
        {meta?.allow_registration !== false && (
          <p className="text-base text-muted-foreground">
            Don&apos;t have an account?{' '}
            <Link href="/register" className="font-semibold text-primary hover:underline">
              Sign up
            </Link>
          </p>
        )}
      </div>
      <form onSubmit={handleSubmit} className="space-y-4">
        <FormError message={error} />
        <div className="space-y-2">
          <Label htmlFor="email">Email</Label>
          <Input
            id="email"
            type="email"
            autoComplete="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="password">Password</Label>
          <Input
            id="password"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </div>
        <Button type="submit" disabled={loading} className="w-full">
          {loading ? 'Logging in...' : 'Log in'}
        </Button>
      </form>
    </div>
  )
}
