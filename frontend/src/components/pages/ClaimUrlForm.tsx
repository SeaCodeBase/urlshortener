'use client'

import { useId, useState, type FormEvent } from 'react'
import { Check, Loader2, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useMeta, useSlugCheck } from '@/lib/hooks'
import { cn } from '@/lib/utils'
import type { SlugCheckReason } from '@/types'
import { SLUG_MAX } from './designPresets'

const REASON_LABEL: Record<Exclude<SlugCheckReason, ''>, string> = {
  taken: 'Already taken',
  invalid: 'Invalid',
  reserved: 'Reserved',
}

/** Lowercases and drops everything outside `[a-z0-9-]`, capped at 40 chars. */
export function sanitizeSlug(raw: string): string {
  return raw
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, '')
    .slice(0, SLUG_MAX)
}

/** Host of the short-link base URL (e.g. `sho.rt`), or the raw value when it isn't a URL. */
function useBaseHost(): string {
  const { data: meta } = useMeta()
  if (!meta?.base_url) return ''
  try {
    return new URL(meta.base_url).host
  } catch {
    return meta.base_url
  }
}

export interface ClaimUrlFormProps {
  initialSlug?: string
  /** A slug the server rejected with 409; shown as "Already taken" while it is the input. */
  conflictSlug?: string | null
  onSubmit: (slug: string) => void
}

/** Step 1 of page creation: `{host}/m/` + slug with a live availability check. */
export function ClaimUrlForm({ initialSlug = '', conflictSlug = null, onSubmit }: ClaimUrlFormProps) {
  const [slug, setSlug] = useState(() => sanitizeSlug(initialSlug))
  const check = useSlugCheck(slug)
  const host = useBaseHost()
  const inputId = useId()
  const statusId = useId()

  const conflict = conflictSlug != null && conflictSlug === slug
  const current = check.slug === slug && !check.isChecking ? check.data : null
  const available = !conflict && current?.available === true
  let status: { label: string; ok: boolean } | null = null
  if (conflict) status = { label: REASON_LABEL.taken, ok: false }
  else if (slug && check.isChecking) status = null
  else if (current) status = current.available ? { label: 'Available', ok: true } : { label: REASON_LABEL[current.reason || 'taken'], ok: false }
  else if (check.error) status = { label: "Couldn't check availability", ok: false }

  function submit(e: FormEvent) {
    e.preventDefault()
    if (available) onSubmit(slug)
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-6 rounded-xl bg-card p-6">
      <div className="flex flex-col gap-1">
        <h2 className="text-xl font-semibold text-foreground">Claim your URL</h2>
        <p className="text-base text-muted-foreground">
          Pick the address people will visit. 3–40 characters: lowercase letters, numbers and hyphens.
        </p>
      </div>
      <div className="flex flex-col gap-2">
        <label htmlFor={inputId} className="text-sm font-semibold text-foreground">
          Page URL
        </label>
        <div className="flex h-10 items-center rounded border border-border bg-white focus-within:ring-2 focus-within:ring-ring/50 dark:bg-background">
          <span className="shrink-0 border-r border-border px-3 text-base text-muted-foreground">{host}/m/</span>
          <input
            id={inputId}
            type="text"
            value={slug}
            onChange={(e) => setSlug(sanitizeSlug(e.target.value))}
            maxLength={SLUG_MAX}
            autoComplete="off"
            autoCapitalize="none"
            spellCheck={false}
            autoFocus
            placeholder="your-name"
            aria-describedby={statusId}
            aria-invalid={status != null && !status.ok}
            className="h-full min-w-0 flex-1 bg-transparent px-3 text-base text-foreground outline-none"
          />
        </div>
        <div id={statusId} aria-live="polite" className="min-h-5 text-sm">
          {slug && check.isChecking && !conflict ? (
            <span className="inline-flex items-center gap-1 text-muted-foreground">
              <Loader2 className="size-4 animate-spin" aria-hidden />
              Checking…
            </span>
          ) : status ? (
            <span className={cn('inline-flex items-center gap-1', status.ok ? 'text-success' : 'text-destructive')}>
              {status.ok ? <Check className="size-4" aria-hidden /> : <X className="size-4" aria-hidden />}
              {status.label}
            </span>
          ) : null}
        </div>
      </div>
      <div>
        <Button type="submit" disabled={!available}>
          Create your page
        </Button>
      </div>
    </form>
  )
}
