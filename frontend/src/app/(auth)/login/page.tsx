'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { startAuthentication } from '@simplewebauthn/browser'
import { Key } from 'lucide-react'

export default function LoginPage() {
  const router = useRouter()
  const { setAuth } = useAuthStore()
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
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <div className="mx-auto w-12 h-12 bg-primary/10 rounded-full flex items-center justify-center mb-4">
            <Key className="h-6 w-6 text-primary" />
          </div>
          <CardTitle>Verify your identity</CardTitle>
          <CardDescription>
            Use your passkey to complete login
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {error && (
            <div className="bg-red-50 text-red-500 p-3 rounded-md text-sm">
              {error}
            </div>
          )}
          <Button
            onClick={handlePasskeyVerify}
            disabled={verifyingPasskey}
            className="w-full bg-primary hover:bg-primary/90"
          >
            {verifyingPasskey ? 'Verifying...' : 'Use Passkey'}
          </Button>
          <Button
            variant="ghost"
            onClick={() => {
              setRequiresPasskey(false)
              setUserId(null)
              setError('')
            }}
            className="w-full"
          >
            Go back
          </Button>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Login</CardTitle>
        <CardDescription>Enter your credentials to access your account</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <div className="p-3 text-sm text-red-500 bg-red-50 rounded-md">
              {error}
            </div>
          )}
          <div className="space-y-2">
            <Label htmlFor="email">Email</Label>
            <Input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              placeholder="you@example.com"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              placeholder="••••••••"
            />
          </div>
          <Button
            type="submit"
            disabled={loading}
            className="w-full bg-primary hover:bg-primary/90"
          >
            {loading ? 'Signing in...' : 'Sign in'}
          </Button>
        </form>
        <p className="mt-4 text-center text-sm text-gray-600">
          Don&apos;t have an account?{' '}
          <Link href="/register" className="text-primary hover:underline">
            Register
          </Link>
        </p>
      </CardContent>
    </Card>
  )
}
