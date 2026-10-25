'use client'

import { Suspense, useMemo, useState, type ReactNode } from 'react'
import NextLink from 'next/link'
import { useParams } from 'next/navigation'
import { CornerDownRight, EyeOff, Palette, Plus, QrCode } from 'lucide-react'
import { toast } from 'sonner'
import { CopyButton } from '@/components/common/CopyButton'
import { EmptyState } from '@/components/common/EmptyState'
import { AddToCampaignDialog } from '@/components/links/AddToCampaignDialog'
import { AddToPageDialog, pageLabel } from '@/components/links/AddToPageDialog'
import { DynamicRoutingCard } from '@/components/links/DynamicRoutingCard'
import { LinkAnalytics } from '@/components/links/LinkAnalytics'
import { LinkDetailsHeader } from '@/components/links/LinkDetailsHeader'
import { TagEditor } from '@/components/links/TagEditor'
import { QrDownloadMenu } from '@/components/qr/QrDownloadMenu'
import { QrPreview } from '@/components/qr/QrPreview'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDate, formatDateTime, formatShortUrl } from '@/lib/format'
import { useBulkLinks, useLink, useLinkCampaigns, usePages, useQrCode } from '@/lib/hooks'
import { normalizeQrDesign } from '@/lib/qr/design'
import type { Link } from '@/types'

const CARD = 'rounded-xl bg-card pt-4 pr-4 pb-6 pl-8'

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="space-y-1">
      <div className="text-sm font-semibold text-muted-foreground">{label}</div>
      <div className="min-w-0">{children}</div>
    </div>
  )
}

function HiddenBanner({ link }: { link: Link }) {
  const bulk = useBulkLinks()
  return (
    <div
      role="status"
      className="mb-4 flex items-center justify-between gap-4 rounded-xl bg-sidebar-active-bg px-6 py-3 text-base text-foreground"
    >
      <span className="inline-flex items-center gap-2">
        <EyeOff className="size-5 text-muted-foreground" />
        This link is hidden.
      </span>
      <Button
        type="button"
        variant="secondary"
        disabled={bulk.isPending}
        onClick={() =>
          bulk.mutate(
            { action: 'unarchive', ids: [link.id] },
            {
              onSuccess: () => toast.success('Link shown'),
              onError: () => toast.error("Couldn't show the link"),
            },
          )
        }
      >
        Show link
      </Button>
    </div>
  )
}

function DetailsCard({ link }: { link: Link }) {
  const history = link.history ?? []
  return (
    <section className={`${CARD} space-y-5 lg:col-span-2`}>
      <h2 className="text-xl font-semibold text-foreground">Details</h2>
      <Field label="Short link">
        <div className="flex min-w-0 items-center gap-1">
          <a
            href={link.short_url}
            target="_blank"
            rel="noopener noreferrer"
            className="truncate text-base font-semibold text-primary hover:underline"
          >
            {formatShortUrl(link.short_url)}
          </a>
          <CopyButton text={link.short_url} />
        </div>
      </Field>
      <Field label="Destination">
        <div className="flex min-w-0 items-center gap-1.5">
          <CornerDownRight className="size-4 shrink-0 text-muted-foreground" aria-hidden />
          <a
            href={link.original_url}
            target="_blank"
            rel="noopener noreferrer"
            title={link.original_url}
            className="truncate text-base font-medium text-primary hover:underline"
          >
            {link.original_url}
          </a>
        </div>
      </Field>
      <Field label="Tags">
        <TagEditor linkId={link.id} tags={link.tags ?? []} />
      </Field>
      <Field label="Created on">
        <span className="text-sm text-foreground">{formatDateTime(link.created_at)}</span>
      </Field>
      {history.length > 0 && (
        <Field label="Previous short links">
          <ul className="space-y-1">
            {history.map((h) => (
              <li key={h.id} className="flex flex-wrap items-center gap-x-3 text-sm">
                <a
                  href={h.short_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="font-medium text-primary hover:underline"
                >
                  {formatShortUrl(h.short_url)}
                </a>
                <span className="text-muted-foreground">Created {formatDate(h.created_at)}</span>
              </li>
            ))}
          </ul>
        </Field>
      )}
    </section>
  )
}

function QrCard({ link }: { link: Link }) {
  if (link.qr_code_id) return <LinkQrCode link={link} qrId={link.qr_code_id} />
  return <CreateQrCard link={link} />
}

/** No QR Code yet (or it can't be loaded): "+ Create QR Code". */
function CreateQrCard({ link }: { link: Link }) {
  return (
    <section className={CARD}>
      <div className="mb-4 flex items-center justify-between gap-2">
        <h2 className="text-xl font-semibold text-foreground">QR Code</h2>
        <NextLink
          href={`/dashboard/qr-codes/create?link=${link.id}`}
          className="inline-flex items-center gap-1 text-sm font-semibold text-primary hover:underline"
        >
          <Plus className="size-4" />
          Create QR Code
        </NextLink>
      </div>
      <div
        className="flex size-[120px] items-center justify-center rounded-lg border-2 border-dashed border-border text-muted-foreground"
        aria-hidden
      >
        <QrCode className="size-10" />
      </div>
    </section>
  )
}

/** The link's QR Code: preview, Download and Customize. */
function LinkQrCode({ link, qrId }: { link: Link; qrId: number }) {
  const { data: qr, isError } = useQrCode(qrId)
  const customizations = qr?.render_customizations
  const design = useMemo(() => normalizeQrDesign(customizations), [customizations])
  if (isError) return <CreateQrCard link={link} />
  return (
    <section className={CARD}>
      <h2 className="mb-4 text-xl font-semibold text-foreground">QR Code</h2>
      <div className="flex flex-wrap items-center gap-6">
        {qr ? (
          <NextLink href={`/dashboard/qr-codes/${qrId}`} aria-label="View QR Code">
            <QrPreview design={design} data={qr.qr_url} size={120} />
          </NextLink>
        ) : (
          <Skeleton className="size-[120px] rounded-lg" />
        )}
        {qr && (
          <div className="flex flex-wrap gap-2">
            <QrDownloadMenu qr={qr} label="Download" />
            <Button variant="outline" asChild>
              <NextLink href={`/dashboard/qr-codes/${qrId}/customize`}>
                <Palette className="size-4" />
                Customize
              </NextLink>
            </Button>
          </div>
        )}
      </div>
    </section>
  )
}

function PagesCard({ link }: { link: Link }) {
  const [open, setOpen] = useState(false)
  const { data } = usePages()
  const ids = new Set(link.page_ids ?? [])
  const pages = (data?.pages ?? []).filter((p) => ids.has(p.id))
  return (
    <section className={CARD}>
      <div className="mb-4 flex items-center justify-between gap-2">
        <h2 className="text-xl font-semibold text-foreground">Pages</h2>
        <button
          type="button"
          onClick={() => setOpen(true)}
          className="inline-flex items-center gap-1 text-sm font-semibold text-primary hover:underline"
        >
          <Plus className="size-4" />
          Add to a page
        </button>
      </div>
      {pages.length > 0 ? (
        <ul className="space-y-1">
          {pages.map((p) => (
            <li key={p.id}>
              <NextLink href={`/dashboard/pages/${p.id}`} className="text-sm font-semibold text-primary hover:underline">
                {pageLabel(p)}
              </NextLink>
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-sm text-muted-foreground">Not on any pages yet</p>
      )}
      <AddToPageDialog open={open} onOpenChange={setOpen} link={link} />
    </section>
  )
}

function CampaignsCard({ link }: { link: Link }) {
  const [open, setOpen] = useState(false)
  const { data } = useLinkCampaigns(link.id)
  const memberships = data ?? []
  return (
    <section className={CARD}>
      <div className="mb-4 flex items-center justify-between gap-2">
        <h2 className="text-xl font-semibold text-foreground">Campaigns</h2>
        <button
          type="button"
          onClick={() => setOpen(true)}
          className="inline-flex items-center gap-1 text-sm font-semibold text-primary hover:underline"
        >
          <Plus className="size-4" />
          Add to campaign
        </button>
      </div>
      {memberships.length > 0 ? (
        <ul className="space-y-1">
          {memberships.map((m) => (
            <li key={`${m.campaign_id}-${m.channel_id}`}>
              <NextLink
                href={`/dashboard/campaigns/${m.campaign_id}`}
                className="text-sm font-semibold text-primary hover:underline"
              >
                {m.campaign_name} · {m.channel_name}
              </NextLink>
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-sm text-muted-foreground">Not in a campaign</p>
      )}
      <AddToCampaignDialog open={open} onOpenChange={setOpen} link={link} />
    </section>
  )
}

function DetailsSkeleton() {
  return (
    <div className="space-y-4 pt-8">
      <Skeleton className="h-10 w-80" />
      <div className="grid gap-4 lg:grid-cols-3">
        <Skeleton className="h-72 rounded-xl lg:col-span-2" />
        <Skeleton className="h-72 rounded-xl" />
      </div>
    </div>
  )
}

export default function LinkDetailsPage() {
  const params = useParams<{ id: string }>()
  const id = Number(params.id)
  const { data: link, isLoading, isError } = useLink(Number.isFinite(id) ? id : null)

  if (isLoading) return <DetailsSkeleton />
  if (isError || !link) {
    return (
      <div className="pt-8">
        <EmptyState
          title="Link not found"
          description="It may have been deleted, or it belongs to another group."
          action={
            <Button asChild>
              <NextLink href="/dashboard/links">Back to links</NextLink>
            </Button>
          }
        />
      </div>
    )
  }

  return (
    <div className="pb-12">
      <LinkDetailsHeader link={link} />
      {link.archived && <HiddenBanner link={link} />}
      <div className="grid gap-4 lg:grid-cols-3">
        <DetailsCard link={link} />
        <div className="space-y-4">
          <QrCard link={link} />
          <PagesCard link={link} />
          <CampaignsCard link={link} />
        </div>
      </div>
      <div className="mt-4">
        <DynamicRoutingCard linkId={link.id} initialRules={link.routing_rules ?? []} />
      </div>
      <div className="mt-8">
        {/* useSearchParams needs a Suspense boundary. */}
        <Suspense>
          <LinkAnalytics linkId={link.id} destination={link.original_url} />
        </Suspense>
      </div>
    </div>
  )
}
