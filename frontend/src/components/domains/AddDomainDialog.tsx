'use client'

import { useState } from 'react'
import { CheckCircle2, XCircle } from 'lucide-react'
import { toast } from 'sonner'
import { CopyButton } from '@/components/common/CopyButton'
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
import { useCreateDomain, useMeta, useVerifyDomain } from '@/lib/hooks'
import { cn } from '@/lib/utils'
import type { Domain } from '@/types'
import { GroupSelect, groupValueToId } from './GroupSelect'

const STEPS = ['Enter domain', 'Configure DNS', 'Verify'] as const
type Step = 0 | 1 | 2

/** Subdomains (3+ labels) can CNAME to the redirect host; apex domains need an A record. */
export function isSubdomain(domain: string): boolean {
  return domain.split('.').filter(Boolean).length >= 3
}

export interface DnsRecord {
  type: 'CNAME' | 'A' | 'TXT'
  name: string
  value: string
}

/** The routing record (CNAME or A) followed by the ownership TXT record. */
export function dnsRecords(domain: Domain, redirectHost: string, redirectIp: string): DnsRecord[] {
  const records: DnsRecord[] = []
  if (isSubdomain(domain.domain) || !redirectIp) {
    records.push({ type: 'CNAME', name: domain.domain, value: redirectHost })
  } else {
    records.push({ type: 'A', name: domain.domain, value: redirectIp })
  }
  records.push({ type: 'TXT', name: `_urlshortener.${domain.domain}`, value: `verify=${domain.verification_token}` })
  return records
}

/**
 * "Add a domain" stepper: Enter domain → Configure DNS → Verify.
 * With `initialDomain` it opens at the DNS step for an existing domain ("View DNS instructions").
 */
export function AddDomainDialog({
  open,
  onOpenChange,
  initialDomain,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  initialDomain?: Domain | null
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[600px]">
        {/* Mounted only while open so the stepper resets every time. */}
        {open && <AddDomainBody initialDomain={initialDomain ?? null} onClose={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  )
}

function Stepper({ step }: { step: Step }) {
  return (
    <ol className="flex items-center gap-2 text-sm" aria-label="Progress">
      {STEPS.map((label, i) => (
        <li key={label} className="flex items-center gap-2" aria-current={i === step ? 'step' : undefined}>
          <span
            className={cn(
              'flex size-6 items-center justify-center rounded-full text-xs font-semibold',
              i <= step ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'
            )}
          >
            {i + 1}
          </span>
          <span className={cn(i === step ? 'font-semibold text-foreground' : 'text-muted-foreground')}>{label}</span>
          {i < STEPS.length - 1 && <span className="mx-1 h-px w-6 bg-border" aria-hidden />}
        </li>
      ))}
    </ol>
  )
}

function RecordRow({ record }: { record: DnsRecord }) {
  return (
    <div
      data-testid={`dns-record-${record.type}`}
      className="grid grid-cols-[56px_1fr] gap-x-3 gap-y-1 rounded border border-border bg-muted px-3 py-2 sm:grid-cols-[56px_1fr_1fr]"
    >
      <span className="self-center text-xs font-semibold text-foreground">{record.type}</span>
      <div className="flex min-w-0 items-center gap-1">
        <code className="min-w-0 flex-1 font-mono text-sm break-all text-foreground">{record.name}</code>
        <CopyButton text={record.name} />
      </div>
      <div className="col-start-2 flex min-w-0 items-center gap-1 sm:col-start-3">
        <code className="min-w-0 flex-1 font-mono text-sm break-all text-foreground">{record.value}</code>
        <CopyButton text={record.value} />
      </div>
    </div>
  )
}

function AddDomainBody({ initialDomain, onClose }: { initialDomain: Domain | null; onClose: () => void }) {
  const { data: meta } = useMeta()
  const create = useCreateDomain()
  const verify = useVerifyDomain()

  const [step, setStep] = useState<Step>(initialDomain ? 1 : 0)
  const [name, setName] = useState('')
  const [group, setGroup] = useState('all')
  const [error, setError] = useState<string | null>(null)
  const [domain, setDomain] = useState<Domain | null>(initialDomain)
  const [verifying, setVerifying] = useState(false)

  async function submitDomain(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    const value = name.trim().toLowerCase()
    if (!value) {
      setError('Enter a domain')
      return
    }
    try {
      const created = await create.mutateAsync({ domain: value, group_id: groupValueToId(group) })
      setDomain(created)
      setStep(1)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add domain')
    }
  }

  async function runVerify() {
    if (!domain) return
    setVerifying(true)
    try {
      const updated = await verify.mutateAsync(domain.id)
      setDomain(updated)
      setStep(2)
      if (updated.verification_status === 'ok') toast.success('Domain verified')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Verification failed')
    } finally {
      setVerifying(false)
    }
  }

  return (
    <div className="grid gap-5">
      <DialogHeader>
        <DialogTitle>{initialDomain ? 'DNS instructions' : 'Add a domain'}</DialogTitle>
        <DialogDescription className="sr-only">Add and verify a custom domain</DialogDescription>
      </DialogHeader>
      <Stepper step={step} />

      {step === 0 && (
        <form onSubmit={submitDomain} className="grid gap-4" noValidate>
          <div className="grid gap-2">
            <Label htmlFor="domain-name">Domain</Label>
            <Input
              id="domain-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="links.example.com"
              maxLength={253}
              autoFocus
              aria-invalid={error ? true : undefined}
              aria-describedby="domain-name-help"
            />
            <p id="domain-name-help" className="text-sm text-muted-foreground">
              Use a domain or subdomain you own (up to 253 characters).
            </p>
            {error && (
              <p role="alert" className="text-sm text-destructive">
                {error}
              </p>
            )}
          </div>
          <div className="grid gap-2">
            <Label htmlFor="domain-group">Group</Label>
            <GroupSelect id="domain-group" value={group} onChange={setGroup} />
          </div>
          <DialogFooter>
            <Button type="button" variant="secondary" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={create.isPending}>
              {create.isPending ? 'Adding...' : 'Next'}
            </Button>
          </DialogFooter>
        </form>
      )}

      {step === 1 && domain && (
        <div className="grid gap-4">
          <DnsInstructions domain={domain} redirectHost={meta?.redirect_host ?? ''} redirectIp={meta?.redirect_ip ?? ''} />
          <DialogFooter>
            {/* The domain already exists once step 1 succeeded, so going back is not possible. */}
            <Button type="button" variant="secondary" disabled>
              Back
            </Button>
            <Button type="button" onClick={runVerify} disabled={verifying}>
              {verifying ? 'Verifying...' : 'Verify'}
            </Button>
          </DialogFooter>
        </div>
      )}

      {step === 2 && domain && (
        <div className="grid gap-4">
          {domain.verification_status === 'ok' ? (
            <div className="flex flex-col items-center gap-2 py-4 text-center">
              <CheckCircle2 className="size-10 text-success" aria-hidden />
              <p className="text-lg font-semibold text-foreground">Your domain is verified</p>
              <p className="text-sm text-muted-foreground">{domain.domain} is ready to use in your links.</p>
            </div>
          ) : (
            <div className="flex flex-col items-center gap-2 py-4 text-center">
              <XCircle className="size-10 text-destructive" aria-hidden />
              <p className="text-lg font-semibold text-foreground">We couldn&apos;t verify {domain.domain}</p>
              <p role="alert" className="text-sm text-destructive">
                {domain.verification_error || 'DNS records not found yet.'}
              </p>
            </div>
          )}
          <DialogFooter>
            {domain.verification_status === 'ok' ? (
              <Button type="button" onClick={onClose}>
                Done
              </Button>
            ) : (
              <>
                <Button type="button" variant="secondary" onClick={onClose}>
                  I&apos;ll do this later
                </Button>
                <Button type="button" onClick={runVerify} disabled={verifying}>
                  {verifying ? 'Verifying...' : 'Try again'}
                </Button>
              </>
            )}
          </DialogFooter>
        </div>
      )}
    </div>
  )
}

function DnsInstructions({
  domain,
  redirectHost,
  redirectIp,
}: {
  domain: Domain
  redirectHost: string
  redirectIp: string
}) {
  const [routing, txt] = dnsRecords(domain, redirectHost, redirectIp)
  return (
    <div className="grid gap-4">
      <section className="grid gap-2">
        <h3 className="text-base font-semibold text-foreground">Point your domain to this server</h3>
        <RecordRow record={routing} />
      </section>
      <section className="grid gap-2">
        <h3 className="text-base font-semibold text-foreground">Prove ownership (required for root domains)</h3>
        <RecordRow record={txt} />
      </section>
      <p className="text-sm text-muted-foreground">DNS changes can take up to 24 hours to propagate.</p>
    </div>
  )
}
