'use client'

import { useState, type FormEvent } from 'react'
import NextLink from 'next/link'
import { useParams, useRouter } from 'next/navigation'
import { toast } from 'sonner'
import { EmptyState } from '@/components/common/EmptyState'
import { PageHeader } from '@/components/common/PageHeader'
import { minExpiration, toDatetimeLocal } from '@/components/links/ExpirationField'
import { emptyRule, fromRoutingRule, toRoutingRuleInputs } from '@/components/links/RoutingRulesEditor'
import {
  QrContentForm,
  emptyQrContentDraft,
  type QrContentDraft,
  type QrContentErrors,
} from '@/components/qr/QrContentForm'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useQrCode, useUpdateQrCode } from '@/lib/hooks'
import { normalizeDestination } from '@/lib/url'
import type { QrCode, UpdateQrInput } from '@/types'

function draftFrom(qr: QrCode): QrContentDraft {
  const rules = qr.link?.routing_rules ?? []
  return {
    ...emptyQrContentDraft(qr.original_url),
    title: qr.title ?? '',
    tags: qr.tags ?? [],
    expirationOn: qr.expires_at != null,
    expiresAt: qr.expires_at ? toDatetimeLocal(new Date(qr.expires_at)) : '',
    routingOn: rules.length > 0,
    rules: rules.length ? rules.map(fromRoutingRule) : [emptyRule()],
  }
}

function sameTags(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false
  const set = new Set(a)
  return b.every((t) => set.has(t))
}

function EditQrForm({ qr }: { qr: QrCode }) {
  const router = useRouter()
  const updateQr = useUpdateQrCode()
  const [initial] = useState<QrContentDraft>(() => draftFrom(qr))
  const [draft, setDraft] = useState<QrContentDraft>(initial)
  const [errors, setErrors] = useState<QrContentErrors>({})
  const detailsHref = `/dashboard/qr-codes/${qr.id}`

  function changeDraft(next: QrContentDraft) {
    setDraft(next)
    setErrors((e) => ({
      destination: next.destination === draft.destination ? e.destination : null,
      expiresAt: next.expiresAt === draft.expiresAt ? e.expiresAt : null,
    }))
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (updateQr.isPending) return

    const found: QrContentErrors = {}
    const destination = normalizeDestination(draft.destination)
    if (!destination) found.destination = 'Enter a valid http(s) URL (up to 2048 characters)'

    // Only changed fields are sent: routing rules are replaced wholesale, and an unchanged
    // title must not pin the displayed (link) title onto the code.
    const changes: UpdateQrInput = {}
    const title = draft.title.trim()
    if (title !== initial.title) changes.title = title
    if (!sameTags(draft.tags, initial.tags)) changes.tags = draft.tags
    const rules = draft.routingOn ? toRoutingRuleInputs(draft.rules) : []
    const initialRules = initial.routingOn ? toRoutingRuleInputs(initial.rules) : []
    if (JSON.stringify(rules) !== JSON.stringify(initialRules)) changes.routing_rules = rules
    if (destination && destination !== qr.original_url) changes.long_url = destination

    if (draft.expirationOn) {
      const d = new Date(draft.expiresAt)
      const unchanged = qr.expires_at != null && new Date(qr.expires_at).getTime() === d.getTime()
      if (!draft.expiresAt || Number.isNaN(d.getTime())) found.expiresAt = 'Pick an expiration date'
      else if (!unchanged && d < minExpiration()) found.expiresAt = 'Pick a date at least 5 minutes from now'
      else if (!unchanged) changes.expires_at = d.toISOString()
    } else if (qr.expires_at) {
      changes.clear_expiration = true
    }

    setErrors(found)
    if (found.destination || found.expiresAt) return

    if (Object.keys(changes).length === 0) {
      router.push(detailsHref)
      return
    }
    try {
      await updateQr.mutateAsync({ id: qr.id, data: changes })
      toast.success('QR Code updated')
      router.push(detailsHref)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Couldn't update the QR Code")
    }
  }

  return (
    <form onSubmit={submit} noValidate>
      <div className="pb-24">
        <QrContentForm value={draft} onChange={changeDraft} mode="edit" errors={errors} />
      </div>
      <div className="sticky bottom-0 -mx-4 border-t border-border bg-background px-4 py-3">
        <div className="mx-auto flex max-w-[720px] justify-end gap-3">
          <Button type="button" variant="secondary" asChild>
            <NextLink href={detailsHref}>Cancel</NextLink>
          </Button>
          <Button type="submit" disabled={updateQr.isPending}>
            {updateQr.isPending ? 'Saving…' : 'Save'}
          </Button>
        </div>
      </div>
    </form>
  )
}

export default function EditQrCodePage() {
  const params = useParams<{ id: string }>()
  const id = Number(params.id)
  const { data: qr, isLoading, isError } = useQrCode(Number.isFinite(id) ? id : null)

  return (
    <div className="mx-auto w-full max-w-[720px]">
      <PageHeader
        title="Edit QR Code"
        backHref={Number.isFinite(id) ? `/dashboard/qr-codes/${id}` : '/dashboard/qr-codes'}
      />
      {isLoading ? (
        <div className="space-y-4">
          <Skeleton className="h-56 rounded-xl" />
          <Skeleton className="h-40 rounded-xl" />
        </div>
      ) : isError || !qr ? (
        <EmptyState
          title="QR Code not found"
          description="It may have been deleted, or it belongs to another group."
          action={
            <Button asChild>
              <NextLink href="/dashboard/qr-codes">Back to QR Codes</NextLink>
            </Button>
          }
        />
      ) : (
        // Keyed so the form state re-initialises if the id changes.
        <EditQrForm key={qr.id} qr={qr} />
      )}
    </div>
  )
}
