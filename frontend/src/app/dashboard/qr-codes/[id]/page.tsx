'use client'

import { Suspense, useMemo, type ReactNode } from 'react'
import NextLink from 'next/link'
import { useParams } from 'next/navigation'
import { CornerDownRight, EyeOff } from 'lucide-react'
import { toast } from 'sonner'
import { CopyButton } from '@/components/common/CopyButton'
import { EmptyState } from '@/components/common/EmptyState'
import { LinkAnalytics } from '@/components/links/LinkAnalytics'
import { TagEditor } from '@/components/links/TagEditor'
import { QrDetailsHeader } from '@/components/qr/QrDetailsHeader'
import { QrPreview } from '@/components/qr/QrPreview'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDateTime, formatShortUrl } from '@/lib/format'
import { useBulkQrCodes, useQrCode, useSetQrTags } from '@/lib/hooks'
import { normalizeQrDesign } from '@/lib/qr/design'
import type { QrCode } from '@/types'

const CARD = 'rounded-xl bg-card pt-4 pr-4 pb-6 pl-8'

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="space-y-1">
      <div className="text-sm font-semibold text-muted-foreground">{label}</div>
      <div className="min-w-0">{children}</div>
    </div>
  )
}

function HiddenBanner({ qr }: { qr: QrCode }) {
  const bulk = useBulkQrCodes()
  return (
    <div
      role="status"
      className="mb-4 flex items-center justify-between gap-4 rounded-xl bg-sidebar-active-bg px-6 py-3 text-base text-foreground"
    >
      <span className="inline-flex items-center gap-2">
        <EyeOff className="size-5 text-muted-foreground" />
        This QR Code is hidden.
      </span>
      <Button
        type="button"
        variant="secondary"
        disabled={bulk.isPending}
        onClick={() =>
          bulk.mutate(
            { action: 'unarchive', ids: [qr.id] },
            {
              onSuccess: () => toast.success('QR Code shown'),
              onError: () => toast.error("Couldn't show the QR Code"),
            },
          )
        }
      >
        Show code
      </Button>
    </div>
  )
}

function DetailsCard({ qr }: { qr: QrCode }) {
  const setTags = useSetQrTags()
  // A code whose link is listed under Links shares that short link; otherwise it has its own code URL.
  const sharesLink = qr.link_source === 'link'
  const codeUrl = sharesLink ? qr.short_url : qr.qr_url
  const design = useMemo(() => normalizeQrDesign(qr.render_customizations), [qr.render_customizations])

  return (
    <section className={CARD}>
      <h2 className="mb-4 text-xl font-semibold text-foreground">Details</h2>
      <div className="flex flex-col gap-6 sm:flex-row">
        <QrPreview design={design} data={qr.qr_url} size={200} className="shrink-0" />
        <div className="min-w-0 flex-1 space-y-5">
          <Field label="Destination">
            <div className="flex min-w-0 items-center gap-1.5">
              <CornerDownRight className="size-4 shrink-0 text-muted-foreground" aria-hidden />
              <a
                href={qr.original_url}
                target="_blank"
                rel="noopener noreferrer"
                title={qr.original_url}
                className="truncate text-base font-medium text-primary hover:underline"
              >
                {qr.original_url}
              </a>
            </div>
          </Field>
          <Field label={sharesLink ? 'Short link' : 'Code URL'}>
            <div className="flex min-w-0 items-center gap-1">
              <a
                href={codeUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="truncate text-base font-semibold text-primary hover:underline"
              >
                {formatShortUrl(codeUrl)}
              </a>
              <CopyButton text={codeUrl} />
            </div>
          </Field>
          <Field label="Tags">
            <TagEditor tags={qr.tags ?? []} tagType="qr" onSave={(tags) => setTags.mutateAsync({ id: qr.id, tags })} />
          </Field>
          <Field label="Created on">
            <span className="text-sm text-foreground">{formatDateTime(qr.created_at)}</span>
          </Field>
        </div>
      </div>
    </section>
  )
}

function DetailsSkeleton() {
  return (
    <div className="space-y-4 pt-8">
      <Skeleton className="h-10 w-80" />
      <Skeleton className="h-72 rounded-xl" />
    </div>
  )
}

export default function QrCodeDetailsPage() {
  const params = useParams<{ id: string }>()
  const id = Number(params.id)
  const { data: qr, isLoading, isError } = useQrCode(Number.isFinite(id) ? id : null)

  if (isLoading) return <DetailsSkeleton />
  if (isError || !qr) {
    return (
      <div className="pt-8">
        <EmptyState
          title="QR Code not found"
          description="It may have been deleted, or it belongs to another group."
          action={
            <Button asChild>
              <NextLink href="/dashboard/qr-codes">Back to QR Codes</NextLink>
            </Button>
          }
        />
      </div>
    )
  }

  return (
    <div className="pb-12">
      <QrDetailsHeader qr={qr} />
      {qr.archived && <HiddenBanner qr={qr} />}
      <DetailsCard qr={qr} />
      <div className="mt-8">
        {/* useSearchParams needs a Suspense boundary. */}
        <Suspense>
          <LinkAnalytics subject={{ kind: 'qr', id: qr.id }} destination={qr.original_url} />
        </Suspense>
      </div>
    </div>
  )
}
