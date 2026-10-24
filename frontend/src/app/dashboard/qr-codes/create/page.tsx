'use client'

import { Suspense, useEffect, useState, type FormEvent } from 'react'
import Link from 'next/link'
import { useRouter, useSearchParams } from 'next/navigation'
import { toast } from 'sonner'
import { PageHeader } from '@/components/common/PageHeader'
import { minExpiration } from '@/components/links/ExpirationField'
import { toRoutingRuleInputs } from '@/components/links/RoutingRulesEditor'
import {
  domainIdForCreate,
  useGroupDefaultDomainId,
  usableDomains,
  validateBackHalf,
} from '@/components/links/ShortLinkField'
import { cleanUtm, rememberUtm } from '@/components/links/UtmBuilder'
import {
  QrContentForm,
  emptyQrContentDraft,
  type QrContentDraft,
  type QrContentErrors,
} from '@/components/qr/QrContentForm'
import { QrDesigner } from '@/components/qr/QrDesigner'
import { QrReadyDialog } from '@/components/qr/QrReadyDialog'
import { QrStepper } from '@/components/qr/QrStepper'
import { Button } from '@/components/ui/button'
import { ApiError } from '@/lib/api'
import { useActiveGroupId, useCreateQrCode, useDomains, useLink, useQrCode } from '@/lib/hooks'
import { normalizeQrDesign } from '@/lib/qr/design'
import { normalizeDestination } from '@/lib/url'
import { DEFAULT_QR_DESIGN, type CreateQrInput, type QrCode, type QrDesign } from '@/types'

const PREVIEW_PLACEHOLDER = 'https://example.com'

function positiveId(raw: string | null): number | null {
  const n = Number(raw)
  return Number.isInteger(n) && n > 0 ? n : null
}

function CreateQrFlow() {
  const router = useRouter()
  const params = useSearchParams()
  const linkId = positiveId(params.get('link'))
  const designId = positiveId(params.get('design'))
  const createQr = useCreateQrCode()
  const linkQuery = useLink(linkId)
  const { data: designSource, isError: designFailed } = useQrCode(designId)
  const { data: domains } = useDomains()
  const groupDefaultDomainId = useGroupDefaultDomainId(usableDomains(domains, useActiveGroupId()))

  const [step, setStep] = useState<1 | 2>(1)
  const [draft, setDraft] = useState<QrContentDraft>(() => emptyQrContentDraft(params.get('url') ?? ''))
  const [errors, setErrors] = useState<QrContentErrors>({})
  const [design, setDesign] = useState<QrDesign>(DEFAULT_QR_DESIGN)
  const [designSeeded, setDesignSeeded] = useState(false)
  const [created, setCreated] = useState<QrCode | null>(null)

  // ?design={id}: start from that code's design once it loads.
  if (designSource && !designSeeded) {
    setDesignSeeded(true)
    setDesign(normalizeQrDesign(designSource.render_customizations))
  }

  useEffect(() => {
    if (designFailed) toast.error("Couldn't load the design to start from")
  }, [designFailed])

  /** User edits win over a `?design=` response that arrives later. */
  function changeDesign(next: QrDesign) {
    setDesignSeeded(true)
    setDesign(next)
  }

  const attachedLink = linkId ? linkQuery.data : undefined

  function changeDraft(next: QrContentDraft) {
    setDraft(next)
    setErrors((e) => ({
      destination: next.destination === draft.destination ? e.destination : null,
      backHalf: next.backHalf === draft.backHalf && next.domainChoice === draft.domainChoice ? e.backHalf : null,
      expiresAt: next.expiresAt === draft.expiresAt ? e.expiresAt : null,
    }))
  }

  /** Step 1 errors, or null when the draft can go to the designer. */
  function validate(): QrContentErrors | null {
    if (attachedLink) return null
    const next: QrContentErrors = {}
    if (!normalizeDestination(draft.destination)) next.destination = 'Enter a valid http(s) URL (up to 2048 characters)'
    if (draft.shareAsLink) next.backHalf = validateBackHalf(draft.backHalf.trim())
    if (draft.expirationOn && draft.expiresAt) {
      const d = new Date(draft.expiresAt)
      if (Number.isNaN(d.getTime()) || d < minExpiration()) next.expiresAt = 'Pick a date at least 5 minutes from now'
    }
    return next.destination || next.backHalf || next.expiresAt ? next : null
  }

  function toDesign(e: FormEvent) {
    e.preventDefault()
    const found = validate()
    setErrors(found ?? {})
    if (!found) setStep(2)
  }

  function buildInput(): CreateQrInput {
    const base = {
      title: draft.title.trim() || undefined,
      tags: draft.tags.length ? draft.tags : undefined,
      render_customizations: design,
    }
    if (attachedLink) return { ...base, link_id: attachedLink.id }
    const input: CreateQrInput = {
      ...base,
      long_url: normalizeDestination(draft.destination) ?? draft.destination,
      domain_id: domainIdForCreate(draft.domainChoice, groupDefaultDomainId),
      share_as_link: draft.shareAsLink,
    }
    const backHalf = draft.backHalf.trim()
    if (draft.shareAsLink && backHalf) input.custom_code = backHalf
    if (draft.expirationOn && draft.expiresAt) input.expires_at = new Date(draft.expiresAt).toISOString()
    if (draft.utmOn) input.utm = cleanUtm(draft.utm)
    if (draft.routingOn) {
      const routing = toRoutingRuleInputs(draft.rules)
      if (routing.length) input.routing_rules = routing
    }
    return input
  }

  async function create() {
    if (createQr.isPending) return
    const input = buildInput()
    try {
      const qr = await createQr.mutateAsync(input)
      if (input.utm) rememberUtm(input.utm)
      setCreated(qr)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to create the QR Code'
      if (err instanceof ApiError && err.status === 409 && draft.shareAsLink && !attachedLink) {
        setErrors({ backHalf: message })
        setStep(1)
        return
      }
      toast.error(message)
    }
  }

  if (linkId && linkQuery.isPending) {
    return <p className="text-muted-foreground">Loading link…</p>
  }
  if (linkId && !attachedLink) {
    return (
      <p className="text-muted-foreground">
        We couldn&apos;t load that link.{' '}
        <Link href="/dashboard/qr-codes/create" className="font-semibold text-primary hover:underline">
          Create a QR Code for a new URL
        </Link>
      </p>
    )
  }

  const previewData = attachedLink?.short_url ?? normalizeDestination(draft.destination) ?? PREVIEW_PLACEHOLDER
  const cancel = (
    <Button type="button" variant="secondary" asChild>
      <Link href="/dashboard/qr-codes">Cancel</Link>
    </Button>
  )

  return (
    <>
      <div className="mb-6">
        <QrStepper step={step} />
      </div>
      {step === 1 ? (
        <form onSubmit={toDesign} noValidate>
          <div className="mx-auto max-w-[720px] pb-24">
            <QrContentForm value={draft} onChange={changeDraft} attachedLink={attachedLink} errors={errors} />
          </div>
          <div className="sticky bottom-0 -mx-4 border-t border-border bg-background px-4 py-3">
            <div className="flex justify-end gap-3">
              {cancel}
              <Button type="submit">Design your code →</Button>
            </div>
          </div>
        </form>
      ) : (
        <div>
          <div className="pb-24">
            <QrDesigner design={design} onChange={changeDesign} data={previewData} />
          </div>
          <div className="sticky bottom-0 -mx-4 border-t border-border bg-background px-4 py-3">
            <div className="flex justify-between gap-3">
              <Button type="button" variant="ghost" onClick={() => setStep(1)}>
                ← Back
              </Button>
              <div className="flex gap-3">
                {cancel}
                <Button type="button" onClick={create} disabled={createQr.isPending}>
                  {createQr.isPending ? 'Creating…' : 'Create your code'}
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}
      <QrReadyDialog
        open={created !== null}
        onOpenChange={(open) => {
          if (!open) router.push('/dashboard/qr-codes')
        }}
        qr={created}
      />
    </>
  )
}

export default function CreateQrCodePage() {
  return (
    <div className="mx-auto w-full max-w-[1040px]">
      <PageHeader title="Create a new QR Code" />
      {/* useSearchParams needs a Suspense boundary for static rendering. */}
      <Suspense>
        <CreateQrFlow />
      </Suspense>
    </div>
  )
}
