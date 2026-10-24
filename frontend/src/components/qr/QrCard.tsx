'use client'

import { useMemo } from 'react'
import NextLink from 'next/link'
import { BarChart2, Calendar, CornerDownRight, Palette, Pencil } from 'lucide-react'
import { CopyButton } from '@/components/common/CopyButton'
import { TagEditor } from '@/components/links/TagEditor'
import { QrPreview } from '@/components/qr/QrPreview'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { formatDate, formatNumber, formatShortUrl } from '@/lib/format'
import { useSetQrTags } from '@/lib/hooks'
import { normalizeQrDesign } from '@/lib/qr/design'
import { cn } from '@/lib/utils'
import type { QrCode } from '@/types'
import { QrActionsMenu } from './QrActionsMenu'
import { QrDownloadMenu } from './QrDownloadMenu'

/** "1 scan" / "3 scans". */
export function scansLabel(n: number): string {
  return `${formatNumber(n)} ${n === 1 ? 'scan' : 'scans'}`
}

function hostOf(url: string): string {
  try {
    return new URL(url).host
  } catch {
    return url
  }
}

export interface QrCardProps {
  qr: QrCode
  selected: boolean
  onSelect: (selected: boolean) => void
}

/** QR Codes list card: preview, title, destination, scans/date/short link/tags, and actions. */
export function QrCard({ qr, selected, onSelect }: QrCardProps) {
  const title = qr.title || hostOf(qr.original_url)
  const design = useMemo(() => normalizeQrDesign(qr.render_customizations), [qr.render_customizations])
  const setTags = useSetQrTags()

  return (
    <article
      className={cn(
        'flex gap-4 rounded-xl bg-card pt-4 pr-4 pb-6 pl-8 transition-shadow hover:shadow-sm',
        selected && 'ring-2 ring-primary/40',
      )}
    >
      <div className="flex shrink-0 items-start gap-4 pt-1.5">
        <Checkbox checked={selected} onCheckedChange={onSelect} className="mt-1" aria-label={`Select ${title}`} />
        <NextLink href={`/dashboard/qr-codes/${qr.id}`} tabIndex={-1} aria-hidden>
          <QrPreview design={design} data={qr.qr_url} size={120} className="rounded border border-border bg-white" />
        </NextLink>
      </div>

      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex items-start gap-2">
          <NextLink
            href={`/dashboard/qr-codes/${qr.id}`}
            className="min-w-0 truncate text-xl leading-7 font-semibold text-foreground hover:underline"
          >
            {title}
          </NextLink>
          {qr.archived && (
            <Badge variant="secondary" className="mt-1">
              Hidden
            </Badge>
          )}
        </div>
        <span className="text-sm font-medium text-muted-foreground">Website</span>
        <div className="flex min-w-0 items-center gap-1 text-sm text-foreground">
          <CornerDownRight className="size-4 shrink-0 text-muted-foreground" aria-hidden />
          <span className="truncate" title={qr.original_url}>
            {qr.original_url}
          </span>
        </div>
        <div className="mt-4 flex flex-wrap items-center gap-x-6 gap-y-2 text-sm text-muted-foreground">
          <span className="inline-flex items-center gap-1.5">
            <BarChart2 className="size-4" aria-hidden />
            {scansLabel(qr.total_scans)}
          </span>
          <span className="inline-flex items-center gap-1.5">
            <Calendar className="size-4" aria-hidden />
            {formatDate(qr.created_at)}
          </span>
          {qr.link_source === 'link' && (
            <span className="inline-flex min-w-0 items-center gap-1">
              <a
                href={qr.short_url}
                target="_blank"
                rel="noopener noreferrer"
                className="truncate font-medium text-primary hover:underline"
              >
                {formatShortUrl(qr.short_url)}
              </a>
              <CopyButton text={qr.short_url} />
            </span>
          )}
          <TagEditor tags={qr.tags} tagType="qr" onSave={(tags) => setTags.mutateAsync({ id: qr.id, tags })} />
        </div>
      </div>

      <div className="flex shrink-0 items-center gap-2 self-start">
        <Button asChild variant="secondary" size="icon-sm" className="size-8">
          <NextLink href={`/dashboard/qr-codes/${qr.id}/edit`} aria-label="Edit">
            <Pencil className="size-4" />
          </NextLink>
        </Button>
        <Button asChild variant="secondary" size="icon-sm" className="size-8">
          <NextLink href={`/dashboard/qr-codes/${qr.id}/customize`} aria-label="Customize">
            <Palette className="size-4" />
          </NextLink>
        </Button>
        <QrDownloadMenu qr={qr} />
        <Button asChild variant="secondary" size="icon-sm" className="size-8">
          <NextLink href={`/dashboard/qr-codes/${qr.id}`} aria-label="View details">
            <BarChart2 className="size-4" />
          </NextLink>
        </Button>
        <QrActionsMenu qr={qr} />
      </div>
    </article>
  )
}
