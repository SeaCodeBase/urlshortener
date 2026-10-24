'use client'

import { useId, useState } from 'react'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const MIN_LEAD_MS = 5 * 60 * 1000

/** `Date` → `YYYY-MM-DDTHH:mm` in local time (the `datetime-local` format). */
export function toDatetimeLocal(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** Earliest allowed expiration: now + 5 minutes. */
export function minExpiration(now = new Date()): Date {
  return new Date(now.getTime() + MIN_LEAD_MS)
}

/** "Expiration date" datetime-local input (value in local `YYYY-MM-DDTHH:mm`). */
export function ExpirationField({
  value,
  onChange,
  error,
}: {
  value: string
  onChange: (value: string) => void
  error?: string | null
}) {
  const id = useId()
  // Computed once per mount; the submit handler re-checks against the real clock.
  const [min] = useState(() => toDatetimeLocal(minExpiration()))
  return (
    <div className="space-y-1.5">
      <Label htmlFor={id}>Expiration date</Label>
      <Input
        id={id}
        type="datetime-local"
        min={min}
        value={value}
        aria-invalid={error ? true : undefined}
        onChange={(e) => onChange(e.target.value)}
        className="sm:w-72"
      />
      {error && <p className="text-sm text-destructive">{error}</p>}
      <p className="text-sm text-muted-foreground">
        <span className="font-semibold text-foreground">After expiration:</span> Visitors will be redirected to the
        error page.
      </p>
    </div>
  )
}
