'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { EmptyState } from '@/components/common/EmptyState'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useMeta } from '@/lib/hooks'
import { useAuthStore } from '@/stores/auth'

export default function RegisterPage() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const router = useRouter()
  const register = useAuthStore((state) => state.register)
  const { data: meta } = useMeta()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    if (password.length < 8) {
      setError('Password must be at least 8 characters')
      return
    }

    setLoading(true)
    try {
      await register(email, password)
      router.push('/dashboard')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed')
    } finally {
      setLoading(false)
    }
  }

  if (meta?.allow_registration === false) {
    return (
      <EmptyState
        title="Registration is disabled"
        description="New accounts can't be created on this server."
        action={
          <Button asChild>
            <Link href="/login">Log in</Link>
          </Button>
        }
      />
    )
  }

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <h1 className="text-[32px] leading-10 font-semibold text-foreground">Create your account</h1>
        <p className="text-base text-muted-foreground">
          Already have an account?{' '}
          <Link href="/login" className="font-semibold text-primary hover:underline">
            Log in
          </Link>
        </p>
      </div>
      <form onSubmit={handleSubmit} className="space-y-4">
        {error && (
          <p role="alert" className="rounded bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </p>
        )}
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
            autoComplete="new-password"
            minLength={8}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            aria-describedby="password-help"
          />
          <p id="password-help" className="text-sm text-muted-foreground">
            At least 8 characters
          </p>
        </div>
        <Button type="submit" className="w-full" disabled={loading}>
          {loading ? 'Creating account...' : 'Create free account'}
        </Button>
      </form>
    </div>
  )
}
