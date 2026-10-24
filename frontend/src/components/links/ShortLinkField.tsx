'use client'

import { useId, useState } from 'react'
import { Sparkles } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useActiveGroupId, useGroups, useMeta } from '@/lib/hooks'
import type { Domain } from '@/types'

export const BACK_HALF_RE = /^[A-Za-z0-9_-]{1,255}$/
export const BACK_HALF_ERROR = 'Only letters, numbers, - and _ are allowed'
const ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
const DEFAULT_DOMAIN = 'default'

/** Random 7-character alphanumeric back-half. */
export function generateBackHalf(length = 7): string {
  const bytes = new Uint32Array(length)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (b) => ALPHABET[b % ALPHABET.length]).join('')
}

/** Error message for a back-half, or null when valid (empty is valid: it's optional). */
export function validateBackHalf(value: string): string | null {
  if (!value) return null
  return BACK_HALF_RE.test(value) ? null : BACK_HALF_ERROR
}

/** Host of the server's base URL (`http://localhost:8080` → `localhost:8080`). */
export function useDefaultHost(): string {
  const { data: meta } = useMeta()
  if (!meta?.base_url) return ''
  try {
    return new URL(meta.base_url).host
  } catch {
    return meta.base_url
  }
}

/**
 * Domains a link of the active group can use: verified (the others can't
 * serve links yet) and either shared by all groups or bound to that group.
 */
export function usableDomains(domains: Domain[] | undefined, activeGroupId: number | null): Domain[] {
  return (domains ?? []).filter(
    (d) => d.verification_status === 'ok' && (d.group_id == null || d.group_id === activeGroupId),
  )
}

/** The active group's default domain id when it is among `usable`, else null (the service default). */
export function useGroupDefaultDomainId(usable: Domain[]): number | null {
  const activeGroupId = useActiveGroupId()
  const { data: groups } = useGroups()
  const id = groups?.find((g) => g.id === activeGroupId)?.default_domain_id ?? null
  return id != null && usable.some((d) => d.id === id) ? id : null
}

/**
 * domain_id for a new link: the user's explicit pick (0 = the service
 * default), else the preselected group default; undefined lets the backend
 * apply the group default itself.
 */
export function domainIdForCreate(choice: number | null | undefined, groupDefault: number | null): number | undefined {
  if (choice === undefined) return groupDefault ?? undefined
  return choice ?? 0
}

/** "Short link domain" select + " / " + "Back-half (optional)" with a Generate button. */
export function ShortLinkField({
  domains,
  domainId,
  onDomainChange,
  backHalf,
  onBackHalfChange,
  error,
}: {
  domains: Domain[]
  domainId: number | null
  onDomainChange: (id: number | null) => void
  backHalf: string
  onBackHalfChange: (value: string) => void
  /** Server-side error (e.g. "That back-half is already taken"); wins over local validation. */
  error?: string | null
}) {
  const id = useId()
  const defaultHost = useDefaultHost()
  const [localError, setLocalError] = useState<string | null>(null)
  const shown = error || localError
  const options = usableDomains(domains, useActiveGroupId())

  return (
    <div className="space-y-1.5">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
        <div className="space-y-1.5 sm:w-56">
          <Label htmlFor={`${id}-domain`}>Short link domain</Label>
          <Select
            value={domainId == null ? DEFAULT_DOMAIN : String(domainId)}
            onValueChange={(v) => onDomainChange(v === DEFAULT_DOMAIN ? null : Number(v))}
          >
            <SelectTrigger id={`${id}-domain`} className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={DEFAULT_DOMAIN}>{defaultHost || 'Default domain'}</SelectItem>
              {options.map((d) => (
                <SelectItem key={d.id} value={String(d.id)}>
                  {d.domain}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <span className="hidden pb-2 text-xl text-muted-foreground sm:block" aria-hidden>
          /
        </span>
        <div className="flex-1 space-y-1.5">
          <Label htmlFor={`${id}-backhalf`}>Back-half (optional)</Label>
          <div className="flex gap-2">
            <Input
              id={`${id}-backhalf`}
              placeholder="my-custom-back-half"
              value={backHalf}
              maxLength={255}
              aria-invalid={shown ? true : undefined}
              aria-describedby={shown ? `${id}-error` : undefined}
              onChange={(e) => {
                onBackHalfChange(e.target.value)
                if (localError) setLocalError(validateBackHalf(e.target.value))
              }}
              onBlur={(e) => setLocalError(validateBackHalf(e.target.value))}
            />
            <Button
              type="button"
              variant="secondary"
              onClick={() => {
                onBackHalfChange(generateBackHalf())
                setLocalError(null)
              }}
            >
              <Sparkles className="size-4" />
              Generate
            </Button>
          </div>
        </div>
      </div>
      {shown && (
        <p id={`${id}-error`} className="text-sm text-destructive">
          {shown}
        </p>
      )}
    </div>
  )
}
