'use client'

import { useId, useState, type FormEvent } from 'react'
import { useRouter } from 'next/navigation'
import { ArrowRight, Sparkles } from 'lucide-react'
import { toast } from 'sonner'
import { LinkReadyDialog } from '@/components/links/LinkReadyDialog'
import {
  domainIdForCreate,
  generateBackHalf,
  usableDomains,
  useDefaultHost,
  useGroupDefaultDomainId,
  validateBackHalf,
} from '@/components/links/ShortLinkField'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ApiError } from '@/lib/api'
import { useActiveGroupId, useCreateLink, useDomains } from '@/lib/hooks'
import { normalizeDestination } from '@/lib/url'
import { cn } from '@/lib/utils'

type Mode = 'link' | 'qr'
const DEFAULT_DOMAIN = 'default'

/** Home "What do you want to create?" card: Short link | QR Code. */
export function QuickCreate() {
  const id = useId()
  const router = useRouter()
  const createLink = useCreateLink()
  const { data: domains } = useDomains()
  const defaultHost = useDefaultHost()
  const [mode, setMode] = useState<Mode>('link')
  const [url, setUrl] = useState('')
  const domainOptions = usableDomains(domains, useActiveGroupId())
  const groupDefaultDomainId = useGroupDefaultDomainId(domainOptions)
  // undefined until the user picks a domain: the group default is shown.
  const [domainChoice, setDomainChoice] = useState<number | null | undefined>(undefined)
  const domainId = domainChoice === undefined ? groupDefaultDomainId : domainChoice
  const [backHalf, setBackHalf] = useState('')
  const [backHalfError, setBackHalfError] = useState<string | null>(null)
  const [alsoQr, setAlsoQr] = useState(false)
  const [created, setCreated] = useState<{ id: number; short_url: string; qr: boolean } | null>(null)

  async function submit(e: FormEvent) {
    e.preventDefault()
    const destination = normalizeDestination(url)
    if (!destination) {
      toast.error('Enter a valid http(s) URL')
      return
    }
    if (mode === 'qr') {
      router.push(`/dashboard/qr-codes/create?url=${encodeURIComponent(destination)}`)
      return
    }
    const bhError = validateBackHalf(backHalf.trim())
    if (bhError) {
      setBackHalfError(bhError)
      return
    }
    try {
      const link = await createLink.mutateAsync({
        original_url: destination,
        custom_code: backHalf.trim() || undefined,
        domain_id: domainIdForCreate(domainChoice, groupDefaultDomainId),
      })
      setCreated({ id: link.id, short_url: link.short_url, qr: alsoQr })
      setUrl('')
      setBackHalf('')
      setBackHalfError(null)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to create link'
      if (err instanceof ApiError && err.status === 409) setBackHalfError(message)
      toast.error(message)
    }
  }

  return (
    <div className="mx-auto w-full max-w-[1040px]">
      <div className="mb-6 flex flex-col items-center gap-4">
        <h1 className="text-center text-[28px] leading-9 font-semibold text-foreground">What do you want to create?</h1>
        <div role="radiogroup" aria-label="What to create" className="inline-flex rounded-full bg-muted p-1">
          {(
            [
              ['link', 'Short link'],
              ['qr', 'QR Code'],
            ] as const
          ).map(([value, label]) => (
            <button
              key={value}
              type="button"
              role="radio"
              aria-checked={mode === value}
              onClick={() => setMode(value)}
              className={cn(
                'rounded-full px-5 py-1.5 text-sm font-semibold transition-colors',
                mode === value ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground',
              )}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      <form onSubmit={submit} className="space-y-4 rounded-xl border border-border bg-card p-6">
        <div className="space-y-1.5">
          <Label htmlFor={`${id}-url`}>Enter your destination URL</Label>
          <Input
            id={`${id}-url`}
            placeholder="Paste a long URL to shorten it..."
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            autoComplete="off"
          />
        </div>

        {mode === 'link' && (
          <>
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium text-foreground">Domain:</span>
                <Select
                  value={domainId == null ? DEFAULT_DOMAIN : String(domainId)}
                  onValueChange={(v) => setDomainChoice(v === DEFAULT_DOMAIN ? null : Number(v))}
                >
                  <SelectTrigger aria-label="Domain" className="min-w-40">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={DEFAULT_DOMAIN}>{defaultHost || 'Default domain'}</SelectItem>
                    {domainOptions.map((d) => (
                      <SelectItem key={d.id} value={String(d.id)}>
                        {d.domain}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <span className="hidden text-muted-foreground sm:block" aria-hidden>
                /
              </span>
              <Input
                aria-label="custom back-half"
                placeholder="custom back-half"
                value={backHalf}
                maxLength={255}
                aria-invalid={backHalfError ? true : undefined}
                className="flex-1"
                onChange={(e) => {
                  setBackHalf(e.target.value)
                  if (backHalfError) setBackHalfError(null)
                }}
                onBlur={(e) => setBackHalfError(validateBackHalf(e.target.value.trim()))}
              />
              <Button
                type="button"
                variant="secondary"
                onClick={() => {
                  setBackHalf(generateBackHalf())
                  setBackHalfError(null)
                }}
              >
                <Sparkles className="size-4" />
                Generate
              </Button>
            </div>
            {backHalfError && (
              <p className="text-sm text-destructive">{backHalfError}</p>
            )}
            <div className="flex items-center gap-2">
              <Checkbox id={`${id}-qr`} checked={alsoQr} onCheckedChange={(v) => setAlsoQr(v === true)} />
              <Label htmlFor={`${id}-qr`} className="font-normal">
                Also create a QR Code for this link
              </Label>
            </div>
          </>
        )}

        <div className="flex justify-end">
          <Button type="submit" disabled={createLink.isPending}>
            {mode === 'qr' ? 'Create your code' : createLink.isPending ? 'Creating…' : 'Create your link'}
            <ArrowRight className="size-4" />
          </Button>
        </div>
      </form>

      <LinkReadyDialog
        open={created != null}
        onOpenChange={(o) => !o && setCreated(null)}
        link={created}
        showCreateQr={created?.qr}
      />
    </div>
  )
}
